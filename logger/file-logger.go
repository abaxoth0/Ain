package logger

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"slices"
	"sync"
	"time"

	"github.com/abaxoth0/Ain/structs"
	jsoniter "github.com/json-iterator/go"
)

// Used for this package logs (mostly for errors).
var fileLog = NewSource("LOGGER", Stderr)

const (
	DefaultFallbackBatchSize = 500
	DefaultFallbackWorkers   = 5
	DefaultStopTimeout       = time.Second * 10
	// Default file permission for log files (rw-r--r--).
	FileLoggerDefaultFilePerm os.FileMode = 0644
)

type FileLoggerConfig struct {
	// Path to the directory where log files will be stored
	Path string
	// File permissions for log files
	FilePerm os.FileMode //Default: 0644
	// Amount of goroutines in fallback WorkerPool (which is used only when main ring buffer is overflowed).
	FallbackWorkers   int           // Default: 5
	FallbackBatchSize int           // Default: 500
	StopTimeout       time.Duration // Default: 10 sec; To disable set to < 0

	*LoggerConfig
}

func (c *FileLoggerConfig) fillEmptySettings() {
	if c.FilePerm == 0 {
		c.FilePerm = FileLoggerDefaultFilePerm
	}
	if c.LoggerConfig == nil {
		c.LoggerConfig = DefaultConfig
	}
	if c.FallbackWorkers <= 0 {
		c.FallbackWorkers = DefaultFallbackWorkers
	}
	if c.FallbackBatchSize <= 0 {
		c.FallbackBatchSize = DefaultFallbackBatchSize
	}
	if c.StopTimeout == 0 {
		c.StopTimeout = DefaultStopTimeout
	}
	// To disable timeout just set it to maximum possible value for time.Duration
	if c.StopTimeout < 0 {
		c.StopTimeout = time.Duration((1 << 63) - 1)
	}
}

// Implements concurrent file-based logging with forwarding capabilities.
type FileLogger struct {
	config       *FileLoggerConfig
	isInit       bool
	isRunning    bool
	done         chan struct{}
	buffer       *structs.Disruptor[*LogEntry]
	fallback     *structs.WorkerPool
	logger       *log.Logger
	logFile      *os.File
	forwardings  []Logger
	taskProducer func(entry *LogEntry) *logTask
	streamPool   sync.Pool
}

func NewFileLogger(config *FileLoggerConfig) (*FileLogger, error) {
	if config == nil {
		return nil, errors.New("FileLogger config is nil")
	}

	config.fillEmptySettings()

	logger := &FileLogger{
		done:   make(chan struct{}),
		buffer: structs.NewDisruptor[*LogEntry](),
		fallback: structs.NewWorkerPool(context.Background(), &structs.WorkerPoolOptions{
			BatchSize:   config.FallbackBatchSize,
			StopTimeout: config.StopTimeout,
		}),
		forwardings: []Logger{},
		streamPool: sync.Pool{
			New: func() any {
				return jsoniter.NewStream(jsoniter.ConfigFastest, nil, 1024)
			},
		},
		config: config,
	}
	logger.taskProducer = newTaskProducer(logger)

	return logger, nil
}

func (l *FileLogger) Init() error {
	if l.isInit {
		return errors.New("logger already initialized")
	}

	info, err := os.Stat(path.Dir(l.config.Path))
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("file at Path isn't directory")
	}
	if err := os.MkdirAll(l.config.Path, l.config.FilePerm); err != nil {
		return err
	}

	fileName := fmt.Sprintf(
		"%s:%s[%s].log",
		l.config.ApplicationName, l.config.AppInstance, time.Now().Format(time.RFC3339),
	)

	f, err := os.OpenFile(
		path.Join(l.config.Path, fileName),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		l.config.FilePerm,
	)
	if err != nil {
		return err
	}

	logger := log.New(f, "", log.LstdFlags|log.Lmicroseconds)

	l.logger = logger
	l.logFile = f
	l.taskProducer = newTaskProducer(l)
	l.isInit = true

	return nil
}

func (l *FileLogger) Start() error {
	if !l.isInit {
		return errors.New("logger isn't initialized")
	}
	if l.isRunning {
		return errors.New("logger already started")
	}

	// canceled WorkerPool can't be started
	if l.fallback.IsCanceled() {
		l.fallback = structs.NewWorkerPool(context.Background(), &structs.WorkerPoolOptions{
			BatchSize:   l.config.FallbackBatchSize,
			StopTimeout: l.config.StopTimeout,
		})
	}

	l.isRunning = true

	go l.buffer.Consume(l.handler)
	go l.fallback.Start(l.config.FallbackWorkers)

	return nil
}

func (l *FileLogger) Stop() error {
	if !l.isRunning {
		return errors.New("logger isn't started, hence can't be stopped")
	}

	l.isRunning = false

	l.buffer.Close()

	bufferProcessed := false
	timeout := time.After(l.config.StopTimeout)

	for !bufferProcessed {
		select {
		case <-timeout:
			fileLog.Error("buffer processing timeout during shutdown", "", nil)
			bufferProcessed = true
		default:
			if l.buffer.IsEmpty() {
				bufferProcessed = true
			} else {
				time.Sleep(time.Millisecond * 10)
			}
		}
	}

	if err := l.fallback.Cancel(); err != nil {
		return err
	}

	// Flush any remaining data in the file buffer
	if err := l.logger.Writer().(*os.File).Sync(); err != nil {
		fileLog.Error("failed to sync log file during shutdown", err.Error(), nil)
	}

	if err := l.logFile.Close(); err != nil {
		return err
	}

	close(l.done)

	return nil
}

func (l *FileLogger) handler(entry *LogEntry) {
	stream := l.streamPool.Get().(*jsoniter.Stream)
	defer l.streamPool.Put(stream)

	stream.Reset(nil)
	stream.Error = nil

	stream.WriteVal(entry)
	if stream.Error != nil {
		fileLog.Error("failed to write log", stream.Error.Error(), nil)
		return
	}

	if stream.Buffered() > 0 {
		// Add newline to ensure each log entry is on its own line
		stream.WriteRaw("\n")
	}

	// NOTE:
	// Logger from built-in "log" package uses mutexes and atomic operations
	// under the hood, so it's already thread safe.
	l.logger.Writer().Write(stream.Buffer())
}

func (l *FileLogger) log(entry *LogEntry) {
	if ok := l.buffer.Publish(entry); ok {
		return
	}
	l.fallback.Push(l.taskProducer(entry))
}

func (l *FileLogger) Log(entry *LogEntry) {
	if !preprocess(entry, l.forwardings, l.config.LoggerConfig) {
		return
	}

	l.log(entry)

	if entry.rawLevel >= FatalLogLevel {
		handleCritical(entry)
	}
}

func (l *FileLogger) NewForwarding(logger Logger) error {
	if logger == nil {
		return errors.New("received nil instead of logger")
	}
	if l == logger {
		return errors.New("can't forward logs to self")
	}
	if slices.Contains(l.forwardings, logger) {
		return errors.New("logger already has this forwarding")
	}

	l.forwardings = append(l.forwardings, logger)

	return nil
}

func (l *FileLogger) RemoveForwarding(logger Logger) error {
	if logger == nil {
		return errors.New("received nil instead of Logger")
	}

	for idx, forwarding := range l.forwardings {
		if forwarding == logger {
			l.forwardings = slices.Delete(l.forwardings, idx, idx+1)
			return nil
		}
	}

	return errors.New("forwarding not found")
}
