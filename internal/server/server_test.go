package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/camilbenameur/lightning/internal/language"
	"github.com/camilbenameur/lightning/internal/metrics"
	"github.com/camilbenameur/lightning/internal/sandbox"
)

type runFunc func(ctx context.Context, spec sandbox.Spec, sink sandbox.Sink) (sandbox.Result, error)

// fakeExecutor records the specs it is asked to run.
type fakeExecutor struct {
	mu    sync.Mutex
	specs []sandbox.Spec
	run   runFunc
}

func (f *fakeExecutor) Run(ctx context.Context, spec sandbox.Spec, sink sandbox.Sink) (sandbox.Result, error) {
	f.mu.Lock()
	f.specs = append(f.specs, spec)
	f.mu.Unlock()
	return f.run(ctx, spec, sink)
}

func (f *fakeExecutor) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.specs)
}

type fakeBackend struct{ err error }

func (f fakeBackend) Ping(context.Context) error { return f.err }

// helloRun emulates a successful run printing to both streams.
func helloRun(_ context.Context, _ sandbox.Spec, sink sandbox.Sink) (sandbox.Result, error) {
	sink.Started()
	sink.Output(sandbox.Stdout, []byte("hello\n"))
	sink.Output(sandbox.Stderr, []byte("warning\n"))
	return sandbox.Result{ExitCode: 0, Duration: 123 * time.Millisecond}, nil
}

type testServer struct {
	http.Handler
	srv     *server
	exec    *fakeExecutor
	metrics *metrics.Metrics
}

func newTestServer(t *testing.T, run runFunc, configure ...func(*Options)) *testServer {
	t.Helper()
	langs, err := language.NewRegistry([]string{"python", "go", "bash"})
	if err != nil {
		t.Fatal(err)
	}
	exec := &fakeExecutor{run: run}
	opts := Options{
		Version:   "1.2.3",
		Commit:    "abc123",
		Languages: langs,
		Executor:  exec,
		Backend:   fakeBackend{},
		Metrics:   metrics.New(),
		Logger:    slog.New(slog.DiscardHandler),
		Limits: Limits{
			Timeout:        10 * time.Second,
			MemoryMB:       256,
			CPUs:           1.5,
			PidsLimit:      64,
			MaxCodeBytes:   100,
			MaxStdinBytes:  10,
			MaxOutputBytes: 65536,
		},
		ImagePrefix: "registry.example/lightning-runtime-",
		ImageTag:    "v1",
	}
	for _, c := range configure {
		c(&opts)
	}
	srv := newServer(opts)
	return &testServer{Handler: srv.routes(), srv: srv, exec: exec, metrics: opts.Metrics}
}

func (s *testServer) do(t *testing.T, method, target, body string, header ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, body io.Reader) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(body).Decode(&v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return v
}

func assertError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, status, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content type = %q", ct)
	}
	env := decode[map[string]map[string]string](t, rec.Body)
	if env["error"]["code"] != code || env["error"]["message"] == "" {
		t.Fatalf("error envelope = %v, want code %q", env, code)
	}
}

func TestLanguages(t *testing.T) {
	s := newTestServer(t, helloRun)
	rec := s.do(t, http.MethodGet, "/api/v1/languages", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	got := decode[languagesResponse](t, rec.Body)
	want := languagesResponse{Languages: []languageResponse{
		{ID: "python", Name: "Python", Version: "3.14", Monaco: "python", FileName: "main.py"},
		{ID: "go", Name: "Go", Version: "1.27", Monaco: "go", FileName: "main.go"},
		{ID: "bash", Name: "Bash", Version: "5.3", Monaco: "shell", FileName: "main.sh"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("languages = %+v", got)
	}
}

func TestInfo(t *testing.T) {
	s := newTestServer(t, helloRun)
	rec := s.do(t, http.MethodGet, "/api/v1/info", "")
	want := `{"version":"1.2.3","commit":"abc123","limits":{"timeoutMs":10000,"memoryMb":256,"maxCodeBytes":100,"maxStdinBytes":10,"maxOutputBytes":65536,"network":false}}`
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != want {
		t.Fatalf("info = %d %s", rec.Code, rec.Body)
	}
}

func TestHealth(t *testing.T) {
	s := newTestServer(t, helloRun)
	for _, path := range []string{"/healthz", "/readyz"} {
		rec := s.do(t, http.MethodGet, path, "")
		if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"status":"ok"}` {
			t.Errorf("%s = %d %s", path, rec.Code, rec.Body)
		}
	}

	down := newTestServer(t, helloRun, func(o *Options) { o.Backend = fakeBackend{err: errors.New("connection refused")} })
	assertError(t, down.do(t, http.MethodGet, "/readyz", ""), http.StatusServiceUnavailable, "sandbox_unavailable")
	if rec := down.do(t, http.MethodGet, "/healthz", ""); rec.Code != http.StatusOK {
		t.Errorf("liveness must not depend on the engine: %d", rec.Code)
	}
}

func TestRun(t *testing.T) {
	s := newTestServer(t, helloRun)
	rec := s.do(t, http.MethodPost, "/api/v1/run", `{"language":"python","code":"print('hello')","stdin":"x"}`)
	want := `{"stdout":"hello\n","stderr":"warning\n","exitCode":0,"durationMs":123,"timedOut":false,"oomKilled":false,"truncated":false}`
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != want {
		t.Fatalf("run = %d %s", rec.Code, rec.Body)
	}
	wantSpec := sandbox.Spec{
		Image:       "registry.example/lightning-runtime-python:v1",
		Source:      "print('hello')",
		Stdin:       "x",
		MemoryBytes: 256 << 20,
		NanoCPUs:    1_500_000_000,
		PidsLimit:   64,
		Timeout:     10 * time.Second,
		MaxOutput:   65536,
	}
	if got := s.exec.specs[0]; got != wantSpec {
		t.Fatalf("spec:\n got %+v\nwant %+v", got, wantSpec)
	}
}

func TestRunAppliesLanguageLimits(t *testing.T) {
	s := newTestServer(t, helloRun)
	tests := []struct {
		lang    string
		image   string
		memory  int64
		timeout time.Duration
	}{
		{"python", "registry.example/lightning-runtime-python:v1", 256 << 20, 10 * time.Second},
		{"go", "registry.example/lightning-runtime-go:v1", 512 << 20, 15 * time.Second},
		{"bash", "registry.example/lightning-runtime-bash:v1", 256 << 20, 10 * time.Second},
	}
	for i, tt := range tests {
		if rec := s.do(t, http.MethodPost, "/api/v1/run", `{"language":"`+tt.lang+`","code":"x"}`); rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d", tt.lang, rec.Code)
		}
		spec := s.exec.specs[i]
		if spec.Image != tt.image || spec.MemoryBytes != tt.memory || spec.Timeout != tt.timeout {
			t.Errorf("%s: image %s, memory %d, timeout %v", tt.lang, spec.Image, spec.MemoryBytes, spec.Timeout)
		}
	}
}

func TestOutcome(t *testing.T) {
	tests := []struct {
		res  sandbox.Result
		err  error
		want string
	}{
		{sandbox.Result{}, nil, "success"},
		{sandbox.Result{ExitCode: 1}, nil, "nonzero_exit"},
		{sandbox.Result{ExitCode: 137, TimedOut: true}, nil, "timeout"},
		{sandbox.Result{ExitCode: 137, OOMKilled: true}, nil, "oom"},
		{sandbox.Result{ExitCode: 137, Truncated: true}, nil, "output_limit"},
		{sandbox.Result{}, sandbox.ErrBusy, "busy"},
		{sandbox.Result{}, context.Canceled, "canceled"},
		{sandbox.Result{}, fmt.Errorf("wrapped: %w", sandbox.ErrImageNotFound), "unavailable"},
		{sandbox.Result{}, errors.New("boom"), "error"},
	}
	for _, tt := range tests {
		if got := outcome(tt.res, tt.err); got != tt.want {
			t.Errorf("outcome(%+v, %v) = %q, want %q", tt.res, tt.err, got, tt.want)
		}
	}
}

func TestRunReportsProgramFailuresAs200(t *testing.T) {
	s := newTestServer(t, func(_ context.Context, _ sandbox.Spec, sink sandbox.Sink) (sandbox.Result, error) {
		sink.Started()
		return sandbox.Result{ExitCode: 137, TimedOut: true, OOMKilled: true, Truncated: true, Duration: 10 * time.Second}, nil
	})
	rec := s.do(t, http.MethodPost, "/api/v1/run", `{"language":"go","code":"package main"}`)
	want := `{"stdout":"","stderr":"","exitCode":137,"durationMs":10000,"timedOut":true,"oomKilled":true,"truncated":true}`
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != want {
		t.Fatalf("run = %d %s", rec.Code, rec.Body)
	}
}

func TestRunValidation(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		status int
		code   string
	}{
		{"empty body", ``, 400, "invalid_json"},
		{"malformed", `{"language":`, 400, "invalid_json"},
		{"syntax error", `{"language" "python"}`, 400, "invalid_json"},
		{"not an object", `["python"]`, 400, "invalid_json"},
		{"wrong type", `{"language":"python","code":42}`, 400, "invalid_json"},
		{"unknown field", `{"language":"python","code":"x","timeout":99}`, 400, "invalid_json"},
		{"trailing data", `{"language":"python","code":"x"} {}`, 400, "invalid_json"},
		{"missing language", `{"code":"print(1)"}`, 400, "invalid_request"},
		{"missing code", `{"language":"python"}`, 400, "invalid_request"},
		{"empty code", `{"language":"python","code":""}`, 400, "invalid_request"},
		{"unknown language", `{"language":"cobol","code":"x"}`, 400, "unsupported_language"},
		{"disabled language", `{"language":"java","code":"x"}`, 400, "unsupported_language"},
		{"code too large", `{"language":"python","code":"` + strings.Repeat("x", 101) + `"}`, 413, "payload_too_large"},
		{"stdin too large", `{"language":"python","code":"x","stdin":"` + strings.Repeat("x", 11) + `"}`, 413, "payload_too_large"},
		{"body too large", `{"language":"python","code":"` + strings.Repeat("x", maxBodyBytes) + `"}`, 413, "payload_too_large"},
	}
	for _, path := range []string{"/api/v1/run", "/api/v1/run/stream"} {
		for _, tt := range tests {
			t.Run(path+"/"+tt.name, func(t *testing.T) {
				s := newTestServer(t, helloRun)
				assertError(t, s.do(t, http.MethodPost, path, tt.body), tt.status, tt.code)
				if s.exec.calls() != 0 {
					t.Fatal("invalid request reached the executor")
				}
			})
		}
	}
}

func TestRunAcceptsLimitSizedInput(t *testing.T) {
	s := newTestServer(t, helloRun)
	body := `{"language":"python","code":"` + strings.Repeat("x", 100) + `","stdin":"` + strings.Repeat("y", 10) + `"}`
	if rec := s.do(t, http.MethodPost, "/api/v1/run", body); rec.Code != http.StatusOK {
		t.Fatalf("status = %d %s", rec.Code, rec.Body)
	}
}

// TestRunBodyTimeouts checks, over real connections, that a slow upload is
// accepted as long as it keeps coming, while a stalled or endless one gets an
// error response.
func TestRunBodyTimeouts(t *testing.T) {
	const idle, total, pause = 300 * time.Millisecond, 1500 * time.Millisecond, 50 * time.Millisecond
	body := `{"language":"python","code":"print('hello')"}`
	var pieces []string
	for p := range slices.Chunk([]byte(body), 5) {
		pieces = append(pieces, string(p))
	}
	tests := []struct {
		name   string
		path   string
		length int      // declared Content-Length
		pieces []string // body pieces, sent pause apart
		more   string   // then sent pause apart until the response arrives
		status int
	}{
		{"steady", "/api/v1/run", len(body), pieces, "", http.StatusOK},
		{"stalled", "/api/v1/run", len(body), pieces[:3], "", http.StatusRequestTimeout},
		{"stalled stream", "/api/v1/run/stream", len(body), pieces[:3], "", http.StatusRequestTimeout},
		{"endless", "/api/v1/run", 1 << 16, pieces[:6], "x", http.StatusRequestTimeout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestServer(t, helloRun)
			s.srv.bodyIdle, s.srv.bodyTimeout = idle, total
			srv := httptest.NewServer(s)
			defer srv.Close()

			start := time.Now()
			resp := slowPost(t, srv.Listener.Addr().String(), tt.path, tt.length, pause, tt.pieces, tt.more)
			elapsed := time.Since(start)
			if resp.StatusCode != tt.status {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.status)
			}
			if tt.status == http.StatusOK {
				if got := decode[runResponse](t, resp.Body); got.Stdout != "hello\n" || s.exec.calls() != 1 {
					t.Fatalf("response %+v after %d runs", got, s.exec.calls())
				}
				return
			}
			env := decode[map[string]map[string]string](t, resp.Body)
			if env["error"]["code"] != "invalid_request" || resp.Header.Get("Content-Type") != "application/json" {
				t.Fatalf("response %v, content type %q", env, resp.Header.Get("Content-Type"))
			}
			if !resp.Close {
				t.Error("the connection must be closed: the rest of the body was not read")
			}
			if stalled := tt.more == ""; stalled != (elapsed < total) {
				t.Errorf("answered after %v: stalled bodies must be cut after %v, endless ones after %v", elapsed, idle, total)
			}
			if s.exec.calls() != 0 {
				t.Fatal("an incomplete request reached the executor")
			}
			srv.Close() // wait for the request to be logged and counted
			rec := httptest.NewRecorder()
			s.metrics.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
			if want := `lightning_http_requests_total{route="POST ` + tt.path + `",status="408"} 1`; !strings.Contains(rec.Body.String(), want) {
				t.Errorf("metrics missing %q", want)
			}
		})
	}
}

// TestRunAbandonedRequests checks that a client that half-closes its
// connection gets no response at all rather than an empty 200.
func TestRunAbandonedRequests(t *testing.T) {
	body := `{"language":"python","code":"x"}`
	for _, tt := range []struct{ name, sent string }{
		{"during the upload", body[:10]},
		{"during the run", body},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestServer(t, func(ctx context.Context, _ sandbox.Spec, _ sandbox.Sink) (sandbox.Result, error) {
				<-ctx.Done()
				return sandbox.Result{}, ctx.Err()
			})
			srv := httptest.NewServer(s)
			defer srv.Close()
			conn, err := net.Dial("tcp", srv.Listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if _, err := fmt.Fprintf(conn, "POST /api/v1/run HTTP/1.1\r\nHost: test\r\nContent-Length: %d\r\n\r\n%s", len(body), tt.sent); err != nil {
				t.Fatal(err)
			}
			if err := conn.(*net.TCPConn).CloseWrite(); err != nil {
				t.Fatal(err)
			}
			if resp, err := http.ReadResponse(bufio.NewReader(conn), nil); err == nil {
				t.Fatalf("got a %s response", resp.Status)
			}
			srv.Close() // wait for the request to be logged and counted
			rec := httptest.NewRecorder()
			s.metrics.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
			if want := `lightning_http_requests_total{route="POST /api/v1/run",status="499"} 1`; !strings.Contains(rec.Body.String(), want) {
				t.Errorf("metrics missing %q", want)
			}
		})
	}
}

// slowPost sends a POST request over a raw connection, writing the body in
// pieces separated by pause, then writing more (if any) after every pause
// until the response arrives.
func slowPost(t *testing.T, addr, path string, length int, pause time.Duration, pieces []string, more string) *http.Response {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if _, err := fmt.Fprintf(conn, "POST %s HTTP/1.1\r\nHost: test\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n", path, length); err != nil {
		t.Fatal(err)
	}
	answered := make(chan struct{})
	defer close(answered)
	go func() {
		for i := 0; i < len(pieces) || more != ""; i++ {
			select {
			case <-answered:
				return
			case <-time.After(pause):
			}
			piece := more
			if i < len(pieces) {
				piece = pieces[i]
			}
			if _, err := io.WriteString(conn, piece); err != nil {
				return
			}
		}
	}()
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func TestRunErrors(t *testing.T) {
	tests := []struct {
		err    error
		status int
		code   string
	}{
		{sandbox.ErrBusy, 503, "server_busy"},
		{sandbox.ErrImageNotFound, 503, "sandbox_unavailable"},
		{sandbox.ErrUnavailable, 503, "sandbox_unavailable"},
		{errors.New("unexpected"), 500, "internal_error"},
	}
	for _, path := range []string{"/api/v1/run", "/api/v1/run/stream"} {
		for _, tt := range tests {
			s := newTestServer(t, func(context.Context, sandbox.Spec, sandbox.Sink) (sandbox.Result, error) {
				return sandbox.Result{}, tt.err
			})
			rec := s.do(t, http.MethodPost, path, `{"language":"python","code":"x"}`)
			assertError(t, rec, tt.status, tt.code)
		}
	}
}

func TestRunPanicIsRecovered(t *testing.T) {
	s := newTestServer(t, func(context.Context, sandbox.Spec, sandbox.Sink) (sandbox.Result, error) {
		panic("boom")
	})
	assertError(t, s.do(t, http.MethodPost, "/api/v1/run", `{"language":"python","code":"x"}`), 500, "internal_error")
}

func streamLines(t *testing.T, body io.Reader) []map[string]any {
	t.Helper()
	var lines []map[string]any
	sc := bufio.NewScanner(body)
	for sc.Scan() {
		var v map[string]any
		if err := json.Unmarshal(sc.Bytes(), &v); err != nil {
			t.Fatalf("line %q is not JSON: %v", sc.Text(), err)
		}
		lines = append(lines, v)
	}
	return lines
}

func TestRunStream(t *testing.T) {
	s := newTestServer(t, func(_ context.Context, _ sandbox.Spec, sink sandbox.Sink) (sandbox.Result, error) {
		sink.Started()
		sink.Output(sandbox.Stdout, []byte("a"))
		sink.Output(sandbox.Stderr, []byte("b"))
		sink.Output(sandbox.Stdout, []byte("<€>\n"))
		return sandbox.Result{ExitCode: 3, Duration: 42 * time.Millisecond}, nil
	})
	rec := s.do(t, http.MethodPost, "/api/v1/run/stream", `{"language":"bash","code":"echo"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	for header, want := range map[string]string{
		"Content-Type":      "application/x-ndjson",
		"Cache-Control":     "no-cache",
		"X-Accel-Buffering": "no",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	want := `{"type":"started"}
{"type":"stdout","data":"a"}
{"type":"stderr","data":"b"}
{"type":"stdout","data":"<€>\n"}
{"type":"result","exitCode":3,"durationMs":42,"timedOut":false,"oomKilled":false,"truncated":false}
`
	if rec.Body.String() != want {
		t.Fatalf("stream:\n%s\nwant:\n%s", rec.Body, want)
	}
}

func TestRunStreamCoalescesChunks(t *testing.T) {
	s := newTestServer(t, func(_ context.Context, _ sandbox.Spec, sink sandbox.Sink) (sandbox.Result, error) {
		sink.Started()
		for range 1000 {
			sink.Output(sandbox.Stdout, []byte("line\n"))
		}
		return sandbox.Result{}, nil
	})
	rec := s.do(t, http.MethodPost, "/api/v1/run/stream", `{"language":"bash","code":"yes"}`)
	lines := streamLines(t, rec.Body)
	var out strings.Builder
	events := 0
	for _, l := range lines {
		if l["type"] == "stdout" {
			events++
			out.WriteString(l["data"].(string))
		}
	}
	if out.String() != strings.Repeat("line\n", 1000) {
		t.Fatal("coalesced output differs from the program output")
	}
	if events > 10 {
		t.Fatalf("%d stdout events for 1000 tiny chunks, expected coalescing", events)
	}
	if last := lines[len(lines)-1]; last["type"] != "result" {
		t.Fatalf("last event = %v", last)
	}
}

func TestRunStreamErrorAfterStart(t *testing.T) {
	s := newTestServer(t, func(_ context.Context, _ sandbox.Spec, sink sandbox.Sink) (sandbox.Result, error) {
		sink.Started()
		sink.Output(sandbox.Stdout, []byte("partial"))
		return sandbox.Result{}, sandbox.ErrUnavailable
	})
	rec := s.do(t, http.MethodPost, "/api/v1/run/stream", `{"language":"python","code":"x"}`)
	lines := streamLines(t, rec.Body)
	if rec.Code != http.StatusOK || len(lines) != 3 {
		t.Fatalf("status %d, lines %v", rec.Code, lines)
	}
	if lines[1]["data"] != "partial" {
		t.Fatalf("pending output was not flushed before the error: %v", lines)
	}
	last := lines[2]
	if last["type"] != "error" || last["error"].(map[string]any)["code"] != "sandbox_unavailable" {
		t.Fatalf("last event = %v", last)
	}
}

// TestRunStreamOverHTTP checks, over a real connection, that events are
// flushed as they happen and that a client disconnect cancels the run.
func TestRunStreamOverHTTP(t *testing.T) {
	proceed := make(chan struct{})
	canceled := make(chan struct{})
	s := newTestServer(t, func(ctx context.Context, _ sandbox.Spec, sink sandbox.Sink) (sandbox.Result, error) {
		sink.Started()
		<-proceed
		sink.Output(sandbox.Stdout, []byte("tick\n"))
		<-ctx.Done()
		close(canceled)
		return sandbox.Result{}, ctx.Err()
	})
	srv := httptest.NewServer(s)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/v1/run/stream", "application/json", strings.NewReader(`{"language":"python","code":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	r := bufio.NewReader(resp.Body)
	readLine := func() string {
		t.Helper()
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("read event: %v", err)
		}
		return line
	}
	if got := readLine(); got != "{\"type\":\"started\"}\n" {
		t.Fatalf("first event = %q", got)
	}
	close(proceed)
	if got := readLine(); got != "{\"type\":\"stdout\",\"data\":\"tick\\n\"}\n" {
		t.Fatalf("second event = %q", got)
	}
	_ = resp.Body.Close()

	select {
	case <-canceled:
	case <-time.After(5 * time.Second):
		t.Fatal("run was not canceled after the client disconnected")
	}
}

// TestRunAbortedByShutdown checks that runs aborted at shutdown report an
// error to their client, which is still connected.
func TestRunAbortedByShutdown(t *testing.T) {
	s := newTestServer(t, func(ctx context.Context, _ sandbox.Spec, sink sandbox.Sink) (sandbox.Result, error) {
		sink.Started()
		sink.Output(sandbox.Stdout, []byte("partial"))
		<-ctx.Done()
		return sandbox.Result{}, ctx.Err()
	})
	run := func(path string) *httptest.ResponseRecorder {
		ctx, abort := context.WithCancelCause(t.Context())
		defer abort(nil)
		req := httptest.NewRequestWithContext(ctx, http.MethodPost, path, strings.NewReader(`{"language":"python","code":"x"}`))
		rec := httptest.NewRecorder()
		time.AfterFunc(10*time.Millisecond, func() { abort(ErrShuttingDown) })
		s.ServeHTTP(rec, req)
		return rec
	}

	assertError(t, run("/api/v1/run"), http.StatusServiceUnavailable, "sandbox_unavailable")

	rec := run("/api/v1/run/stream")
	lines := streamLines(t, rec.Body)
	if rec.Code != http.StatusOK || len(lines) != 3 || lines[1]["data"] != "partial" {
		t.Fatalf("status %d, events %v", rec.Code, lines)
	}
	if last := lines[2]; last["type"] != "error" || last["error"].(map[string]any)["code"] != "sandbox_unavailable" {
		t.Fatalf("last event = %v", last)
	}
}

func TestNotFoundAndMethodNotAllowed(t *testing.T) {
	s := newTestServer(t, helloRun)
	assertError(t, s.do(t, http.MethodGet, "/nope", ""), 404, "not_found")
	assertError(t, s.do(t, http.MethodGet, "/api/v1/languages/", ""), 404, "not_found")

	rec := s.do(t, http.MethodGet, "/api/v1/run", "")
	assertError(t, rec, 405, "method_not_allowed")
	if rec.Header().Get("Allow") != "POST" {
		t.Errorf("Allow = %q", rec.Header().Get("Allow"))
	}
	rec = s.do(t, http.MethodDelete, "/api/v1/languages", "")
	assertError(t, rec, 405, "method_not_allowed")
	if rec.Header().Get("Allow") != "GET, HEAD" {
		t.Errorf("Allow = %q", rec.Header().Get("Allow"))
	}
}

func TestRequestID(t *testing.T) {
	s := newTestServer(t, helloRun)
	if got := s.do(t, http.MethodGet, "/healthz", "", "X-Request-ID", "abc-123").Header().Get("X-Request-ID"); got != "abc-123" {
		t.Errorf("incoming id not echoed: %q", got)
	}
	generated := s.do(t, http.MethodGet, "/nope", "").Header().Get("X-Request-ID")
	if len(generated) < 16 {
		t.Errorf("generated id = %q", generated)
	}
	if got := s.do(t, http.MethodGet, "/healthz", "", "X-Request-ID", "bad id\n<script>").Header().Get("X-Request-ID"); got == "bad id\n<script>" || got == "" {
		t.Errorf("invalid incoming id handled badly: %q", got)
	}
}

func TestRateLimit(t *testing.T) {
	s := newTestServer(t, helloRun, func(o *Options) {
		o.RateLimit = 6 // one every 10s
		o.RateBurst = 2
		o.TrustProxy = true
	})
	body := `{"language":"python","code":"x"}`
	for i := range 2 {
		if rec := s.do(t, http.MethodPost, "/api/v1/run", body, "X-Forwarded-For", "1.1.1.1, 10.0.0.1"); rec.Code != 200 {
			t.Fatalf("request %d: status %d", i, rec.Code)
		}
	}
	rec := s.do(t, http.MethodPost, "/api/v1/run/stream", body, "X-Forwarded-For", "9.9.9.9, 10.0.0.1")
	assertError(t, rec, 429, "rate_limited")
	if ra := rec.Header().Get("Retry-After"); ra == "" || ra == "0" {
		t.Fatalf("Retry-After = %q", ra)
	}
	if rec := s.do(t, http.MethodPost, "/api/v1/run", body, "X-Forwarded-For", "10.0.0.2"); rec.Code != 200 {
		t.Fatalf("other client was limited: %d", rec.Code)
	}
	if rec := s.do(t, http.MethodGet, "/api/v1/languages", "", "X-Forwarded-For", "10.0.0.1"); rec.Code != 200 {
		t.Fatalf("non-run endpoints must not be rate limited: %d", rec.Code)
	}
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		remote, xff string
		trust       bool
		want        string
	}{
		{"192.0.2.1:1234", "", false, "192.0.2.1"},
		{"192.0.2.1:1234", "203.0.113.9", false, "192.0.2.1"},
		{"192.0.2.1:1234", "203.0.113.9", true, "203.0.113.9"},
		{"192.0.2.1:1234", "spoofed, 203.0.113.9", true, "203.0.113.9"},
		{"192.0.2.1:1234", "203.0.113.9, garbage", true, "192.0.2.1"},
		{"[2001:db8::1]:443", "", false, "2001:db8::1"},
		{"[::ffff:192.0.2.7]:80", "", false, "192.0.2.7"},
		{"192.0.2.1:1234", " 2001:db8::5 ", true, "2001:db8::5"},
	}
	for _, tt := range tests {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = tt.remote
		if tt.xff != "" {
			r.Header.Set("X-Forwarded-For", tt.xff)
		}
		if got := clientIP(r, tt.trust).String(); got != tt.want {
			t.Errorf("clientIP(%q, %q, %v) = %s, want %s", tt.remote, tt.xff, tt.trust, got, tt.want)
		}
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "192.0.2.1:1"
	r.Header.Add("X-Forwarded-For", "198.51.100.1")
	r.Header.Add("X-Forwarded-For", "198.51.100.2")
	if got := clientIP(r, true).String(); got != "198.51.100.2" {
		t.Errorf("multiple headers: got %s", got)
	}
}

func TestRateKey(t *testing.T) {
	r1 := httptest.NewRequest(http.MethodGet, "/", nil)
	r1.RemoteAddr = "[2001:db8:1:2:aaaa::1]:1"
	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	r2.RemoteAddr = "[2001:db8:1:2:bbbb::2]:1"
	if a, b := rateKey(clientIP(r1, false)), rateKey(clientIP(r2, false)); a != b || a != "2001:db8:1:2::/64" {
		t.Errorf("IPv6 keys = %q, %q; want the same /64", a, b)
	}
	r1.RemoteAddr = "192.0.2.1:1"
	if got := rateKey(clientIP(r1, false)); got != "192.0.2.1" {
		t.Errorf("IPv4 key = %q", got)
	}
	r1.RemoteAddr = "pipe"
	if got := rateKey(clientIP(r1, false)); got != "unknown" {
		t.Errorf("invalid address key = %q", got)
	}
}

func TestCORS(t *testing.T) {
	disabled := newTestServer(t, helloRun)
	if rec := disabled.do(t, http.MethodGet, "/api/v1/info", "", "Origin", "https://evil.example"); rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("CORS must be disabled by default")
	}

	s := newTestServer(t, helloRun, func(o *Options) { o.CORSOrigins = []string{"https://app.example"} })
	rec := s.do(t, http.MethodGet, "/api/v1/info", "", "Origin", "https://app.example")
	if rec.Header().Get("Access-Control-Allow-Origin") != "https://app.example" || rec.Header().Get("Vary") != "Origin" {
		t.Fatalf("allowed origin headers = %v", rec.Header())
	}
	rec = s.do(t, http.MethodGet, "/api/v1/info", "", "Origin", "https://evil.example")
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("disallowed origin received CORS headers")
	}
	rec = s.do(t, http.MethodOptions, "/api/v1/run", "", "Origin", "https://app.example", "Access-Control-Request-Method", "POST")
	if rec.Code != http.StatusNoContent || !strings.Contains(rec.Header().Get("Access-Control-Allow-Methods"), "POST") {
		t.Fatalf("preflight = %d %v", rec.Code, rec.Header())
	}

	wildcard := newTestServer(t, helloRun, func(o *Options) { o.CORSOrigins = []string{"*"} })
	if got := wildcard.do(t, http.MethodGet, "/api/v1/info", "", "Origin", "https://any.example").Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("wildcard origin = %q", got)
	}
}

func TestHTTPMetrics(t *testing.T) {
	s := newTestServer(t, helloRun)
	s.do(t, http.MethodPost, "/api/v1/run", `{"language":"python","code":"x"}`)
	s.do(t, http.MethodGet, "/does/not/exist", "")

	rec := httptest.NewRecorder()
	s.metrics.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	for _, want := range []string{
		`lightning_http_requests_total{route="POST /api/v1/run",status="200"} 1`,
		`lightning_http_requests_total{route="unmatched",status="404"} 1`,
		`lightning_runs_total{language="python",outcome="success"} 1`,
	} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("metrics missing %q", want)
		}
	}
}
