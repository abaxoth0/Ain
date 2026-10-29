package common

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryStopsAfterExhaustion(t *testing.T) {
	for _, extraSucceeds := range []bool{false, true} {
		name := "extra failure"
		if extraSucceeds {
			name = "extra success"
		}
		t.Run(name, func(t *testing.T) {
			failure := errors.New("operation failed")
			var calls atomic.Int32
			extra := make(chan struct{}, 1)
			err := Retry(func() error {
				if calls.Add(1) > 2 {
					extra <- struct{}{}
					if extraSucceeds {
						return nil
					}
				}
				return failure
			}, &RetryConfig{MaxAttempts: 2, MaxBackoff: time.Millisecond})
			if err != failure {
				t.Fatalf("returned %v, want the final operation error", err)
			}
			select {
			case <-extra:
				t.Error("callback executed beyond the attempt limit")
			case <-time.After(30 * time.Millisecond):
			}
			if count := calls.Load(); count != 2 {
				t.Errorf("callback executed %d times, want 2", count)
			}
			stacks := make([]byte, 1<<20)
			n := runtime.Stack(stacks, true)
			if strings.Contains(string(stacks[:n]), "github.com/abaxoth0/Ain/common.Retry.func") {
				t.Error("retry goroutine remains after exhaustion")
			}
		})
	}
}

func TestRetryAttemptLimits(t *testing.T) {
	for _, tc := range []struct {
		name      string
		limit     int
		succeedAt int
		wantCalls int
		wantError bool
	}{
		{"initial success", 3, 1, 1, false},
		{"one failed attempt", 1, 0, 1, true},
		{"success at limit", 3, 3, 3, false},
		{"exhaustion", 3, 0, 3, true},
		{"unlimited zero", 0, 5, 5, false},
		{"unlimited negative", -1, 5, 5, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			err := Retry(func() error {
				if int(calls.Add(1)) == tc.succeedAt {
					return nil
				}
				return errors.New("failed")
			}, &RetryConfig{MaxAttempts: tc.limit})
			if (err != nil) != tc.wantError {
				t.Errorf("returned error %v, want error=%v", err, tc.wantError)
			}
			if count := int(calls.Load()); count != tc.wantCalls {
				t.Errorf("callback executed %d times, want %d", count, tc.wantCalls)
			}
		})
	}
}

func TestRetryCallbackPanicPropagates(t *testing.T) {
	for _, panicAt := range []int{1, 2} {
		t.Run(fmt.Sprint(panicAt), func(t *testing.T) {
			failure := errors.New("callback panic")
			defer func() {
				if recovered := recover(); recovered != failure {
					t.Errorf("recovered %v, want callback panic", recovered)
				}
			}()
			calls := 0
			_ = Retry(func() error {
				calls++
				if calls == panicAt {
					panic(failure)
				}
				return errors.New("retry")
			}, &RetryConfig{MaxAttempts: 2})
		})
	}
}
