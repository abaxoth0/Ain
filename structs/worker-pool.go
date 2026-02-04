package structs

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
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
	// Maximum time to wait for tasks to complete on shutdown.
	// Default: 1s. If <= 0, will be set to default.
	StopTimeout time.Duration
}

// Manages a pool of goroutine workers that process tasks concurrently.
type WorkerPool struct {
	canceled atomic.Bool
	queue    *SyncQueue[Task]
	ctx      context.Context
	cancel   context.CancelFunc
	wg       *sync.WaitGroup
	wgMu     sync.Mutex
	once     sync.Once
	stopOnce sync.Once
	opt      *WorkerPoolOptions
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

	return &WorkerPool{
		queue:  NewSyncQueue[Task](0),
		ctx:    ctx,
		cancel: cancel,
		wg:     new(sync.WaitGroup),
		opt:    opt,
	}
}

// Starts the worker pool with the specified number of worker goroutines.
// This method is idempotent - subsequent calls will have no effect.
func (wp *WorkerPool) Start(workerCount int) {
	wp.once.Do(func() {
		for range workerCount {
			go wp.work()
		}
	})
}

// Attempts to process remaining tasks within the timeout period.
// Returns nil if all tasks processed, errs.StatusTimeout if timeout exceeded.
func (wp *WorkerPool) stop() error {
	timeout := time.After(wp.opt.StopTimeout)
	for {
		select {
		case <-timeout:
			return errs.StatusTimeout
		default:
			tasks, ok := wp.queue.PopN(wp.opt.BatchSize)
			if !ok {
				return nil
			}
			wp.executeTasks(tasks)
		}
	}
}

// Main worker loop that processes tasks from the queue.
func (wp *WorkerPool) work() {
	for {
		select {
		case <-wp.ctx.Done():
			wp.stopOnce.Do(func() {
				wp.stop()
			})
			return
		default:
			if wp.queue.Size() == 0 {
				wp.queue.WaitTillNotEmpty(0)
				continue
			}
			tasks, ok := wp.queue.PopN(wp.opt.BatchSize)
			if !ok {
				continue
			}
			wp.process(tasks)
		}
	}
}

// Executes a batch of tasks and manages the WaitGroup.
func (wp *WorkerPool) process(tasks []Task) {
	wp.wgMu.Lock()
	wp.wg.Add(1)
	wp.wgMu.Unlock()
	defer wp.wg.Done()

	wp.executeTasks(tasks)
}

func (wp *WorkerPool) executeTasks(tasks []Task) {
	for _, task := range tasks {
		task.Process()
	}
}

// Returns true if the worker pool has been canceled.
func (wp *WorkerPool) IsCanceled() bool {
	return wp.canceled.Load()
}

// Gracefully shuts down the worker pool.
// The worker pool will finish all its tasks before stopping.
// Once canceled, the worker pool cannot be started again.
func (wp *WorkerPool) Cancel() error {
	if wp.canceled.Load() {
		return errors.New("worker pool is already canceled")
	}

	wp.canceled.Store(true)
	wp.cancel()
	wp.wgMu.Lock()
	wp.wg.Wait()
	wp.wgMu.Unlock()

	return nil
}

// Adds a new task to the worker pool queue.
// Returns error if worker pool is canceled.
func (wp *WorkerPool) Push(t Task) error {
	if wp.canceled.Load() {
		return errors.New("can't push in canceled worker pool")
	}

	wp.queue.Push(t)

	return nil
}
