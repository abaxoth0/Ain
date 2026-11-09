package structs

import "testing"

func checkQueueZeroRollback[T comparable](t *testing.T, zero, nonzero T) {
	t.Helper()
	q := NewSyncQueue[T](0)
	q.RollBack()
	_ = q.Push(zero)
	q.PreserveAndPop()
	q.RollBack()
	if value, ok := q.Pop(); !ok || value != zero {
		t.Fatalf("empty queue rollback returned %v/%v, want %v/true", value, ok, zero)
	}
	_ = q.Push(nonzero)
	q.PreserveAndPop()
	_ = q.Push(zero)
	q.PreserveAndPop()
	_ = q.Push(nonzero)
	q.RollBack()
	for _, want := range []T{zero, nonzero} {
		if value, ok := q.Pop(); !ok || value != want {
			t.Fatalf("rollback returned %v/%v, want %v/true", value, ok, want)
		}
	}
	q.RollBack()
	if q.Size() != 0 {
		t.Error("rollback restored the same preserved element twice")
	}
}

func TestSyncQueueZeroValueRollback(t *testing.T) {
	t.Run("int", func(t *testing.T) { checkQueueZeroRollback(t, 0, 1) })
	t.Run("bool", func(t *testing.T) { checkQueueZeroRollback(t, false, true) })
	t.Run("string", func(t *testing.T) { checkQueueZeroRollback(t, "", "value") })
	t.Run("pointer", func(t *testing.T) { checkQueueZeroRollback(t, (*int)(nil), new(int)) })
}
