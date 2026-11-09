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
	MinBufferSize         = 1 << 6
	disruptorClosed       = uint64(1) << 63
	disruptorSequenceMask = disruptorClosed - 1
)

// Represents an atomic sequence number with padding to prevent false sharing.
type sequence struct {
	Value atomic.Uint64
	// Padding to prevent false sharing on cache lines
	_ [56]byte
}

// Defines an interface for waiting on sequence values.
type seqWaiter interface {
	WaitFor(seq uint64, cursor *atomic.Uint64)
	WaitForChange(seq uint64, cursor *atomic.Uint64, done <-chan struct{})
}

// Implements a yielding wait strategy that sleeps briefly between checks.
type yieldSeqWait struct {
	waitFor time.Duration
}

func (w yieldSeqWait) WaitFor(seq uint64, cursor *atomic.Uint64) {
	for cursor.Load() != seq {
		time.Sleep(w.waitFor)
	}
}

func (w yieldSeqWait) WaitForChange(seq uint64, cursor *atomic.Uint64, done <-chan struct{}) {
	for cursor.Load() == seq {
		select {
		case <-done:
			return
		default:
			time.Sleep(w.waitFor)
		}
	}
}

// Disruptor is a ring buffer with lock-free publication and one consumer.
// Each reserved slot becomes readable only after its publisher marks it ready.
type Disruptor[T any] struct {
	buffer     []T
	available  []atomic.Uint64
	bufferMask uint64    // Size minus one, used for capacity and slot indices
	writer     sequence  // Reservation count plus shutdown bit
	reader     sequence  // Completed consumption count
	waiter     seqWaiter // Wait strategy for consumers
	done       chan struct{}
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
		available:  make([]atomic.Uint64, size),
		bufferMask: uint64(size - 1),
		done:       make(chan struct{}),
	}
	return d, nil
}

// Close signals shutdown without waiting for Consume to return.
// In-flight reservations may finish publishing afterward and are drained by Consume.
// Calling Close more than once panics. The Disruptor cannot be reused.
func (d *Disruptor[T]) Close() {
	for {
		writer := d.writer.Value.Load()
		if writer&disruptorClosed != 0 {
			panic("can't close an already closed Disruptor")
		}
		if d.writer.Value.CompareAndSwap(writer, writer|disruptorClosed) {
			close(d.done)
			return
		}
	}
}

// IsEmpty reports whether the reader and writer cursors are equal.
// It is a snapshot, not a barrier against concurrent publication.
func (d *Disruptor[T]) IsEmpty() bool {
	return d.writer.Value.Load()&disruptorSequenceMask == d.reader.Value.Load()
}

// Attempts to add an entry to the disruptor buffer.
// Returns false if buffer is full or if Disruptor is closed.
// Publication reserves a slot with CAS, then publishes its payload without allocation.
func (d *Disruptor[T]) Publish(entry T) bool {
	for {
		writer := d.writer.Value.Load()
		if writer&disruptorClosed != 0 {
			return false
		}
		if (writer-d.reader.Value.Load())&disruptorSequenceMask >= d.bufferMask {
			if d.writer.Value.Load() != writer {
				continue
			}
			return false
		}
		nextWriter := (writer + 1) & disruptorSequenceMask
		if d.writer.Value.CompareAndSwap(writer, nextWriter) {
			index := writer & d.bufferMask
			d.buffer[index] = entry
			d.available[index].Store(nextWriter)
			return true
		}
	}
}

// Starts consuming entries from the disruptor using the provided handler.
// Consume blocks, calls handler synchronously, and drains accepted entries on Close.
// It returns an error if already closed. Only one consumer may be used.
func (d *Disruptor[T]) Consume(handler func(T)) error {
	return d.ConsumeWithReady(handler, nil)
}

// ConsumeWithReady signals readiness after entering the consumer, before processing entries.
// ready may be nil; otherwise it must return promptly. Only one consumer may be used.
func (d *Disruptor[T]) ConsumeWithReady(handler func(T), ready func()) error {
	if d.writer.Value.Load()&disruptorClosed != 0 {
		return fmt.Errorf("can't start canceled Disruptor")
	}
	if ready != nil {
		ready()
	}

	reader := d.reader.Value.Load()

	for {
		writer := d.writer.Value.Load()
		limit := writer & disruptorSequenceMask
		if reader == limit {
			if writer&disruptorClosed != 0 {
				return nil
			}
			d.waiter.WaitForChange(writer, &d.writer.Value, d.done)
			continue
		}
		for reader != limit {
			index := reader & d.bufferMask
			nextReader := (reader + 1) & disruptorSequenceMask
			// Close stops reservations; it must not skip a publisher still writing its slot.
			d.waiter.WaitFor(nextReader, &d.available[index])
			handler(d.buffer[index])
			reader = nextReader
			d.reader.Value.Store(reader)
		}
	}
}
