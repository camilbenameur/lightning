// Command lightning serves the Lightning code runner API.
//
// Usage:
//
//	lightning              start the server (configured with LIGHTNING_* variables)
//	lightning healthcheck  probe /healthz of a local server; exit status 0 when healthy
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/moby/moby/client"

	"github.com/camilbenameur/lightning/internal/config"
	"github.com/camilbenameur/lightning/internal/language"
	"github.com/camilbenameur/lightning/internal/metrics"
	"github.com/camilbenameur/lightning/internal/sandbox"
	"github.com/camilbenameur/lightning/internal/server"
)

// Set at build time with -ldflags "-X main.version=... -X main.commit=...".
var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "lightning:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		return fmt.Errorf("invalid configuration:\n%w", err)
	}
	logger := newLogger(cfg)
	slog.SetDefault(logger)

	langs, err := language.NewRegistry(cfg.Languages)
	if err != nil {
		return err
	}
	cli, err := client.New(client.FromEnv)
	if err != nil {
		return fmt.Errorf("docker client: %w", err)
	}
	defer cli.Close()
	docker := sandbox.NewDocker(cli, cfg.SandboxRuntime, logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	images := make([]string, 0, len(langs.List()))
	for _, l := range langs.List() {
		images = append(images, l.Image(cfg.ImagePrefix, cfg.ImageTag))
	}
	checkImages(ctx, logger, docker, images, cfg.PullImages)
	// A sandbox lives for at most its run timeout plus the time to kill and
	// remove it; older ones were left behind by a crash or a failed removal.
	go removeStaleSandboxes(ctx, logger, docker, cfg.RunTimeout+langs.MaxCompileTimeout()+time.Minute)

	m := metrics.New()
	pool := sandbox.NewPool(docker, cfg.MaxConcurrent, cfg.QueueTimeout, m)
	handler := server.New(server.Options{
		Version:   version,
		Commit:    commit,
		Languages: langs,
		Executor:  pool,
		Backend:   docker,
		Metrics:   m,
		Logger:    logger,
		Limits: server.Limits{
			Timeout:        cfg.RunTimeout,
			MemoryMB:       cfg.MemoryMB,
			CPUs:           cfg.CPUs,
			PidsLimit:      cfg.PidsLimit,
			MaxCodeBytes:   cfg.MaxCodeBytes,
			MaxStdinBytes:  cfg.MaxStdinBytes,
			MaxOutputBytes: cfg.MaxOutputBytes,
		},
		ImagePrefix: cfg.ImagePrefix,
		ImageTag:    cfg.ImageTag,
		TrustProxy:  cfg.TrustProxy,
		CORSOrigins: cfg.CORSOrigins,
		RateLimit:   cfg.RateLimit,
		RateBurst:   cfg.RateBurst,
	})

	// Request contexts derive from runCtx: canceling it aborts every run in
	// progress, which kills and removes its sandbox.
	runCtx, abortRuns := context.WithCancelCause(context.Background())
	defer abortRuns(nil)
	api := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		// No ReadTimeout: it would expire during long runs and cancel their
		// request context. Body reads get their own deadline in the handlers.
		// The write timeout fits a slow upload, then a queued run with the
		// longest compile allowance, and the killing and removal of its sandbox.
		WriteTimeout: server.BodyReadTimeout + cfg.QueueTimeout + cfg.RunTimeout + langs.MaxCompileTimeout() + time.Minute,
		IdleTimeout:  2 * time.Minute,
		BaseContext:  func(net.Listener) context.Context { return runCtx },
		ErrorLog:     slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}
	servers := []*http.Server{api}
	if cfg.MetricsAddr != "" {
		mux := http.NewServeMux()
		mux.Handle("GET /metrics", m.Handler())
		servers = append(servers, &http.Server{
			Addr:              cfg.MetricsAddr,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
			ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
		})
	}

	errc := make(chan error, len(servers))
	for _, srv := range servers {
		ln, err := new(net.ListenConfig).Listen(ctx, "tcp", srv.Addr)
		if err != nil {
			return err
		}
		logger.Info("listening", "addr", ln.Addr().String(), "metrics", srv != api)
		go func() {
			if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
				errc <- err
			}
		}()
	}
	logger.Info("lightning started", "version", version, "commit", commit, "languages", len(langs.List()))

	var serveErr error
	select {
	case serveErr = <-errc:
		logger.Error("server failed", "error", serveErr)
	case <-ctx.Done():
	}
	stop()
	logger.Info("shutting down, waiting for runs in progress", "timeout", cfg.ShutdownTimeout)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := api.Shutdown(shutdownCtx); err != nil {
		logger.Warn("runs still in progress at the shutdown deadline, aborting them")
		abortRuns(server.ErrShuttingDown)
		// Aborted runs remove their sandbox, then report the error to their client.
		abortCtx, cancelAbort := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelAbort()
		_ = api.Shutdown(abortCtx)
		pool.Wait()
	}
	for _, srv := range servers[1:] {
		_ = srv.Close()
	}
	logger.Info("shutdown complete")
	if serveErr != nil {
		return fmt.Errorf("server: %w", serveErr)
	}
	return nil
}

func newLogger(cfg config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	if cfg.LogFormat == "text" {
		return slog.New(slog.NewTextHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}

// checkImages reports missing runtime images, optionally pulling them in the
// background. Failures are logged, not fatal: /readyz reports the engine's state.
func checkImages(ctx context.Context, logger *slog.Logger, docker *sandbox.Docker, images []string, pull bool) {
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := docker.Ping(checkCtx); err != nil {
		logger.Error("docker engine is not reachable; runs will fail until it is", "error", err)
		return
	}
	missing, err := docker.MissingImages(checkCtx, images)
	if err != nil {
		logger.Error("check runtime images", "error", err)
		return
	}
	if len(missing) == 0 {
		return
	}
	if !pull {
		logger.Warn("runtime images missing; runs of these languages will fail", "images", missing)
		return
	}
	logger.Info("pulling missing runtime images in the background", "images", missing)
	go func() {
		for _, image := range missing {
			start := time.Now()
			if err := docker.Pull(ctx, image); err != nil {
				logger.Error("pull runtime image", "image", image, "error", err)
				continue
			}
			logger.Info("pulled runtime image", "image", image, "duration", time.Since(start).Round(time.Millisecond))
		}
	}()
}

// removeStaleSandboxes removes the sandboxes older than maxAge at startup and
// then every minute until ctx ends.
func removeStaleSandboxes(ctx context.Context, logger *slog.Logger, docker *sandbox.Docker, maxAge time.Duration) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		sweepCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		n, err := docker.RemoveStale(sweepCtx, maxAge)
		cancel()
		switch {
		case err != nil && ctx.Err() == nil:
			logger.Warn("remove stale sandboxes", "error", err)
		case n > 0:
			logger.Warn("removed stale sandboxes", "count", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// healthcheck probes the local server; it is the container health check of
// the distroless image, which has no shell or curl.
func healthcheck() int {
	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		return 1
	}
	_, port, _ := net.SplitHostPort(cfg.Addr)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+net.JoinHostPort("127.0.0.1", port)+"/healthz", http.NoBody)
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		return 1
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "healthcheck: status", resp.Status)
		return 1
	}
	return 0
}
