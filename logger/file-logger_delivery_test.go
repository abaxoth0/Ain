package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
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
	file, err := os.Open(l.logFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
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
