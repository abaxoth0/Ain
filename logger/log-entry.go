package logger

import (
	"strings"
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

// Returns colour for SGR sequence (ANSI X3.64)
func (level logLevel) getColour() string {
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
	panic("Unknow logLevel")
}

type LogEntry struct {
	rawLevel  logLevel
	Timestamp time.Time    `json:"ts"`
	Service   string       `json:"service"`
	Instance  string       `json:"instance"`
	Level     string       `json:"level"`
	Source    string       `json:"source,omitempty"`
	Message   string       `json:"msg"`
	Error     string       `json:"error,omitempty"`
	Meta      structs.Meta `json:"meta,omitempty"`
}

var (
	appName     string = "undefined"
	appInstance string = "undefined"
)

func SetApplicationName(name string) {
	name = strings.Trim(name, " \r\n")
	if name == "" {
		return
	}
	appName = name
}

func GetApplicationName() string {
	return appName
}

func SetApplicationInstance(instance string) {
	instance = strings.Trim(instance, " \r\n")
	if instance == "" {
		return
	}
	appInstance = instance
}

func GetApplicationInstance() string {
	return appInstance
}

// Creates a new log entry. Timestamp is time.Now().
// If level is not error, fatal or panic, then Error will be empty, even if err specified.
func NewLogEntry(level logLevel, src string, msg string, err string, meta structs.Meta) LogEntry {
	e := LogEntry{
		rawLevel:  level,
		Timestamp: time.Now(),
		Service:   appName,
		Instance:  appInstance,
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
