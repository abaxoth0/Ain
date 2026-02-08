package logger

import (
	"log"
	"os"
)

type StdErrLogger struct {
	logger *log.Logger
}

func NewStdErrLogger(prefix string) *StdErrLogger {
	return &StdErrLogger{
		// log package sends logs to stderr by default
		// but we want to add prefix to logs and ability to adjust the flags
		logger: log.New(os.Stderr, prefix, log.Ldate|log.Ltime),
	}
}

func (l *StdErrLogger) log(entry *LogEntry) {
	msg := "[" + entry.Source + ": \033[" + entry.rawLevel.getColor() + "m" + entry.Level + "\033[0m] " + entry.Message
	if entry.rawLevel >= ErrorLogLevel {
		msg += ": " + entry.Error
	}
	l.logger.Println(msg + " " + entry.Meta.String())
}

func (l *StdErrLogger) Log(entry *LogEntry) {
	if ok := preprocess(entry, nil, nil); !ok {
		return
	}

	l.log(entry)

	if entry.rawLevel >= FatalLogLevel {
		handleCritical(entry)
	}
}
