package structs

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/abaxoth0/Ain/errs"
)

type Task interface {
	Process()
}

type WorkerPoolOptions struct {
	// Default: 1. If <= 0, then will be set to the default
	BatchSize int
	// Default: 1s. If <= 0, then will be set to the default
	StopTimeout time.Duration
}

type WorkerPool struct {
	canceled atomic.Bool
	queue    *SyncQueue[Task]
	ctx      context.Context
	cancel   context.CancelFunc
	wg       *sync.WaitGroup
	once     sync.Once
	stopOnce sync.Once
	opt      *WorkerPoolOptions
}

const (
	workerPoolDefaultBatchSize   int           = 1
	workerPoolDefaultStopTimeout time.Duration = 1 * time.Second
)

// Creates new worker pool with parent context and options.
// If opt is nil then it will be created using default values of WorkerPoolOptions fields.
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

func (wp *WorkerPool) Start(workerCount int) {
	wp.once.Do(func() {
		for range workerCount {
			go wp.work()
		}
	})
}

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
			wp.process(tasks)
		}
	}
}

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

func (wp *WorkerPool) process(tasks []Task) {
	wp.wg.Add(1)
	defer wp.wg.Done()

	for _, task := range tasks {
		task.Process()
	}
}

func (wp *WorkerPool) IsCanceled() bool {
	return wp.canceled.Load()
}

// Cancels worker pool.
// Worker pool will finish all its tasks before stopping.
// Once canceled, worker pool can't be started again.
func (wp *WorkerPool) Cancel() error {
	if wp.canceled.Load() {
		return errors.New("worker pool is already canceled")
	}

	wp.canceled.Store(true)
	wp.cancel()
	// Fixed: WaitGroup reuse issue by using stopOnce.Do() in work() method
	// to ensure stop() is only called once from a single worker goroutine
	wp.wg.Wait()

	return nil
}

// Pushes a new task into a worker pool.
// Returns error if worker pool is canceled.
func (wp *WorkerPool) Push(t Task) error {
	if wp.canceled.Load() {
		return errors.New("can't push in canceled worker pool")
	}

	wp.queue.Push(t)

	return nil
}
