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
	"sync/atomic"
	"time"

	"github.com/abaxoth0/Ain/structs"
	jsoniter "github.com/json-iterator/go"
)

// Used for this package logs (mostly for errors).
var fileLog = NewSource("LOGGER", Stderr)

const (
	fallbackBatchSize = 500
	fallbackWorkers   = 5
	stopTimeout       = time.Second * 10
)

type FileLoggerConfig struct {
	// Path to the directory with logs
	Path 	 string
	FilePerm os.FileMode

	*LoggerConfig
}

const FileLoggerDefaultFilePerm os.FileMode = 0644

// Satisfies Logger and ForwardingLogger interfaces
type FileLogger struct {
	isInit       bool
	done         chan struct{}
	isRunning    atomic.Bool
	disruptor    *structs.Disruptor[*LogEntry]
	fallback     *structs.WorkerPool
	logger       *log.Logger
	logFile      *os.File
	forwardings  []Logger
	taskProducer func(entry *LogEntry) *logTask
	streamPool   sync.Pool
	config       *FileLoggerConfig
}

func NewFileLogger(config *FileLoggerConfig) (*FileLogger, error) {
	if config == nil {
		return nil, errors.New("FileLogger config is nil")
	}
	info, err := os.Stat(path.Dir(config.Path))
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("Specified file isn't directory")
	}
	if config.FilePerm == 0 {
		config.FilePerm = FileLoggerDefaultFilePerm
	}
	if err := os.MkdirAll(config.Path, config.FilePerm); err != nil {
		return nil, err
	}
	if config.LoggerConfig == nil {
		config.LoggerConfig = DefaultConfig
	}

	logger := &FileLogger{
		done:      make(chan struct{}),
		disruptor: structs.NewDisruptor[*LogEntry](),
		fallback: structs.NewWorkerPool(context.Background(), &structs.WorkerPoolOptions{
			BatchSize:   fallbackBatchSize,
			StopTimeout: stopTimeout,
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

func (l *FileLogger) Init() {
	fileName := fmt.Sprintf(
		"%s:%s[%s].log",
		l.config.ApplicationName, l.config.AppInstance, time.Now().Format(time.RFC3339),
	)

	f, err := os.OpenFile(
		path.Join(l.config.Path,fileName),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		l.config.FilePerm,
	)
	if err != nil {
		fileLog.Fatal("Failed to initialize log module (can't open/create log file)", err.Error(), nil)
	}

	logger := log.New(f, "", log.LstdFlags|log.Lmicroseconds)

	l.logger = logger
	l.logFile = f
	l.taskProducer = newTaskProducer(l)
	l.isInit = true
}

func (l *FileLogger) Start() error {
	if !l.isInit {
		return errors.New("logger isn't initialized")
	}

	if l.isRunning.Load() {
		return errors.New("logger already started")
	}

	// canceled WorkerPool can't be started
	if l.fallback.IsCanceled() {
		l.fallback = structs.NewWorkerPool(context.Background(), &structs.WorkerPoolOptions{
			BatchSize:   fallbackBatchSize,
			StopTimeout: stopTimeout,
		})
	}

	l.isRunning.Store(true)

	go l.disruptor.Consume(l.handler)
	go l.fallback.Start(fallbackWorkers)

	return nil
}

func (l *FileLogger) Stop() error {
	if !l.isRunning.Load() {
		return errors.New("logger isn't started, hence can't be stopped")
	}

	l.isRunning.Store(false)

	l.disruptor.Close()

	disruptorDone := false
	timeout := time.After(stopTimeout)

	for !disruptorDone {
		select {
		case <-timeout:
			fileLog.Error("disruptor processing timeout during shutdown", "", nil)
			disruptorDone = true
		default:
			if l.disruptor.IsEmpty() {
				disruptorDone = true
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
		// Without this all logs will be written in single line
		stream.WriteRaw("\n")
	}

	// NOTE: log.Logger use mutex and atomic operations under the hood,
	//       so it's thread safe by default
	l.logger.Writer().Write(stream.Buffer())
}

func (l *FileLogger) log(entry *LogEntry) {
	if ok := l.disruptor.Publish(entry); ok {
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

	return errors.New("forwarding now found")
}
