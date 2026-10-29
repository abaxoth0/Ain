package common

import (
	"math"
	"math/rand"
	"time"
)

// Defines the retry behavior for operations that may fail temporarily.
//
// About backoff and why it's even needed (The Thundering Herd Problem):
// When multiple clients retry failed operations simultaneously without coordination,
// they can overwhelm the recovering service, causing a cascade of failures. This is
// known as the "thundering herd" problem.
//
// Example: A database goes down for maintenance. When it comes back online,
// 1000 clients retrying every second will immediately overwhelm it, potentially
// causing it to crash again.
//
// How backoff solves this: Exponential backoff with jitter spreads retry attempts over time.
//
// - Exponential component: 2^n ensures longer delays as failures persist
//
// - Jitter: Randomization prevents synchronized retry storms
//
// - Max backoff: Caps delay to reasonable limits
//
// This gives the service time to recover and prevents retry storms.
type RetryConfig struct {
	// Maximum number of retry attempts.
	// <= 0 means unlimited retries (use with caution)
	// 1 means no retries (only initial attempt)
	MaxAttempts int

	// Maximum delay between retry attempts.
	// Prevents excessively long waits during extended outages.
	// 0 currently clamps the delay to zero.
	MaxBackoff time.Duration

	// Base delay for exponential backoff calculation.
	// The actual delay is: 2^attempts * BackoffScale + jitter
	// Default: 1 millisecond if not specified
	BackoffScale time.Duration
}

// Retry executes a function with configurable retry logic using exponential backoff with jitter.
// Returns the last error encountered if all attempts fail, or nil if successful.
//
// The initial call runs in the caller; retries run in a goroutine while the caller
// blocks for the result. Use a separate configuration for concurrent calls, since
// a zero BackoffScale may be replaced with the default.
// At the attempt limit, the retry goroutine sends an error but continues running.
// It can call fn after Retry returns and block forever sending another result.
//
// Backoff Algorithm:
// delay = 2^attempts * BackoffScale + random jitter(0-10ms)
//
// - Exponential component increases delay with each failure
// - Jitter prevents synchronized retry attempts across multiple clients
// - MaxBackoff caps the delay to prevent excessive waits
func Retry(fn func() error, cfg *RetryConfig) error {
	// Handle nil config with sensible defaults
	if cfg == nil {
		cfg = &RetryConfig{
			MaxAttempts:  3,
			MaxBackoff:   time.Second * 5,
			BackoffScale: time.Millisecond,
		}
	}

	err := fn()
	if err == nil {
		return nil
	}
	if cfg.MaxAttempts == 1 {
		return err
	}
	if cfg.BackoffScale == 0 {
		cfg.BackoffScale = time.Millisecond
	}

	errChan := make(chan error)
	attempts := 1

	go func() {
		for err != nil {
			if cfg.MaxAttempts > 0 && attempts >= cfg.MaxAttempts {
				errChan <- err
			}

			backoff := time.Duration(math.Pow(2, float64(attempts))) * cfg.BackoffScale
			backoff += time.Duration(rand.Intn(10)) * time.Millisecond // #nosec G404 -- Retry jitter does not require cryptographic randomness.
			if backoff > cfg.MaxBackoff {
				backoff = cfg.MaxBackoff
			}

			time.Sleep(backoff)

			err = fn()
			attempts++
		}
		errChan <- err
	}()

	return <-errChan
}
