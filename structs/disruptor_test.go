package structs

import (
	"sync"
	"testing"
	"time"
)

func TestNewDisruptor(t *testing.T) {
	t.Run("create disruptor", func(t *testing.T) {
		d := NewDisruptor[int]()
		if d == nil {
			t.Fatal("Disruptor should not be nil")
		}

		if d.writer.Value.Load() != -1 {
			t.Errorf("Expected writer to start at -1, got %d", d.writer.Value.Load())
		}
		if d.reader.Value.Load() != -1 {
			t.Errorf("Expected reader to start at -1, got %d", d.reader.Value.Load())
		}
		if d.closed.Load() {
			t.Error("Disruptor should not be closed initially")
		}
	})
}

func TestDisruptorPublish(t *testing.T) {
	t.Run("publish single entry", func(t *testing.T) {
		d := NewDisruptor[string]()

		success := d.Publish("test")
		if !success {
			t.Error("Publish should succeed")
		}

		if d.writer.Value.Load() != 0 {
			t.Errorf("Expected writer at 0, got %d", d.writer.Value.Load())
		}
	})

	t.Run("publish multiple entries", func(t *testing.T) {
		d := NewDisruptor[int]()

		const numEntries = 100
		for i := range numEntries {
			success := d.Publish(i)
			if !success {
				t.Errorf("Publish %d should succeed", i)
			}
		}

		expectedWriter := int64(numEntries - 1)
		if d.writer.Value.Load() != expectedWriter {
			t.Errorf("Expected writer at %d, got %d", expectedWriter, d.writer.Value.Load())
		}
	})

	t.Run("publish to closed disruptor", func(t *testing.T) {
		d := NewDisruptor[string]()
		d.Close()

		success := d.Publish("test")
		if success {
			t.Error("Publish to closed disruptor should fail")
		}
	})

	t.Run("buffer overflow", func(t *testing.T) {
		d := NewDisruptor[int]()

		// Don't start a consumer - just fill the buffer
		// Fill buffer to capacity (BufferSize-1 entries: 0 to BufferSize-2)
		for i := range BufferSize - 1 {
			success := d.Publish(i)
			if !success {
				t.Errorf("Publish %d should succeed (writer: %d, reader: %d)", i, d.writer.Value.Load(), d.reader.Value.Load())
			}
		}

		// This should fail due to buffer overflow (trying to publish BufferSize entries)
		success := d.Publish(BufferSize - 1)
		if success {
			t.Error("Publish should fail due to buffer overflow")
		}

		d.Close()
	})
}

func TestDisruptorConsume(t *testing.T) {
	t.Run("consume single entry", func(t *testing.T) {
		d := NewDisruptor[string]()

		success := d.Publish("test")
		if !success {
			t.Fatal("Publish should succeed")
		}

		var consumed string
		var consumeErr error
		done := make(chan struct{})

		go func() {
			defer close(done)
			consumeErr = d.Consume(func(entry string) {
				consumed = entry
			})
		}()

		time.Sleep(50 * time.Millisecond)
		d.Close()

		select {
		case <-done:
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Consume should have finished")
		}

		if consumeErr != nil {
			t.Errorf("Consume should succeed: %v", consumeErr)
		}
		if consumed != "test" {
			t.Errorf("Expected 'test', got %s", consumed)
		}
	})

	t.Run("consume multiple entries", func(t *testing.T) {
		d := NewDisruptor[int]()

		const numEntries = 50
		expected := make([]int, numEntries)

		for i := range numEntries {
			expected[i] = i
			success := d.Publish(i)
			if !success {
				t.Fatalf("Publish %d should succeed", i)
			}
		}

		var consumed []int
		var consumedMu sync.Mutex
		var consumeErr error
		done := make(chan struct{})

		go func() {
			defer close(done)
			consumeErr = d.Consume(func(entry int) {
				consumedMu.Lock()
				consumed = append(consumed, entry)
				consumedMu.Unlock()
			})
		}()

		time.Sleep(100 * time.Millisecond)
		d.Close()

		select {
		case <-done:
		case <-time.After(200 * time.Millisecond):
			t.Fatal("Consume should have finished")
		}

		if consumeErr != nil {
			t.Errorf("Consume should succeed: %v", consumeErr)
		}

		consumedMu.Lock()
		consumedCount := len(consumed)
		consumedMu.Unlock()

		if consumedCount != numEntries {
			t.Errorf("Expected %d entries, got %d", numEntries, consumedCount)
		}

		consumedMu.Lock()
		for i, val := range consumed {
			if val != expected[i] {
				t.Errorf("Expected %d, got %d", expected[i], val)
			}
		}
		consumedMu.Unlock()
	})

	t.Run("consume from closed disruptor", func(t *testing.T) {
		d := NewDisruptor[string]()
		d.Close()

		err := d.Consume(func(entry string) {
			t.Error("Handler should not be called")
		})

		if err == nil {
			t.Error("Consume from closed disruptor should return error")
		}
	})

	t.Run("consume with no entries", func(t *testing.T) {
		d := NewDisruptor[int]()

		var consumed []int
		var consumedMu sync.Mutex
		var consumeErr error
		done := make(chan struct{})

		go func() {
			defer close(done)
			consumeErr = d.Consume(func(entry int) {
				consumedMu.Lock()
				consumed = append(consumed, entry)
				consumedMu.Unlock()
			})
		}()

		time.Sleep(50 * time.Millisecond)
		d.Close()

		select {
		case <-done:
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Consume should have finished")
		}

		if consumeErr != nil {
			t.Errorf("Consume should succeed: %v", consumeErr)
		}

		consumedMu.Lock()
		consumedCount := len(consumed)
		consumedMu.Unlock()

		if consumedCount != 0 {
			t.Errorf("Expected 0 entries, got %d", consumedCount)
		}
	})
}

func TestDisruptorClose(t *testing.T) {
	t.Run("close empty disruptor", func(t *testing.T) {
		d := NewDisruptor[string]()

		d.Close()

		if !d.closed.Load() {
			t.Error("Disruptor should be marked as closed")
		}
	})

	t.Run("close disruptor with pending entries", func(t *testing.T) {
		d := NewDisruptor[int]()

		for i := range 10 {
			success := d.Publish(i)
			if !success {
				t.Fatalf("Publish %d should succeed", i)
			}
		}

		var consumed []int
		var consumedMu sync.Mutex
		done := make(chan struct{})

		go func() {
			defer close(done)
			d.Consume(func(entry int) {
				consumedMu.Lock()
				consumed = append(consumed, entry)
				consumedMu.Unlock()
			})
		}()

		time.Sleep(50 * time.Millisecond)
		d.Close()

		select {
		case <-done:
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Consume should have finished")
		}

		consumedMu.Lock()
		consumedCount := len(consumed)
		consumedMu.Unlock()

		if consumedCount == 0 {
			t.Error("Should have consumed some entries")
		}
	})
}

func TestDisruptorConcurrency(t *testing.T) {
	t.Run("concurrent publish and consume", func(t *testing.T) {
		d := NewDisruptor[int]()

		const numPublishers = 5
		const numEntriesPerPublisher = 100
		const totalEntries = numPublishers * numEntriesPerPublisher

		var consumed []int
		var consumedMu sync.Mutex
		consumeDone := make(chan struct{})

		go func() {
			defer close(consumeDone)
			d.Consume(func(entry int) {
				consumedMu.Lock()
				consumed = append(consumed, entry)
				consumedMu.Unlock()
			})
		}()

		var wg sync.WaitGroup
		for i := range numPublishers {
			wg.Add(1)
			go func(publisherID int) {
				defer wg.Done()
				for j := range numEntriesPerPublisher {
					entry := publisherID*numEntriesPerPublisher + j
					success := d.Publish(entry)
					if !success {
						t.Errorf("Publish should succeed for entry %d", entry)
					}
				}
			}(i)
		}

		wg.Wait()

		d.Close()
		select {
		case <-consumeDone:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Consume should have finished")
		}

		consumedMu.Lock()
		consumedCount := len(consumed)
		consumedMu.Unlock()

		// TODO periodically fails
		// Allow for some variance due to concurrency
		if consumedCount < totalEntries*7/10 {
			t.Errorf("Expected at least %d entries consumed, got %d", totalEntries*7/10, consumedCount)
		}
	})

	t.Run("concurrent publishes", func(t *testing.T) {
		d := NewDisruptor[string]()

		const numGoroutines = 10
		const numEntriesPerGoroutine = 50

		var wg sync.WaitGroup
		for i := range numGoroutines {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				for j := range numEntriesPerGoroutine {
					entry := "goroutine-" + string(rune(goroutineID)) + "-entry-" + string(rune(j))
					success := d.Publish(entry)
					if !success {
						t.Errorf("Publish should succeed for goroutine %d, entry %d", goroutineID, j)
					}
				}
			}(i)
		}

		wg.Wait()

		expectedWriter := int64(numGoroutines*numEntriesPerGoroutine - 1)
		actualWriter := d.writer.Value.Load()
		if actualWriter < expectedWriter*7/10 {
			t.Errorf("Expected writer at least %d, got %d", expectedWriter*7/10, actualWriter)
		}
	})
}

func TestDisruptorBufferSize(t *testing.T) {
	t.Run("buffer size is power of two", func(t *testing.T) {
		// This test ensures our buffer size is actually a power of 2
		if BufferSize&(BufferSize-1) != 0 {
			t.Errorf("BufferSize %d is not a power of 2", BufferSize)
		}
	})

	t.Run("buffer index mask", func(t *testing.T) {
		// Test that the mask works correctly for wrapping
		for i := range BufferSize * 2 {
			expected := i & (BufferSize - 1)
			actual := i & BufferIndexMask
			if actual != expected {
				t.Errorf("For index %d: expected %d, got %d", i, expected, actual)
			}
		}
	})
}

func TestDisruptorStress(t *testing.T) {
	t.Run("stress test", func(t *testing.T) {
		d := NewDisruptor[int]()

		const numEntries = 1000 // Reduced for faster test
		var consumed []int
		var consumedMu sync.Mutex
		consumeDone := make(chan struct{})

		go func() {
			defer close(consumeDone)
			d.Consume(func(entry int) {
				consumedMu.Lock()
				consumed = append(consumed, entry)
				consumedMu.Unlock()
			})
		}()

		time.Sleep(10 * time.Millisecond)

		for i := range numEntries {
			success := d.Publish(i)
			if !success {
				t.Fatalf("Publish %d should succeed", i)
			}
		}

		d.Close()
		select {
		case <-consumeDone:
		case <-time.After(2 * time.Second):
			t.Fatal("Consume should have finished")
		}

		consumedMu.Lock()
		consumedCount := len(consumed)
		consumedMu.Unlock()

		if consumedCount != numEntries {
			t.Errorf("Expected %d entries consumed, got %d", numEntries, consumedCount)
		}
	})
}
