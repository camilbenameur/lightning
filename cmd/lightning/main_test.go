package main

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestHealthcheck(t *testing.T) {
	var status atomic.Int32
	status.Store(http.StatusOK)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(int(status.Load()))
	}))
	defer srv.Close()
	// The probe always targets 127.0.0.1 on the port of LIGHTNING_ADDR.
	t.Setenv("LIGHTNING_ADDR", srv.Listener.Addr().String())

	if code := healthcheck(); code != 0 {
		t.Fatalf("healthy server: exit status %d, want 0", code)
	}
	status.Store(http.StatusServiceUnavailable)
	if code := healthcheck(); code != 1 {
		t.Fatalf("unhealthy server: exit status %d, want 1", code)
	}
	srv.Close()
	if code := healthcheck(); code != 1 {
		t.Fatalf("no server: exit status %d, want 1", code)
	}
}

func TestHealthcheckInvalidConfig(t *testing.T) {
	t.Setenv("LIGHTNING_ADDR", "not an address")
	if code := healthcheck(); code != 1 {
		t.Fatalf("exit status %d, want 1", code)
	}
}
