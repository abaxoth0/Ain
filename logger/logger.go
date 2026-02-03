package logger

import (
	"os"
)

type Logger interface {
	Log(entry *LogEntry)
	// Just logs specified entry.
	// This method mustn't cause any side effects. It's required for ForwardingLogger to work correctly.
	// e.g. entry with panic level won't cause panic when
	// forwarded to another logger, only when main logger will handle it
	log(entry *LogEntry)
}

type ConcurrentLogger interface {
	Logger

	Start() error
	Stop() error
}

// Logger that can forward logs to another loggers.
type ForwardingLogger interface {
	Logger

	// Binds another logger to this logger.
	// On calling Log() it also will be called on all bound loggers
	// (entry will be the same for all loggers)
	//
	// Can't bind to self. Can't bind to one logger more than once.
	NewForwarding(logger Logger) error

	// Removes existing forwarding.
	// Will return error if forwarding to specified logger doesn't exist.
	RemoveForwarding(logger Logger) error
}

// Returns false if log must not be processed.
// Will use DefaultConfig if config is nil.
func preprocess(entry *LogEntry, forwardings []Logger, config *LoggerConfig) bool {
	if config == nil {
		config = DefaultConfig
	}

	if entry.rawLevel == DebugLogLevel && !config.Debug {
		return false
	}

	if entry.rawLevel == TraceLogLevel && !config.Trace {
		return false
	}

	if forwardings != nil && len(forwardings) != 0 {
		for _, forwarding := range forwardings {
			// Must call log() not Log(), since log() just does logging
			// without any additional side effects.
			forwarding.log(entry)
		}
	}

	return true
}

// If log entry rawLevel is:
//   - FatalLogLevel: will call os.Exit(1)
//   - PanicLogLevel: will cause panic with entry.Message and entry.Error
func handleCritical(entry *LogEntry) {
	if entry.rawLevel == PanicLogLevel {
		panic(entry.Message + "\n" + entry.Error)
	}
	os.Exit(1)
}

var (
	Default = func() *FileLogger {
		logger, err := NewFileLogger(&FileLoggerConfig{
			Path: "/tmp/ain-logs",
		})
		if err != nil {
			fileLog.Fatal("Failed to initialize default logger", err.Error(), nil)
		}
		return logger
	}()
	Stdout = newStdoutLogger()
	Stderr = newStderrLogger()
)
