package logger

// Task for worker pool
type logTask struct {
	entry  *LogEntry
	logger *FileLogger
}

func (t logTask) Process() {
	t.logger.handler(t.entry)
}

func newTaskProducer(logger *FileLogger) func(*LogEntry) *logTask {
	return func(entry *LogEntry) *logTask {
		return &logTask{entry, logger}
	}
}
