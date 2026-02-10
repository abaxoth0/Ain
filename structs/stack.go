package structs

import (
	"fmt"
	"reflect"
	"sync"
)

// Represents Last-In-First-Out stack.
type Stack[T any] interface {
	// Pushes new element on top of the stack.
	// Returns false if stack is overflowed (only if it can be overflowed).
	Push(v T) bool
	// Removes top element from the stack.
	// Returns false if stack is empty.
	Pop() (T, bool)
	// Gets top element from the stack.
	// Returns false if stack is empty.
	Peek() (T, bool)
	// Returns amount of elements in the stack.
	Size() int64
	IsEmpty() bool
}

// Wrapper that makes S thread-Safe.
// It just wraps Stack methods of S using mutex,
// so it's not a some kind of stack on it's own.
// (although it inherently implements Stack interface)
type SyncStack[T any, S Stack[T]] struct {
	stack S
	mut   sync.Mutex
}

func NewSyncStack[T any, S Stack[T]](s S) *SyncStack[T, S] {
	if reflect.ValueOf(s).IsNil() {
		panic("can't create SyncStack from nil")
	}
	return &SyncStack[T, S]{
		stack: s,
	}
}

// Pushes new element on top of S.
// Returns false if S is overflowed (only if it can be overflowed).
func (s *SyncStack[T, S]) Push(v T) bool {
	s.mut.Lock()
	defer s.mut.Unlock()
	return s.stack.Push(v)
}

// Removes top element from S.
// Returns false if stack is empty.
func (s *SyncStack[T, S]) Pop() (T, bool) {
	s.mut.Lock()
	defer s.mut.Unlock()
	return s.stack.Pop()
}

// Gets top element from S.
// Returns false if stack is empty.
func (s *SyncStack[T, S]) Peek() (T, bool) {
	s.mut.Lock()
	defer s.mut.Unlock()
	return s.stack.Peek()
}

// Returns amount of elements in S.
func (s *SyncStack[T, S]) Size() int64 {
	s.mut.Lock()
	defer s.mut.Unlock()
	return s.stack.Size()
}

func (s *SyncStack[T, S]) IsEmpty() bool {
	s.mut.Lock()
	defer s.mut.Unlock()
	return s.Size() == 0
}

// Last-In-First-Out static stack data structure.
type StaticStack[T any] struct {
	buffer []T
	cap    int64
	cursor int64
}

// Panics if capacity is <= 0.
func NewStaticStack[T any](capacity int64) *StaticStack[T] {
	if capacity <= 0 {
		panic(fmt.Sprintf("invalid stack capacity: %d", capacity))
	}
	return &StaticStack[T]{
		cap:    capacity,
		buffer: make([]T, capacity),
		cursor: -1,
	}
}

// Pushes new element on top of the stack.
// Returns false if stack is overflowed.
func (s *StaticStack[T]) Push(v T) bool {
	if s.cursor >= s.cap-1 {
		return false
	}
	s.cursor++
	s.buffer[s.cursor] = v
	return true
}

// Gets top element from the stack.
// Returns false if stack is empty.
func (s *StaticStack[T]) Peek() (T, bool) {
	if s.cursor < 0 {
		var zero T
		return zero, false
	}
	return s.buffer[s.cursor], true
}

// Removes top element from the stack.
// Returns false if stack is empty.
func (s *StaticStack[T]) Pop() (T, bool) {
	if s.cursor < 0 {
		var zero T
		return zero, false
	}
	s.cursor--
	return s.buffer[s.cursor+1], true
}

// Returns max stack size.
func (s *StaticStack[T]) Capacity() int64 {
	return s.cap
}

// Returns amount of elements in the stack.
func (s *StaticStack[T]) Size() int64 {
	return s.cursor + 1
}

func (s *StaticStack[T]) IsEmpty() bool {
	return s.cursor == -1
}

func (s *StaticStack[T]) IsFull() bool {
	return s.Size() == s.cap
}

// Last-In-First-Out dynamic stack data structure.
type DynamicStack[T any] struct {
	buffer []T
}

// Sets capacity to default if it's <= 0.
func NewDynamicStack[T any](capacity int) *DynamicStack[T] {
	if capacity <= 0 {
		capacity = 16
	}
	return &DynamicStack[T]{
		buffer: make([]T, 0, capacity),
	}
}

// Pushes new element on top of the stack.
// Always returns true.
func (s *DynamicStack[T]) Push(v T) bool {
	s.buffer = append(s.buffer, v)
	return true
}

// Gets top element from the stack.
// Returns false if stack is empty.
func (s *DynamicStack[T]) Peek() (T, bool) {
	idx := len(s.buffer) - 1
	if idx < 0 {
		var zero T
		return zero, false
	}
	return s.buffer[idx], true
}

// Removes top element from the stack.
// Returns false if stack is empty.
func (s *DynamicStack[T]) Pop() (T, bool) {
	if len(s.buffer) == 0 {
		var zero T
		return zero, false
	}
	lastIdx := len(s.buffer) - 1
	last := s.buffer[lastIdx]
	s.buffer = s.buffer[:lastIdx]
	return last, true
}

// Returns amount of elements in the stack.
func (s *DynamicStack[T]) Size() int64 {
	return int64(len(s.buffer))
}

func (s *DynamicStack[T]) IsEmpty() bool {
	return s.Size() == 0
}
