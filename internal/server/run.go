package server

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/camilbenameur/lightning/internal/language"
	"github.com/camilbenameur/lightning/internal/sandbox"
)

// job is a validated run request.
type job struct {
	lang  language.Language
	code  string
	stdin string
}

// execute runs j in a sandbox, streaming its events to sink, and records the
// outcome in the logs and metrics.
func (s *server) execute(r *http.Request, j job, sink sandbox.Sink) (sandbox.Result, error) {
	res, err := s.Executor.Run(r.Context(), s.spec(j), sink)

	var duration time.Duration
	if err == nil {
		duration = res.Duration
		s.logger(r).Info("run finished",
			"language", j.lang.ID,
			"codeBytes", len(j.code),
			"exitCode", res.ExitCode,
			"durationMs", res.Duration.Milliseconds(),
			"timedOut", res.TimedOut,
			"oomKilled", res.OOMKilled,
			"truncated", res.Truncated,
		)
	}
	s.Metrics.RunFinished(j.lang.ID, outcome(res, err), duration)
	if err != nil && errors.Is(context.Cause(r.Context()), ErrShuttingDown) {
		err = ErrShuttingDown
	}
	return res, err
}

// spec applies the configured limits, adjusted for the language, to j.
func (s *server) spec(j job) sandbox.Spec {
	l := s.Limits
	return sandbox.Spec{
		Image:       j.lang.Image(s.ImagePrefix, s.ImageTag),
		Source:      j.code,
		Stdin:       j.stdin,
		MemoryBytes: int64(j.lang.MemoryMB(l.MemoryMB)) << 20,
		NanoCPUs:    int64(l.CPUs * 1e9),
		PidsLimit:   int64(l.PidsLimit),
		Timeout:     j.lang.Timeout(l.Timeout),
		MaxOutput:   l.MaxOutputBytes,
	}
}

// outcome classifies a run for metrics.
func outcome(res sandbox.Result, err error) string {
	switch {
	case errors.Is(err, sandbox.ErrBusy):
		return "busy"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "canceled"
	case errors.Is(err, sandbox.ErrUnavailable):
		return "unavailable"
	case err != nil:
		return "error"
	case res.TimedOut:
		return "timeout"
	case res.OOMKilled:
		return "oom"
	case res.Truncated:
		return "output_limit"
	case res.ExitCode != 0:
		return "nonzero_exit"
	default:
		return "success"
	}
}

// outputBuffer is a sandbox.Sink collecting output in memory.
type outputBuffer [2]bytes.Buffer

func (b *outputBuffer) Started() {}

func (b *outputBuffer) Output(s sandbox.Stream, p []byte) { _, _ = b[s].Write(p) }
