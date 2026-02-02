package structs

import (
	"fmt"
	"sync/atomic"
	"time"
)

const (
	BufferSize      = 1 << 16 // Must be a power of 2 for correct indexing
	BufferIndexMask = BufferSize - 1
)

var yieldWaiter = yieldSeqWait{}

type sequence struct {
	Value atomic.Int64
	// Padding to prevent false sharing
	_padding [56]byte
}

type seqWaiter interface {
	WaitFor(seq int64, cursor *sequence, done <-chan struct{})
}

type yieldSeqWait struct{}

// Waits till either sequence is greater or equal cursor, either done is closed
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

// Implements LMAX Disruptor
type Disruptor[T any] struct {
	buffer [BufferSize]T
	writer sequence // write position (starts at -1)
	reader sequence // read position (starts at -1)
	waiter seqWaiter
	closed atomic.Bool
	done   chan struct{}
}

func NewDisruptor[T any]() *Disruptor[T] {
	// If is not power of two
	if BufferSize&(BufferSize-1) != 0 {
		panic(fmt.Sprintf("invalid disruptor buffer size - %d: it must be a power of two", BufferSize))
	}

	d := &Disruptor[T]{
		done:   make(chan struct{}),
		waiter: yieldWaiter,
	}
	d.writer.Value.Store(-1)
	d.reader.Value.Store(-1)
	return d
}

// Closes Disruptor, after that it can't be started again.
func (d *Disruptor[T]) Close() {
	close(d.done)
	// Set closed flag immediately if no consumer is running
	d.closed.Store(true)
}

// IsEmpty returns true if all entries have been processed
func (d *Disruptor[T]) IsEmpty() bool {
	return d.writer.Value.Load() == d.reader.Value.Load()
}

// Returns false if buffer is overflowed or if Disruptor is closed
func (d *Disruptor[T]) Publish(entry T) bool {
	select {
	case <-d.done:
		return false
	default:
		writer := d.writer.Value.Load()
		reader := d.reader.Value.Load()
		nextWriter := writer + 1

		// Check if buffer is full
		// NOTE: For buffer sizes ≤ 8, use (nextWriter - reader) > (BufferSize - 1)
		// to avoid off-by-one overwrites. For larger buffers (≥1024) keep current condition
		if nextWriter-reader >= BufferSize {
			return false
		}

		d.buffer[nextWriter&BufferIndexMask] = entry
		d.writer.Value.Store(nextWriter)

		return true
	}
}

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
			entry := d.buffer[i&BufferIndexMask]
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
