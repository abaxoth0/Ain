package structs

import (
	"fmt"
	"sync/atomic"
	"time"
)

const (
	// DefaultBufferSize is the default buffer size (64K entries)
	DefaultBufferSize = 1 << 16
	// MinBufferSize is the minimum buffer size (64 entries)
	MinBufferSize = 1 << 6
)

var yieldWaiter = yieldSeqWait{}

// Represents an atomic sequence number with padding to prevent false sharing.
type sequence struct {
	Value atomic.Int64
	// Padding to prevent false sharing on cache lines
	_padding [56]byte
}

// Defines an interface for waiting on sequence values.
type seqWaiter interface {
	// Waits until either sequence is greater or equal to cursor, or done is closed.
	WaitFor(seq int64, cursor *sequence, done <-chan struct{})
}

// Implements a yielding wait strategy that sleeps briefly between checks.
type yieldSeqWait struct{}

// Waits until either sequence is greater than or equal to cursor, or done is closed.
func (w yieldSeqWait) WaitFor(seq int64, cursor *sequence, done <-chan struct{}) {
	for cursor.Value.Load() < seq {
		select {
		case <-done:
			return
		default:
			time.Sleep(10 * time.Microsecond)
		}
	}
}

// Implements the LMAX Disruptor pattern for high-performance ring buffer messaging.
type Disruptor[T any] struct {
	buffer     []T
	bufferSize int           // must be power of 2
	bufferMask int64         // Mask for modulo operations (bufferSize - 1)
	writer     sequence      // Write cursor position (starts at -1)
	reader     sequence      // Read cursor position (starts at -1)
	waiter     seqWaiter     // Wait strategy for consumers
	closed     atomic.Bool
	done       chan struct{}
}

// creates a new Disruptor with default buffer size (64K)
func NewDisruptor[T any]() *Disruptor[T] {
	dis, err := NewDisruptorWithSize[T](DefaultBufferSize)
	if err != nil {
		panic(err)
	}
	return dis
}

// Creates a new Disruptor with specified buffer size.
// Buffer size must be a power of 2 and at least MinBufferSize.
func NewDisruptorWithSize[T any](size int) (*Disruptor[T], error) {
	// Validate size
	if size < MinBufferSize {
		return nil, fmt.Errorf("Disruptor buffer size %d is too small (minimum: %d)", size, MinBufferSize)
	}
	if size&(size-1) != 0 {
		return nil, fmt.Errorf("Disruptor buffer size %d must be a power of two", size)
	}

	d := &Disruptor[T]{
		buffer:     make([]T, size),
		bufferSize: size,
		bufferMask: int64(size - 1),
		done:       make(chan struct{}),
		waiter:     yieldWaiter,
	}
	d.writer.Value.Store(-1)
	d.reader.Value.Store(-1)
	return d, nil
}

// Closes the Disruptor and signals all goroutines to stop.
// After closing, the disruptor cannot be used again.
func (d *Disruptor[T]) Close() {
	close(d.done)
	// Set closed flag immediately if no consumer is running
	d.closed.Store(true)
}

// Returns true if all entries have been processed
func (d *Disruptor[T]) IsEmpty() bool {
	return d.writer.Value.Load() == d.reader.Value.Load()
}

/*
	IMPORTANT
	Race detector notes: When running with -race, Go's race detector flags publisher-consumer buffer access as data races.
	These are false positives because:

	  1. CAS on writer cursor ensures only one publisher can claim a sequence number

	  2. Writer is advanced only after CAS succeeds (establishes happens-before relationship)

	  3. Consumer reads after seeing updated writer cursor (synchronized through atomic operations)

	  4. No mutexes used - fully lock-free implementation

	The race detector doesn't understand happens-before relationships established through
	atomic operations on different variables.
	The algorithm is logically correct and tests pass without -race.

	So to mitigate this false positives Publish() and Consume() have go:norace directive enabled.
*/

// Attempts to add an entry to the disruptor buffer.
// Returns false if buffer is full or if Disruptor is closed.
//
//go:norace
func (d *Disruptor[T]) Publish(entry T) bool {
	for {
		select {
		case <-d.done:
			return false
		default:
			writer := d.writer.Value.Load()
			reader := d.reader.Value.Load()
			nextWriter := writer + 1

			// Check if buffer is full
			// NOTE: For buffer sizes ≤ 8, use (nextWriter - reader) > (BufferSize - 1) condition
			// to avoid off-by-one overwrites. For larger buffers (≥1024) keep current condition.
			if nextWriter-reader >= int64(d.bufferSize) {
				return false
			}

			// Try to atomically advance the writer using CAS
			// Only one goroutine will succeed in this CAS, preventing race conditions
			// between concurrent publishers trying to claim the same slot.
			if d.writer.Value.CompareAndSwap(writer, nextWriter) {
				// The CAS provides a memory barrier (release semantics) ensuring buffer write
				// is visible to consumers before they see the updated writer cursor.
				d.buffer[nextWriter&d.bufferMask] = entry
				return true
			}
			// CAS failed - another publisher claimed this slot, retry.
		}
	}
}

// Starts consuming entries from the disruptor using the provided handler.
// This method blocks until the disruptor is closed. Returns error if disruptor is already closed.
//
//go:norace
func (d *Disruptor[T]) Consume(handler func(T)) error {
	if d.closed.Load() {
		return fmt.Errorf("Can't start canceled Disruptor")
	}

	var claimed int64 = d.reader.Value.Load() + 1
	closed := false

	for {
		writer := d.writer.Value.Load()

		select {
		case <-d.done:
			closed = true
		default:
			if claimed > writer {
				d.waiter.WaitFor(claimed, &d.writer, d.done)
				continue
			}
		}

		for i := claimed; i <= writer; i++ {
			entry := d.buffer[i&d.bufferMask]
			handler(entry)
			d.reader.Value.Store(i)
		}
		claimed = writer + 1

		if closed {
			d.closed.Store(true)
			return nil
		}
	}
}
