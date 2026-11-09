package structs

import (
	"math"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
)

func stackImplementations() map[string]func() Stack[int] {
	return map[string]func() Stack[int]{
		"static":       func() Stack[int] { return NewStaticStack[int](3) },
		"dynamic":      func() Stack[int] { return NewDynamicStack[int](3) },
		"sync static":  func() Stack[int] { return NewSyncStack[int](NewStaticStack[int](3)) },
		"sync dynamic": func() Stack[int] { return NewSyncStack[int](NewDynamicStack[int](3)) },
	}
}

func TestStackPopBatchContract(t *testing.T) {
	for name, newStack := range stackImplementations() {
		t.Run(name, func(t *testing.T) {
			for _, test := range []struct {
				name   string
				values []int
				n      int64
				want   []int
				ok     bool
			}{
				{name: "empty", n: 1},
				{name: "empty zero", n: 0, want: []int{}, ok: true},
				{name: "negative", values: []int{1, 2, 3}, n: -1, want: []int{}, ok: true},
				{name: "zero", values: []int{1, 2, 3}, n: 0, want: []int{}, ok: true},
				{name: "one", values: []int{1, 2, 3}, n: 1, want: []int{3}, ok: true},
				{name: "partial", values: []int{1, 2, 3}, n: 2, want: []int{3, 2}, ok: true},
				{name: "all", values: []int{1, 2, 3}, n: 3, want: []int{3, 2, 1}, ok: true},
				{name: "oversized", values: []int{1, 2, 3}, n: math.MaxInt64, want: []int{3, 2, 1}, ok: true},
			} {
				t.Run(test.name, func(t *testing.T) {
					stack, repeated := newStack(), newStack()
					if !stack.PushBatch(test.values...) || !repeated.PushBatch(test.values...) {
						t.Fatal("failed to seed stacks")
					}
					batch, ok := stack.PopBatch(test.n)
					if ok != test.ok || !slices.Equal(batch, test.want) || (batch == nil) != (test.want == nil) {
						t.Fatalf("PopBatch(%d) = %v, %v; want %v, %v", test.n, batch, ok, test.want, test.ok)
					}
					for i, value := range batch {
						want, ok := repeated.Pop()
						if !ok || value != want {
							t.Errorf("batch[%d] = %d, repeated Pop = %d, %v", i, value, want, ok)
						}
					}
					if !slices.Equal(stack.ToSlice(), repeated.ToSlice()) {
						t.Errorf("remaining stack = %v, repeated Pop leaves %v", stack.ToSlice(), repeated.ToSlice())
					}
					if want := int64(len(test.values) - len(test.want)); stack.Size() != want {
						t.Errorf("size = %d, want %d", stack.Size(), want)
					}
				})
			}
		})
	}
}

func TestStackPopBatchOwnership(t *testing.T) {
	for name, newStack := range stackImplementations() {
		t.Run(name, func(t *testing.T) {
			stack := newStack()
			if !stack.PushBatch(1, 2, 3) {
				t.Fatal("failed to seed stack")
			}
			batch, ok := stack.PopBatch(2)
			if !ok || len(batch) != 2 {
				t.Fatalf("PopBatch = %v, %v", batch, ok)
			}
			original := slices.Clone(batch)
			if !stack.Push(9) {
				t.Fatal("failed to reuse popped slot")
			}
			if !slices.Equal(batch, original) {
				t.Errorf("Push changed returned batch: %v -> %v", original, batch)
			}
			for i := range batch {
				batch[i] = 99
			}
			if got := stack.ToSlice(); !slices.Equal(got, []int{1, 9}) {
				t.Errorf("batch mutation changed stack: %v", got)
			}
			if !stack.Push(10) {
				t.Fatal("failed to push after batch mutation")
			}
			if got, ok := stack.PopBatch(3); !ok || !slices.Equal(got, []int{10, 9, 1}) {
				t.Errorf("subsequent PopBatch = %v, %v; want [10 9 1], true", got, ok)
			}
		})
	}
}

func TestSyncStackReturnedBatchConcurrentUse(t *testing.T) {
	for _, name := range []string{"sync static", "sync dynamic"} {
		t.Run(name, func(t *testing.T) {
			stack := stackImplementations()[name]()
			if !stack.PushBatch(1, 2, 3) {
				t.Fatal("failed to seed stack")
			}
			batch, ok := stack.PopBatch(2)
			if !ok || len(batch) != 2 {
				t.Fatalf("PopBatch = %v, %v", batch, ok)
			}
			original := slices.Clone(batch)
			start := make(chan struct{})
			var workers sync.WaitGroup
			var changed atomic.Bool
			workers.Add(2)
			go func() {
				defer workers.Done()
				<-start
				for range 1000 {
					if !slices.Equal(batch, original) {
						changed.Store(true)
					}
				}
			}()
			go func() {
				defer workers.Done()
				<-start
				for range 1000 {
					stack.Push(9)
					stack.Pop()
				}
			}()
			close(start)
			workers.Wait()
			if changed.Load() || !slices.Equal(batch, original) {
				t.Errorf("concurrent pushes changed returned batch: %v -> %v", original, batch)
			}
		})
	}
}

type valueStack struct{ *DynamicStack[int] }

func TestNewSyncStackValueImplementation(t *testing.T) {
	defer func() {
		if failure := recover(); failure != nil {
			t.Fatalf("valid value implementation panicked: %v", failure)
		}
	}()
	stack := NewSyncStack[int](valueStack{NewDynamicStack[int](3)})
	if !stack.PushBatch(1, 2, 3) {
		t.Fatal("failed to push through value implementation")
	}
	if got, ok := stack.PopBatch(2); !ok || !slices.Equal(got, []int{3, 2}) {
		t.Fatalf("PopBatch = %v, %v", got, ok)
	}
	if got, ok := stack.Pop(); !ok || got != 1 {
		t.Fatalf("Pop = %d, %v", got, ok)
	}
}

func TestNewSyncStackRejectsNil(t *testing.T) {
	var typedNil *DynamicStack[int]
	for name, construct := range map[string]func(){
		"nil pointer":                   func() { NewSyncStack[int](typedNil) },
		"nil interface":                 func() { NewSyncStack[int, Stack[int]](nil) },
		"interface holding nil pointer": func() { NewSyncStack[int, Stack[int]](typedNil) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if failure := recover(); failure != "can't create SyncStack from nil" {
					t.Errorf("nil rejection = %v, want constructor's nil panic", failure)
				}
			}()
			construct()
		})
	}
}
