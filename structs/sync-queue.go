package structs

import (
	"errors"
	"sync"
	"time"

	"github.com/abaxoth0/Ain/errs"
)

// Concurrency-safe first-in-first-out (FIFO) queue.
type SyncQueue[T comparable] struct {
	sizeLimit int          // 0 = no limit
	elems     []T
	head      int          // Index of the first element
	mut       sync.RWMutex
	cond      *sync.Cond
	preserved T            // Preserved element for rollback operations
}

// Creates a new thread-safe queue with optional size limit.
// To disable size limit, set sizeLimit <= 0.
func NewSyncQueue[T comparable](sizeLimit int) *SyncQueue[T] {
	q := new(SyncQueue[T])

	q.cond = sync.NewCond(&q.mut)
	q.sizeLimit = sizeLimit
	q.elems = make([]T, 0, 16)
	q.head = 0

	return q
}

// Appends v to the end of queue. Returns error if size limit is exceeded.
func (q *SyncQueue[T]) Push(v T) error {
	q.mut.Lock()

	if q.sizeLimit > 0 && len(q.elems)-q.head >= q.sizeLimit {
		q.mut.Unlock()
		return errors.New("Queue size exceeded")
	}

	wasEmpty := len(q.elems) == q.head

	q.elems = append(q.elems, v)

	q.mut.Unlock()

	if wasEmpty {
		q.cond.Broadcast()
	}

	return nil
}

// Returns the first element of the queue without removing it.
// If queue isn't empty - returns first element and true.
// If queue is empty - returns zero-value of T and false.
func (q *SyncQueue[T]) Peek() (T, bool) {
	q.mut.Lock()
	defer q.mut.Unlock()

	var v T

	if len(q.elems) == q.head {
		return v, false
	}

	return q.elems[q.head], true
}

// Removes and returns the first element of the queue.
// Same as Peek(), but also deletes first element in queue.
func (q *SyncQueue[T]) Pop() (T, bool) {
	q.mut.Lock()
	defer q.mut.Unlock()

	var v T

	if len(q.elems) == q.head {
		return v, false
	}

	v = q.elems[q.head]
	q.head++

	if q.head >= cap(q.elems)/4 && q.head > 0 {
		q.compact()
	}

	if len(q.elems) == q.head {
		q.cond.Broadcast()
	}

	return v, true
}

func (q *SyncQueue[T]) compact() {
	newElems := make([]T, len(q.elems)-q.head)
	copy(newElems, q.elems[q.head:])
	q.elems = newElems
	q.head = 0
}

// Removes and returns up to n elements from the queue.
// If n is greater than queue size, n will be adjusted to the queue size to prevent panic.
// If queue is empty - returns nil and false.
func (q *SyncQueue[T]) PopN(n int) ([]T, bool) {
	q.mut.Lock()
	defer q.mut.Unlock()

	size := len(q.elems) - q.head

	if size == 0 {
		return nil, false
	}

	if n > size {
		n = size
	}

	s := make([]T, n)
	copy(s, q.elems[q.head:q.head+n])
	q.head += n

	if q.head >= cap(q.elems)/4 && q.head > 0 {
		q.compact()
	}

	if len(q.elems) == q.head {
		q.cond.Broadcast()
	}

	return s, true
}

// Saves the head element of the queue for potential rollback.
func (q *SyncQueue[T]) Preserve() {
	q.mut.Lock()
	defer q.mut.Unlock()

	if len(q.elems) == q.head {
		return
	}

	q.preserved = q.elems[q.head]
}

// Restores the previously preserved element to the front of the queue.
// Does nothing if no element was preserved.
func (q *SyncQueue[T]) RollBack() {
	q.mut.Lock()
	defer q.mut.Unlock()

	var zero T

	if q.preserved == zero {
		return
	}

	swap := make([]T, len(q.elems)-q.head+1)

	swap[0] = q.preserved
	q.preserved = zero

	copy(swap[1:], q.elems[q.head:])
	q.elems = swap
	q.head = 0
}

// Preserves the current head element and then pops it from the queue.
// This is equivalent to calling Preserve() followed by Pop().
func (q *SyncQueue[T]) PreserveAndPop() (T, bool) {
	q.Preserve()
	return q.Pop()
}

func (q *SyncQueue[T]) Size() int {
	q.mut.RLock()
	l := len(q.elems) - q.head
	q.mut.RUnlock()
	return l
}

// Blocks until waitCond returns false or timeout occurs.
// If timeout <= 0: waits until 'waitCond' returns false.
// If timeout > 0: waits until either 'waitCond' returns false or timeout is exceeded.
func (q *SyncQueue[T]) wait(timeout time.Duration, waitCond func() bool) error {
	q.mut.Lock()
	defer q.mut.Unlock()

	if timeout <= 0 {
		for waitCond() {
			q.cond.Wait()
		}
		return nil
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for waitCond() {
		done := make(chan bool)

		go func() {
			q.cond.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-timer.C:
			q.cond.Broadcast()
			/*
			   IMPORTANT
			    Need wait till q.cond.Wait() finish it's work,
			    cuz it's unlocks mutex while waiting and lock it again before returning,
			    so if q.cond.Wait() still waits that means mutext is unlocked.
			    On this state may occur 2 type of erros:
			    1) If mutex unlocking before returning from this function (which is currently so):
			       Attempt to unlock a mutex that is already unlocked by q.cond.Wait() will cause panic.
			    2) If mutex isn't unlocking before returning:
			       q.cond.Wait() will lock it after finishing it's work and that will cause a deadlock.
			*/
			<-done
			return errs.StatusTimeout
		}
	}

	return nil
}

// Blocks until the queue becomes empty.
// To disable timeout, set it to <= 0.
// Returns errs.StatusTimeout if timeout exceeded, nil otherwise.
func (q *SyncQueue[T]) WaitTillEmpty(timeout time.Duration) error {
	q.mut.Lock()

	if len(q.elems) == q.head {
		q.mut.Unlock()
		return nil
	}

	q.mut.Unlock()

	return q.wait(timeout, func() bool { return len(q.elems) > q.head })
}

// Blocks until the queue contains at least one element.
// To disable timeout, set it to <= 0.
// Returns errs.StatusTimeout if timeout exceeded, nil otherwise.
func (q *SyncQueue[T]) WaitTillNotEmpty(timeout time.Duration) error {
	q.mut.Lock()

	if len(q.elems) > q.head {
		q.mut.Unlock()
		return nil
	}

	q.mut.Unlock()

	return q.wait(timeout, func() bool { return len(q.elems) == q.head })
}

// Returns a copy of the internal slice containing all current queue elements.
func (q *SyncQueue[T]) Unwrap() []T {
	q.mut.Lock()

	size := len(q.elems) - q.head
	r := make([]T, size)

	copy(r, q.elems[q.head:])

	q.mut.Unlock()

	return r
}

// Returns a copy of all current queue elements and removes them from the queue.
// Same as Unwrap, but also deletes all elements in queue.
func (q *SyncQueue[T]) UnwrapAndFlush() []T {
	q.mut.Lock()

	size := len(q.elems) - q.head
	r := make([]T, size)

	copy(r, q.elems[q.head:])

	q.elems = make([]T, 0, cap(q.elems))
	q.head = 0

	q.mut.Unlock()

	return r
}
