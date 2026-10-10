package sandbox

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrBusy is returned by a Pool when no execution slot frees up within its
// queue timeout.
var ErrBusy = errors.New("all sandboxes are busy")

// PoolObserver is notified of a Pool's activity, e.g. to export metrics.
type PoolObserver interface {
	QueueWaited(time.Duration)
	SandboxStarted()
	SandboxFinished()
}

// Pool is an Executor that bounds the number of concurrent executions of
// another Executor. Runs beyond the limit wait for a free slot for up to the
// queue timeout.
type Pool struct {
	exec         Executor
	slots        chan struct{}
	queueTimeout time.Duration
	observer     PoolObserver
	running      sync.WaitGroup
}

// NewPool returns a Pool running at most size executions of exec at a time.
func NewPool(exec Executor, size int, queueTimeout time.Duration, observer PoolObserver) *Pool {
	return &Pool{
		exec:         exec,
		slots:        make(chan struct{}, size),
		queueTimeout: queueTimeout,
		observer:     observer,
	}
}

// Run implements Executor. It returns ErrBusy if no slot frees up within the
// queue timeout, or ctx.Err() if ctx ends first.
func (p *Pool) Run(ctx context.Context, spec Spec, sink Sink) (Result, error) {
	p.running.Add(1)
	defer p.running.Done()

	if err := p.acquire(ctx); err != nil {
		return Result{}, err
	}
	defer func() { <-p.slots }()

	p.observer.SandboxStarted()
	defer p.observer.SandboxFinished()
	return p.exec.Run(ctx, spec, sink)
}

func (p *Pool) acquire(ctx context.Context) error {
	start := time.Now()
	defer func() { p.observer.QueueWaited(time.Since(start)) }()

	select {
	case p.slots <- struct{}{}:
		return nil
	default:
	}
	timer := time.NewTimer(p.queueTimeout)
	defer timer.Stop()
	select {
	case p.slots <- struct{}{}:
		return nil
	case <-timer.C:
		return ErrBusy
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Wait blocks until every Run in progress has returned.
func (p *Pool) Wait() {
	p.running.Wait()
}
