// Package sandbox executes untrusted programs in isolated, throw-away
// containers and reports their output as it is produced.
package sandbox

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Stream identifies an output stream of a sandboxed program.
type Stream uint8

// Output streams.
const (
	Stdout Stream = iota
	Stderr
)

func (s Stream) String() string {
	if s == Stderr {
		return "stderr"
	}
	return "stdout"
}

// Spec describes one execution.
type Spec struct {
	Image       string // runtime image implementing the runtime contract
	Source      string // program source code
	Stdin       string // data available on the program's standard input
	MemoryBytes int64  // memory limit (swap disabled)
	NanoCPUs    int64  // CPU quota in billionths of a CPU
	PidsLimit   int64
	Timeout     time.Duration // wall-clock limit, including compilation
	MaxOutput   int           // combined stdout+stderr limit in bytes
}

// Result describes a finished execution.
type Result struct {
	ExitCode  int
	Duration  time.Duration // from container start to exit
	TimedOut  bool          // killed for outliving Spec.Timeout; Duration is at least that long
	OOMKilled bool          // a process exceeded the memory limit
	Truncated bool          // output exceeded Spec.MaxOutput: it was cut there and the program killed
}

// Sink receives the events of an execution. Its methods are never called
// concurrently, and never after Run has returned.
type Sink interface {
	// Started is called once the program is running.
	Started()
	// Output delivers a chunk of output. Chunks never split a UTF-8 encoded
	// rune. data is only valid for the duration of the call.
	Output(stream Stream, data []byte)
}

// Executor runs programs in a sandbox.
type Executor interface {
	// Run executes spec and blocks until the program has exited and its
	// sandbox has been destroyed. A program that fails, times out or runs
	// out of memory is not an error: that is reported in Result. When ctx is
	// canceled the program is killed and ctx.Err() is returned.
	Run(ctx context.Context, spec Spec, sink Sink) (Result, error)
}

var (
	// ErrUnavailable reports that the sandbox backend cannot run programs.
	ErrUnavailable = errors.New("sandbox unavailable")
	// ErrImageNotFound reports that the runtime image is not present.
	ErrImageNotFound = fmt.Errorf("%w: runtime image not found", ErrUnavailable)
)
