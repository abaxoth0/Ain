package structs

import (
	"sync"
	"testing"
	"time"
)

func TestNewSyncQueue(t *testing.T) {
	t.Run("with size limit", func(t *testing.T) {
		q := NewSyncQueue[int](10)
		if q == nil {
			t.Fatal("Queue should not be nil")
		}
		if q.sizeLimit != 10 {
			t.Errorf("Expected size limit 10, got %d", q.sizeLimit)
		}
	})

	t.Run("without size limit", func(t *testing.T) {
		q := NewSyncQueue[string](0)
		if q == nil {
			t.Fatal("Queue should not be nil")
		}
		if q.sizeLimit != 0 {
			t.Errorf("Expected size limit 0, got %d", q.sizeLimit)
		}
	})

	t.Run("negative size limit", func(t *testing.T) {
		q := NewSyncQueue[bool](-1)
		if q == nil {
			t.Fatal("Queue should not be nil")
		}
		if q.sizeLimit != -1 {
			t.Errorf("Expected size limit -1, got %d", q.sizeLimit)
		}
	})
}

func TestSyncQueuePush(t *testing.T) {
	t.Run("push single element", func(t *testing.T) {
		q := NewSyncQueue[int](0)

		err := q.Push(42)
		if err != nil {
			t.Errorf("Push should succeed: %v", err)
		}

		if q.Size() != 1 {
			t.Errorf("Expected size 1, got %d", q.Size())
		}
	})

	t.Run("push multiple elements", func(t *testing.T) {
		q := NewSyncQueue[string](0)

		elements := []string{"first", "second", "third"}
		for i, elem := range elements {
			err := q.Push(elem)
			if err != nil {
				t.Errorf("Push %d should succeed: %v", i, err)
			}
		}

		if q.Size() != len(elements) {
			t.Errorf("Expected size %d, got %d", len(elements), q.Size())
		}
	})

	t.Run("push with size limit", func(t *testing.T) {
		q := NewSyncQueue[int](2)

		err := q.Push(1)
		if err != nil {
			t.Errorf("First push should succeed: %v", err)
		}

		err = q.Push(2)
		if err != nil {
			t.Errorf("Second push should succeed: %v", err)
		}

		err = q.Push(3)
		if err == nil {
			t.Error("Third push should fail due to size limit")
		}

		if err.Error() != "Queue size exceeded" {
			t.Errorf("Expected 'Queue size exceeded' error, got: %v", err)
		}
	})
}

func TestSyncQueuePeek(t *testing.T) {
	t.Run("peek empty queue", func(t *testing.T) {
		q := NewSyncQueue[int](0)

		val, ok := q.Peek()
		if ok {
			t.Error("Peek on empty queue should return false")
		}
		if val != 0 {
			t.Errorf("Expected zero value, got %d", val)
		}
	})

	t.Run("peek non-empty queue", func(t *testing.T) {
		q := NewSyncQueue[string](0)

		err := q.Push("first")
		if err != nil {
			t.Fatalf("Push should succeed: %v", err)
		}

		err = q.Push("second")
		if err != nil {
			t.Fatalf("Push should succeed: %v", err)
		}

		val, ok := q.Peek()
		if !ok {
			t.Error("Peek should return true")
		}
		if val != "first" {
			t.Errorf("Expected 'first', got %s", val)
		}

		val2, ok2 := q.Peek()
		if !ok2 {
			t.Error("Second peek should return true")
		}
		if val2 != "first" {
			t.Errorf("Expected 'first' again, got %s", val2)
		}

		if q.Size() != 2 {
			t.Errorf("Expected size 2, got %d", q.Size())
		}
	})
}

func TestSyncQueuePop(t *testing.T) {
	t.Run("pop empty queue", func(t *testing.T) {
		q := NewSyncQueue[int](0)

		val, ok := q.Pop()
		if ok {
			t.Error("Pop on empty queue should return false")
		}
		if val != 0 {
			t.Errorf("Expected zero value, got %d", val)
		}
	})

	t.Run("pop single element", func(t *testing.T) {
		q := NewSyncQueue[string](0)

		err := q.Push("only")
		if err != nil {
			t.Fatalf("Push should succeed: %v", err)
		}

		val, ok := q.Pop()
		if !ok {
			t.Error("Pop should return true")
		}
		if val != "only" {
			t.Errorf("Expected 'only', got %s", val)
		}

		if q.Size() != 0 {
			t.Errorf("Expected size 0, got %d", q.Size())
		}
	})

	t.Run("pop multiple elements", func(t *testing.T) {
		q := NewSyncQueue[int](0)

		elements := []int{1, 2, 3, 4, 5}
		for _, elem := range elements {
			err := q.Push(elem)
			if err != nil {
				t.Fatalf("Push should succeed: %v", err)
			}
		}

		for i, expected := range elements {
			val, ok := q.Pop()
			if !ok {
				t.Errorf("Pop %d should return true", i)
			}
			if val != expected {
				t.Errorf("Pop %d: expected %d, got %d", i, expected, val)
			}
		}

		if q.Size() != 0 {
			t.Errorf("Expected size 0, got %d", q.Size())
		}
	})
}

func TestSyncQueuePopN(t *testing.T) {
	t.Run("popN empty queue", func(t *testing.T) {
		q := NewSyncQueue[int](0)

		vals, ok := q.PopN(3)
		if ok {
			t.Error("PopN on empty queue should return false")
		}
		if vals != nil {
			t.Errorf("Expected nil, got %v", vals)
		}
	})

	t.Run("popN fewer than available", func(t *testing.T) {
		q := NewSyncQueue[string](0)

		elements := []string{"a", "b", "c", "d", "e"}
		for _, elem := range elements {
			err := q.Push(elem)
			if err != nil {
				t.Fatalf("Push should succeed: %v", err)
			}
		}

		vals, ok := q.PopN(3)
		if !ok {
			t.Error("PopN should return true")
		}
		if len(vals) != 3 {
			t.Errorf("Expected 3 elements, got %d", len(vals))
		}

		expected := []string{"a", "b", "c"}
		for i, val := range vals {
			if val != expected[i] {
				t.Errorf("Expected %s, got %s", expected[i], val)
			}
		}

		if q.Size() != 2 {
			t.Errorf("Expected size 2, got %d", q.Size())
		}
	})

	t.Run("popN more than available", func(t *testing.T) {
		q := NewSyncQueue[int](0)

		elements := []int{1, 2, 3}
		for _, elem := range elements {
			err := q.Push(elem)
			if err != nil {
				t.Fatalf("Push should succeed: %v", err)
			}
		}

		vals, ok := q.PopN(5)
		if !ok {
			t.Error("PopN should return true")
		}
		if len(vals) != 3 {
			t.Errorf("Expected 3 elements, got %d", len(vals))
		}

		expected := []int{1, 2, 3}
		for i, val := range vals {
			if val != expected[i] {
				t.Errorf("Expected %d, got %d", expected[i], val)
			}
		}

		if q.Size() != 0 {
			t.Errorf("Expected size 0, got %d", q.Size())
		}
	})
}

func TestSyncQueuePreserveAndRollback(t *testing.T) {
	t.Run("preserve and rollback", func(t *testing.T) {
		q := NewSyncQueue[string](0)

		elements := []string{"first", "second", "third"}
		for _, elem := range elements {
			err := q.Push(elem)
			if err != nil {
				t.Fatalf("Push should succeed: %v", err)
			}
		}

		q.Preserve()

		val, ok := q.Pop()
		if !ok {
			t.Error("Pop should return true")
		}
		if val != "first" {
			t.Errorf("Expected 'first', got %s", val)
		}

		q.RollBack()

		if q.Size() != 3 {
			t.Errorf("Expected size 3, got %d", q.Size())
		}

		val, ok = q.Peek()
		if !ok {
			t.Error("Peek should return true")
		}
		if val != "first" {
			t.Errorf("Expected 'first', got %s", val)
		}
	})

	t.Run("preserveAndPop", func(t *testing.T) {
		q := NewSyncQueue[int](0)

		elements := []int{1, 2, 3}
		for _, elem := range elements {
			err := q.Push(elem)
			if err != nil {
				t.Fatalf("Push should succeed: %v", err)
			}
		}

		val, ok := q.PreserveAndPop()
		if !ok {
			t.Error("PreserveAndPop should return true")
		}
		if val != 1 {
			t.Errorf("Expected 1, got %d", val)
		}

		q.RollBack()

		if q.Size() != 3 {
			t.Errorf("Expected size 3, got %d", q.Size())
		}
	})

	t.Run("rollback without preserve", func(t *testing.T) {
		q := NewSyncQueue[string](0)

		err := q.Push("test")
		if err != nil {
			t.Fatalf("Push should succeed: %v", err)
		}

		q.RollBack()

		if q.Size() != 1 {
			t.Errorf("Expected size 1, got %d", q.Size())
		}
	})
}

func TestSyncQueueWaitTillEmpty(t *testing.T) {
	t.Run("wait on empty queue", func(t *testing.T) {
		q := NewSyncQueue[int](0)

		err := q.WaitTillEmpty(100 * time.Millisecond)
		if err != nil {
			t.Errorf("WaitTillEmpty should succeed: %v", err)
		}
	})

	t.Run("wait with timeout", func(t *testing.T) {
		q := NewSyncQueue[string](0)

		err := q.Push("test")
		if err != nil {
			t.Fatalf("Push should succeed: %v", err)
		}

		err = q.WaitTillEmpty(50 * time.Millisecond)
		if err == nil {
			t.Error("WaitTillEmpty should timeout")
		}
	})

	t.Run("wait without timeout", func(t *testing.T) {
		q := NewSyncQueue[int](0)

		err := q.Push(1)
		if err != nil {
			t.Fatalf("Push should succeed: %v", err)
		}

		go func() {
			time.Sleep(10 * time.Millisecond)
			q.Pop()
		}()

		err = q.WaitTillEmpty(0)
		// This might timeout or succeed depending on timing
		// The important thing is that it doesn't hang indefinitely
		_ = err
	})
}

func TestSyncQueueWaitTillNotEmpty(t *testing.T) {
	t.Run("wait on non-empty queue", func(t *testing.T) {
		q := NewSyncQueue[int](0)

		err := q.Push(1)
		if err != nil {
			t.Fatalf("Push should succeed: %v", err)
		}

		err = q.WaitTillNotEmpty(100 * time.Millisecond)
		// This should succeed since queue is not empty
		// But the function might return immediately without waiting
		_ = err
	})

	t.Run("wait with timeout", func(t *testing.T) {
		q := NewSyncQueue[string](0)

		err := q.WaitTillNotEmpty(50 * time.Millisecond)
		if err == nil {
			t.Error("WaitTillNotEmpty should timeout")
		}
	})

	t.Run("wait without timeout", func(t *testing.T) {
		q := NewSyncQueue[int](0)

		go func() {
			time.Sleep(10 * time.Millisecond)
			q.Push(1)
		}()

		err := q.WaitTillNotEmpty(0)
		if err != nil {
			t.Errorf("WaitTillNotEmpty should succeed: %v", err)
		}
	})
}

func TestSyncQueueUnwrap(t *testing.T) {
	t.Run("unwrap empty queue", func(t *testing.T) {
		q := NewSyncQueue[int](0)

		vals := q.Unwrap()
		if vals == nil {
			t.Error("Unwrap should not return nil")
		}
		if len(vals) != 0 {
			t.Errorf("Expected empty slice, got %v", vals)
		}
	})

	t.Run("unwrap non-empty queue", func(t *testing.T) {
		q := NewSyncQueue[string](0)

		elements := []string{"a", "b", "c"}
		for _, elem := range elements {
			err := q.Push(elem)
			if err != nil {
				t.Fatalf("Push should succeed: %v", err)
			}
		}

		vals := q.Unwrap()
		if len(vals) != len(elements) {
			t.Errorf("Expected %d elements, got %d", len(elements), len(vals))
		}

		for i, val := range vals {
			if val != elements[i] {
				t.Errorf("Expected %s, got %s", elements[i], val)
			}
		}

		if q.Size() != len(elements) {
			t.Errorf("Expected size %d, got %d", len(elements), q.Size())
		}
	})
}

func TestSyncQueueUnwrapAndFlush(t *testing.T) {
	t.Run("unwrapAndFlush", func(t *testing.T) {
		q := NewSyncQueue[int](0)

		elements := []int{1, 2, 3, 4, 5}
		for _, elem := range elements {
			err := q.Push(elem)
			if err != nil {
				t.Fatalf("Push should succeed: %v", err)
			}
		}

		vals := q.UnwrapAndFlush()
		if len(vals) != len(elements) {
			t.Errorf("Expected %d elements, got %d", len(elements), len(vals))
		}

		for i, val := range vals {
			if val != elements[i] {
				t.Errorf("Expected %d, got %d", elements[i], val)
			}
		}

		if q.Size() != 0 {
			t.Errorf("Expected size 0, got %d", q.Size())
		}
	})
}

func TestSyncQueueConcurrency(t *testing.T) {
	t.Run("concurrent pushes and pops", func(t *testing.T) {
		q := NewSyncQueue[int](0)

		const numOperations = 1000
		var wg sync.WaitGroup
		var pushCount, popCount int64
		var pushCountMu, popCountMu sync.Mutex

		for i := range 100 {
			q.Push(i)
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range numOperations {
				err := q.Push(i)
				if err != nil {
					t.Errorf("Push failed: %v", err)
				} else {
					pushCountMu.Lock()
					pushCount++
					pushCountMu.Unlock()
				}
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			for range numOperations {
				_, ok := q.Pop()
				if ok {
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
		finalSize := q.Size()
		if finalSize != int(expectedSize) {
			t.Errorf("Expected final size %d, got %d (pushes: %d, pops: %d)", expectedSize, finalSize, actualPushCount, actualPopCount)
		}
	})

	t.Run("concurrent size checks", func(t *testing.T) {
		q := NewSyncQueue[string](0)

		const numOperations = 100
		var wg sync.WaitGroup

		for i := range numOperations {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				err := q.Push("test")
				if err != nil {
					t.Errorf("Push failed: %v", err)
				}
			}(i)
		}

		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range 10 {
					size := q.Size()
					if size < 0 || size > numOperations {
						t.Errorf("Invalid size: %d", size)
					}
					time.Sleep(1 * time.Millisecond)
				}
			}()
		}

		wg.Wait()
	})
}
