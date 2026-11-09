package logger

import (
	"encoding/json"
	"io"
	"os"
	"testing"
)

func readFileLoggerEntries(t *testing.T, l *FileLogger) map[string]LogEntry {
	t.Helper()
	file, err := os.Open(l.logFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	entries := map[string]LogEntry{}
	decoder := json.NewDecoder(file)
	for {
		var entry LogEntry
		if err := decoder.Decode(&entry); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		if _, exists := entries[entry.Message]; exists {
			t.Fatalf("duplicate record %q", entry.Message)
		}
		entries[entry.Message] = entry
	}
	return entries
}

func TestSourceFileLoggerIdentityAndForwarding(t *testing.T) {
	originConfig := &LoggerConfig{ApplicationName: "payments", AppInstance: "node-1"}
	targetConfig := &LoggerConfig{ApplicationName: "archive", AppInstance: "node-2"}
	explicitConfig := &LoggerConfig{ApplicationName: "import", AppInstance: "node-3"}
	origin, err := NewFileLogger(&FileLoggerConfig{Path: t.TempDir(), LoggerConfig: originConfig})
	if err != nil {
		t.Fatal(err)
	}
	cleanupFileLogger(t, origin)
	target, err := NewFileLogger(&FileLoggerConfig{Path: t.TempDir(), LoggerConfig: targetConfig})
	if err != nil {
		t.Fatal(err)
	}
	cleanupFileLogger(t, target)
	if err := origin.AddForwarding(target); err != nil {
		t.Fatal(err)
	}
	source := NewSource("identity", origin)
	source.Info("configured", nil)
	explicit := NewLogEntryWithConfig(InfoLogLevel, "explicit", "explicit", "", nil, explicitConfig)
	origin.Log(&explicit)
	empty := NewLogEntryWithConfig(InfoLogLevel, "explicit", "empty", "", nil, &LoggerConfig{})
	origin.Log(&empty)
	defaultEntry := NewLogEntry(InfoLogLevel, "default", "default", "", nil)
	origin.Log(&defaultEntry)
	NewSource("target", target).Info("target", nil)
	for _, l := range []*FileLogger{origin, target} {
		if err := l.Init(); err != nil {
			t.Fatal(err)
		}
		if err := l.Start(); err != nil {
			t.Fatal(err)
		}
	}
	source.Info("running", nil)
	for _, l := range []*FileLogger{origin, target} {
		if err := l.Stop(true); err != nil {
			t.Fatal(err)
		}
		entries := readFileLoggerEntries(t, l)
		want := map[string]*LoggerConfig{
			"configured": originConfig, "running": originConfig,
			"explicit": explicitConfig, "empty": {}, "default": defaultLoggerConfig,
		}
		if l == target {
			want["target"] = targetConfig
		}
		if len(entries) != len(want) {
			t.Fatalf("persisted %d entries, want %d", len(entries), len(want))
		}
		for message, config := range want {
			entry, ok := entries[message]
			if !ok || entry.Service != config.ApplicationName || entry.Instance != config.AppInstance {
				t.Errorf("%q identity = %q/%q, want %q/%q", message,
					entry.Service, entry.Instance, config.ApplicationName, config.AppInstance)
			}
		}
	}
	if explicit.Service != explicitConfig.ApplicationName || explicit.Instance != explicitConfig.AppInstance {
		t.Error("logging mutated an explicitly configured entry")
	}
}
