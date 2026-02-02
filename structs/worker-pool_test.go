package structs

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// Mock task for testing
type mockTask struct {
	id       int
	executed bool
	mu       sync.Mutex
}

func (t *mockTask) Process() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.executed = true
}

func (t *mockTask) IsExecuted() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.executed
}

// Test task that can be controlled
type controllableTask struct {
	executeChan chan struct{}
	doneChan    chan struct{}
	executed    bool
	mu          sync.Mutex
}

func newControllableTask() *controllableTask {
	return &controllableTask{
		executeChan: make(chan struct{}),
		doneChan:    make(chan struct{}),
	}
}

func (t *controllableTask) Process() {
	t.mu.Lock()
	t.executed = true
	t.mu.Unlock()

	// Wait for signal to continue
	<-t.executeChan
	t.doneChan <- struct{}{}
}

func (t *controllableTask) AllowExecution() {
	t.executeChan <- struct{}{}
}

func (t *controllableTask) WaitForCompletion() {
	<-t.doneChan
}

func (t *controllableTask) IsExecuted() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.executed
}

func TestNewWorkerPool(t *testing.T) {
	ctx := context.Background()

	t.Run("with default options", func(t *testing.T) {
		wp := NewWorkerPool(ctx, nil)

		if wp == nil {
			t.Fatal("WorkerPool should not be nil")
		}

		if wp.opt.BatchSize != workerPoolDefaultBatchSize {
			t.Errorf("Expected batch size %d, got %d", workerPoolDefaultBatchSize, wp.opt.BatchSize)
		}

		if wp.opt.StopTimeout != workerPoolDefaultStopTimeout {
			t.Errorf("Expected stop timeout %v, got %v", workerPoolDefaultStopTimeout, wp.opt.StopTimeout)
		}
	})

	t.Run("with custom options", func(t *testing.T) {
		opts := &WorkerPoolOptions{
			BatchSize:   5,
			StopTimeout: 2 * time.Second,
		}

		wp := NewWorkerPool(ctx, opts)

		if wp.opt.BatchSize != 5 {
			t.Errorf("Expected batch size 5, got %d", wp.opt.BatchSize)
		}

		if wp.opt.StopTimeout != 2*time.Second {
			t.Errorf("Expected stop timeout 2s, got %v", wp.opt.StopTimeout)
		}
	})

	t.Run("with invalid options", func(t *testing.T) {
		opts := &WorkerPoolOptions{
			BatchSize:   -1,               // Invalid
			StopTimeout: -1 * time.Second, // Invalid
		}

		wp := NewWorkerPool(ctx, opts)

		// Should fall back to defaults
		if wp.opt.BatchSize != workerPoolDefaultBatchSize {
			t.Errorf("Expected batch size %d, got %d", workerPoolDefaultBatchSize, wp.opt.BatchSize)
		}

		if wp.opt.StopTimeout != workerPoolDefaultStopTimeout {
			t.Errorf("Expected stop timeout %v, got %v", workerPoolDefaultStopTimeout, wp.opt.StopTimeout)
		}
	})
}

func TestWorkerPoolStart(t *testing.T) {
	ctx := context.Background()
	wp := NewWorkerPool(ctx, nil)

	t.Run("start workers", func(t *testing.T) {
		workerCount := 3
		wp.Start(workerCount)

		time.Sleep(10 * time.Millisecond)

		task := &mockTask{id: 1}
		err := wp.Push(task)
		if err != nil {
			t.Fatalf("Failed to push task: %v", err)
		}

		// Wait for task to be processed
		time.Sleep(50 * time.Millisecond)

		if !task.IsExecuted() {
			t.Error("Task should have been executed")
		}
	})

	t.Run("start multiple times should be no-op", func(t *testing.T) {
		wp2 := NewWorkerPool(ctx, nil)
		wp2.Start(2)
		wp2.Start(5) // Should be ignored

		// Should still work
		task := &mockTask{id: 2}
		err := wp2.Push(task)
		if err != nil {
			t.Fatalf("Failed to push task: %v", err)
		}

		time.Sleep(50 * time.Millisecond)

		if !task.IsExecuted() {
			t.Error("Task should have been executed")
		}
	})
}

func TestWorkerPoolPush(t *testing.T) {
	ctx := context.Background()
	wp := NewWorkerPool(ctx, nil)
	wp.Start(1)

	t.Run("push single task", func(t *testing.T) {
		task := &mockTask{id: 1}
		err := wp.Push(task)
		if err != nil {
			t.Fatalf("Failed to push task: %v", err)
		}

		time.Sleep(50 * time.Millisecond)

		if !task.IsExecuted() {
			t.Error("Task should have been executed")
		}
	})

	t.Run("push multiple tasks", func(t *testing.T) {
		tasks := make([]*mockTask, 5)
		for i := range 5 {
			tasks[i] = &mockTask{id: i}
			err := wp.Push(tasks[i])
			if err != nil {
				t.Fatalf("Failed to push task %d: %v", i, err)
			}
		}

		time.Sleep(100 * time.Millisecond)

		for i, task := range tasks {
			if !task.IsExecuted() {
				t.Errorf("Task %d should have been executed", i)
			}
		}
	})

	t.Run("push to canceled pool", func(t *testing.T) {
		wp2 := NewWorkerPool(ctx, nil)
		wp2.Start(1)
		wp2.Cancel()

		task := &mockTask{id: 1}
		err := wp2.Push(task)
		if err == nil {
			t.Error("Should return error when pushing to canceled pool")
		}

		if err.Error() != "can't push in canceled worker pool" {
			t.Errorf("Expected specific error, got: %v", err)
		}
	})
}

func TestWorkerPoolCancel(t *testing.T) {
	ctx := context.Background()

	t.Run("cancel empty pool", func(t *testing.T) {
		wp := NewWorkerPool(ctx, nil)
		wp.Start(1)

		err := wp.Cancel()
		if err != nil {
			t.Errorf("Cancel should succeed on empty pool: %v", err)
		}

		if !wp.IsCanceled() {
			t.Error("Pool should be marked as canceled")
		}
	})

	t.Run("cancel pool with pending tasks", func(t *testing.T) {
		wp := NewWorkerPool(ctx, &WorkerPoolOptions{
			BatchSize:   1,
			StopTimeout: 100 * time.Millisecond,
		})
		wp.Start(1)

		// Add a controllable task
		task := newControllableTask()
		err := wp.Push(task)
		if err != nil {
			t.Fatalf("Failed to push task: %v", err)
		}

		// Wait for task to start processing
		time.Sleep(10 * time.Millisecond)

		// Allow task to complete before canceling
		task.AllowExecution()
		task.WaitForCompletion()

		// Cancel the pool
		err = wp.Cancel()
		if err != nil {
			t.Errorf("Cancel should succeed: %v", err)
		}

		if !wp.IsCanceled() {
			t.Error("Pool should be marked as canceled")
		}
	})

	t.Run("double cancel should return error", func(t *testing.T) {
		wp := NewWorkerPool(ctx, nil)
		wp.Start(1)

		err := wp.Cancel()
		if err != nil {
			t.Errorf("First cancel should succeed: %v", err)
		}

		err = wp.Cancel()
		if err == nil {
			t.Error("Second cancel should return error")
		}

		if err.Error() != "worker pool is already canceled" {
			t.Errorf("Expected specific error, got: %v", err)
		}
	})
}

func TestWorkerPoolBatchProcessing(t *testing.T) {
	ctx := context.Background()

	t.Run("batch size 1", func(t *testing.T) {
		wp := NewWorkerPool(ctx, &WorkerPoolOptions{
			BatchSize: 1,
		})
		wp.Start(1)

		tasks := make([]*mockTask, 3)
		for i := range 3 {
			tasks[i] = &mockTask{id: i}
			err := wp.Push(tasks[i])
			if err != nil {
				t.Fatalf("Failed to push task %d: %v", i, err)
			}
		}

		time.Sleep(100 * time.Millisecond)

		for i, task := range tasks {
			if !task.IsExecuted() {
				t.Errorf("Task %d should have been executed", i)
			}
		}
	})

	t.Run("batch size 3", func(t *testing.T) {
		wp := NewWorkerPool(ctx, &WorkerPoolOptions{
			BatchSize: 3,
		})
		wp.Start(1)

		tasks := make([]*mockTask, 5)
		for i := range 5 {
			tasks[i] = &mockTask{id: i}
			err := wp.Push(tasks[i])
			if err != nil {
				t.Fatalf("Failed to push task %d: %v", i, err)
			}
		}

		time.Sleep(100 * time.Millisecond)

		for i, task := range tasks {
			if !task.IsExecuted() {
				t.Errorf("Task %d should have been executed", i)
			}
		}
	})
}

func TestWorkerPoolConcurrency(t *testing.T) {
	ctx := context.Background()
	wp := NewWorkerPool(ctx, nil)
	wp.Start(3)

	t.Run("concurrent pushes", func(t *testing.T) {
		const numTasks = 100
		tasks := make([]*mockTask, numTasks)

		var wg sync.WaitGroup
		wg.Add(numTasks)

		for i := range numTasks {
			go func(i int) {
				defer wg.Done()
				tasks[i] = &mockTask{id: i}
				err := wp.Push(tasks[i])
				if err != nil {
					t.Errorf("Failed to push task %d: %v", i, err)
				}
			}(i)
		}

		wg.Wait()

		time.Sleep(200 * time.Millisecond)

		for i, task := range tasks {
			if !task.IsExecuted() {
				t.Errorf("Task %d should have been executed", i)
			}
		}
	})
}

func TestWorkerPoolIsCanceled(t *testing.T) {
	ctx := context.Background()
	wp := NewWorkerPool(ctx, nil)

	t.Run("initially not canceled", func(t *testing.T) {
		if wp.IsCanceled() {
			t.Error("Pool should not be canceled initially")
		}
	})

	t.Run("canceled after cancel", func(t *testing.T) {
		wp.Start(1)
		wp.Cancel()

		if !wp.IsCanceled() {
			t.Error("Pool should be canceled after cancel")
		}
	})
}

// Test for the bug mentioned in the comment (line 134-137)
func TestWorkerPoolBugFix(t *testing.T) {
	ctx := context.Background()

	// Test with different worker counts to reproduce the bug if it exists
	workerCounts := []int{1, 2, 3, 4, 5}

	for _, count := range workerCounts {
		t.Run(fmt.Sprintf("worker_count_%d", count), func(t *testing.T) {
			wp := NewWorkerPool(ctx, &WorkerPoolOptions{
				BatchSize:   1,
				StopTimeout: 50 * time.Millisecond,
			})
			wp.Start(count)

			// Add some tasks
			for i := range 5 {
				task := &mockTask{id: i}
				err := wp.Push(task)
				if err != nil {
					t.Fatalf("Failed to push task %d: %v", i, err)
				}
			}

			// Cancel immediately - this might trigger the bug
			err := wp.Cancel()
			if err != nil {
				t.Errorf("Cancel failed with %d workers: %v", count, err)
			}

			// If we get here without panic, the test passes
		})
	}
}
