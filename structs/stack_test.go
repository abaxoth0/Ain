package structs

import (
	"sync"
	"testing"
)

func TestNewStaticStack(t *testing.T) {
	t.Run("valid capacity", func(t *testing.T) {
		s := NewStaticStack[int](5)
		if s == nil {
			t.Fatal("Stack should not be nil")
		}
		if s.Capacity() != 5 {
			t.Errorf("Expected capacity 5, got %d", s.Capacity())
		}
		if s.Size() != 0 {
			t.Errorf("Expected initial size 0, got %d", s.Size())
		}
		if !s.IsEmpty() {
			t.Error("Stack should be empty initially")
		}
	})

	t.Run("zero capacity", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for zero capacity")
			}
		}()
		NewStaticStack[string](0)
	})

	t.Run("negative capacity", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for negative capacity")
			}
		}()
		NewStaticStack[int](-1)
	})
}

func TestStaticStackPush(t *testing.T) {
	t.Run("push within capacity", func(t *testing.T) {
		s := NewStaticStack[int](3)

		if !s.Push(1) {
			t.Error("First push should succeed")
		}
		if s.Size() != 1 {
			t.Errorf("Expected size 1, got %d", s.Size())
		}

		if !s.Push(2) {
			t.Error("Second push should succeed")
		}
		if s.Size() != 2 {
			t.Errorf("Expected size 2, got %d", s.Size())
		}
	})

	t.Run("push to full capacity", func(t *testing.T) {
		s := NewStaticStack[string](2)

		if !s.Push("first") {
			t.Error("First push should succeed")
		}
		if !s.Push("second") {
			t.Error("Second push should succeed")
		}
		if s.Size() != 2 {
			t.Errorf("Expected size 2, got %d", s.Size())
		}
		if !s.IsFull() {
			t.Error("Stack should be full")
		}
	})

	t.Run("push beyond capacity", func(t *testing.T) {
		s := NewStaticStack[int](1)

		if !s.Push(42) {
			t.Error("First push should succeed")
		}

		if s.Push(99) {
			t.Error("Push beyond capacity should fail")
		}
		if s.Size() != 1 {
			t.Errorf("Expected size 1, got %d", s.Size())
		}
	})
}

func TestStaticStackPop(t *testing.T) {
	t.Run("pop from empty stack", func(t *testing.T) {
		s := NewStaticStack[int](5)

		val, ok := s.Pop()
		if ok {
			t.Error("Pop from empty stack should return false")
		}
		if val != 0 {
			t.Errorf("Expected zero value, got %d", val)
		}
		if s.Size() != 0 {
			t.Errorf("Expected size 0, got %d", s.Size())
		}
	})

	t.Run("pop single element", func(t *testing.T) {
		s := NewStaticStack[string](5)
		s.Push("only")

		val, ok := s.Pop()
		if !ok {
			t.Error("Pop should succeed")
		}
		if val != "only" {
			t.Errorf("Expected 'only', got %s", val)
		}
		if s.Size() != 0 {
			t.Errorf("Expected size 0, got %d", s.Size())
		}
		if !s.IsEmpty() {
			t.Error("Stack should be empty after pop")
		}
	})

	t.Run("pop multiple elements LIFO", func(t *testing.T) {
		s := NewStaticStack[int](5)
		elements := []int{1, 2, 3}

		for _, elem := range elements {
			s.Push(elem)
		}

		// Pop in reverse order (LIFO)
		for i := len(elements) - 1; i >= 0; i-- {
			val, ok := s.Pop()
			if !ok {
				t.Errorf("Pop %d should succeed", i)
			}
			if val != elements[i] {
				t.Errorf("Pop %d: expected %d, got %d", i, elements[i], val)
			}
		}

		if s.Size() != 0 {
			t.Errorf("Expected final size 0, got %d", s.Size())
		}
	})
}

func TestStaticStackPeek(t *testing.T) {
	t.Run("peek empty stack", func(t *testing.T) {
		s := NewStaticStack[string](5)

		val, ok := s.Peek()
		if ok {
			t.Error("Peek from empty stack should return false")
		}
		if val != "" {
			t.Errorf("Expected zero value, got %s", val)
		}
		if s.Size() != 0 {
			t.Errorf("Expected size 0, got %d", s.Size())
		}
	})

	t.Run("peek non-empty stack", func(t *testing.T) {
		s := NewStaticStack[int](5)
		s.Push(1)
		s.Push(2)
		s.Push(3)

		val, ok := s.Peek()
		if !ok {
			t.Error("Peek should succeed")
		}
		if val != 3 {
			t.Errorf("Expected 3, got %d", val)
		}

		// Multiple peeks should return same value
		val2, ok2 := s.Peek()
		if !ok2 {
			t.Error("Second peek should succeed")
		}
		if val2 != 3 {
			t.Errorf("Expected 3 again, got %d", val2)
		}

		// Peek should not change size
		if s.Size() != 3 {
			t.Errorf("Expected size 3, got %d", s.Size())
		}
	})
}

func TestStaticStackSizeAndEmpty(t *testing.T) {
	s := NewStaticStack[int](3)

	if s.Size() != 0 {
		t.Errorf("Initial size should be 0, got %d", s.Size())
	}
	if !s.IsEmpty() {
		t.Error("Initial stack should be empty")
	}
	if s.IsFull() {
		t.Error("Initial stack should not be full")
	}

	s.Push(1)
	if s.Size() != 1 {
		t.Errorf("Size should be 1, got %d", s.Size())
	}
	if s.IsEmpty() {
		t.Error("Stack should not be empty after push")
	}
	if s.IsFull() {
		t.Error("Stack should not be full with one element")
	}

	s.Push(2)
	s.Push(3)
	if s.Size() != 3 {
		t.Errorf("Size should be 3, got %d", s.Size())
	}
	if s.IsEmpty() {
		t.Error("Stack should not be empty")
	}
	if !s.IsFull() {
		t.Error("Stack should be full")
	}
}

func TestNewDynamicStack(t *testing.T) {
	t.Run("positive capacity", func(t *testing.T) {
		s := NewDynamicStack[int](10)
		if s == nil {
			t.Fatal("Stack should not be nil")
		}
		if s.Size() != 0 {
			t.Errorf("Expected initial size 0, got %d", s.Size())
		}
		if !s.IsEmpty() {
			t.Error("Stack should be empty initially")
		}
	})

	t.Run("zero capacity", func(t *testing.T) {
		s := NewDynamicStack[string](0)
		if s == nil {
			t.Fatal("Stack should not be nil")
		}
		// Should use default capacity of 16
	})

	t.Run("negative capacity", func(t *testing.T) {
		s := NewDynamicStack[int](-5)
		if s == nil {
			t.Fatal("Stack should not be nil")
		}
		// Should use default capacity of 16
	})
}

func TestDynamicStackPush(t *testing.T) {
	t.Run("push single element", func(t *testing.T) {
		s := NewDynamicStack[int](0)

		if !s.Push(42) {
			t.Error("Push should always succeed")
		}
		if s.Size() != 1 {
			t.Errorf("Expected size 1, got %d", s.Size())
		}
	})

	t.Run("push multiple elements", func(t *testing.T) {
		s := NewDynamicStack[string](2) // small initial capacity

		// Push more than initial capacity to test growth
		elements := []string{"a", "b", "c", "d", "e"}
		for i, elem := range elements {
			if !s.Push(elem) {
				t.Errorf("Push %d should succeed", i)
			}
		}

		if s.Size() != int64(len(elements)) {
			t.Errorf("Expected size %d, got %d", len(elements), s.Size())
		}
	})
}

func TestDynamicStackPop(t *testing.T) {
	t.Run("pop from empty stack", func(t *testing.T) {
		s := NewDynamicStack[int](0)

		val, ok := s.Pop()
		if ok {
			t.Error("Pop from empty stack should return false")
		}
		if val != 0 {
			t.Errorf("Expected zero value, got %d", val)
		}
	})

	t.Run("pop multiple elements LIFO", func(t *testing.T) {
		s := NewDynamicStack[int](0)
		elements := []int{10, 20, 30, 40, 50}

		for _, elem := range elements {
			s.Push(elem)
		}

		// Pop in reverse order
		for i := len(elements) - 1; i >= 0; i-- {
			val, ok := s.Pop()
			if !ok {
				t.Errorf("Pop %d should succeed", i)
			}
			if val != elements[i] {
				t.Errorf("Pop %d: expected %d, got %d", i, elements[i], val)
			}
		}

		if s.Size() != 0 {
			t.Errorf("Expected final size 0, got %d", s.Size())
		}
		if !s.IsEmpty() {
			t.Error("Stack should be empty after popping all elements")
		}
	})
}

func TestDynamicStackPeek(t *testing.T) {
	t.Run("peek empty stack", func(t *testing.T) {
		s := NewDynamicStack[string](0)

		val, ok := s.Peek()
		if ok {
			t.Error("Peek from empty stack should return false")
		}
		if val != "" {
			t.Errorf("Expected zero value, got %s", val)
		}
	})

	t.Run("peek non-empty stack", func(t *testing.T) {
		s := NewDynamicStack[int](0)
		s.Push(100)
		s.Push(200)

		val, ok := s.Peek()
		if !ok {
			t.Error("Peek should succeed")
		}
		if val != 200 {
			t.Errorf("Expected 200, got %d", val)
		}

		// Peek should not change size
		if s.Size() != 2 {
			t.Errorf("Expected size 2, got %d", s.Size())
		}
	})
}

func TestNewSyncStack(t *testing.T) {
	t.Run("valid stack", func(t *testing.T) {
		staticStack := NewStaticStack[int](5)
		syncStack := NewSyncStack[int, *StaticStack[int]](staticStack)

		if syncStack == nil {
			t.Fatal("SyncStack should not be nil")
		}
		if syncStack.Size() != 0 {
			t.Errorf("Expected initial size 0, got %d", syncStack.Size())
		}
		if !syncStack.IsEmpty() {
			t.Error("Stack should be empty initially")
		}
	})

	t.Run("nil stack", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for nil stack")
			}
		}()
		NewSyncStack[int, *StaticStack[int]](nil)
	})
}

func TestSyncStackOperations(t *testing.T) {
	t.Run("basic operations", func(t *testing.T) {
		staticStack := NewStaticStack[string](3)
		syncStack := NewSyncStack[string, *StaticStack[string]](staticStack)

		// Push
		if !syncStack.Push("first") {
			t.Error("Push should succeed")
		}
		if syncStack.Size() != 1 {
			t.Errorf("Expected size 1, got %d", syncStack.Size())
		}

		// Peek
		val, ok := syncStack.Peek()
		if !ok {
			t.Error("Peek should succeed")
		}
		if val != "first" {
			t.Errorf("Expected 'first', got %s", val)
		}

		// Push another
		if !syncStack.Push("second") {
			t.Error("Second push should succeed")
		}

		// Pop
		val, ok = syncStack.Pop()
		if !ok {
			t.Error("Pop should succeed")
		}
		if val != "second" {
			t.Errorf("Expected 'second', got %s", val)
		}
		if syncStack.Size() != 1 {
			t.Errorf("Expected size 1, got %d", syncStack.Size())
		}
	})
}

func TestSyncStackConcurrency(t *testing.T) {
	t.Run("concurrent operations", func(t *testing.T) {
		staticStack := NewDynamicStack[int](0)
		syncStack := NewSyncStack[int, *DynamicStack[int]](staticStack)

		const numOperations = 1000
		var wg sync.WaitGroup
		var pushCount, popCount int64
		var pushCountMu, popCountMu sync.Mutex

		// Add some initial elements
		for i := range 100 {
			syncStack.Push(i)
		}

		// Concurrent pushes
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range numOperations {
				if syncStack.Push(i) {
					pushCountMu.Lock()
					pushCount++
					pushCountMu.Unlock()
				}
			}
		}()

		// Concurrent pops
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range numOperations {
				if _, ok := syncStack.Pop(); ok {
					popCountMu.Lock()
					popCount++
					popCountMu.Unlock()
				}
			}
		}()

		wg.Wait()

		pushCountMu.Lock()
		actualPushCount := pushCount
		pushCountMu.Unlock()

		popCountMu.Lock()
		actualPopCount := popCount
		popCountMu.Unlock()

		if actualPushCount == 0 {
			t.Error("No pushes succeeded")
		}
		if actualPopCount == 0 {
			t.Error("No pops succeeded")
		}

		expectedSize := 100 + actualPushCount - actualPopCount
		finalSize := syncStack.Size()
		if finalSize != expectedSize {
			t.Errorf("Expected final size %d, got %d", expectedSize, finalSize)
		}
	})

	t.Run("concurrent size checks", func(t *testing.T) {
		staticStack := NewDynamicStack[string](0)
		syncStack := NewSyncStack[string, *DynamicStack[string]](staticStack)

		const numOperations = 100
		var wg sync.WaitGroup

		// Concurrent pushes
		for i := range numOperations {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				syncStack.Push("test")
			}(i)
		}

		// Concurrent size checks
		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range 10 {
					size := syncStack.Size()
					if size < 0 || size > numOperations {
						t.Errorf("Invalid size: %d", size)
					}
				}
			}()
		}

		wg.Wait()
	})
}
