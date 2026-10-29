package structs

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// DefaultBufferSize is the default buffer size (64K entries)
	DefaultBufferSize = 1 << 16
	// MinBufferSize is the minimum buffer size (64 entries)
	MinBufferSize = 1 << 6
)

// Represents an atomic sequence number with padding to prevent false sharing.
type sequence struct {
	Value atomic.Int64
	// Padding to prevent false sharing on cache lines
	_ [56]byte
}

// Defines an interface for waiting on sequence values.
type seqWaiter interface {
	// Waits until either sequence is greater or equal to cursor, or done is closed.
	WaitFor(seq int64, cursor *sequence, done <-chan struct{})
}

// Implements a yielding wait strategy that sleeps briefly between checks.
type yieldSeqWait struct {
	waitFor time.Duration
}

// Waits until either sequence is greater than or equal to cursor, or done is closed.
func (w yieldSeqWait) WaitFor(seq int64, cursor *sequence, done <-chan struct{}) {
	for cursor.Value.Load() < seq {
		select {
		case <-done:
			return
		default:
			time.Sleep(w.waitFor)
		}
	}
}

// Disruptor is a ring buffer supporting concurrent publishers and one consumer.
// Publishers serialize slot writes before making them visible to the consumer.
type Disruptor[T any] struct {
	buffer     []T
	bufferSize int       // must be power of 2
	bufferMask int64     // Mask for modulo operations (bufferSize - 1)
	writer     sequence  // Write cursor position (starts at -1)
	reader     sequence  // Read cursor position (starts at -1)
	waiter     seqWaiter // Wait strategy for consumers
	closed     atomic.Bool
	done       chan struct{}
	publishMu  sync.Mutex
}

type DisruptorConfig struct {
	// Determines how often Consumer will wake up and check for updates.
	IdleCheckInterval time.Duration
}

const (
	DisruptorDefaultIdleCheckInterval = 10 * time.Millisecond
)

var defaultDisruptorConfig = &DisruptorConfig{
	IdleCheckInterval: DisruptorDefaultIdleCheckInterval,
}

// creates a new Disruptor with default buffer size (64K)
func NewDisruptor[T any](conf *DisruptorConfig) *Disruptor[T] {
	dis, err := NewDisruptorWithSize[T](DefaultBufferSize, conf)
	if err != nil {
		panic(err)
	}
	return dis
}

// Creates a new Disruptor with specified buffer size.
// Buffer size must be a power of 2 and at least MinBufferSize.
func NewDisruptorWithSize[T any](size int, conf *DisruptorConfig) (*Disruptor[T], error) {
	// Validate size
	if size < MinBufferSize {
		return nil, fmt.Errorf("Disruptor buffer size %d is too small (minimum: %d)", size, MinBufferSize)
	}
	if size&(size-1) != 0 {
		return nil, fmt.Errorf("Disruptor buffer size %d must be a power of two", size)
	}

	if conf == nil {
		conf = defaultDisruptorConfig
	}

	d := &Disruptor[T]{
		waiter: yieldSeqWait{
			waitFor: conf.IdleCheckInterval,
		},
		buffer:     make([]T, size),
		bufferSize: size,
		bufferMask: int64(size - 1),
		done:       make(chan struct{}),
	}
	d.writer.Value.Store(-1)
	d.reader.Value.Store(-1)
	return d, nil
}

// Close signals shutdown without waiting for Consume to return.
// Calling Close more than once panics. The Disruptor cannot be reused.
func (d *Disruptor[T]) Close() {
	d.publishMu.Lock()
	defer d.publishMu.Unlock()
	d.closed.Store(true)
	close(d.done)
}

// IsEmpty reports whether the reader and writer cursors are equal.
// It is a snapshot, not a barrier against concurrent publication.
func (d *Disruptor[T]) IsEmpty() bool {
	return d.writer.Value.Load() == d.reader.Value.Load()
}

// Attempts to add an entry to the disruptor buffer.
// Returns false if buffer is full or if Disruptor is closed.
func (d *Disruptor[T]) Publish(entry T) bool {
	d.publishMu.Lock()
	defer d.publishMu.Unlock()
	if d.closed.Load() {
		return false
	}
	nextWriter := d.writer.Value.Load() + 1
	if nextWriter-d.reader.Value.Load() >= int64(d.bufferSize) {
		return false
	}
	d.buffer[nextWriter&d.bufferMask] = entry
	d.writer.Value.Store(nextWriter)
	return true
}

// Starts consuming entries from the disruptor using the provided handler.
// Consume blocks, calls handler synchronously, and drains accepted entries on Close.
// It returns an error if already closed. Only one consumer may be used.
func (d *Disruptor[T]) Consume(handler func(T)) error {
	if d.closed.Load() {
		return fmt.Errorf("can't start canceled Disruptor")
	}

	claimed := d.reader.Value.Load() + 1
	closed := false

	for {
		select {
		case <-d.done:
			closed = true
		default:
		}

		// Read after observing shutdown so the final snapshot includes every publication.
		writer := d.writer.Value.Load()
		if !closed && claimed > writer {
			d.waiter.WaitFor(claimed, &d.writer, d.done)
			continue
		}

		for i := claimed; i <= writer; i++ {
			entry := d.buffer[i&d.bufferMask]
			handler(entry)
			d.reader.Value.Store(i)
		}
		claimed = writer + 1

		if closed {
			return nil
		}
	}
}
