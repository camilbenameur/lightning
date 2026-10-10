package server

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/camilbenameur/lightning/internal/sandbox"
)

const (
	// coalesceWindow is the longest output is held back to be merged with
	// what follows, so that chatty programs produce a few larger events
	// rather than one event per line.
	coalesceWindow = 25 * time.Millisecond
	// maxEventData triggers an immediate flush of pending output.
	maxEventData = 16 << 10
)

type event struct {
	Type string `json:"type"`
}

type outputEvent struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

type resultEvent struct {
	Type string `json:"type"`
	runResult
}

type errorEvent struct {
	Type  string    `json:"type"`
	Error *apiError `json:"error"`
}

// eventStream is a sandbox.Sink that writes run events as NDJSON, one
// flushed line per event. Chunks of one stream arriving within
// coalesceWindow are merged; since each chunk ends on a rune boundary, so
// does every event.
type eventStream struct {
	w   http.ResponseWriter
	rc  *http.ResponseController
	enc *json.Encoder

	mu      sync.Mutex
	started bool
	closed  bool
	broken  bool // a write failed: the client is gone
	stream  sandbox.Stream
	pending []byte
	timer   *time.Timer
	armed   bool
}

func newEventStream(w http.ResponseWriter) *eventStream {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return &eventStream{w: w, rc: http.NewResponseController(w), enc: enc}
}

// Started implements sandbox.Sink. It commits the 200 response.
func (e *eventStream) Started() {
	e.mu.Lock()
	defer e.mu.Unlock()
	h := e.w.Header()
	h.Set("Content-Type", "application/x-ndjson")
	h.Set("Cache-Control", "no-cache")
	h.Set("X-Accel-Buffering", "no")
	h.Set("X-Content-Type-Options", "nosniff")
	e.w.WriteHeader(http.StatusOK)
	e.started = true
	e.send(event{Type: "started"})
}

// Output implements sandbox.Sink.
func (e *eventStream) Output(s sandbox.Stream, p []byte) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return
	}
	if len(e.pending) > 0 && s != e.stream {
		e.flushPending()
	}
	e.stream = s
	e.pending = append(e.pending, p...)
	if len(e.pending) >= maxEventData {
		e.flushPending()
		return
	}
	if !e.armed {
		e.armed = true
		if e.timer == nil {
			e.timer = time.AfterFunc(coalesceWindow, e.tick)
		} else {
			e.timer.Reset(coalesceWindow)
		}
	}
}

func (e *eventStream) tick() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.armed = false
	if !e.closed {
		e.flushPending()
	}
}

func (e *eventStream) hasStarted() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.started
}

// finish flushes pending output, writes the final event (if any) and stops
// all further writes.
func (e *eventStream) finish(final any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return
	}
	e.closed = true
	if e.timer != nil {
		e.timer.Stop()
	}
	e.flushPending()
	if final != nil {
		e.send(final)
	}
}

// flushPending sends buffered output. e.mu must be held.
func (e *eventStream) flushPending() {
	if len(e.pending) == 0 {
		return
	}
	e.send(outputEvent{Type: e.stream.String(), Data: string(e.pending)})
	e.pending = e.pending[:0]
}

// send writes one event and flushes it. e.mu must be held.
func (e *eventStream) send(v any) {
	if e.broken {
		return
	}
	if err := e.enc.Encode(v); err != nil {
		e.broken = true
		return
	}
	if err := e.rc.Flush(); err != nil {
		e.broken = true
	}
}
