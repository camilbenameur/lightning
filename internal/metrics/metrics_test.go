package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func scrape(t *testing.T, m *Metrics) string {
	t.Helper()
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body, _ := io.ReadAll(rec.Body)
	return string(body)
}

func TestExposition(t *testing.T) {
	m := New()
	m.RunFinished("python", "success", 1500*time.Millisecond)
	m.RunFinished("python", "busy", 0)
	m.SandboxStarted()
	m.SandboxStarted()
	m.SandboxFinished()
	m.QueueWaited(30 * time.Millisecond)
	m.HTTPRequest("POST /api/v1/run", 200, 10*time.Millisecond)

	out := scrape(t, m)
	for _, want := range []string{
		`lightning_runs_total{language="python",outcome="success"} 1`,
		`lightning_runs_total{language="python",outcome="busy"} 1`,
		`lightning_run_duration_seconds_count{language="python"} 1`,
		`lightning_active_sandboxes 1`,
		`lightning_queue_wait_seconds_count 1`,
		`lightning_http_requests_total{route="POST /api/v1/run",status="200"} 1`,
		`lightning_http_request_duration_seconds_count{route="POST /api/v1/run"} 1`,
		`go_goroutines`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("metrics output is missing %q", want)
		}
	}
}
