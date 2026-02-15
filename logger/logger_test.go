package logger

import (
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/abaxoth0/Ain/structs"
)

type mockLogger struct {
	entries []*LogEntry
	mu      sync.Mutex
}

func (m *mockLogger) Log(entry *LogEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, entry)
}

func (m *mockLogger) log(entry *LogEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, entry)
}

func (m *mockLogger) getEntries() []*LogEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]*LogEntry{}, m.entries...)
}

func TestLogLevelString(t *testing.T) {
	t.Run("trace level", func(t *testing.T) {
		if TraceLogLevel.String() != "TRACE" {
			t.Errorf("Expected TRACE, got %s", TraceLogLevel.String())
		}
	})

	t.Run("debug level", func(t *testing.T) {
		if DebugLogLevel.String() != "DEBUG" {
			t.Errorf("Expected DEBUG, got %s", DebugLogLevel.String())
		}
	})

	t.Run("info level", func(t *testing.T) {
		if InfoLogLevel.String() != "INFO" {
			t.Errorf("Expected INFO, got %s", InfoLogLevel.String())
		}
	})

	t.Run("warning level", func(t *testing.T) {
		if WarningLogLevel.String() != "WARNING" {
			t.Errorf("Expected WARNING, got %s", WarningLogLevel.String())
		}
	})

	t.Run("error level", func(t *testing.T) {
		if ErrorLogLevel.String() != "ERROR" {
			t.Errorf("Expected ERROR, got %s", ErrorLogLevel.String())
		}
	})

	t.Run("fatal level", func(t *testing.T) {
		if FatalLogLevel.String() != "FATAL" {
			t.Errorf("Expected FATAL, got %s", FatalLogLevel.String())
		}
	})

	t.Run("panic level", func(t *testing.T) {
		if PanicLogLevel.String() != "PANIC" {
			t.Errorf("Expected PANIC, got %s", PanicLogLevel.String())
		}
	})
}

func TestLogLevelGetColour(t *testing.T) {
	t.Run("trace colour", func(t *testing.T) {
		if TraceLogLevel.getColor() != "34" {
			t.Errorf("Expected 34, got %s", TraceLogLevel.getColor())
		}
	})

	t.Run("debug colour", func(t *testing.T) {
		if DebugLogLevel.getColor() != "36" {
			t.Errorf("Expected 36, got %s", DebugLogLevel.getColor())
		}
	})

	t.Run("info colour", func(t *testing.T) {
		if InfoLogLevel.getColor() != "32" {
			t.Errorf("Expected 32, got %s", InfoLogLevel.getColor())
		}
	})

	t.Run("warning colour", func(t *testing.T) {
		if WarningLogLevel.getColor() != "33" {
			t.Errorf("Expected 33, got %s", WarningLogLevel.getColor())
		}
	})

	t.Run("error colour", func(t *testing.T) {
		if ErrorLogLevel.getColor() != "31" {
			t.Errorf("Expected 31, got %s", ErrorLogLevel.getColor())
		}
	})

	t.Run("fatal colour", func(t *testing.T) {
		if FatalLogLevel.getColor() != "35" {
			t.Errorf("Expected 35, got %s", FatalLogLevel.getColor())
		}
	})

	t.Run("panic colour", func(t *testing.T) {
		if PanicLogLevel.getColor() != "35" {
			t.Errorf("Expected 35, got %s", PanicLogLevel.getColor())
		}
	})
}

func TestNewLogEntry(t *testing.T) {
	t.Run("basic log entry", func(t *testing.T) {
		entry := NewLogEntry(InfoLogLevel, "test_source", "test message", "", nil)

		if entry.Message != "test message" {
			t.Errorf("Expected 'test message', got %s", entry.Message)
		}
		if entry.Source != "test_source" {
			t.Errorf("Expected 'test_source', got %s", entry.Source)
		}
		if entry.Level != "INFO" {
			t.Errorf("Expected 'INFO', got %s", entry.Level)
		}
		if entry.Error != "" {
			t.Errorf("Expected empty error, got %s", entry.Error)
		}
	})

	t.Run("log entry with error", func(t *testing.T) {
		entry := NewLogEntry(ErrorLogLevel, "test_source", "error message", "error details", nil)

		if entry.Message != "error message" {
			t.Errorf("Expected 'error message', got %s", entry.Message)
		}
		if entry.Error != "error details" {
			t.Errorf("Expected 'error details', got %s", entry.Error)
		}
	})

	t.Run("info log entry ignores error", func(t *testing.T) {
		entry := NewLogEntry(InfoLogLevel, "test_source", "info message", "error details", nil)

		if entry.Error != "" {
			t.Errorf("Expected empty error, got %s", entry.Error)
		}
	})

	t.Run("log entry with meta", func(t *testing.T) {
		meta := structs.Meta{"key": "value"}
		entry := NewLogEntry(InfoLogLevel, "test_source", "test message", "", meta)

		if entry.Meta["key"] != "value" {
			t.Errorf("Expected meta 'key' to be 'value'")
		}
	})
}

func TestPreprocess(t *testing.T) {
	t.Run("debug log when debug disabled returns false", func(t *testing.T) {
		originalDebug := defaultLoggerConfig.Debug
		defaultLoggerConfig.Debug = false

		entry := NewLogEntry(DebugLogLevel, "source", "message", "", nil)
		result := preprocess(&entry, nil, nil)
		if result {
			t.Error("Expected false when debug is disabled")
		}
		defaultLoggerConfig.Debug = originalDebug
	})

	t.Run("debug log when debug enabled returns true", func(t *testing.T) {
		originalDebug := defaultLoggerConfig.Debug
		defaultLoggerConfig.Debug = true

		entry := NewLogEntry(DebugLogLevel, "source", "message", "", nil)
		result := preprocess(&entry, nil, nil)
		if !result {
			t.Error("Expected true when debug is enabled")
		}
		defaultLoggerConfig.Debug = originalDebug
	})

	t.Run("trace log when trace disabled returns false", func(t *testing.T) {
		originalTrace := defaultLoggerConfig.Trace
		defaultLoggerConfig.Trace = false

		entry := NewLogEntry(TraceLogLevel, "source", "message", "", nil)
		result := preprocess(&entry, nil, nil)
		if result {
			t.Error("Expected false when trace is disabled")
		}
		defaultLoggerConfig.Trace = originalTrace
	})

	t.Run("trace log when trace enabled returns true", func(t *testing.T) {
		originalTrace := defaultLoggerConfig.Trace
		defaultLoggerConfig.Trace = true

		entry := NewLogEntry(TraceLogLevel, "source", "message", "", nil)
		result := preprocess(&entry, nil, nil)
		if !result {
			t.Error("Expected true when trace is enabled")
		}
		defaultLoggerConfig.Trace = originalTrace
	})

	t.Run("info log always returns true", func(t *testing.T) {
		entry := NewLogEntry(InfoLogLevel, "source", "message", "", nil)
		result := preprocess(&entry, nil, nil)
		if !result {
			t.Error("Expected true for info level")
		}
	})

	t.Run("forwards to all loggers", func(t *testing.T) {
		mock1 := &mockLogger{}
		mock2 := &mockLogger{}
		mock3 := &mockLogger{}
		forwardings := []Logger{mock1, mock2, mock3}

		entry := NewLogEntry(InfoLogLevel, "source", "message", "", nil)
		preprocess(&entry, forwardings, nil)

		if len(mock1.getEntries()) != 1 {
			t.Errorf("Expected 1 entry in mock1, got %d", len(mock1.getEntries()))
		}
		if len(mock2.getEntries()) != 1 {
			t.Errorf("Expected 1 entry in mock2, got %d", len(mock2.getEntries()))
		}
		if len(mock3.getEntries()) != 1 {
			t.Errorf("Expected 1 entry in mock3, got %d", len(mock3.getEntries()))
		}
	})
}

func TestNewSource(t *testing.T) {
	t.Run("create source", func(t *testing.T) {
		mock := &mockLogger{}
		source := NewSource("test_source", mock)

		if source == nil {
			t.Fatal("Source should not be nil")
		}
	})

	t.Run("source info log", func(t *testing.T) {
		mock := &mockLogger{}
		source := NewSource("test_source", mock)

		source.Info("test message", nil)

		entries := mock.getEntries()
		if len(entries) != 1 {
			t.Fatalf("Expected 1 entry, got %d", len(entries))
		}
		if entries[0].Message != "test message" {
			t.Errorf("Expected 'test message', got %s", entries[0].Message)
		}
		if entries[0].Source != "test_source" {
			t.Errorf("Expected 'test_source', got %s", entries[0].Source)
		}
		if entries[0].Level != "INFO" {
			t.Errorf("Expected 'INFO', got %s", entries[0].Level)
		}
	})

	t.Run("source debug log", func(t *testing.T) {
		originalDebug := defaultLoggerConfig.Debug
		defaultLoggerConfig.Debug = true

		mock := &mockLogger{}
		source := NewSource("test_source", mock)

		source.Debug("debug message", nil)

		entries := mock.getEntries()
		if len(entries) != 1 {
			t.Fatalf("Expected 1 entry, got %d", len(entries))
		}
		if entries[0].Level != "DEBUG" {
			t.Errorf("Expected 'DEBUG', got %s", entries[0].Level)
		}
		defaultLoggerConfig.Debug = originalDebug
	})

	t.Run("source trace log", func(t *testing.T) {
		originalTrace := defaultLoggerConfig.Trace
		defaultLoggerConfig.Trace = true

		mock := &mockLogger{}
		source := NewSource("test_source", mock)

		source.Trace("trace message", nil)

		entries := mock.getEntries()
		if len(entries) != 1 {
			t.Fatalf("Expected 1 entry, got %d", len(entries))
		}
		if entries[0].Level != "TRACE" {
			t.Errorf("Expected 'TRACE', got %s", entries[0].Level)
		}
		defaultLoggerConfig.Trace = originalTrace
	})

	t.Run("source warning log", func(t *testing.T) {
		mock := &mockLogger{}
		source := NewSource("test_source", mock)

		source.Warning("warning message", nil)

		entries := mock.getEntries()
		if len(entries) != 1 {
			t.Fatalf("Expected 1 entry, got %d", len(entries))
		}
		if entries[0].Level != "WARNING" {
			t.Errorf("Expected 'WARNING', got %s", entries[0].Level)
		}
	})

	t.Run("source error log", func(t *testing.T) {
		mock := &mockLogger{}
		source := NewSource("test_source", mock)

		source.Error("error message", "error details", nil)

		entries := mock.getEntries()
		if len(entries) != 1 {
			t.Fatalf("Expected 1 entry, got %d", len(entries))
		}
		if entries[0].Level != "ERROR" {
			t.Errorf("Expected 'ERROR', got %s", entries[0].Level)
		}
		if entries[0].Error != "error details" {
			t.Errorf("Expected 'error details', got %s", entries[0].Error)
		}
	})

	t.Run("source fatal log", func(t *testing.T) {
		mock := &mockLogger{}
		source := NewSource("test_source", mock)

		source.Fatal("fatal message", "fatal details", nil)

		entries := mock.getEntries()
		if len(entries) != 1 {
			t.Fatalf("Expected 1 entry, got %d", len(entries))
		}
		if entries[0].Level != "FATAL" {
			t.Errorf("Expected 'FATAL', got %s", entries[0].Level)
		}
		if entries[0].Error != "fatal details" {
			t.Errorf("Expected 'fatal details', got %s", entries[0].Error)
		}
	})

	t.Run("source panic log", func(t *testing.T) {
		mock := &mockLogger{}
		source := NewSource("test_source", mock)

		source.Panic("panic message", "panic details", nil)

		entries := mock.getEntries()
		if len(entries) != 1 {
			t.Fatalf("Expected 1 entry, got %d", len(entries))
		}
		if entries[0].Level != "PANIC" {
			t.Errorf("Expected 'PANIC', got %s", entries[0].Level)
		}
		if entries[0].Error != "panic details" {
			t.Errorf("Expected 'panic details', got %s", entries[0].Error)
		}
	})

	t.Run("source log with meta", func(t *testing.T) {
		mock := &mockLogger{}
		source := NewSource("test_source", mock)

		meta := structs.Meta{"key": "value"}
		source.Info("message", meta)

		entries := mock.getEntries()
		if len(entries) != 1 {
			t.Fatalf("Expected 1 entry, got %d", len(entries))
		}
		if entries[0].Meta["key"] != "value" {
			t.Errorf("Expected meta 'key' to be 'value'")
		}
	})
}

func TestStdoutLogger(t *testing.T) {
	stdout := NewStdOutLogger("")
	t.Run("log entry", func(t *testing.T) {
		entry := NewLogEntry(InfoLogLevel, "test_source", "test message", "", nil)
		stdout.Log(&entry)

		if entry.Message != "test message" {
			t.Errorf("Expected 'test message', got %s", entry.Message)
		}
		if entry.Level != "INFO" {
			t.Errorf("Expected 'INFO', got %s", entry.Level)
		}
	})

	t.Run("log entry with error", func(t *testing.T) {
		entry := NewLogEntry(ErrorLogLevel, "test_source", "error message", "error details", nil)
		stdout.Log(&entry)

		if entry.Message != "error message" {
			t.Errorf("Expected 'error message', got %s", entry.Message)
		}
		if entry.Error != "error details" {
			t.Errorf("Expected 'error details', got %s", entry.Error)
		}
	})
}

func TestStderrLogger(t *testing.T) {
	stderr := NewStdErrLogger("")
	t.Run("log entry", func(t *testing.T) {
		entry := NewLogEntry(InfoLogLevel, "test_source", "test message", "", nil)
		stderr.Log(&entry)

		if entry.Message != "test message" {
			t.Errorf("Expected 'test message', got %s", entry.Message)
		}
		if entry.Level != "INFO" {
			t.Errorf("Expected 'INFO', got %s", entry.Level)
		}
	})
}

func TestFileLogger(t *testing.T) {
	t.Run("new file logger", func(t *testing.T) {
		tmpDir := t.TempDir()
		logger, err := NewFileLogger(&FileLoggerConfig{Path: tmpDir})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if logger == nil {
			t.Fatal("Expected logger to be non-nil")
		}
	})

	t.Run("new file logger with invalid path", func(t *testing.T) {
		invalidPath := "/root/nonexistent/path/that/should/fail"
		logger, err := NewFileLogger(&FileLoggerConfig{Path: invalidPath})
		if err != nil {
			t.Fatal("Unexpected error creating logger")
		}

		err = logger.Init()
		if err == nil {
			t.Error("Expected error for invalid path during init")
		}
	})

	t.Run("init file logger", func(t *testing.T) {
		tmpDir := t.TempDir()
		logger, err := NewFileLogger(&FileLoggerConfig{Path: tmpDir})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		logger.Init()
		if !logger.isInit {
			t.Error("Expected logger to be initialized")
		}
	})

	t.Run("start file logger without init fails", func(t *testing.T) {
		tmpDir := t.TempDir()
		logger, err := NewFileLogger(&FileLoggerConfig{Path: tmpDir})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		err = logger.Start()
		if err == nil {
			t.Error("Expected error when starting uninitialized logger")
		}
	})

	t.Run("log entry", func(t *testing.T) {
		tmpDir := t.TempDir()
		logger, err := NewFileLogger(&FileLoggerConfig{Path: tmpDir})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		logger.Init()
		if err := logger.Start(); err != nil {
			t.Fatalf("Expected no error starting logger, got %v", err)
		}

		entry := NewLogEntry(InfoLogLevel, "test_source", "test message", "", nil)
		logger.Log(&entry)

		time.Sleep(100 * time.Millisecond)
		if err := logger.Stop(true); err != nil {
			t.Errorf("Failed to stop logger: %v\n", err)
		}
	})

	t.Run("stop file logger without start fails", func(t *testing.T) {
		tmpDir := t.TempDir()
		logger, err := NewFileLogger(&FileLoggerConfig{Path: tmpDir})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		logger.Init()
		err = logger.Stop(true)
		if err == nil {
			t.Error("Expected error when stopping unstarted logger")
		}
	})

	t.Run("new forwarding", func(t *testing.T) {
		tmpDir := t.TempDir()
		logger, err := NewFileLogger(&FileLoggerConfig{Path: tmpDir})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		mock := &mockLogger{}
		err = logger.AddForwarding(mock)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if len(logger.forwardings) != 1 {
			t.Errorf("Expected 1 forwarding, got %d", len(logger.forwardings))
		}
	})

	t.Run("new forwarding with nil logger fails", func(t *testing.T) {
		tmpDir := t.TempDir()
		logger, err := NewFileLogger(&FileLoggerConfig{Path: tmpDir})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		err = logger.AddForwarding(nil)
		if err == nil {
			t.Error("Expected error when forwarding to nil logger")
		}
	})

	t.Run("new forwarding to self fails", func(t *testing.T) {
		tmpDir := t.TempDir()
		logger, err := NewFileLogger(&FileLoggerConfig{Path: tmpDir})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		err = logger.AddForwarding(logger)
		if err == nil {
			t.Error("Expected error when forwarding to self")
		}
	})

	t.Run("new forwarding duplicate fails", func(t *testing.T) {
		tmpDir := t.TempDir()
		logger, err := NewFileLogger(&FileLoggerConfig{Path: tmpDir})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		mock := &mockLogger{}
		err = logger.AddForwarding(mock)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		err = logger.AddForwarding(mock)
		if err == nil {
			t.Error("Expected error when adding duplicate forwarding")
		}
	})

	t.Run("remove forwarding", func(t *testing.T) {
		tmpDir := t.TempDir()
		logger, err := NewFileLogger(&FileLoggerConfig{Path: tmpDir})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		mock := &mockLogger{}
		err = logger.AddForwarding(mock)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		err = logger.RemoveForwarding(mock)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if len(logger.forwardings) != 0 {
			t.Errorf("Expected 0 forwardings, got %d", len(logger.forwardings))
		}
	})

	t.Run("remove non-existent forwarding fails", func(t *testing.T) {
		tmpDir := t.TempDir()
		logger, err := NewFileLogger(&FileLoggerConfig{Path: tmpDir})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		mock := &mockLogger{}
		err = logger.RemoveForwarding(mock)
		if err == nil {
			t.Error("Expected error when removing non-existent forwarding")
		}
	})

	t.Run("remove forwarding with nil fails", func(t *testing.T) {
		tmpDir := t.TempDir()
		logger, err := NewFileLogger(&FileLoggerConfig{Path: tmpDir})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		err = logger.RemoveForwarding(nil)
		if err == nil {
			t.Error("Expected error when removing nil forwarding")
		}
	})

	t.Run("log forwards to all loggers", func(t *testing.T) {
		tmpDir := t.TempDir()
		logger, err := NewFileLogger(&FileLoggerConfig{Path: tmpDir})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		mock1 := &mockLogger{}
		mock2 := &mockLogger{}
		logger.AddForwarding(mock1)
		logger.AddForwarding(mock2)

		entry := NewLogEntry(InfoLogLevel, "test_source", "test message", "", nil)
		logger.Log(&entry)

		time.Sleep(100 * time.Millisecond)

		if len(mock1.getEntries()) != 1 {
			t.Errorf("Expected 1 entry in mock1, got %d", len(mock1.getEntries()))
		}
		if len(mock2.getEntries()) != 1 {
			t.Errorf("Expected 1 entry in mock2, got %d", len(mock2.getEntries()))
		}
	})
}

func TestLogTask(t *testing.T) {
	t.Run("process task", func(t *testing.T) {
		tmpDir := t.TempDir()
		logger, err := NewFileLogger(&FileLoggerConfig{Path: tmpDir})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		logger.Init()

		entry := NewLogEntry(InfoLogLevel, "test_source", "test message", "", nil)
		task := logTask{entry: &entry, logger: logger}
		task.Process()
	})
}

func TestHandleCritical(t *testing.T) {
	t.Run("panic level causes panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				if !strings.Contains(r.(string), "test message") {
					t.Errorf("Expected panic to contain 'test message', got %v", r)
				}
			} else {
				t.Error("Expected panic to occur")
			}
		}()

		entry := NewLogEntry(PanicLogLevel, "source", "test message", "error details", nil)
		handleCritical(&entry)
	})
}

type mockSerializer struct {
	resetCalled bool
	writeCalled bool
	data        []byte
}

func (s *mockSerializer) Reset() {
	s.resetCalled = true
	s.data = s.data[:0]
}

func (s *mockSerializer) WriteVal(v any) error {
	s.writeCalled = true
	s.data = append(s.data, `{"test":"data"}`...)
	return nil
}

func (s *mockSerializer) Buffer() []byte {
	return s.data
}

func TestFileLoggerSerializerDI(t *testing.T) {
	t.Run("custom serializer producer is used", func(t *testing.T) {
		tmpDir := t.TempDir()

		producerCalled := false
		logger, err := NewFileLogger(&FileLoggerConfig{
			Path: tmpDir,
			SerializerProducer: func() Serializer {
				producerCalled = true
				return &mockSerializer{}
			},
		})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		logger.Init()
		if err := logger.Start(); err != nil {
			t.Fatalf("Expected no error starting logger, got %v", err)
		}

		entry := NewLogEntry(InfoLogLevel, "test_source", "test message", "", nil)
		logger.Log(&entry)

		time.Sleep(100 * time.Millisecond)
		if err := logger.Stop(true); err != nil {
			t.Errorf("Failed to stop logger: %v\n", err)
		}

		if !producerCalled {
			t.Error("Expected serializer producer to be called")
		}
	})

	t.Run("default serializer when producer is nil", func(t *testing.T) {
		tmpDir := t.TempDir()

		logger, err := NewFileLogger(&FileLoggerConfig{
			Path: tmpDir,
		})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if logger == nil {
			t.Fatal("Expected logger to be non-nil with nil producer")
		}

		logger.Init()
		if err := logger.Start(); err != nil {
			t.Fatalf("Expected no error starting logger, got %v", err)
		}

		entry := NewLogEntry(InfoLogLevel, "test_source", "test message", "", nil)
		logger.Log(&entry)

		time.Sleep(100 * time.Millisecond)
		if err := logger.Stop(true); err != nil {
			t.Errorf("Failed to stop logger: %v\n", err)
		}
	})

	t.Run("serializer receives reset and write calls", func(t *testing.T) {
		tmpDir := t.TempDir()

		serializer := &mockSerializer{}
		logger, err := NewFileLogger(&FileLoggerConfig{
			Path: tmpDir,
			SerializerProducer: func() Serializer {
				return serializer
			},
		})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		logger.Init()
		if err := logger.Start(); err != nil {
			t.Fatalf("Expected no error starting logger, got %v", err)
		}

		entry := NewLogEntry(InfoLogLevel, "test_source", "test message", "", nil)
		logger.Log(&entry)

		time.Sleep(100 * time.Millisecond)
		if err := logger.Stop(true); err != nil {
			t.Errorf("Failed to stop logger: %v\n", err)
		}

		if !serializer.resetCalled {
			t.Error("Expected serializer Reset() to be called")
		}
		if !serializer.writeCalled {
			t.Error("Expected serializer WriteVal() to be called")
		}
	})
}

func TestConcurrentLogging(t *testing.T) {
	t.Run("concurrent source logging", func(t *testing.T) {
		mock := &mockLogger{}
		source := NewSource("test_source", mock)

		var wg sync.WaitGroup
		numGoroutines := 10
		logsPerGoroutine := 100

		for i := range numGoroutines {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for range logsPerGoroutine {
					source.Info("message", nil)
				}
			}(i)
		}

		wg.Wait()

		expectedLogs := numGoroutines * logsPerGoroutine
		actualLogs := len(mock.getEntries())
		if actualLogs != expectedLogs {
			t.Errorf("Expected %d logs, got %d", expectedLogs, actualLogs)
		}
	})

	t.Run("concurrent file logger with forwardings", func(t *testing.T) {
		tmpDir := t.TempDir()
		logger, err := NewFileLogger(&FileLoggerConfig{Path: tmpDir})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		logger.Init()
		if err := logger.Start(); err != nil {
			t.Fatalf("Expected no error starting logger, got %v", err)
		}

		mock1 := &mockLogger{}
		mock2 := &mockLogger{}
		logger.AddForwarding(mock1)
		logger.AddForwarding(mock2)

		source := NewSource("test_source", logger)

		var wg sync.WaitGroup
		numGoroutines := 5
		logsPerGoroutine := 50

		for i := range numGoroutines {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for range logsPerGoroutine {
					source.Info("message", nil)
				}
			}(i)
		}

		wg.Wait()

		time.Sleep(200 * time.Millisecond)

		expectedLogs := numGoroutines * logsPerGoroutine
		if len(mock1.getEntries()) != expectedLogs {
			t.Errorf("Expected %d logs in mock1, got %d", expectedLogs, len(mock1.getEntries()))
		}
		if len(mock2.getEntries()) != expectedLogs {
			t.Errorf("Expected %d logs in mock2, got %d", expectedLogs, len(mock2.getEntries()))
		}

		if err := logger.Stop(true); err != nil {
			t.Errorf("Failed to stop logger: %v\n", err)
		}
	})
}

func TestFileLoggerStart(t *testing.T) {
	t.Run("start returns error when already started", func(t *testing.T) {
		tmpDir := t.TempDir()
		logger, err := NewFileLogger(&FileLoggerConfig{Path: tmpDir})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		logger.Init()

		done := make(chan bool)
		go func() {
			logger.Start()
			done <- true
		}()

		time.Sleep(100 * time.Millisecond)
		if !logger.isRunning.Load() {
			t.Error("Expected logger to be running")
		}

		err = logger.Start()
		if err == nil {
			t.Error("Expected error when starting already started logger")
		}

		if err := logger.Stop(true); err != nil {
			t.Errorf("Failed to stop logger: %v\n", err)
		}
		<-done
	})

	t.Run("start creates log file", func(t *testing.T) {
		tmpDir := t.TempDir()
		if tmpDir[len(tmpDir)-1] != '/' {
			tmpDir += "/"
		}
		logger, err := NewFileLogger(&FileLoggerConfig{Path: tmpDir})
		if err != nil {
			t.Fatalf("Expected no error, get %v", err)
		}

		logger.Init()

		done := make(chan bool)
		go func() {
			logger.Start()
			done <- true
		}()

		time.Sleep(100 * time.Millisecond)

		if err := logger.Stop(true); err != nil {
			t.Errorf("Failed to stop logger: %v\n", err)
		}
		<-done

		files, err := os.ReadDir(tmpDir)
		if err != nil {
			t.Fatalf("Expected no error reading directory, got %v", err)
		}
		if len(files) != 1 {
			t.Errorf("Expected 1 log file, got %d", len(files))
		}
		if len(files) > 0 && !strings.HasSuffix(files[0].Name(), ".log") {
			t.Errorf("Expected file with .log extension, got %s", files[0].Name())
		}
	})
}
