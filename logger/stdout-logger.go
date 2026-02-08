package logger

import (
	"log"
	"os"
)

type StdOutLogger struct {
	logger *log.Logger
}

func NewStdOutLogger(prefix string) *StdOutLogger {
	return &StdOutLogger{
		logger: log.New(os.Stdout, prefix, log.Ldate|log.Ltime),
	}
}

func (l *StdOutLogger) log(entry *LogEntry) {
	msg := "[" + entry.Source + ": \033[" + entry.rawLevel.getColor() + "m" + entry.Level + "\033[0m] " + entry.Message
	if entry.rawLevel >= ErrorLogLevel {
		msg += ": " + entry.Error
	}
	l.logger.Println(msg + " " + entry.Meta.String())
}

func (l *StdOutLogger) Log(entry *LogEntry) {
	if ok := preprocess(entry, nil, nil); !ok {
		return
	}

	l.log(entry)

	if entry.rawLevel >= FatalLogLevel {
		handleCritical(entry)
	}
}
