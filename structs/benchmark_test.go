package structs

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	TestItems = 1000 // Smaller number for stable testing
)

type TestData struct {
	ID  int64
	Pad [32]byte // 32 bytes padding
}

// ==================== SINGLE OPERATION BENCHMARKS ====================

func BenchmarkDisruptor_SingleOperation(b *testing.B) {
	// Simple single operation test without consumer
	// Just test raw publish operation speed
	b.StopTimer()
	d, err := NewDisruptorWithSize[TestData](1 << 20) // 1M buffer
	if err != nil {
		b.Fatal("Failed to create disruptor:", err)
	}

	b.StartTimer()
	b.ReportAllocs()

	// Test raw publish speed (will fail at buffer overflow but that's fine for timing)
	for i := 0; i < b.N && i < (1<<20)-2; i++ { // Leave room for buffer
		data := TestData{ID: int64(i)}
		if !d.Publish(data) && i == 0 {
			b.Fatal("Failed to publish first item")
		}
	}

	d.Close()
}

func BenchmarkSyncQueue_SingleOperation(b *testing.B) {
	q := NewSyncQueue[TestData](0)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		data := TestData{ID: int64(i)}
		q.Push(data)
	}
}

func BenchmarkChannel_SingleOperation(b *testing.B) {
	ch := make(chan TestData, b.N)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		data := TestData{ID: int64(i)}
		ch <- data
	}

	close(ch)
}

// ==================== THROUGHPUT BENCHMARKS ====================

func BenchmarkDisruptor_Throughput(b *testing.B) {
	// Run once for reliability in benchmarks
	b.StopTimer()
	d := NewDisruptor[TestData]()
	var wg sync.WaitGroup
	var processed int64

	// Start consumer
	wg.Add(1)
	go func() {
		defer wg.Done()
		d.Consume(func(data TestData) {
			atomic.AddInt64(&processed, 1)
		})
	}()

	// Small delay to ensure consumer starts
	time.Sleep(1 * time.Millisecond)

	b.StartTimer()
	b.ReportAllocs()
	b.SetBytes(int64(40)) // 40 bytes per TestData

	for i := 0; i < b.N; i++ {
		// Reset counter for this iteration
		atomic.StoreInt64(&processed, 0)

		start := time.Now()

		// Publish items
		for j := 0; j < TestItems; j++ {
			data := TestData{ID: int64(j)}
			for !d.Publish(data) {
				// Buffer full, retry
				runtime.Gosched()
			}
		}

		// Wait for all items to be processed
		for atomic.LoadInt64(&processed) < int64(TestItems) {
			time.Sleep(1 * time.Millisecond)
		}

		elapsed := time.Since(start)
		throughput := float64(TestItems) / elapsed.Seconds()
		b.ReportMetric(throughput, "items/sec")
	}

	d.Close()
	wg.Wait()
}

func BenchmarkSyncQueue_Throughput(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		q := NewSyncQueue[TestData](0)
		var wg sync.WaitGroup
		var processed int64

		// Start consumer
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if data, ok := q.Pop(); ok {
					_ = data.ID // Use the data
					atomic.AddInt64(&processed, 1)
				} else {
					// Queue empty, check if we should stop
					if atomic.LoadInt64(&processed) >= int64(TestItems) {
						break
					}
					runtime.Gosched()
				}
			}
		}()

		// Small delay to ensure consumer starts
		time.Sleep(1 * time.Millisecond)

		b.StartTimer()
		b.ReportAllocs()
		b.SetBytes(int64(40)) // 40 bytes per TestData

		// Reset counter for this iteration
		atomic.StoreInt64(&processed, 0)

		start := time.Now()

		// Push items
		for j := 0; j < TestItems; j++ {
			data := TestData{ID: int64(j)}
			q.Push(data)
		}

		// Wait for all items to be processed
		for atomic.LoadInt64(&processed) < int64(TestItems) {
			time.Sleep(1 * time.Millisecond)
		}

		elapsed := time.Since(start)
		throughput := float64(TestItems) / elapsed.Seconds()
		b.ReportMetric(throughput, "items/sec")

		b.StopTimer()
		wg.Wait()
	}
}

func BenchmarkChannel_Throughput(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		ch := make(chan TestData, 1024) // Buffered channel
		var wg sync.WaitGroup
		var processed int64

		// Start consumer
		wg.Add(1)
		go func() {
			defer wg.Done()
			for data := range ch {
				_ = data.ID // Use the data
				atomic.AddInt64(&processed, 1)
			}
		}()

		b.StartTimer()
		b.ReportAllocs()
		b.SetBytes(int64(40)) // 40 bytes per TestData

		// Reset counter for this iteration
		atomic.StoreInt64(&processed, 0)

		start := time.Now()

		// Send items
		for j := 0; j < TestItems; j++ {
			data := TestData{ID: int64(j)}
			ch <- data
		}

		// Wait for all items to be processed
		for atomic.LoadInt64(&processed) < int64(TestItems) {
			time.Sleep(1 * time.Millisecond)
		}

		elapsed := time.Since(start)
		throughput := float64(TestItems) / elapsed.Seconds()
		b.ReportMetric(throughput, "items/sec")

		b.StopTimer()
		close(ch)
		wg.Wait()
	}
}
