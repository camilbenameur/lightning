package ratelimit

import (
	"strconv"
	"sync"
	"testing"
	"time"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time          { return c.t }
func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestLimiter(perMinute, burst int) (*Limiter, *clock) {
	c := &clock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	l := New(perMinute, burst)
	l.now = c.now
	return l, c
}

func TestBurstThenRefill(t *testing.T) {
	l, c := newTestLimiter(30, 3) // one token every 2s

	for i := range 3 {
		if ok, _ := l.Allow("a"); !ok {
			t.Fatalf("request %d within burst was rejected", i)
		}
	}
	ok, wait := l.Allow("a")
	if ok {
		t.Fatal("request beyond burst was allowed")
	}
	if wait <= 0 || wait > 2*time.Second {
		t.Fatalf("retry after = %v, want (0, 2s]", wait)
	}

	c.advance(wait)
	if ok, _ := l.Allow("a"); !ok {
		t.Fatal("request after waiting the advertised delay was rejected")
	}
	if ok, _ := l.Allow("a"); ok {
		t.Fatal("only one token should have been refilled")
	}
}

func TestRejectedRequestsDoNotConsumeTokens(t *testing.T) {
	l, c := newTestLimiter(60, 1)
	l.Allow("a")
	for range 10 {
		if ok, _ := l.Allow("a"); ok {
			t.Fatal("expected rejection")
		}
	}
	c.advance(time.Second)
	if ok, _ := l.Allow("a"); !ok {
		t.Fatal("rejections must not push the next token further away")
	}
}

func TestKeysAreIndependent(t *testing.T) {
	l, _ := newTestLimiter(1, 1)
	if ok, _ := l.Allow("a"); !ok {
		t.Fatal("a rejected")
	}
	if ok, _ := l.Allow("b"); !ok {
		t.Fatal("b rejected because of a")
	}
	if ok, _ := l.Allow("a"); ok {
		t.Fatal("a allowed twice")
	}
}

func TestIdleKeysAreEvicted(t *testing.T) {
	l, c := newTestLimiter(60, 10) // full refill after 10s
	l.Allow("a")
	l.Allow("b")
	c.advance(5 * time.Second)
	l.Allow("b")
	if l.Len() != 2 {
		t.Fatalf("len = %d, want 2", l.Len())
	}

	c.advance(sweepInterval)
	l.Allow("c")
	if l.Len() != 1 {
		t.Fatalf("idle keys were not evicted: len = %d", l.Len())
	}
}

func TestConcurrentUse(t *testing.T) {
	l := New(1, 5) // refills too slowly to matter during the test
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	for i := range 50 {
		wg.Go(func() {
			if ok, _ := l.Allow(strconv.Itoa(i % 2)); ok {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	if allowed != 10 {
		t.Fatalf("allowed = %d, want 10 (burst 5 for each of 2 keys)", allowed)
	}
}
