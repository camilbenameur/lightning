package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/camilbenameur/lightning/internal/sandbox"
)

type errorEnvelope struct {
	Error *apiError `json:"error"`
}

type languageResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Version  string `json:"version"`
	Monaco   string `json:"monaco"`
	FileName string `json:"fileName"`
}

type languagesResponse struct {
	Languages []languageResponse `json:"languages"`
}

type limitsResponse struct {
	TimeoutMs      int64 `json:"timeoutMs"`
	MemoryMB       int   `json:"memoryMb"`
	MaxCodeBytes   int   `json:"maxCodeBytes"`
	MaxStdinBytes  int   `json:"maxStdinBytes"`
	MaxOutputBytes int   `json:"maxOutputBytes"`
	Network        bool  `json:"network"`
}

type infoResponse struct {
	Version string         `json:"version"`
	Commit  string         `json:"commit"`
	Limits  limitsResponse `json:"limits"`
}

type statusResponse struct {
	Status string `json:"status"`
}

type runRequest struct {
	Language string `json:"language"`
	Code     string `json:"code"`
	Stdin    string `json:"stdin"`
}

type runResult struct {
	ExitCode   int   `json:"exitCode"`
	DurationMs int64 `json:"durationMs"`
	TimedOut   bool  `json:"timedOut"`
	OOMKilled  bool  `json:"oomKilled"`
	Truncated  bool  `json:"truncated"`
}

func newRunResult(res sandbox.Result) runResult {
	return runResult{
		ExitCode:   res.ExitCode,
		DurationMs: res.Duration.Milliseconds(),
		TimedOut:   res.TimedOut,
		OOMKilled:  res.OOMKilled,
		Truncated:  res.Truncated,
	}
}

type runResponse struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	runResult
}

func (s *server) handleLanguages(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.languages)
}

func (s *server) handleInfo(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.info)
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, statusResponse{Status: "ok"})
}

func (s *server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.Backend.Ping(ctx); err != nil {
		s.logger(r).Warn("readiness check failed", "error", err)
		writeError(w, errorf(http.StatusServiceUnavailable, "sandbox_unavailable", "the container engine is not reachable"))
		return
	}
	writeJSON(w, http.StatusOK, statusResponse{Status: "ok"})
}

func (s *server) handleRun(w http.ResponseWriter, r *http.Request) {
	j, err := s.decodeJob(w, r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	var out outputBuffer
	res, err := s.execute(r, j, &out)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, runResponse{
		Stdout:    out[sandbox.Stdout].String(),
		Stderr:    out[sandbox.Stderr].String(),
		runResult: newRunResult(res),
	})
}

func (s *server) handleRunStream(w http.ResponseWriter, r *http.Request) {
	j, err := s.decodeJob(w, r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	events := newEventStream(w)
	res, err := s.execute(r, j, events)
	switch {
	case err == nil:
		events.finish(resultEvent{Type: "result", runResult: newRunResult(res)})
	case !events.hasStarted():
		// Nothing was sent yet, so the error can still use a proper status code.
		s.fail(w, r, err)
	case clientGone(r):
		events.finish(nil)
	default:
		e := toAPIError(err)
		s.logFailure(r, e, err)
		events.finish(errorEvent{Type: "error", Error: e})
	}
}

// decodeJob parses and validates a run request body.
func (s *server) decodeJob(w http.ResponseWriter, r *http.Request) (job, error) {
	var req runRequest
	if err := s.readBody(w, r, &req); err != nil {
		return job{}, err
	}
	switch {
	case req.Language == "":
		return job{}, errorf(http.StatusBadRequest, "invalid_request", "language is required")
	case req.Code == "":
		return job{}, errorf(http.StatusBadRequest, "invalid_request", "code is required")
	}
	lang, ok := s.Languages.Get(req.Language)
	if !ok {
		return job{}, errorf(http.StatusBadRequest, "unsupported_language", "language %q is not supported", req.Language)
	}
	if len(req.Code) > s.Limits.MaxCodeBytes {
		return job{}, errorf(http.StatusRequestEntityTooLarge, "payload_too_large", "code exceeds the limit of %d bytes", s.Limits.MaxCodeBytes)
	}
	if len(req.Stdin) > s.Limits.MaxStdinBytes {
		return job{}, errorf(http.StatusRequestEntityTooLarge, "payload_too_large", "stdin exceeds the limit of %d bytes", s.Limits.MaxStdinBytes)
	}
	return job{lang: lang, code: req.Code, stdin: req.Stdin}, nil
}

// readBody decodes a request body made of a single JSON object into v. The
// client must send it within s.bodyTimeout, without pausing for longer than
// s.bodyIdle.
func (s *server) readBody(w http.ResponseWriter, r *http.Request, v any) error {
	rc := http.NewResponseController(w)
	dec := json.NewDecoder(&deadlineReader{
		r:    http.MaxBytesReader(w, r.Body, maxBodyBytes),
		rc:   rc,
		idle: s.bodyIdle,
		end:  time.Now().Add(s.bodyTimeout),
	})
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		// The deadline stays: it bounds what net/http reads of the rest of
		// the body before responding. Once expired, it reads nothing more.
		return decodeError(err)
	}
	// Reading to EOF rejects trailing data and lets the server notice when
	// the client disconnects during the run.
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errorf(http.StatusBadRequest, "invalid_json", "request body must contain a single JSON object")
		}
		return decodeError(err)
	}
	// The deadline must not expire during the run: that would cancel the request.
	_ = rc.SetReadDeadline(time.Time{})
	return nil
}

// deadlineReader reads a request body, moving the connection's read deadline
// idle into the future before each read, but never past end. Writers without
// deadline support (tests) are fine to ignore.
type deadlineReader struct {
	r    io.Reader
	rc   *http.ResponseController
	idle time.Duration
	end  time.Time
	done bool // the body ended: net/http now reads the connection to watch for disconnects
}

func (d *deadlineReader) Read(p []byte) (int, error) {
	if !d.done {
		deadline := time.Now().Add(d.idle)
		if deadline.After(d.end) {
			deadline = d.end
		}
		_ = d.rc.SetReadDeadline(deadline)
	}
	n, err := d.r.Read(p)
	d.done = d.done || err != nil
	return n, err
}

func decodeError(err error) *apiError {
	var (
		timeout   net.Error
		tooLarge  *http.MaxBytesError
		syntax    *json.SyntaxError
		wrongType *json.UnmarshalTypeError
	)
	switch {
	case errors.As(err, &timeout) && timeout.Timeout():
		return errBodyTimeout
	case errors.As(err, &tooLarge):
		return errorf(http.StatusRequestEntityTooLarge, "payload_too_large", "request body exceeds %d bytes", tooLarge.Limit)
	case errors.Is(err, io.EOF):
		return errorf(http.StatusBadRequest, "invalid_json", "request body must be a JSON object")
	case errors.As(err, &syntax), errors.Is(err, io.ErrUnexpectedEOF):
		return errorf(http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
	case errors.As(err, &wrongType):
		if wrongType.Field == "" {
			return errorf(http.StatusBadRequest, "invalid_json", "request body must be a JSON object")
		}
		return errorf(http.StatusBadRequest, "invalid_json", "field %q must be a %s", wrongType.Field, wrongType.Type)
	default: // e.g. unknown fields
		return errorf(http.StatusBadRequest, "invalid_json", "%s", strings.TrimPrefix(err.Error(), "json: "))
	}
}
