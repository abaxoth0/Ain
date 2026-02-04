package logger

import (
	"log"
	"os"
)

type stdoutLogger struct {
	logger *log.Logger
}

func newStdoutLogger() stdoutLogger {
	return stdoutLogger{
		logger: log.New(os.Stdout, "", log.Ldate|log.Ltime),
	}
}

func (l stdoutLogger) log(entry *LogEntry) {
	msg := "[" + entry.Source + ": \033[" + entry.rawLevel.getColour() + "m" + entry.Level + "\033[0m] " + entry.Message
	if entry.rawLevel >= ErrorLogLevel {
		msg += ": " + entry.Error
	}
	l.logger.Println(msg + " " + entry.Meta.String())
}

func (l stdoutLogger) Log(entry *LogEntry) {
	if ok := preprocess(entry, nil, nil); !ok {
		return
	}

	l.log(entry)

	if entry.rawLevel >= FatalLogLevel {
		handleCritical(entry)
	}
}
