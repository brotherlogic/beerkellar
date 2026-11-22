package server

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type funcQueueable struct {
	fn func(ctx context.Context) error
}

func (f funcQueueable) run(ctx context.Context) error {
	return f.fn(ctx)
}

func TestQueue_ConcurrentEnqueueRunQueueFlush(t *testing.T) {
	q := NewQueue()

	var completed atomic.Int64
	const producers = 8
	const perProducer = 50
	const flushers = 4

	var wg sync.WaitGroup
	for range producers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range perProducer {
				q.Enqueue(funcQueueable{fn: func(ctx context.Context) error {
					completed.Add(1)
					return nil
				}})
			}
		}()
	}

	for range flushers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 25 {
				q.Flush()
				time.Sleep(time.Millisecond)
			}
		}()
	}

	wg.Wait()
	q.Flush()

	want := int64(producers * perProducer)
	if got := completed.Load(); got != want {
		t.Fatalf("completed = %d, want %d", got, want)
	}
}

func TestQueue_EnqueueDoesNotBlockDuringSlowTask(t *testing.T) {
	q := NewTestQueue()

	started := make(chan struct{})
	release := make(chan struct{})
	var secondRan atomic.Bool

	q.Enqueue(funcQueueable{fn: func(ctx context.Context) error {
		close(started)
		<-release
		return nil
	}})

	flushDone := make(chan struct{})
	go func() {
		q.Flush()
		close(flushDone)
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for slow task to start")
	}

	enqueued := make(chan struct{})
	go func() {
		q.Enqueue(funcQueueable{fn: func(ctx context.Context) error {
			secondRan.Store(true)
			return nil
		}})
		close(enqueued)
	}()

	select {
	case <-enqueued:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("Enqueue blocked while slow task was executing")
	}

	close(release)

	select {
	case <-flushDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Flush to complete")
	}

	if !secondRan.Load() {
		t.Fatal("expected second enqueued task to run during Flush")
	}
}

func TestQueue_RunQueueRequeuesFailedTask(t *testing.T) {
	q := NewQueue()

	var attempts atomic.Int32
	retriedDone := make(chan struct{})
	otherDone := make(chan struct{})

	q.Enqueue(funcQueueable{fn: func(ctx context.Context) error {
		if attempts.Add(1) == 1 {
			return errors.New("transient failure")
		}
		close(retriedDone)
		return nil
	}})
	q.Enqueue(funcQueueable{fn: func(ctx context.Context) error {
		close(otherDone)
		return nil
	}})

	timeout := time.After(5 * time.Second)
	for _, ch := range []chan struct{}{otherDone, retriedDone} {
		select {
		case <-ch:
		case <-timeout:
			t.Fatalf("timed out waiting for tasks to finish (attempts=%d)", attempts.Load())
		}
	}

	if got := attempts.Load(); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
}
