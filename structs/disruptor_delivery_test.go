package structs

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

type disruptorPayload struct {
	ID   int
	Data [128]int
}

func waitDisruptorSignal(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("disruptor operation did not finish")
	}
}

func TestDisruptorPayloadsThroughWraparound(t *testing.T) {
	for _, publishers := range []int{1, 4} {
		t.Run(fmt.Sprintf("%d publishers", publishers), func(t *testing.T) {
			const perPublisher = 10000
			d, err := NewDisruptorWithSize[disruptorPayload](MinBufferSize, &DisruptorConfig{
				IdleCheckInterval: time.Nanosecond,
			})
			if err != nil {
				t.Fatal(err)
			}
			var closeOnce sync.Once
			closeBuffer := func() { closeOnce.Do(d.Close) }
			defer closeBuffer()
			ready, consumed := make(chan struct{}), make(chan struct{})
			seen := make([]int, publishers*perPublisher+1)
			if !d.Publish(disruptorPayload{}) {
				t.Fatal("could not publish startup entry")
			}
			go func() {
				defer close(consumed)
				if err := d.Consume(func(p disruptorPayload) {
					if p.ID == 0 && seen[0] == 0 {
						seen[0]++
						close(ready)
						return
					}
					if p.ID <= 0 || p.ID >= len(seen) {
						t.Errorf("invalid payload ID %d", p.ID)
						return
					}
					seen[p.ID]++
					for _, value := range p.Data {
						if value != p.ID {
							t.Errorf("payload %d contains %d", p.ID, value)
							break
						}
					}
				}); err != nil {
					t.Error(err)
				}
			}()
			waitDisruptorSignal(t, ready)
			deadline := time.Now().Add(5 * time.Second)
			var producers sync.WaitGroup
			for producer := range publishers {
				producers.Add(1)
				go func() {
					defer producers.Done()
					for i := range perPublisher {
						p := disruptorPayload{ID: producer*perPublisher + i + 1}
						for j := range p.Data {
							p.Data[j] = p.ID
						}
						for !d.Publish(p) {
							if time.Now().After(deadline) {
								t.Error("publication stalled")
								return
							}
							runtime.Gosched()
						}
					}
				}()
			}
			producers.Wait()
			closeBuffer()
			waitDisruptorSignal(t, consumed)
			for id, count := range seen {
				if count != 1 {
					t.Fatalf("payload %d delivered %d times", id, count)
				}
			}
			if !d.IsEmpty() {
				t.Error("closed consumer left accepted entries pending")
			}
		})
	}
}

func TestDisruptorCloseDuringPublication(t *testing.T) {
	for iteration := range 100 {
		d, err := NewDisruptorWithSize[int](MinBufferSize, nil)
		if err != nil {
			t.Fatal(err)
		}
		entered, release, consumed := make(chan struct{}), make(chan struct{}), make(chan struct{})
		var entries []int
		d.Publish(0)
		go func() {
			defer close(consumed)
			if err := d.Consume(func(id int) {
				if id == 0 {
					close(entered)
					<-release
				}
				entries = append(entries, id)
			}); err != nil {
				t.Error(err)
			}
		}()
		waitDisruptorSignal(t, entered)
		expected := map[int]bool{0: true}
		fill := 1
		if iteration%2 == 0 {
			fill = MinBufferSize - 1
		}
		for id := 1; id < fill; id++ {
			if !d.Publish(id) {
				t.Fatalf("publication %d rejected before capacity", id)
			}
			expected[id] = true
		}
		if fill == MinBufferSize-1 && d.Publish(MinBufferSize-1) {
			t.Fatal("buffer overwrote a slot still being consumed")
		}
		start := make(chan struct{})
		accepted := make(chan int, 4)
		var producers sync.WaitGroup
		for producer := range 4 {
			producers.Add(1)
			go func() {
				defer producers.Done()
				<-start
				id := MinBufferSize + producer
				if d.Publish(id) {
					accepted <- id
				}
			}()
		}
		close(start)
		if fill == 1 {
			select {
			case id := <-accepted:
				expected[id] = true
			case <-time.After(5 * time.Second):
				t.Fatal("publishers did not accept an entry before Close")
			}
		}
		d.Close()
		producers.Wait()
		close(accepted)
		for id := range accepted {
			if fill == MinBufferSize-1 {
				t.Error("publication accepted while full")
			}
			expected[id] = true
		}
		close(release)
		waitDisruptorSignal(t, consumed)
		if len(entries) != len(expected) {
			t.Fatalf("delivered %d entries, want %d", len(entries), len(expected))
		}
		for _, value := range entries {
			if !expected[value] {
				t.Fatalf("unexpected or duplicate entry %d", value)
			}
			delete(expected, value)
		}
		if d.Publish(1000) || !d.IsEmpty() {
			t.Fatal("closed buffer accepted work or failed to drain")
		}
	}
}
