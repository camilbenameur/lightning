// Package server implements the HTTP API.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/camilbenameur/lightning/internal/language"
	"github.com/camilbenameur/lightning/internal/metrics"
	"github.com/camilbenameur/lightning/internal/ratelimit"
	"github.com/camilbenameur/lightning/internal/sandbox"
)

const (
	// maxBodyBytes caps request bodies regardless of the code and stdin limits.
	maxBodyBytes = 256 << 10
	// BodyReadTimeout bounds how long a client may take to send a request body.
	BodyReadTimeout = 30 * time.Second
	// bodyIdleTimeout bounds how long a request body may stall: a slow client
	// can take its time to send a large body as long as it keeps sending.
	bodyIdleTimeout = 10 * time.Second
)

// Pinger reports whether the sandbox backend is reachable.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Limits are applied to every run; /api/v1/info advertises them.
type Limits struct {
	Timeout        time.Duration // base wall-clock limit, extended per language
	MemoryMB       int           // default memory limit, raised per language
	CPUs           float64
	PidsLimit      int
	MaxCodeBytes   int
	MaxStdinBytes  int
	MaxOutputBytes int // stdout and stderr combined
}

// Options configure the API handler.
type Options struct {
	Version   string
	Commit    string
	Languages *language.Registry
	Executor  sandbox.Executor
	Backend   Pinger
	Metrics   *metrics.Metrics
	Logger    *slog.Logger
	Limits    Limits

	ImagePrefix string // runtime images are <ImagePrefix><language>:<ImageTag>
	ImageTag    string

	TrustProxy  bool     // derive client IPs from X-Forwarded-For
	CORSOrigins []string // allowed origins; empty disables CORS, "*" allows any
	RateLimit   int      // runs per minute per client; 0 disables rate limiting
	RateBurst   int
}

type server struct {
	Options
	limiter   *ratelimit.Limiter
	languages languagesResponse
	info      infoResponse

	bodyTimeout, bodyIdle time.Duration // BodyReadTimeout and bodyIdleTimeout, except in tests
}

// New returns the API handler.
func New(opts Options) http.Handler {
	return newServer(opts).routes()
}

func newServer(opts Options) *server {
	s := &server{Options: opts, bodyTimeout: BodyReadTimeout, bodyIdle: bodyIdleTimeout}
	if opts.RateLimit > 0 {
		s.limiter = ratelimit.New(opts.RateLimit, opts.RateBurst)
	}
	s.languages.Languages = []languageResponse{}
	for _, l := range opts.Languages.List() {
		s.languages.Languages = append(s.languages.Languages, languageResponse{
			ID: l.ID, Name: l.Name, Version: l.Version, Monaco: l.Monaco, FileName: l.FileName,
		})
	}
	s.info = infoResponse{
		Version: opts.Version,
		Commit:  opts.Commit,
		Limits: limitsResponse{
			TimeoutMs:      opts.Limits.Timeout.Milliseconds(),
			MemoryMB:       opts.Limits.MemoryMB,
			MaxCodeBytes:   opts.Limits.MaxCodeBytes,
			MaxStdinBytes:  opts.Limits.MaxStdinBytes,
			MaxOutputBytes: opts.Limits.MaxOutputBytes,
		},
	}
	return s
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	route := func(method, path string, h http.HandlerFunc) {
		mux.Handle(method+" "+path, h)
		mux.Handle(path, methodNotAllowed(method))
	}
	route(http.MethodGet, "/api/v1/languages", s.handleLanguages)
	route(http.MethodGet, "/api/v1/info", s.handleInfo)
	route(http.MethodPost, "/api/v1/run", s.limitRate(s.handleRun))
	route(http.MethodPost, "/api/v1/run/stream", s.limitRate(s.handleRunStream))
	route(http.MethodGet, "/healthz", handleHealth)
	route(http.MethodGet, "/readyz", s.handleReady)
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, errorf(http.StatusNotFound, "not_found", "no such route"))
	})

	var h http.Handler = mux
	if len(s.CORSOrigins) > 0 {
		h = cors(s.CORSOrigins, h)
	}
	h = s.recoverPanic(h)
	h = s.observe(h)
	return requestID(h)
}

// logger returns the logger annotated with the request ID.
func (s *server) logger(r *http.Request) *slog.Logger {
	return s.Logger.With("requestId", requestIDFrom(r.Context()))
}
