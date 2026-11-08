package structs

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/abaxoth0/Ain/errs"
)

type workerPoolCallbackTask struct{ fn func() }

func (t *workerPoolCallbackTask) Process() { t.fn() }

func newTestWorkerPool(t *testing.T, ctx context.Context, opt *WorkerPoolOptions) *WorkerPool {
	t.Helper()
	pool := NewWorkerPool(ctx, opt)
	t.Cleanup(func() {
		_ = pool.Cancel()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := pool.Wait(ctx); err != nil {
			t.Errorf("worker pool cleanup did not finish: %v", err)
		}
	})
	return pool
}

func assertWorkerPoolStopped(t *testing.T, pool *WorkerPool) {
	t.Helper()
	select {
	case <-pool.Done():
	default:
		t.Error("shutdown returned before worker completion")
	}
	pool.mu.Lock()
	workers := pool.workers
	pool.mu.Unlock()
	if workers != 0 || pool.queue.Size() != 0 {
		t.Errorf("shutdown left %d workers and %d queued tasks", workers, pool.queue.Size())
	}
}

func awaitWorkerPoolSignal(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker pool operation did not complete")
	}
}

func TestWorkerPoolImmediateCancelCannotReportBlockedDrainComplete(t *testing.T) {
	previous := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(previous)
	started, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	pool := newTestWorkerPool(t, context.Background(), &WorkerPoolOptions{StopTimeout: 20 * time.Millisecond})
	if err := pool.Push(&workerPoolCallbackTask{fn: func() {
		close(started)
		<-release
		close(finished)
	}}); err != nil {
		t.Fatal(err)
	}
	pool.Start(1)
	if err := pool.Cancel(); !errors.Is(err, errs.StatusTimeout) {
		t.Errorf("Cancel returned %v while accepted drain work was blocked, want timeout", err)
	}
	awaitWorkerPoolSignal(t, started)
	select {
	case <-finished:
		t.Error("blocked task finished before release")
	default:
	}
	unblock()
	awaitWorkerPoolSignal(t, finished)
}

func TestWorkerPoolParentCancellationRejectsWork(t *testing.T) {
	for _, started := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		pool := newTestWorkerPool(t, ctx, nil)
		if started {
			pool.Start(2)
		}
		cancel()
		if !pool.IsCanceled() {
			t.Error("parent cancellation was not reflected in IsCanceled")
		}
		if err := pool.Push(&mockTask{}); err == nil {
			t.Error("parent-canceled pool accepted undeliverable work")
		}
		if !started {
			pool.Start(1)
		}
		if err := pool.Cancel(); err != nil {
			t.Errorf("could not join parent-canceled pool: %v", err)
		}
		assertWorkerPoolStopped(t, pool)
	}
}

func TestWorkerPoolAlreadyCanceledParent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pool := newTestWorkerPool(t, ctx, nil)
	pool.Start(4)
	if !pool.IsCanceled() {
		t.Error("already-canceled parent produced an accepting pool")
	}
	if err := pool.Push(&mockTask{}); err == nil {
		t.Error("already-canceled parent accepted work")
	}
	awaitWorkerPoolSignal(t, pool.Done())
	assertWorkerPoolStopped(t, pool)
}

func TestWorkerPoolIdleShutdownJoinsWorkers(t *testing.T) {
	for _, parent := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		pool := newTestWorkerPool(t, ctx, nil)
		entered := observeQueueWaits(pool.queue)
		pool.Start(4)
		for range 4 {
			awaitQueueWait(t, entered)
		}
		if parent {
			cancel()
			awaitWorkerPoolSignal(t, pool.Done())
		} else if err := pool.Cancel(); err != nil {
			t.Fatal(err)
		}
		assertWorkerPoolStopped(t, pool)
	}
}

func TestWorkerPoolImmediateShutdownCompletesAcceptedTasks(t *testing.T) {
	for _, workers := range []int{0, 1, 3, 8} {
		for _, batchSize := range []int{1, 4, 17} {
			t.Run(fmt.Sprintf("workers=%d/batch=%d", workers, batchSize), func(t *testing.T) {
				pool := newTestWorkerPool(t, context.Background(), &WorkerPoolOptions{
					BatchSize: batchSize, StopTimeout: 2 * time.Second,
				})
				counts := make([]atomic.Int32, 257)
				for id := range counts {
					if err := pool.Push(&workerPoolCallbackTask{fn: func() { counts[id].Add(1) }}); err != nil {
						t.Fatal(err)
					}
				}
				pool.Start(workers)
				if err := pool.Cancel(); err != nil {
					t.Fatal(err)
				}
				assertWorkerPoolStopped(t, pool)
				for id := range counts {
					if count := counts[id].Load(); count != 1 {
						t.Fatalf("accepted task %d executed %d times", id, count)
					}
				}
			})
		}
	}
}

func TestWorkerPoolShutdownBeforeStartDrainsAcceptedTasks(t *testing.T) {
	for _, parent := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		pool := newTestWorkerPool(t, ctx, nil)
		var completed atomic.Int32
		for range 32 {
			if err := pool.Push(&workerPoolCallbackTask{fn: func() { completed.Add(1) }}); err != nil {
				t.Fatal(err)
			}
		}
		if parent {
			cancel()
			awaitWorkerPoolSignal(t, pool.Done())
		} else if err := pool.Cancel(); err != nil {
			t.Fatal(err)
		}
		pool.Start(3)
		assertWorkerPoolStopped(t, pool)
		if count := completed.Load(); count != 32 {
			t.Fatalf("shutdown before Start completed %d accepted tasks, want 32", count)
		}
	}
}

func TestWorkerPoolTimeoutRetainsWorkUntilCompletion(t *testing.T) {
	pool := newTestWorkerPool(t, context.Background(), &WorkerPoolOptions{
		BatchSize: 4, StopTimeout: 20 * time.Millisecond,
	})
	started, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	counts := make([]atomic.Int32, 12)
	for id := range counts {
		if err := pool.Push(&workerPoolCallbackTask{fn: func() {
			if id == 0 {
				close(started)
				<-release
			}
			counts[id].Add(1)
		}}); err != nil {
			t.Fatal(err)
		}
	}
	pool.Start(1)
	awaitWorkerPoolSignal(t, started)
	result := make(chan error, 1)
	go func() { result <- pool.Cancel() }()
	select {
	case err := <-result:
		if !errors.Is(err, errs.StatusTimeout) {
			t.Errorf("blocked shutdown returned %v, want timeout", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Cancel did not honor StopTimeout for a running callback")
	}
	select {
	case <-pool.Done():
		t.Error("timeout reported completion while a callback was active")
	default:
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	if err := pool.Wait(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("waiting on unfinished shutdown returned %v", err)
	}
	cancel()
	if err := pool.Push(&mockTask{}); err == nil {
		t.Error("timed-out shutdown reopened admission")
	}
	unblock()
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := pool.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	assertWorkerPoolStopped(t, pool)
	for id := range counts {
		if count := counts[id].Load(); count != 1 {
			t.Fatalf("timeout lost or duplicated task %d: count=%d", id, count)
		}
	}
	cancel()
	if err := pool.Wait(ctx); err != nil {
		t.Errorf("completed shutdown lost precedence over canceled wait: %v", err)
	}
}

func TestWorkerPoolPushRacingCancellation(t *testing.T) {
	for _, parent := range []bool{false, true} {
		t.Run(fmt.Sprintf("parent=%v", parent), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			pool := newTestWorkerPool(t, ctx, &WorkerPoolOptions{BatchSize: 7, StopTimeout: 2 * time.Second})
			const producers, perProducer = 4, 250
			counts := make([]atomic.Int32, producers*perProducer+1)
			accepted := make([]bool, len(counts))
			if err := pool.Push(&workerPoolCallbackTask{fn: func() { counts[0].Add(1) }}); err != nil {
				t.Fatal(err)
			}
			accepted[0] = true
			pool.Start(4)
			start := make(chan struct{})
			var publishers sync.WaitGroup
			for producer := range producers {
				publishers.Add(1)
				go func() {
					defer publishers.Done()
					<-start
					for i := range perProducer {
						id := producer*perProducer + i + 1
						accepted[id] = pool.Push(&workerPoolCallbackTask{fn: func() { counts[id].Add(1) }}) == nil
					}
				}()
			}
			result := make(chan error, 1)
			go func() {
				<-start
				if parent {
					cancel()
					result <- nil
				} else {
					result <- pool.Cancel()
				}
			}()
			close(start)
			publishers.Wait()
			if err := <-result; err != nil {
				t.Fatal(err)
			}
			awaitWorkerPoolSignal(t, pool.Done())
			assertWorkerPoolStopped(t, pool)
			for id, ok := range accepted {
				want := int32(0)
				if ok {
					want = 1
				}
				if count := counts[id].Load(); count != want {
					t.Fatalf("task %d accepted=%v executed %d times, want %d", id, ok, count, want)
				}
			}
		})
	}
}

func TestWorkerPoolWaitDoesNotCancel(t *testing.T) {
	pool := newTestWorkerPool(t, context.Background(), nil)
	pool.Start(1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := pool.Wait(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("Wait returned %v, want context.Canceled", err)
	}
	if pool.IsCanceled() {
		t.Error("stopping a wait canceled the pool")
	}
	completed := make(chan struct{})
	if err := pool.Push(&workerPoolCallbackTask{fn: func() { close(completed) }}); err != nil {
		t.Fatal(err)
	}
	awaitWorkerPoolSignal(t, completed)
}

func TestWorkerPoolStartRacingCancel(t *testing.T) {
	for range 100 {
		pool := newTestWorkerPool(t, context.Background(), nil)
		var completed atomic.Int32
		if err := pool.Push(&workerPoolCallbackTask{fn: func() { completed.Add(1) }}); err != nil {
			t.Fatal(err)
		}
		start, started := make(chan struct{}), make(chan struct{})
		result := make(chan error, 1)
		go func() {
			<-start
			pool.Start(4)
			close(started)
		}()
		go func() {
			<-start
			result <- pool.Cancel()
		}()
		close(start)
		awaitWorkerPoolSignal(t, started)
		select {
		case err := <-result:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("concurrent Start and Cancel did not finish")
		}
		assertWorkerPoolStopped(t, pool)
		if count := completed.Load(); count != 1 {
			t.Fatalf("concurrent Start and Cancel executed accepted task %d times", count)
		}
	}
}

func TestWorkerPoolInFlightPushIsCoveredByParentShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	pool := newTestWorkerPool(t, ctx, nil)
	var completed atomic.Int32
	pool.queue.mut.Lock()
	var unlockOnce sync.Once
	unlock := func() { unlockOnce.Do(pool.queue.mut.Unlock) }
	defer unlock()
	result := make(chan error, 1)
	go func() {
		result <- pool.Push(&workerPoolCallbackTask{fn: func() { completed.Add(1) }})
	}()
	deadline := time.Now().Add(2 * time.Second)
	for pool.mu.TryLock() {
		pool.mu.Unlock()
		if time.Now().After(deadline) {
			t.Fatal("Push did not enter admission before the queue mutation")
		}
		runtime.Gosched()
	}
	cancel()
	unlock()
	if err := <-result; err != nil {
		t.Fatalf("in-flight Push was rejected after admission: %v", err)
	}
	awaitWorkerPoolSignal(t, pool.Done())
	assertWorkerPoolStopped(t, pool)
	if count := completed.Load(); count != 1 {
		t.Fatalf("parent shutdown executed in-flight accepted task %d times", count)
	}
}
