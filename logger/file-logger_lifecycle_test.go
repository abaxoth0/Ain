package logger

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/abaxoth0/Ain/errs"
	"github.com/abaxoth0/Ain/structs"
)

func cleanupFileLogger(t *testing.T, l *FileLogger) {
	t.Helper()
	t.Cleanup(func() {
		l.mu.RLock()
		state := l.state
		l.mu.RUnlock()
		if state == fileLoggerCreated {
			_ = l.fallback.Cancel()
			if l.logFile != nil {
				_ = l.logFile.Close()
			}
			return
		}
		_ = l.Stop(false)
		select {
		case <-l.Done():
		case <-time.After(2 * time.Second):
			t.Error("logger cleanup did not finish")
		}
	})
}

func TestFileLoggerImmediateStartLogStop(t *testing.T) {
	previous := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(previous)
	l, err := NewFileLogger(&FileLoggerConfig{Path: t.TempDir(), StopTimeout: time.Second})
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
	NewSource("immediate", l).Info("payload-0", nil)
	if err := l.Stop(true); err != nil {
		t.Fatalf("immediate shutdown failed: %v", err)
	}
	assertFileLoggerPayloads(t, l.logFile.Name(), 1)
}

func TestFileLoggerStopTimeoutCanFinishCleanup(t *testing.T) {
	for _, strict := range []bool{true, false} {
		gate := newFileLoggerWriteGate()
		defer gate.unblock()
		l, err := NewFileLogger(&FileLoggerConfig{
			Path: t.TempDir(), StopTimeout: 20 * time.Millisecond,
			SerializerProducer: func() Serializer {
				return &gatedJSONSerializer{NewJSONSerializer(), map[string]*fileLoggerWriteGate{"payload-0": gate}}
			},
		})
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
		NewSource("blocked", l).Info("payload-0", nil)
		select {
		case <-gate.entered:
		case <-time.After(2 * time.Second):
			t.Fatal("serializer did not start")
		}
		if err := l.Stop(strict); !errors.Is(err, errs.StatusTimeout) {
			t.Errorf("strict=%v Stop returned %v, want timeout", strict, err)
		}
		if _, err := l.logFile.Stat(); err != nil {
			t.Errorf("strict=%v timeout closed an active writer's file: %v", strict, err)
		}
		gate.unblock()
		deadline := time.Now().Add(2 * time.Second)
		for {
			err := l.Stop(true)
			if err == nil {
				break
			}
			if !errors.Is(err, errs.StatusTimeout) || time.Now().After(deadline) {
				t.Fatalf("retry could not finish cleanup: %v", err)
			}
		}
		if _, err := l.logFile.Write(nil); !errors.Is(err, os.ErrClosed) {
			t.Errorf("completed shutdown did not close the file: %v", err)
		}
		assertFileLoggerPayloads(t, l.logFile.Name(), 1)
	}
}

type lifecycleFileOutput struct {
	*os.File
	syncErr  error
	closeErr error
	closes   atomic.Int32
}

func (o *lifecycleFileOutput) Sync() error { return o.syncErr }

func (o *lifecycleFileOutput) Close() error {
	o.closes.Add(1)
	return errors.Join(o.File.Close(), o.closeErr)
}

func TestFileLoggerCleanupRetainsSyncAndCloseErrors(t *testing.T) {
	for _, closeFailure := range []bool{false, true} {
		l, err := NewFileLogger(&FileLoggerConfig{Path: t.TempDir()})
		if err != nil {
			t.Fatal(err)
		}
		if err := l.Init(); err != nil {
			t.Fatal(err)
		}
		cleanupFileLogger(t, l)
		syncErr, closeErr := errors.New("sync failed"), errors.New("close failed")
		output := &lifecycleFileOutput{File: l.logFile, syncErr: syncErr}
		if closeFailure {
			output.closeErr = closeErr
		}
		l.output = output
		if err := l.Start(); err != nil {
			t.Fatal(err)
		}
		NewSource("cleanup", l).Info("payload-0", nil)
		if err := l.Stop(true); !errors.Is(err, syncErr) || (closeFailure && !errors.Is(err, closeErr)) {
			t.Errorf("strict Stop lost cleanup failures: %v", err)
		}
		if _, err := l.logFile.Write(nil); !errors.Is(err, os.ErrClosed) {
			t.Errorf("sync error skipped closing the file: %v", err)
		}
		if err := l.Stop(false); (err != nil) != closeFailure || (closeFailure && !errors.Is(err, closeErr)) {
			t.Errorf("non-strict Stop returned unexpected cleanup result: %v", err)
		}
		if count := output.closes.Load(); count != 1 {
			t.Errorf("cleanup closed the output %d times", count)
		}
		assertFileLoggerPayloads(t, l.logFile.Name(), 1)
	}
}

func TestFileLoggerConsumerStartFailureCleansUp(t *testing.T) {
	l, err := NewFileLogger(&FileLoggerConfig{Path: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Init(); err != nil {
		t.Fatal(err)
	}
	cleanupFileLogger(t, l)
	l.buffer.Close()
	if err := l.Start(); err == nil {
		t.Fatal("Start ignored consumer failure")
	}
	if err := l.Stop(true); err == nil || !errors.Is(err, l.consumerErr) {
		t.Errorf("Stop lost consumer startup error: %v", err)
	}
	if _, err := l.logFile.Write(nil); !errors.Is(err, os.ErrClosed) {
		t.Errorf("consumer failure leaked the file: %v", err)
	}
	select {
	case <-l.fallback.Done():
	default:
		t.Error("consumer failure left fallback workers active")
	}
}

func TestFileLoggerIdleShutdownAndRestartRejection(t *testing.T) {
	for range 25 {
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
		if err := l.Stop(true); err != nil {
			t.Fatal(err)
		}
		for _, done := range []<-chan struct{}{l.Done(), l.consumerDone, l.fallback.Done()} {
			select {
			case <-done:
			default:
				t.Error("idle Stop returned before owned processing exited")
			}
		}
		if err := l.Start(); err == nil {
			t.Error("stopped logger restarted closed resources")
		}
	}
}

type gatedFileOutput struct {
	fileLoggerOutput
	stage string
	gate  *fileLoggerWriteGate
}

func (o *gatedFileOutput) wait(stage string) {
	if o.stage == stage {
		o.gate.entered <- struct{}{}
		<-o.gate.release
	}
}

func (o *gatedFileOutput) Write(data []byte) (int, error) {
	o.wait("write")
	return o.fileLoggerOutput.Write(data)
}

func (o *gatedFileOutput) Sync() error {
	o.wait("sync")
	return o.fileLoggerOutput.Sync()
}

func (o *gatedFileOutput) Close() error {
	o.wait("close")
	return o.fileLoggerOutput.Close()
}

func TestFileLoggerTimeoutWhileOutputBlocked(t *testing.T) {
	for _, stage := range []string{"write", "sync", "close"} {
		t.Run(stage, func(t *testing.T) {
			gate := newFileLoggerWriteGate()
			defer gate.unblock()
			l, err := NewFileLogger(&FileLoggerConfig{Path: t.TempDir(), StopTimeout: 20 * time.Millisecond})
			if err != nil {
				t.Fatal(err)
			}
			if err := l.Init(); err != nil {
				t.Fatal(err)
			}
			cleanupFileLogger(t, l)
			l.output = &gatedFileOutput{l.output, stage, gate}
			if err := l.Start(); err != nil {
				t.Fatal(err)
			}
			NewSource("output", l).Info("payload-0", nil)
			result := make(chan error, 1)
			go func() { result <- l.Stop(false) }()
			select {
			case <-gate.entered:
			case <-time.After(2 * time.Second):
				t.Fatal("output did not reach blocked operation")
			}
			select {
			case err := <-result:
				if !errors.Is(err, errs.StatusTimeout) {
					t.Errorf("Stop: %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("blocked output made Stop unbounded")
			}
			select {
			case <-l.Done():
				t.Error("cleanup reported completion with output blocked")
			default:
			}
			if _, err := l.logFile.Stat(); err != nil {
				t.Errorf("closed active output: %v", err)
			}
			gate.unblock()
			select {
			case <-l.Done():
			case <-time.After(2 * time.Second):
				t.Fatal("cleanup did not finish after releasing output")
			}
			if err := l.Stop(true); err != nil {
				t.Fatal(err)
			}
			assertFileLoggerPayloads(t, l.logFile.Name(), 1)
		})
	}
}

func TestFileLoggerStartDoesNotWaitForQueuedSerialization(t *testing.T) {
	gate := newFileLoggerWriteGate()
	defer gate.unblock()
	l, err := NewFileLogger(&FileLoggerConfig{
		Path: t.TempDir(),
		SerializerProducer: func() Serializer {
			return &gatedJSONSerializer{NewJSONSerializer(), map[string]*fileLoggerWriteGate{"payload-0": gate}}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	cleanupFileLogger(t, l)
	NewSource("queued", l).Info("payload-0", nil)
	if err := l.Init(); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { result <- l.Start() }()
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start waited for queued serialization")
	}
	select {
	case <-gate.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("queued record did not reach consumer")
	}
	gate.unblock()
	if err := l.Stop(true); err != nil {
		t.Fatal(err)
	}
	assertFileLoggerPayloads(t, l.logFile.Name(), 1)
}

func TestFileLoggerTimeoutKeepsFallbackOutputOpen(t *testing.T) {
	const total = structs.MinBufferSize
	ring, overflow := newFileLoggerWriteGate(), newFileLoggerWriteGate()
	defer ring.unblock()
	defer overflow.unblock()
	gates := map[string]*fileLoggerWriteGate{"payload-0": ring, "payload-63": overflow}
	l, err := NewFileLogger(&FileLoggerConfig{
		Path: t.TempDir(), FallbackWorkers: 1, FallbackBatchSize: 1, StopTimeout: 20 * time.Millisecond,
		SerializerProducer: func() Serializer { return &gatedJSONSerializer{NewJSONSerializer(), gates} },
	})
	if err != nil {
		t.Fatal(err)
	}
	l.buffer, err = structs.NewDisruptorWithSize[*LogEntry](total, nil)
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
	source := NewSource("fallback-timeout", l)
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
	case <-overflow.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("fallback writer did not start")
	}
	ring.unblock()
	for range 2 {
		if err := l.Stop(false); !errors.Is(err, errs.StatusTimeout) {
			t.Fatalf("Stop: %v, want timeout", err)
		}
		if _, err := l.logFile.Stat(); err != nil {
			t.Fatalf("closed active fallback output: %v", err)
		}
	}
	overflow.unblock()
	select {
	case <-l.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("cleanup did not finish after fallback release")
	}
	if err := l.Stop(true); err != nil {
		t.Fatal(err)
	}
	assertFileLoggerPayloads(t, l.logFile.Name(), total)
}
