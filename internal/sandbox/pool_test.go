package sandbox

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// blockingExecutor reports each run on started and blocks it until release
// is closed.
type blockingExecutor struct {
	started chan Spec
	release chan struct{}
}

func newBlockingExecutor() *blockingExecutor {
	return &blockingExecutor{started: make(chan Spec, 10), release: make(chan struct{})}
}

func (b *blockingExecutor) Run(ctx context.Context, spec Spec, _ Sink) (Result, error) {
	b.started <- spec
	select {
	case <-b.release:
		return Result{ExitCode: 7}, nil
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
}

// countingObserver records pool events.
type countingObserver struct {
	mu                                     sync.Mutex
	waits, started, finished, active, peak int
}

func (c *countingObserver) QueueWaited(time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.waits++
}

func (c *countingObserver) SandboxStarted() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.started++
	c.active++
	c.peak = max(c.peak, c.active)
}

func (c *countingObserver) SandboxFinished() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.finished++
	c.active--
}

type nopSink struct{}

func (nopSink) Started()              {}
func (nopSink) Output(Stream, []byte) {}

func TestPoolRunsAndReports(t *testing.T) {
	exec := newBlockingExecutor()
	close(exec.release)
	obs := &countingObserver{}
	p := NewPool(exec, 2, time.Second, obs)

	res, err := p.Run(t.Context(), Spec{Image: "img"}, nopSink{})
	if err != nil || res.ExitCode != 7 {
		t.Fatalf("Run = %+v, %v", res, err)
	}
	if spec := <-exec.started; spec.Image != "img" {
		t.Fatalf("spec = %+v", spec)
	}
	if obs.waits != 1 || obs.started != 1 || obs.finished != 1 || obs.active != 0 {
		t.Fatalf("observer = %+v", obs)
	}
}

func TestPoolQueueTimeout(t *testing.T) {
	exec := newBlockingExecutor()
	p := NewPool(exec, 1, 50*time.Millisecond, &countingObserver{})

	done := make(chan error)
	go func() {
		_, err := p.Run(t.Context(), Spec{}, nopSink{})
		done <- err
	}()
	<-exec.started // the first run holds the only slot

	start := time.Now()
	if _, err := p.Run(t.Context(), Spec{}, nopSink{}); !errors.Is(err, ErrBusy) {
		t.Fatalf("second run: err = %v, want ErrBusy", err)
	}
	if waited := time.Since(start); waited < 50*time.Millisecond {
		t.Fatalf("gave up after %v, before the queue timeout", waited)
	}

	close(exec.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestPoolQueuedRunStartsWhenSlotFrees(t *testing.T) {
	exec := newBlockingExecutor()
	obs := &countingObserver{}
	p := NewPool(exec, 1, 5*time.Second, obs)

	errs := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := p.Run(t.Context(), Spec{}, nopSink{})
			errs <- err
		}()
	}
	<-exec.started
	select {
	case <-exec.started:
		t.Fatal("two runs executed concurrently in a pool of size 1")
	case <-time.After(20 * time.Millisecond):
	}
	close(exec.release)
	<-exec.started
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	p.Wait()
	if obs.peak != 1 || obs.started != 2 {
		t.Fatalf("observer = %+v", obs)
	}
}

func TestPoolCanceledWhileQueued(t *testing.T) {
	exec := newBlockingExecutor()
	p := NewPool(exec, 1, time.Minute, &countingObserver{})

	go func() { _, _ = p.Run(t.Context(), Spec{}, nopSink{}) }()
	<-exec.started

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if _, err := p.Run(ctx, Spec{}, nopSink{}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	close(exec.release)
	p.Wait()
}

func TestPoolZeroQueueTimeoutFailsFast(t *testing.T) {
	exec := newBlockingExecutor()
	p := NewPool(exec, 1, 0, &countingObserver{})

	go func() { _, _ = p.Run(t.Context(), Spec{}, nopSink{}) }()
	<-exec.started
	if _, err := p.Run(t.Context(), Spec{}, nopSink{}); !errors.Is(err, ErrBusy) {
		t.Fatalf("err = %v, want ErrBusy", err)
	}
	close(exec.release)
	p.Wait()
}

func TestPoolWaitBlocksUntilRunsReturn(t *testing.T) {
	exec := newBlockingExecutor()
	p := NewPool(exec, 1, time.Second, &countingObserver{})
	go func() { _, _ = p.Run(t.Context(), Spec{}, nopSink{}) }()
	<-exec.started

	waited := make(chan struct{})
	go func() {
		p.Wait()
		close(waited)
	}()
	select {
	case <-waited:
		t.Fatal("Wait returned while a run was in progress")
	case <-time.After(20 * time.Millisecond):
	}
	close(exec.release)
	<-waited
}
