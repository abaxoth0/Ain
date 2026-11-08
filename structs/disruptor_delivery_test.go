package structs

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type disruptorPayload struct {
	ID   int
	Data [128]int
}

type observingSeqWaiter struct {
	seqWaiter
	waiting chan struct{}
	once    sync.Once
}

func (w *observingSeqWaiter) WaitFor(seq uint64, cursor *atomic.Uint64) {
	w.once.Do(func() { close(w.waiting) })
	w.seqWaiter.WaitFor(seq, cursor)
}

func TestDisruptorPendingPublication(t *testing.T) {
	d, err := NewDisruptorWithSize[int](MinBufferSize, &DisruptorConfig{IdleCheckInterval: time.Nanosecond})
	if err != nil {
		t.Fatal(err)
	}
	// Pause the first publisher between reservation and payload publication.
	d.writer.Value.Store(1)
	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() {
			d.buffer[0] = 1
			d.available[0].Store(1)
		})
	}
	defer release()
	published := make(chan bool, 1)
	go func() { published <- d.Publish(2) }()
	select {
	case ok := <-published:
		if !ok {
			t.Fatal("later publisher was rejected despite available capacity")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("later publisher waited for an earlier reservation")
	}
	waiting, consumed := make(chan struct{}), make(chan struct{})
	d.waiter = &observingSeqWaiter{seqWaiter: d.waiter, waiting: waiting}
	var entries []int
	go func() {
		defer close(consumed)
		if err := d.Consume(func(value int) { entries = append(entries, value) }); err != nil {
			t.Error(err)
		}
	}()
	waitDisruptorSignal(t, waiting)
	closed := make(chan struct{})
	go func() {
		d.Close()
		close(closed)
	}()
	waitDisruptorSignal(t, closed)
	if d.IsEmpty() || d.Publish(3) {
		t.Error("shutdown lost a reservation or accepted another entry")
	}
	select {
	case <-consumed:
		t.Fatal("consumer exited before the reserved payload was ready")
	default:
	}
	release()
	waitDisruptorSignal(t, consumed)
	if len(entries) != 2 || entries[0] != 1 || entries[1] != 2 {
		t.Fatalf("delivered %v, want [1 2]", entries)
	}
	if !d.IsEmpty() {
		t.Error("shutdown did not drain all reservations")
	}
}

func TestDisruptorSequenceRollover(t *testing.T) {
	d, err := NewDisruptorWithSize[int](MinBufferSize, nil)
	if err != nil {
		t.Fatal(err)
	}
	start := disruptorSequenceMask - 2
	d.writer.Value.Store(start)
	d.reader.Value.Store(start)
	for offset := range MinBufferSize {
		seq := (start - uint64(offset)) & disruptorSequenceMask
		d.available[(seq-1)&d.bufferMask].Store(seq)
	}
	if !d.Publish(1) {
		t.Fatal("could not publish before sequence rollover")
	}
	count := 0
	if err := d.Consume(func(value int) {
		count++
		if value != count {
			t.Errorf("received %d, want %d", value, count)
		}
		if count == MinBufferSize*3 {
			d.Close()
		} else if !d.Publish(count + 1) {
			t.Error("publication rejected during rollover")
			d.Close()
		}
	}); err != nil {
		t.Fatal(err)
	}
	if count != MinBufferSize*3 || !d.IsEmpty() {
		t.Fatalf("drained %d entries across rollover, empty=%v", count, d.IsEmpty())
	}
}

func TestDisruptorPublishDoesNotAllocate(t *testing.T) {
	d, err := NewDisruptorWithSize[disruptorPayload](2048, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	allocs := testing.AllocsPerRun(1000, func() {
		if !d.Publish(disruptorPayload{ID: 1}) {
			t.Fatal("publication unexpectedly rejected")
		}
	})
	if allocs != 0 {
		t.Fatalf("Publish allocated %g objects per call", allocs)
	}
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
