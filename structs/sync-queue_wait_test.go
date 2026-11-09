package structs

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/abaxoth0/Ain/errs"
)

type queueWaitObserver struct {
	sync.Locker
	entered chan struct{}
}

type queueCancelOnRegistration struct {
	context.Context
	cancel context.CancelFunc
	once   sync.Once
}

func (c *queueCancelOnRegistration) Done() <-chan struct{} {
	c.once.Do(c.cancel)
	return c.Context.Done()
}

func TestSyncQueueCancellationBeforeRegistration(t *testing.T) {
	for _, empty := range []bool{false, true} {
		q := NewSyncQueue[int](0)
		if empty {
			_ = q.Push(1)
		}
		ctx, cancel := context.WithCancel(context.Background())
		observed := &queueCancelOnRegistration{Context: ctx, cancel: cancel}
		result := make(chan error, 1)
		go func() { result <- queueContextWait(q, empty)(observed) }()
		awaitQueueResult(t, result, context.Canceled)
		cancel()
	}
}

func queueContextWait(q *SyncQueue[int], empty bool) func(context.Context) error {
	if empty {
		return q.WaitTillEmptyContext
	}
	return q.WaitTillNotEmptyContext
}

func TestSyncQueueContextAlreadyDone(t *testing.T) {
	for _, empty := range []bool{false, true} {
		for _, deadline := range []bool{false, true} {
			q := NewSyncQueue[int](0)
			if empty {
				_ = q.Push(1)
			}
			ctx, cancel := context.WithCancel(context.Background())
			want := context.Canceled
			if deadline {
				cancel()
				ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
				want = context.DeadlineExceeded
			}
			cancel()
			wait := queueContextWait(q, empty)
			result := make(chan error, 1)
			go func() { result <- wait(ctx) }()
			awaitQueueResult(t, result, want)
			if empty {
				q.Pop()
			} else {
				_ = q.Push(1)
			}
			if err := wait(ctx); err != nil {
				t.Errorf("satisfied queue condition lost precedence over %v: %v", want, err)
			}
			var err error
			if empty {
				err = q.WaitTillEmpty(time.Nanosecond)
			} else {
				err = q.WaitTillNotEmpty(time.Nanosecond)
			}
			if err != nil {
				t.Errorf("satisfied queue condition timed out: %v", err)
			}
		}
	}
}

func TestSyncQueueContextCancellation(t *testing.T) {
	for _, empty := range []bool{false, true} {
		q := NewSyncQueue[int](0)
		if empty {
			_ = q.Push(1)
		}
		entered := observeQueueWaits(q)
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		result := make(chan error, 8)
		for range cap(result) {
			go func() { result <- queueContextWait(q, empty)(ctx) }()
			awaitQueueWait(t, entered)
		}
		cancel()
		for range cap(result) {
			awaitQueueResult(t, result, context.Canceled)
		}
		wantSize := 0
		if empty {
			wantSize = 1
		}
		if q.Size() != wantSize {
			t.Error("cancellation mutated the queue")
		}
	}
}

func TestSyncQueueContextDeadline(t *testing.T) {
	for _, empty := range []bool{false, true} {
		q := NewSyncQueue[int](0)
		if empty {
			_ = q.Push(1)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		result := make(chan error, 1)
		go func() { result <- queueContextWait(q, empty)(ctx) }()
		awaitQueueResult(t, result, context.DeadlineExceeded)
		cancel()
	}
}

func TestSyncQueueWaitersRecheckCondition(t *testing.T) {
	for _, empty := range []bool{false, true} {
		q := NewSyncQueue[int](0)
		if empty {
			_ = q.Push(1)
		}
		entered := observeQueueWaits(q)
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		survivorCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
		t.Cleanup(stop)
		canceled, survivor := make(chan error, 1), make(chan error, 1)
		go func() { canceled <- queueContextWait(q, empty)(ctx) }()
		awaitQueueWait(t, entered)
		go func() { survivor <- queueContextWait(q, empty)(survivorCtx) }()
		awaitQueueWait(t, entered)
		cancel()
		awaitQueueResult(t, canceled, context.Canceled)
		awaitQueueWait(t, entered)
		select {
		case err := <-survivor:
			t.Errorf("another waiter's cancellation released a pending waiter: %v", err)
		default:
		}
		q.mut.Lock()
		q.cond.Broadcast()
		q.mut.Unlock()
		awaitQueueWait(t, entered)
		if empty {
			q.PopN(1)
		} else {
			_ = q.Push(1)
		}
		awaitQueueResult(t, survivor, nil)
		stop()
	}
}

func TestSyncQueueWaitAndMutationRaces(t *testing.T) {
	for _, empty := range []bool{false, true} {
		for _, timed := range []bool{false, true} {
			for range 100 {
				q := NewSyncQueue[int](0)
				if empty {
					_ = q.Push(1)
				}
				ctx, cancel := context.WithCancel(context.Background())
				start := make(chan struct{})
				result := make(chan error, 1)
				go func() {
					<-start
					if timed && empty {
						result <- q.WaitTillEmpty(time.Nanosecond)
					} else if timed {
						result <- q.WaitTillNotEmpty(time.Nanosecond)
					} else {
						result <- queueContextWait(q, empty)(ctx)
					}
				}()
				var operations sync.WaitGroup
				operations.Add(2)
				go func() {
					defer operations.Done()
					<-start
					cancel()
				}()
				go func() {
					defer operations.Done()
					<-start
					if empty {
						q.UnwrapAndFlush()
					} else {
						_ = q.Push(1)
					}
				}()
				close(start)
				operations.Wait()
				select {
				case err := <-result:
					want := error(context.Canceled)
					if timed {
						want = errs.StatusTimeout
					}
					if err != nil && !errors.Is(err, want) {
						t.Fatalf("racing wait returned %v, want nil or %v", err, want)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("racing queue wait did not finish")
				}
			}
		}
	}
}

func (w *queueWaitObserver) Unlock() {
	w.Locker.Unlock()
	select {
	case w.entered <- struct{}{}:
	default:
	}
}

func observeQueueWaits[T comparable](q *SyncQueue[T]) <-chan struct{} {
	entered := make(chan struct{}, 128)
	q.cond.L = &queueWaitObserver{Locker: &q.mut, entered: entered}
	return entered
}

func awaitQueueWait(t *testing.T, entered <-chan struct{}) {
	t.Helper()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("queue waiter did not register")
	}
}

func awaitQueueResult(t *testing.T, result <-chan error, want error) {
	t.Helper()
	select {
	case err := <-result:
		if !errors.Is(err, want) {
			t.Errorf("wait returned %v, want %v", err, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("queue waiter did not exit")
	}
}

func TestSyncQueueShortTimeout(t *testing.T) {
	previous := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(previous)
	for _, empty := range []bool{false, true} {
		name := "not empty"
		if empty {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			q := NewSyncQueue[int](0)
			wait := q.WaitTillNotEmpty
			if empty {
				_ = q.Push(1)
				wait = q.WaitTillEmpty
			}
			result := make(chan error, 1)
			go func() { result <- wait(time.Nanosecond) }()
			select {
			case err := <-result:
				if !errors.Is(err, errs.StatusTimeout) {
					t.Errorf("wait returned %v, want timeout", err)
				}
			case <-time.After(100 * time.Millisecond):
				t.Error("timeout required an unrelated queue mutation to return")
				if empty {
					q.Pop()
				} else {
					_ = q.Push(1)
				}
				awaitQueueResult(t, result, errs.StatusTimeout)
			}
		})
	}
}

func TestSyncQueueRollbackAndFlushWakeWaiters(t *testing.T) {
	for _, rollback := range []bool{true, false} {
		name := "flush"
		if rollback {
			name = "rollback"
		}
		t.Run(name, func(t *testing.T) {
			q := NewSyncQueue[int](0)
			_ = q.Push(1)
			if rollback {
				q.PreserveAndPop()
			}
			entered := observeQueueWaits(q)
			const waiters = 4
			result := make(chan error, waiters)
			for range waiters {
				go func() {
					if rollback {
						result <- q.WaitTillNotEmpty(200 * time.Millisecond)
					} else {
						result <- q.WaitTillEmpty(200 * time.Millisecond)
					}
				}()
				awaitQueueWait(t, entered)
			}
			if rollback {
				q.RollBack()
			} else {
				q.UnwrapAndFlush()
			}
			for range waiters {
				awaitQueueResult(t, result, nil)
			}
		})
	}
}
