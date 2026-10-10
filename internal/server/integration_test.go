//go:build integration

package server

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/client"

	"github.com/camilbenameur/lightning/internal/language"
	"github.com/camilbenameur/lightning/internal/metrics"
	"github.com/camilbenameur/lightning/internal/sandbox"
)

// newIntegrationServer serves the API backed by the real Docker executor.
func newIntegrationServer(t *testing.T) (*httptest.Server, *client.Client, *sandbox.Pool) {
	t.Helper()
	prefix, tag := os.Getenv("LIGHTNING_RUNTIME_IMAGE_PREFIX"), os.Getenv("LIGHTNING_RUNTIME_TAG")
	if prefix == "" {
		prefix = "lightning-runtime-"
	}
	if tag == "" {
		tag = "latest"
	}
	cli, err := client.New(client.FromEnv)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cli.Close() })
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	docker := sandbox.NewDocker(cli, os.Getenv("LIGHTNING_SANDBOX_RUNTIME"), logger)
	if missing, err := docker.MissingImages(t.Context(), []string{prefix + "python:" + tag}); err != nil {
		t.Fatal(err)
	} else if len(missing) > 0 {
		t.Skipf("runtime image %s is not available", missing[0])
	}

	langs, _ := language.NewRegistry([]string{"python"})
	m := metrics.New()
	pool := sandbox.NewPool(docker, 2, 10*time.Second, m)
	srv := httptest.NewServer(New(Options{
		Version: "test", Commit: "test", Languages: langs, Executor: pool, Backend: docker, Metrics: m, Logger: logger,
		Limits: Limits{
			Timeout: 30 * time.Second, MemoryMB: 256, CPUs: 1, PidsLimit: 128,
			MaxCodeBytes: 64 << 10, MaxStdinBytes: 64 << 10, MaxOutputBytes: 64 << 10,
		},
		ImagePrefix: prefix, ImageTag: tag,
	}))
	t.Cleanup(srv.Close)
	return srv, cli, pool
}

func post(t *testing.T, url, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestIntegrationRunOverHTTP(t *testing.T) {
	srv, _, _ := newIntegrationServer(t)

	resp := post(t, srv.URL+"/api/v1/run", `{"language":"python","code":"print(input()[::-1])","stdin":"lightning"}`)
	defer resp.Body.Close()
	var got runResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || got.Stdout != "gninthgil\n" || got.ExitCode != 0 || got.DurationMs <= 0 {
		t.Fatalf("status %d, response %+v", resp.StatusCode, got)
	}
}

func TestIntegrationStreamOverHTTP(t *testing.T) {
	srv, _, _ := newIntegrationServer(t)
	code := "import sys, time\nprint('a', flush=True)\ntime.sleep(0.2)\nprint('b', file=sys.stderr, flush=True)\nsys.exit(4)"
	resp := post(t, srv.URL+"/api/v1/run/stream", `{"language":"python","code":`+quote(code)+`}`)
	defer resp.Body.Close()
	if resp.Header.Get("Content-Type") != "application/x-ndjson" {
		t.Fatalf("content type %q", resp.Header.Get("Content-Type"))
	}
	var (
		types []string
		last  map[string]any
	)
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		last = nil // Unmarshal would merge into the previous event
		if err := json.Unmarshal(sc.Bytes(), &last); err != nil {
			t.Fatal(err)
		}
		types = append(types, last["type"].(string)+":"+strings.TrimSpace(asString(last["data"])))
	}
	if strings.Join(types, " ") != "started: stdout:a stderr:b result:" {
		t.Fatalf("events = %v", types)
	}
	if last["exitCode"] != float64(4) {
		t.Fatalf("result = %v", last)
	}
}

func TestIntegrationStreamDisconnectRemovesSandbox(t *testing.T) {
	srv, cli, pool := newIntegrationServer(t)
	before := sandboxes(t, cli)

	resp := post(t, srv.URL+"/api/v1/run/stream", `{"language":"python","code":"import time\nprint('ready', flush=True)\ntime.sleep(120)"}`)
	r := bufio.NewReader(resp.Body)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("stream ended early: %v", err)
		}
		if strings.Contains(line, `"ready\n"`) {
			break
		}
	}
	var created []string
	for id := range sandboxes(t, cli) {
		if !before[id] {
			created = append(created, id)
		}
	}
	if len(created) == 0 {
		t.Fatal("no sandbox container found while the program runs")
	}

	_ = resp.Body.Close()
	disconnected := time.Now()
	for {
		remaining := 0
		now := sandboxes(t, cli)
		for _, id := range created {
			if now[id] {
				remaining++
			}
		}
		if remaining == 0 {
			break
		}
		if time.Since(disconnected) > 30*time.Second {
			t.Fatalf("%d sandbox(es) still present 30s after the client disconnected", remaining)
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Logf("sandbox removed %v after disconnect", time.Since(disconnected))
	pool.Wait()
}

// sandboxes returns the IDs of the executor's containers.
func sandboxes(t *testing.T, cli *client.Client) map[string]bool {
	t.Helper()
	list, err := cli.ContainerList(t.Context(), client.ContainerListOptions{
		All:     true,
		Filters: make(client.Filters).Add("label", sandbox.Label+"=true").Add("name", "lightning-"),
	})
	if err != nil {
		t.Fatal(err)
	}
	ids := make(map[string]bool)
	for _, c := range list.Items {
		ids[c.ID] = true
	}
	return ids
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}
