package logger

import (
	"errors"
	"fmt"
)

type fileLoggerFailureStage uint8

const (
	fileLoggerAdmission fileLoggerFailureStage = iota
	fileLoggerSerialization
	fileLoggerWrite
	fileLoggerConsumer
	fileLoggerSync
	fileLoggerClose
	fileLoggerFailureStages
)

func (s fileLoggerFailureStage) String() string {
	return [...]string{"admission", "serialization", "write", "consumer", "sync", "close"}[s]
}

type fileLoggerFailure struct {
	count uint64
	first error
}

// Err reports the count and first cause of each kind of failure, retaining bounded state.
// The snapshot can change while logging, including rejected calls after shutdown.
func (l *FileLogger) Err() error {
	l.errMu.Lock()
	defer l.errMu.Unlock()
	var result error
	for stage, failure := range l.failures {
		if failure.count != 0 {
			result = errors.Join(result, fmt.Errorf("file logger %s (%d failures): %w",
				fileLoggerFailureStage(stage), failure.count, failure.first))
		}
	}
	return result
}

// Errors provides buffered, nonblocking failure notifications until cleanup finishes.
// Notifications may be dropped when full; Err retains counts and first causes.
// Calls rejected after cleanup update Err without sending to the closed channel.
func (l *FileLogger) Errors() <-chan error {
	return l.errorEvents
}

func (l *FileLogger) recordError(stage fileLoggerFailureStage, err error) {
	if err == nil {
		return
	}
	l.errMu.Lock()
	defer l.errMu.Unlock()
	failure := &l.failures[stage]
	failure.count++
	if failure.first == nil {
		failure.first = err
	}
	if !l.errorsClosed {
		select {
		case l.errorEvents <- fmt.Errorf("file logger %s: %w", stage, err):
		default:
		}
	}
}
