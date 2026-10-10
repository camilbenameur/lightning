package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/camilbenameur/lightning/internal/sandbox"
)

// apiError is an error reported to clients as
// {"error":{"code":"...","message":"..."}}.
type apiError struct {
	status  int
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *apiError) Error() string { return e.Code + ": " + e.Message }

func errorf(status int, code, format string, args ...any) *apiError {
	return &apiError{status: status, Code: code, Message: fmt.Sprintf(format, args...)}
}

var (
	errInternal    = errorf(http.StatusInternalServerError, "internal_error", "internal server error")
	errBodyTimeout = errorf(http.StatusRequestTimeout, "invalid_request", "the request body was not received in time")
)

// ErrShuttingDown is the cause to cancel the server's base context with to
// abort the runs in progress at shutdown: their clients get an error rather
// than a cut connection.
var ErrShuttingDown = errors.New("server shutting down")

// toAPIError maps an error to what clients should see.
func toAPIError(err error) *apiError {
	var apiErr *apiError
	switch {
	case errors.As(err, &apiErr):
		return apiErr
	case errors.Is(err, ErrShuttingDown):
		return errorf(http.StatusServiceUnavailable, "sandbox_unavailable", "the server is restarting, please retry in a moment")
	case errors.Is(err, sandbox.ErrBusy):
		return errorf(http.StatusServiceUnavailable, "server_busy", "all sandboxes are busy, please retry shortly")
	case errors.Is(err, sandbox.ErrImageNotFound):
		return errorf(http.StatusServiceUnavailable, "sandbox_unavailable", "the runtime for this language is not available yet, please retry later")
	case errors.Is(err, sandbox.ErrUnavailable):
		return errorf(http.StatusServiceUnavailable, "sandbox_unavailable", "the code execution backend is unavailable, please retry later")
	default:
		return errInternal
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v) // the client is gone if this fails
}

func writeError(w http.ResponseWriter, e *apiError) {
	writeJSON(w, e.status, errorEnvelope{Error: e})
}

// fail writes err to the client. When the client has gone away, the response
// is aborted instead: returning would make net/http send an empty 200 to a
// client that only half-closed its connection. A body read that timed out ends
// the request context too, but that client is still waiting for an answer.
func (s *server) fail(w http.ResponseWriter, r *http.Request, err error) {
	if clientGone(r) && !errors.Is(err, errBodyTimeout) {
		s.logger(r).Debug("request abandoned by client", "error", err)
		panic(http.ErrAbortHandler)
	}
	e := toAPIError(err)
	s.logFailure(r, e, err)
	writeError(w, e)
}

// logFailure logs a run that failed on the server side.
func (s *server) logFailure(r *http.Request, e *apiError, err error) {
	level := slog.LevelError
	switch {
	case e.status == http.StatusServiceUnavailable: // busy, engine down or restarting
		level = slog.LevelWarn
	case e.status < http.StatusInternalServerError: // client errors are logged with the request
		return
	}
	s.logger(r).Log(r.Context(), level, "run failed", "code", e.Code, "error", err)
}

// clientGone reports whether the client of r went away. Runs aborted at
// shutdown see their request context end too, but their client is still there.
func clientGone(r *http.Request) bool {
	return r.Context().Err() != nil && !errors.Is(context.Cause(r.Context()), ErrShuttingDown)
}
