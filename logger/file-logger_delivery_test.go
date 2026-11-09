package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/abaxoth0/Ain/structs"
)

type notifyingJSONSerializer struct {
	*JSONSerializer
	processed chan<- struct{}
}

func (s *notifyingJSONSerializer) WriteVal(v any) error {
	err := s.JSONSerializer.WriteVal(v)
	s.processed <- struct{}{}
	return err
}

func TestFileLoggerConcurrentPayloadDelivery(t *testing.T) {
	const publishers, perPublisher = 4, 1000
	const total = publishers * perPublisher
	processed := make(chan struct{}, total)
	l, err := NewFileLogger(&FileLoggerConfig{
		Path: filepath.Join(t.TempDir(), "logs"),
		SerializerProducer: func() Serializer {
			return &notifyingJSONSerializer{NewJSONSerializer(), processed}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if l.isRunning.Load() {
			_ = l.Stop(false)
		}
		_ = l.logFile.Close()
	})
	if err := l.Start(); err != nil {
		t.Fatal(err)
	}
	var producers sync.WaitGroup
	for publisher := range publishers {
		producers.Add(1)
		go func() {
			defer producers.Done()
			source := NewSource("delivery", l)
			for i := range perPublisher {
				source.Info(fmt.Sprintf("payload-%d", publisher*perPublisher+i), nil)
			}
		}()
	}
	producers.Wait()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for range total {
		select {
		case <-processed:
		case <-timer.C:
			t.Fatal("logger did not process all accepted entries")
		}
	}
	if err := l.Stop(true); err != nil {
		t.Fatal(err)
	}
	assertFileLoggerPayloads(t, l.logFile.Name(), total)
}

func assertFileLoggerPayloads(t *testing.T, name string, total int) {
	t.Helper()
	file, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	decoder := json.NewDecoder(file)
	seen := make(map[string]int, total)
	for {
		var entry LogEntry
		err := decoder.Decode(&entry)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		seen[entry.Message]++
	}
	if len(seen) != total {
		t.Fatalf("persisted %d unique payloads, want %d", len(seen), total)
	}
	for i := range total {
		message := fmt.Sprintf("payload-%d", i)
		if seen[message] != 1 {
			t.Fatalf("%s persisted %d times", message, seen[message])
		}
	}
}

type fileLoggerWriteGate struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func newFileLoggerWriteGate() *fileLoggerWriteGate {
	return &fileLoggerWriteGate{entered: make(chan struct{}, 1), release: make(chan struct{})}
}

func (g *fileLoggerWriteGate) unblock() { g.once.Do(func() { close(g.release) }) }

type gatedJSONSerializer struct {
	*JSONSerializer
	gates map[string]*fileLoggerWriteGate
}

func (s *gatedJSONSerializer) WriteVal(value any) error {
	if gate := s.gates[value.(*LogEntry).Message]; gate != nil {
		select {
		case gate.entered <- struct{}{}:
		default:
		}
		<-gate.release
	}
	return s.JSONSerializer.WriteVal(value)
}

func TestFileLoggerShutdownWaitsForFallbackDrain(t *testing.T) {
	const total = 100
	ring, normal, drain := newFileLoggerWriteGate(), newFileLoggerWriteGate(), newFileLoggerWriteGate()
	defer ring.unblock()
	defer normal.unblock()
	defer drain.unblock()
	gates := map[string]*fileLoggerWriteGate{"payload-0": ring, "payload-63": normal, "payload-64": drain}
	l, err := NewFileLogger(&FileLoggerConfig{
		Path: t.TempDir(), FallbackWorkers: 1, FallbackBatchSize: 1, StopTimeout: 2 * time.Second,
		SerializerProducer: func() Serializer {
			return &gatedJSONSerializer{NewJSONSerializer(), gates}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	l.buffer, err = structs.NewDisruptorWithSize[*LogEntry](structs.MinBufferSize, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if l.isRunning.Load() {
			_ = l.Stop(false)
		}
		_ = l.fallback.Cancel()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := l.fallback.Wait(ctx); err != nil {
			t.Errorf("fallback workers did not exit: %v", err)
		}
		_ = l.logFile.Close()
	})
	if err := l.Start(); err != nil {
		t.Fatal(err)
	}
	source := NewSource("fallback-delivery", l)
	source.Info("payload-0", nil)
	select {
	case <-ring.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("ring consumer did not start")
	}
	for id := 1; id < total; id++ {
		source.Info(fmt.Sprintf("payload-%d", id), nil)
	}
	select {
	case <-normal.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("overflow did not reach fallback processing")
	}
	result := make(chan error, 1)
	go func() { result <- l.Stop(true) }()
	ring.unblock()
	deadline := time.Now().Add(2 * time.Second)
	for !l.fallback.IsCanceled() {
		if time.Now().After(deadline) {
			t.Fatal("logger did not begin fallback cancellation")
		}
		runtime.Gosched()
	}
	normal.unblock()
	select {
	case <-drain.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown did not drain queued fallback entries")
	}
	stopped := false
	select {
	case err := <-result:
		stopped = true
		t.Errorf("Stop returned %v while fallback draining still owned the file", err)
	case <-time.After(20 * time.Millisecond):
	}
	if _, err := l.logFile.Stat(); err != nil {
		t.Errorf("logger closed its output before fallback completion: %v", err)
	}
	drain.unblock()
	if !stopped {
		select {
		case err := <-result:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("logger shutdown did not finish after releasing the fallback task")
		}
	}
	assertFileLoggerPayloads(t, l.logFile.Name(), total)
}
