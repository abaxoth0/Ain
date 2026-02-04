package logger

import (
	"os"
)

// Interface for all loggers.
type Logger interface {
	// Processes a log entry according to the logger's implementation.
	Log(entry *LogEntry)

	// Internal method that performs the actual logging without side effects.
	// This method must not cause any side effects. It's required for ForwardingLogger to work correctly.
	// For example, an entry with panic level won't cause panic when
	// forwarded to another logger, only when the main logger handles it.
	log(entry *LogEntry)
}

// Extends Logger with lifecycle management methods.
type ConcurrentLogger interface {
	Logger

	// Begins the logger's background processing.
	Start() error
	// Gracefully shuts down the logger and waits for pending operations.
	Stop() error
}

// Can forward logs to other loggers.
type ForwardingLogger interface {
	Logger

	// Binds another logger to this logger.
	// On calling Log() it will also be called on all bound loggers
	// (entry will be the same for all loggers)
	//
	// Can't bind to self. Can't bind to one logger more than once.
	NewForwarding(logger Logger) error

	// Removes existing forwarding.
	// Returns error if forwarding to specified logger doesn't exist.
	RemoveForwarding(logger Logger) error
}

// Filters log entries and handles forwarding.
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

// Processes log entries with Fatal or Panic levels.
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
	Stdout = newStdoutLogger()
	Stderr = newStderrLogger()
)
