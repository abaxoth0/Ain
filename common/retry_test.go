package common

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// TestRetry_Success tests that successful functions return immediately
func TestRetry_Success(t *testing.T) {
	var callCount int64
	var mu sync.Mutex
	fn := func() error {
		mu.Lock()
		callCount++
		mu.Unlock()
		return nil
	}

	config := &RetryConfig{
		MaxAttempts:  3,
		MaxBackoff:   time.Second,
		BackoffScale: time.Millisecond,
	}

	err := Retry(fn, config)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	mu.Lock()
	finalCount := callCount
	mu.Unlock()
	if finalCount != 1 {
		t.Errorf("Expected 1 call, got %d", finalCount)
	}
}

// TestRetry_FailureMaxAttempts tests that retry stops after max attempts
func TestRetry_FailureMaxAttempts(t *testing.T) {
	var callCount int64
	var mu sync.Mutex
	fn := func() error {
		mu.Lock()
		callCount++
		mu.Unlock()
		return errors.New("always fails")
	}

	config := &RetryConfig{
		MaxAttempts:  3,
		MaxBackoff:   time.Second,
		BackoffScale: time.Millisecond,
	}

	start := time.Now()
	err := Retry(fn, config)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Expected error, got nil")
	}
	mu.Lock()
	finalCount := callCount
	mu.Unlock()
	if finalCount != 3 {
		t.Errorf("Expected 3 calls, got %d", finalCount)
	}
	// Should take at least some time due to backoff
	if elapsed < time.Millisecond*2 {
		t.Errorf("Expected some delay due to backoff, got %v", elapsed)
	}
}

// TestRetry_EventualSuccess tests that retry succeeds after initial failures
func TestRetry_EventualSuccess(t *testing.T) {
	var callCount int64
	var mu sync.Mutex
	fn := func() error {
		mu.Lock()
		callCount++
		currentCount := callCount
		mu.Unlock()
		if currentCount < 3 {
			return errors.New("temporary failure")
		}
		return nil
	}

	config := &RetryConfig{
		MaxAttempts:  5,
		MaxBackoff:   time.Second,
		BackoffScale: time.Millisecond,
	}

	err := Retry(fn, config)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	mu.Lock()
	finalCount := callCount
	mu.Unlock()
	if finalCount != 3 {
		t.Errorf("Expected 3 calls, got %d", finalCount)
	}
}

// TestRetry_NoRetries tests that MaxAttempts=1 means no retries
func TestRetry_NoRetries(t *testing.T) {
	var callCount int64
	var mu sync.Mutex
	fn := func() error {
		mu.Lock()
		callCount++
		mu.Unlock()
		return errors.New("always fails")
	}

	config := &RetryConfig{
		MaxAttempts:  1,
		MaxBackoff:   time.Second,
		BackoffScale: time.Millisecond,
	}

	start := time.Now()
	err := Retry(fn, config)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Expected error, got nil")
	}
	mu.Lock()
	finalCount := callCount
	mu.Unlock()
	if finalCount != 1 {
		t.Errorf("Expected 1 call, got %d", finalCount)
	}
	// Should be very fast since no retries
	if elapsed > time.Millisecond*10 {
		t.Errorf("Expected fast execution, took %v", elapsed)
	}
}

// TestRetry_UnlimitedRetries tests that MaxAttempts=0 allows unlimited retries
func TestRetry_UnlimitedRetries(t *testing.T) {
	var callCount int64
	var mu sync.Mutex
	fn := func() error {
		mu.Lock()
		callCount++
		currentCount := callCount
		mu.Unlock()
		if currentCount < 5 {
			return errors.New("temporary failure")
		}
		return nil
	}

	config := &RetryConfig{
		MaxAttempts:  0, // Unlimited
		MaxBackoff:   time.Second,
		BackoffScale: time.Millisecond,
	}

	err := Retry(fn, config)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	mu.Lock()
	finalCount := callCount
	mu.Unlock()
	if finalCount != 5 {
		t.Errorf("Expected 5 calls, got %d", finalCount)
	}
}

// TestRetry_DefaultBackoffScale tests that default backoff scale is applied
func TestRetry_DefaultBackoffScale(t *testing.T) {
	var callCount int64
	var mu sync.Mutex
	var delays []time.Duration
	var lastCall time.Time

	fn := func() error {
		mu.Lock()
		callCount++
		currentCount := callCount
		if callCount > 1 {
			delays = append(delays, time.Since(lastCall))
		}
		lastCall = time.Now()
		mu.Unlock()
		if currentCount < 3 {
			return errors.New("temporary failure")
		}
		return nil
	}

	config := &RetryConfig{
		MaxAttempts:  3,
		MaxBackoff:   time.Second,
		BackoffScale: 0, // Should default to 1ms
	}

	err := Retry(fn, config)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	mu.Lock()
	finalDelays := make([]time.Duration, len(delays))
	copy(finalDelays, delays)
	mu.Unlock()
	if len(finalDelays) != 2 {
		t.Errorf("Expected 2 delays, got %d", len(finalDelays))
	}
	// First retry should be around 2ms (2^1 * 1ms + jitter)
	// Second retry should be around 4ms (2^2 * 1ms + jitter)
	if finalDelays[0] < time.Millisecond || finalDelays[0] > time.Millisecond*20 {
		t.Errorf("First retry delay unexpected: %v", finalDelays[0])
	}
	if finalDelays[1] < time.Millisecond*2 || finalDelays[1] > time.Millisecond*30 {
		t.Errorf("Second retry delay unexpected: %v", finalDelays[1])
	}
}

// TestRetry_MaxBackoffCapping tests that max backoff limits delay
func TestRetry_MaxBackoffCapping(t *testing.T) {
	var callCount int64
	var mu sync.Mutex
	var delays []time.Duration
	var lastCall time.Time

	fn := func() error {
		mu.Lock()
		callCount++
		currentCount := callCount
		if callCount > 1 {
			delays = append(delays, time.Since(lastCall))
		}
		lastCall = time.Now()
		mu.Unlock()
		if currentCount < 4 {
			return errors.New("temporary failure")
		}
		return nil
	}

	config := &RetryConfig{
		MaxAttempts:  4,
		MaxBackoff:   time.Millisecond * 5,  // Low max backoff
		BackoffScale: time.Millisecond * 10, // High scale would normally create longer delays
	}

	err := Retry(fn, config)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	mu.Lock()
	finalDelays := make([]time.Duration, len(delays))
	copy(finalDelays, delays)
	mu.Unlock()
	if len(finalDelays) != 3 {
		t.Errorf("Expected 3 delays, got %d", len(finalDelays))
	}
	// All delays should be capped at 5ms + jitter
	for i, delay := range finalDelays {
		if delay > time.Millisecond*15 { // 5ms + up to 10ms jitter
			t.Errorf("Delay %d not capped properly: %v", i, delay)
		}
	}
}

// TestRetry_ConcurrentExecution tests that retry works correctly with multiple goroutines
func TestRetry_ConcurrentExecution(t *testing.T) {
	var wg sync.WaitGroup
	resultErrors := make(chan error, 10)

	fn := func(id int) func() error {
		return func() error {
			return errors.New("fails")
		}
	}

	config := &RetryConfig{
		MaxAttempts:  2,
		MaxBackoff:   time.Millisecond * 10,
		BackoffScale: time.Millisecond,
	}

	// Run multiple retries concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := Retry(fn(id), config)
			if err == nil {
				resultErrors <- nil
			} else {
				resultErrors <- err
			}
		}(i)
	}

	wg.Wait()
	close(resultErrors)

	errorCount := 0
	for err := range resultErrors {
		if err != nil {
			errorCount++
		}
	}

	if errorCount != 10 {
		t.Errorf("Expected 10 errors, got %d", errorCount)
	}
}

// TestRetry_JitterPreventsSynchronization tests that jitter prevents synchronized retries
func TestRetry_JitterPreventsSynchronization(t *testing.T) {
	var wg sync.WaitGroup
	callTimes := make([]time.Time, 0, 20)
	var mu sync.Mutex

	fn := func() error {
		mu.Lock()
		callTimes = append(callTimes, time.Now())
		mu.Unlock()
		return errors.New("always fails")
	}

	config := &RetryConfig{
		MaxAttempts:  3,
		MaxBackoff:   time.Millisecond * 100,
		BackoffScale: time.Millisecond * 10,
	}

	// Start multiple goroutines at the same time
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			Retry(fn, config)
		}()
	}

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()

	if len(callTimes) != 15 { // 5 goroutines * 3 attempts each
		t.Errorf("Expected 15 total calls, got %d", len(callTimes))
	}

	// Check that calls are spread out over time (not all instantaneous)
	// This is a simple check - jitter should add some variation
	firstBatch := callTimes[:5]
	secondBatch := callTimes[5:10]

	// Second batch should be significantly later than first (exponential backoff)
	if !secondBatch[0].After(firstBatch[0].Add(time.Millisecond * 5)) {
		t.Error("Exponential backoff may not be working")
	}

	// Check that there's some variation in timing within the first batch
	// (not all calls happened at exactly the same time)
	minTime := firstBatch[0]
	maxTime := firstBatch[0]
	for _, callTime := range firstBatch {
		if callTime.Before(minTime) {
			minTime = callTime
		}
		if callTime.After(maxTime) {
			maxTime = callTime
		}
	}

	// If all calls happened within 1ms, they might be too synchronized
	if maxTime.Sub(minTime) < time.Millisecond {
		t.Logf("Warning: calls may be too synchronized (all within %v)", maxTime.Sub(minTime))
	}
}

// TestRetry_NilConfig tests behavior with nil config
func TestRetry_NilConfig(t *testing.T) {
	var callCount int64
	var mu sync.Mutex
	fn := func() error {
		mu.Lock()
		callCount++
		currentCount := callCount
		mu.Unlock()
		if currentCount < 2 {
			return errors.New("temporary failure")
		}
		return nil
	}

	// This should handle nil gracefully with defaults
	err := Retry(fn, nil)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	mu.Lock()
	finalCount := callCount
	mu.Unlock()
	if finalCount != 2 {
		t.Errorf("Expected 2 calls, got %d", finalCount)
	}
}

// BenchmarkRetry_Success benchmarks the retry function with immediate success
func BenchmarkRetry_Success(b *testing.B) {
	fn := func() error {
		return nil
	}

	config := &RetryConfig{
		MaxAttempts:  3,
		MaxBackoff:   time.Second,
		BackoffScale: time.Millisecond,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Retry(fn, config)
	}
}

// BenchmarkRetry_WithRetries benchmarks the retry function with retries
func BenchmarkRetry_WithRetries(b *testing.B) {
	var callCount int64
	var mu sync.Mutex
	fn := func() error {
		mu.Lock()
		callCount++
		currentCount := callCount
		mu.Unlock()
		if currentCount < 3 {
			return errors.New("temporary failure")
		}
		return nil
	}

	config := &RetryConfig{
		MaxAttempts:  5,
		MaxBackoff:   time.Millisecond * 10,
		BackoffScale: time.Millisecond,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mu.Lock()
		callCount = 0 // Reset for each iteration
		mu.Unlock()
		Retry(fn, config)
	}
}
