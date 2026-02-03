package logger

import (
	"log"
	"os"
)

// Satisfies Logger interface
type stderrLogger struct {
	logger *log.Logger
}

func newStderrLogger() *stderrLogger {
	return &stderrLogger{
		// log package sends logs into stderr by default
		// but i want to add prefix to logs and possibility to adjust the flags
		logger: log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime),
	}
}

func (l *stderrLogger) log(entry *LogEntry) {
	msg := "[" + entry.Source + ": \033[" + entry.rawLevel.getColour() + "m" + entry.Level + "\033[0m] " + entry.Message
	if entry.rawLevel >= ErrorLogLevel {
		msg += ": " + entry.Error
	}
	l.logger.Println(msg + " " + entry.Meta.String())
}

func (l *stderrLogger) Log(entry *LogEntry) {
	if ok := preprocess(entry, nil, nil); !ok {
		return
	}

	l.log(entry)

	if entry.rawLevel >= FatalLogLevel {
		handleCritical(entry)
	}
}
