package logger

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/abaxoth0/Ain/structs"
)

type faultyFileOutput struct {
	fileLoggerOutput
	write func([]byte) (int, error)
}

func (o *faultyFileOutput) Write(data []byte) (int, error) { return o.write(data) }

func TestFileLoggerStrictStopReportsRecordFailures(t *testing.T) {
	writeErr := errors.New("injected write failure")
	for _, test := range []struct {
		name  string
		meta  structs.Meta
		write func([]byte) (int, error)
		want  error
	}{
		{name: "serialization", meta: structs.Meta{"channel": make(chan int)}},
		{name: "write", write: func([]byte) (int, error) { return 0, writeErr }, want: writeErr},
		{name: "short write", write: func(data []byte) (int, error) { return len(data) - 1, nil }, want: io.ErrShortWrite},
	} {
		t.Run(test.name, func(t *testing.T) {
			l, err := NewFileLogger(&FileLoggerConfig{Path: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			if err := l.Init(); err != nil {
				t.Fatal(err)
			}
			cleanupFileLogger(t, l)
			if test.write != nil {
				l.output = &faultyFileOutput{l.output, test.write}
			}
			if err := l.Start(); err != nil {
				t.Fatal(err)
			}
			NewSource("failure", l).Info("lost", test.meta)
			err = l.Stop(true)
			if err == nil || (test.want != nil && !errors.Is(err, test.want)) {
				t.Errorf("strict Stop discarded record failure: %v", err)
			}
			if err := l.Stop(false); err != nil {
				t.Errorf("non-strict Stop: %v", err)
			}
		})
	}
}

func TestFileLoggerReportsFallbackRejection(t *testing.T) {
	l, err := NewFileLogger(&FileLoggerConfig{Path: t.TempDir()})
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
	cleanupFileLogger(t, l)
	if err := l.fallback.Cancel(); err != nil {
		t.Fatal(err)
	}
	source := NewSource("admission", l)
	for range structs.MinBufferSize {
		source.Info("entry", nil)
	}
	if err := l.Start(); err != nil {
		t.Fatal(err)
	}
	if err := l.Stop(true); err == nil {
		t.Error("strict Stop discarded fallback rejection")
	}
}

func TestFileLoggerFailureObservationIsBounded(t *testing.T) {
	const failures = 1000
	l, err := NewFileLogger(&FileLoggerConfig{Path: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Init(); err != nil {
		t.Fatal(err)
	}
	cleanupFileLogger(t, l)
	if err := l.Start(); err != nil {
		t.Fatal(err)
	}
	source := NewSource("errors", l)
	for range failures {
		source.Info("invalid", structs.Meta{"channel": make(chan int)})
	}
	source.Info("payload-0", nil)
	if err := l.Stop(true); err == nil {
		t.Fatal("strict Stop discarded failures")
	}
	var unsupported *json.UnsupportedTypeError
	if !errors.As(l.Err(), &unsupported) {
		t.Errorf("Err lost serialization cause: %v", l.Err())
	}
	if count := l.failures[fileLoggerSerialization].count; count != failures {
		t.Errorf("retained %d failures, want %d", count, failures)
	}
	notifications := 0
	for err := range l.Errors() {
		notifications++
		if !errors.As(err, &unsupported) {
			t.Errorf("notification lost cause: %v", err)
		}
	}
	if notifications != cap(l.errorEvents) {
		t.Errorf("buffered %d notifications, want %d", notifications, cap(l.errorEvents))
	}
	assertFileLoggerPayloads(t, l.logFile.Name(), 1)
	source.Info("rejected", nil)
	if count := l.failures[fileLoggerAdmission].count; count != 1 {
		t.Errorf("post-stop rejection count: %d", count)
	}
	if err := l.Stop(true); err == nil {
		t.Error("repeated strict Stop lost failures")
	}
}

func TestFileLoggerErrorNotificationWhileRunning(t *testing.T) {
	l, err := NewFileLogger(&FileLoggerConfig{Path: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Init(); err != nil {
		t.Fatal(err)
	}
	cleanupFileLogger(t, l)
	failure := errors.New("writer unavailable")
	l.output = &faultyFileOutput{l.output, func([]byte) (int, error) { return 0, failure }}
	if err := l.Start(); err != nil {
		t.Fatal(err)
	}
	NewSource("errors", l).Info("lost", nil)
	select {
	case err := <-l.Errors():
		if !errors.Is(err, failure) {
			t.Errorf("notification: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("failure not reported while logger running")
	}
	if !errors.Is(l.Err(), failure) {
		t.Errorf("Err: %v", l.Err())
	}
	if err := l.Stop(true); !errors.Is(err, failure) {
		t.Errorf("strict Stop: %v", err)
	}
}

func TestFileLoggerConcurrentAdmissionAndStop(t *testing.T) {
	const publishers, perPublisher = 4, 500
	l, err := NewFileLogger(&FileLoggerConfig{Path: t.TempDir()})
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
	cleanupFileLogger(t, l)
	if err := l.Start(); err != nil {
		t.Fatal(err)
	}
	var producers sync.WaitGroup
	started := make(chan struct{}, publishers)
	release := make(chan struct{})
	for publisher := range publishers {
		producers.Add(1)
		go func() {
			defer producers.Done()
			source := NewSource("concurrent-stop", l)
			source.Info(fmt.Sprintf("payload-%d", publisher*perPublisher), nil)
			started <- struct{}{}
			<-release
			for i := 1; i < perPublisher; i++ {
				source.Info(fmt.Sprintf("payload-%d", publisher*perPublisher+i), nil)
			}
		}()
	}
	for range publishers {
		<-started
	}
	close(release)
	if err := l.Stop(false); err != nil {
		t.Fatal(err)
	}
	producers.Wait()
	NewSource("stopped", l).Info("late", nil)
	file, err := os.Open(l.logFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	seen := map[string]bool{}
	for {
		var entry LogEntry
		if err := decoder.Decode(&entry); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		if seen[entry.Message] {
			t.Fatalf("duplicate record %q", entry.Message)
		}
		seen[entry.Message] = true
	}
	if count := uint64(len(seen)) + l.failures[fileLoggerAdmission].count; count != publishers*perPublisher+1 {
		t.Errorf("persisted + rejected = %d, want %d", count, publishers*perPublisher+1)
	}
	for stage, failure := range l.failures {
		if fileLoggerFailureStage(stage) != fileLoggerAdmission && failure.count != 0 {
			t.Errorf("unexpected %s failures: %v", fileLoggerFailureStage(stage), failure)
		}
	}
	if err := l.Stop(true); err == nil {
		t.Error("strict Stop ignored rejected late record")
	}
}
