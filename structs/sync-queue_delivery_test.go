package structs

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestSyncQueueContextConsumerDelivery(t *testing.T) {
	const workers, producers, perProducer = 4, 4, 1000
	const total = producers * perProducer
	q := NewSyncQueue[int](0)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	received := make(chan int, total)
	var consumers sync.WaitGroup
	for range workers {
		consumers.Add(1)
		go func() {
			defer consumers.Done()
			for {
				if err := q.WaitTillNotEmptyContext(ctx); err != nil {
					if !errors.Is(err, context.Canceled) {
						t.Errorf("consumer wait failed: %v", err)
					}
					return
				}
				if value, ok := q.Pop(); ok {
					select {
					case received <- value:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
	}
	var publishers sync.WaitGroup
	for producer := range producers {
		publishers.Add(1)
		go func() {
			defer publishers.Done()
			for i := range perProducer {
				if err := q.Push(producer*perProducer + i); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	publishers.Wait()
	seen := make([]bool, total)
delivery:
	for range total {
		select {
		case value := <-received:
			if value < 0 || value >= total || seen[value] {
				t.Errorf("invalid or duplicate delivery %d", value)
			} else {
				seen[value] = true
			}
		case <-ctx.Done():
			t.Error("accepted queue elements were not delivered")
			break delivery
		}
	}
	if err := q.WaitTillEmptyContext(ctx); err != nil {
		t.Error(err)
	}
	cancel()
	stopped := make(chan struct{})
	go func() {
		consumers.Wait()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("canceled queue consumers did not exit")
	}
	if q.Size() != 0 {
		t.Error("delivery left queued elements")
	}
}
