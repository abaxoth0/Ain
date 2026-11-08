package logger

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/abaxoth0/Ain/errs"
	"github.com/abaxoth0/Ain/structs"
)

const (
	DefaultFallbackBatchSize = 500
	DefaultFallbackWorkers   = 5
	DefaultStopTimeout       = time.Second * 10
	// Default file permission for log files (rw-r--r--).
	FileLoggerDefaultFilePerm os.FileMode = 0644
	// Default permission for new log directories (rwxr-xr-x).
	FileLoggerDefaultDirPerm os.FileMode = 0755
)

type FileLoggerConfig struct {
	// Path to the directory where log files will be stored
	Path       string
	TimeLayout string
	// File permissions for log files
	FilePerm os.FileMode //Default: 0644
	// Permissions for newly created directories; existing directories are unchanged.
	DirPerm os.FileMode // Default: 0755
	// Amount of goroutines in fallback WorkerPool (which is used only when main ring buffer is overflowed).
	FallbackWorkers    int               // Default: 5
	FallbackBatchSize  int               // Default: 500
	StopTimeout        time.Duration     // Default: 10 sec; To disable set to < 0
	SerializerProducer func() Serializer // Default: func () Serializer { return NewJSONSerializer() }

	*LoggerConfig
}

func (c *FileLoggerConfig) fillEmptySettings() {
	if c.TimeLayout == "" {
		c.TimeLayout = "2006-01-02_15-04-05"
	}
	if c.FilePerm == 0 {
		c.FilePerm = FileLoggerDefaultFilePerm
	}
	if c.DirPerm == 0 {
		c.DirPerm = FileLoggerDefaultDirPerm
	}
	if c.LoggerConfig == nil {
		c.LoggerConfig = defaultLoggerConfig
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
	if c.SerializerProducer == nil {
		c.SerializerProducer = func() Serializer { return NewJSONSerializer() }
	}
}

type fileLoggerState uint8

const (
	fileLoggerCreated fileLoggerState = iota
	fileLoggerRunning
	fileLoggerStopping
	fileLoggerStopped
)

type fileLoggerOutput interface {
	io.WriteCloser
	Sync() error
}

// FileLogger provides asynchronous file logging with forwarding.
// Configure forwarding before logging and keep entries immutable while they may be in use.
type FileLogger struct {
	mu           sync.RWMutex
	errMu        sync.Mutex
	failures     [fileLoggerFailureStages]fileLoggerFailure
	errorEvents  chan error
	errorsClosed bool
	state        fileLoggerState
	config       *FileLoggerConfig
	isInit       bool
	isRunning    atomic.Bool
	done         chan struct{}
	consumerDone chan struct{}
	consumerErr  error
	cleanupErr   error
	buffer       *structs.Disruptor[*LogEntry]
	fallback     *structs.WorkerPool
	output       fileLoggerOutput
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
		errorEvents:  make(chan error, 64),
		done:         make(chan struct{}),
		consumerDone: make(chan struct{}),
		buffer:       structs.NewDisruptor[*LogEntry](nil),
		fallback: structs.NewWorkerPool(context.Background(), &structs.WorkerPoolOptions{
			BatchSize:   config.FallbackBatchSize,
			StopTimeout: config.StopTimeout,
		}),
		forwardings: []Logger{},
		streamPool: sync.Pool{
			New: func() any {
				return config.SerializerProducer()
			},
		},
		config: config,
	}
	logger.taskProducer = newTaskProducer(logger)

	return logger, nil
}

func (l *FileLogger) Init() error {
	l.mu.Lock()
	defer l.mu.Unlock()
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
	if err := os.MkdirAll(l.config.Path, l.config.DirPerm); err != nil {
		return err
	}

	fileName := fmt.Sprintf(
		"%s_%s[%s].log",
		l.config.ApplicationName, l.config.AppInstance, time.Now().Format(l.config.TimeLayout),
	)

	f, err := os.OpenFile( // #nosec G304 -- The log path is supplied by the application's configuration.
		path.Join(l.config.Path, fileName),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		l.config.FilePerm,
	)
	if err != nil {
		return err
	}

	l.output = f
	l.logFile = f
	l.taskProducer = newTaskProducer(l)
	l.isInit = true

	return nil
}

func (l *FileLogger) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.isInit {
		return errors.New("logger isn't initialized")
	}
	if l.state == fileLoggerRunning {
		return errors.New("logger already started")
	}

	if l.state != fileLoggerCreated {
		return errors.New("stopped logger cannot be restarted")
	}

	l.fallback.Start(l.config.FallbackWorkers)
	ready := make(chan struct{})
	go func() {
		defer close(l.consumerDone)
		l.consumerErr = l.buffer.ConsumeWithReady(l.handler, func() { close(ready) })
		l.recordError(fileLoggerConsumer, l.consumerErr)
	}()
	select {
	case <-ready:
		l.state = fileLoggerRunning
		l.isRunning.Store(true)
		return nil
	case <-l.consumerDone:
		l.state = fileLoggerStopping
		go l.finishStop()
		return fmt.Errorf("start file logger consumer: %w", l.consumerErr)
	}
}

// Stop closes admission and waits up to StopTimeout for writers and safe cleanup.
// Timeout wraps errs.StatusTimeout, even in non-strict mode; cleanup continues afterward.
// Later Stop calls may wait again. strict additionally reports processing and sync errors.
func (l *FileLogger) Stop(strict bool) error {
	l.mu.Lock()
	if l.state == fileLoggerCreated {
		l.mu.Unlock()
		return errors.New("logger isn't started, hence can't be stopped")
	}
	if l.state == fileLoggerRunning {
		l.state = fileLoggerStopping
		l.isRunning.Store(false)
		l.buffer.Close()
		go l.finishStop()
	}
	l.mu.Unlock()

	timer := time.NewTimer(l.config.StopTimeout)
	defer timer.Stop()
	select {
	case <-l.done:
		return l.stopResult(strict)
	default:
	}
	select {
	case <-l.done:
		return l.stopResult(strict)
	case <-timer.C:
		select {
		case <-l.done:
			return l.stopResult(strict)
		default:
			return fmt.Errorf("file logger shutdown incomplete: %w", errs.StatusTimeout)
		}
	}
}

func (l *FileLogger) finishStop() {
	_ = l.fallback.Cancel()
	_ = l.fallback.Wait(context.Background())
	<-l.consumerDone
	if l.consumerErr != nil {
		l.cleanupErr = fmt.Errorf("file logger consumer: %w", l.consumerErr)
	}
	if err := l.output.Sync(); err != nil {
		l.recordError(fileLoggerSync, err)
	}
	if err := l.output.Close(); err != nil {
		l.cleanupErr = errors.Join(l.cleanupErr, fmt.Errorf("close log file: %w", err))
		l.recordError(fileLoggerClose, err)
	}
	l.mu.Lock()
	l.state = fileLoggerStopped
	l.errMu.Lock()
	l.errorsClosed = true
	close(l.errorEvents)
	l.errMu.Unlock()
	close(l.done)
	l.mu.Unlock()
}

func (l *FileLogger) stopResult(strict bool) error {
	if strict {
		return l.Err()
	}
	return l.cleanupErr
}

// Done closes after all writers exit and output cleanup finishes.
func (l *FileLogger) Done() <-chan struct{} {
	return l.done
}

func (l *FileLogger) handler(entry *LogEntry) {
	stream := l.streamPool.Get().(Serializer)
	defer l.streamPool.Put(stream)

	stream.Reset()

	if err := stream.WriteVal(entry); err != nil {
		l.recordError(fileLoggerSerialization, err)
		return
	}

	data := append(stream.Buffer(), '\n')
	n, err := l.output.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	l.recordError(fileLoggerWrite, err)
}

func (l *FileLogger) log(entry *LogEntry) {
	l.mu.RLock()
	var err error
	if l.state == fileLoggerStopping || l.state == fileLoggerStopped {
		err = errors.New("logger is stopping or stopped")
	} else if !l.buffer.Publish(entry) {
		err = l.fallback.Push(l.taskProducer(entry))
	}
	l.recordError(fileLoggerAdmission, err)
	l.mu.RUnlock()
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

func (l *FileLogger) AddForwarding(logger Logger) error {
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
