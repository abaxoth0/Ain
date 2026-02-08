package logger

import (
	"time"

	"github.com/abaxoth0/Ain/structs"
)

type logLevel uint8

const (
	TraceLogLevel logLevel = iota
	DebugLogLevel
	InfoLogLevel
	WarningLogLevel
	ErrorLogLevel
	// Logs with this level will be handled immediately after calling Log().
	// Logger will call os.Exit(1) after this log.
	FatalLogLevel
	// Logs with this level will be handled immediately after calling Log().
	// Logger will call panic() after this log.
	PanicLogLevel
)

var logLevelToStrMap = map[logLevel]string{
	TraceLogLevel:   "TRACE",
	DebugLogLevel:   "DEBUG",
	InfoLogLevel:    "INFO",
	WarningLogLevel: "WARNING",
	ErrorLogLevel:   "ERROR",
	FatalLogLevel:   "FATAL",
	PanicLogLevel:   "PANIC",
}

func (s logLevel) String() string {
	return logLevelToStrMap[s]
}

// Returns colour code for SGR sequence (ANSI X3.64)
func (level logLevel) getColor() string {
	switch level {
	case TraceLogLevel:
		return "34" // blue
	case DebugLogLevel:
		return "36" // cyan
	case InfoLogLevel:
		return "32" // green
	case WarningLogLevel:
		return "33" // yellow
	case ErrorLogLevel:
		return "31" // red
	case FatalLogLevel, PanicLogLevel:
		return "35" // magenta
	}
	panic("Unknown logLevel")
}

// Represents a single log entry with all its metadata.
type LogEntry struct {
	rawLevel  logLevel     // Internal log level representation
	Timestamp time.Time    `json:"ts"`
	Service   string       `json:"service"`
	Instance  string       `json:"instance"`
	Level     string       `json:"level"`
	Source    string       `json:"source,omitempty"`
	Message   string       `json:"msg"`
	Error     string       `json:"error,omitempty"`
	Meta      structs.Meta `json:"meta,omitempty"`
}

// Configuration for loggers.
type LoggerConfig struct {
	Debug           bool   // Enable debug level logging
	Trace           bool   // Enable trace level logging (most verbose)
	ApplicationName string // Name of the application/service
	AppInstance     string // Instance identifier of the application
}

var defaultLoggerConfig = &LoggerConfig{
	ApplicationName: "undefined",
	AppInstance:     "undefined",
}

// Creates a new log entry. Timestamp is time.Now().
// If level is not error, fatal or panic, then Error will be empty, even if err specified.
func NewLogEntry(level logLevel, src string, msg string, err string, meta structs.Meta) LogEntry {
	return NewLogEntryWithConfig(level, src, msg, err, meta, defaultLoggerConfig)
}

// Will use DefaultConfig if config is nil
func NewLogEntryWithConfig(
	level logLevel,
	src string,
	msg string,
	err string,
	meta structs.Meta,
	config *LoggerConfig,
) LogEntry {
	if config == nil {
		config = defaultLoggerConfig
	}

	e := LogEntry{
		rawLevel:  level,
		Timestamp: time.Now(),
		Service:   config.ApplicationName,
		Instance:  config.AppInstance,
		Level:     level.String(),
		Source:    src,
		Message:   msg,
		Meta:      meta,
	}

	// error, fatal, panic
	if level >= ErrorLogLevel {
		e.Error = err
	}

	return e
}
