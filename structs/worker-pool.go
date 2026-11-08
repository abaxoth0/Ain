package structs

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/abaxoth0/Ain/errs"
)

// Represents an executable task that can be processed by a worker pool.
type Task interface {
	// Executes the task's logic.
	Process()
}

// Holds configuration options for a worker pool.
type WorkerPoolOptions struct {
	// Default: 1. If <= 0, will be set to default.
	BatchSize int
	// Maximum time Cancel waits for shutdown; accepted work continues after timeout.
	// Process cannot be interrupted. Wait or Done observes eventual completion.
	// Default: 1s. If <= 0, will be set to default.
	StopTimeout time.Duration
}

// Manages a pool of goroutine workers that process tasks concurrently.
type WorkerPool struct {
	mu           sync.Mutex
	queue        *SyncQueue[Task]
	ctx          context.Context
	cancel       context.CancelFunc
	started      bool
	stopping     bool
	cancelCalled bool
	workers      int
	done         chan struct{}
	opt          *WorkerPoolOptions
}

const (
	workerPoolDefaultBatchSize   int           = 1
	workerPoolDefaultStopTimeout time.Duration = 1 * time.Second
)

// Creates a new worker pool with the given parent context and options.
// If opt is nil, it will be created using default values of WorkerPoolOptions fields.
func NewWorkerPool(ctx context.Context, opt *WorkerPoolOptions) *WorkerPool {
	ctx, cancel := context.WithCancel(ctx)

	if opt == nil {
		opt = &WorkerPoolOptions{
			BatchSize:   workerPoolDefaultBatchSize,
			StopTimeout: workerPoolDefaultStopTimeout,
		}
	}
	if opt.BatchSize <= 0 {
		opt.BatchSize = workerPoolDefaultBatchSize
	}
	if opt.StopTimeout <= 0 {
		opt.StopTimeout = workerPoolDefaultStopTimeout
	}

	pool := &WorkerPool{
		queue:  NewSyncQueue[Task](0),
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
		opt:    opt,
	}
	context.AfterFunc(ctx, pool.beginStop)
	return pool
}

// Starts the worker pool with the specified number of worker goroutines.
// This method is idempotent - subsequent calls will have no effect.
func (wp *WorkerPool) Start(workerCount int) {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	if wp.started || wp.stopping {
		return
	}
	if wp.ctx.Err() != nil {
		wp.stopLocked()
		return
	}
	wp.started = true
	if workerCount > 0 {
		wp.workers = workerCount
		for range workerCount {
			go wp.work()
		}
	}
}

func (wp *WorkerPool) beginStop() {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	wp.stopLocked()
}

func (wp *WorkerPool) stopLocked() {
	if wp.stopping {
		return
	}
	wp.stopping = true
	wp.cancel()
	if wp.workers == 0 {
		if wp.queue.Size() == 0 {
			close(wp.done)
		} else {
			wp.workers = 1
			go wp.work()
		}
	}
}

func (wp *WorkerPool) drain() {
	for {
		tasks, ok := wp.queue.PopN(wp.opt.BatchSize)
		if !ok {
			return
		}
		wp.executeTasks(tasks)
	}
}

// Main worker loop that processes tasks from the queue.
func (wp *WorkerPool) work() {
	defer wp.workerDone()
	for {
		if wp.ctx.Err() != nil {
			wp.beginStop()
			wp.drain()
			return
		}
		if err := wp.queue.WaitTillNotEmptyContext(wp.ctx); err != nil {
			continue
		}
		tasks, ok := wp.queue.PopN(wp.opt.BatchSize)
		if ok {
			wp.executeTasks(tasks)
		}
	}
}

func (wp *WorkerPool) workerDone() {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	wp.workers--
	if wp.stopping && wp.workers == 0 {
		close(wp.done)
	}
}

func (wp *WorkerPool) executeTasks(tasks []Task) {
	for _, task := range tasks {
		task.Process()
	}
}

// Returns true after explicit cancellation or parent context cancellation.
func (wp *WorkerPool) IsCanceled() bool {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	return wp.stopping || wp.ctx.Err() != nil
}

// Cancel closes admission and waits up to StopTimeout for all accepted tasks and workers.
// Timeout wraps errs.StatusTimeout; draining continues and Wait or Done can observe completion.
// The first Cancel may join shutdown started by the parent context; repeated calls return an error.
func (wp *WorkerPool) Cancel() error {
	wp.mu.Lock()
	if wp.cancelCalled {
		wp.mu.Unlock()
		return errors.New("worker pool is already canceled")
	}
	wp.cancelCalled = true
	wp.stopLocked()
	wp.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), wp.opt.StopTimeout)
	defer cancel()
	if err := wp.Wait(ctx); err != nil {
		return fmt.Errorf("worker pool shutdown incomplete: %w", errs.StatusTimeout)
	}
	return nil
}

// Done closes when shutdown has completed, all accepted tasks finished, and all workers exited.
func (wp *WorkerPool) Done() <-chan struct{} {
	return wp.done
}

// Wait observes shutdown without initiating it. It returns ctx.Err() if the caller stops waiting.
// Completed shutdown takes precedence over context cancellation.
func (wp *WorkerPool) Wait(ctx context.Context) error {
	select {
	case <-wp.done:
		return nil
	default:
	}
	select {
	case <-wp.done:
		return nil
	case <-ctx.Done():
		select {
		case <-wp.done:
			return nil
		default:
			return ctx.Err()
		}
	}
}

// Adds a new task to the worker pool queue.
// Returns error if worker pool is canceled.
func (wp *WorkerPool) Push(t Task) error {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	if wp.stopping || wp.ctx.Err() != nil {
		wp.stopLocked()
		return errors.New("can't push in canceled worker pool")
	}

	return wp.queue.Push(t)
}
