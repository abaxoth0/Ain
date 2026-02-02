package logger

import "github.com/abaxoth0/Ain/structs"

// Wrapper for logger L.
// Strictly bound to the single logger's source.
// Provides more convenient and readable methods for creating logs.
type Source[L Logger] struct {
	logger L
	src    string
}

func NewSource[L Logger](src string, logger L) *Source[L] {
	return &Source[L]{
		src:    src,
		logger: logger,
	}
}

func (s *Source[L]) log(level logLevel, msg string, err string, meta structs.Meta) {
	entry := NewLogEntry(level, s.src, msg, err, meta)
	s.logger.Log(&entry)
}

// Will create log only if trace logs are enabled,
// Same as L.Log(), but sets level to the TraceLogLevel.
func (s *Source[L]) Trace(msg string, meta structs.Meta) {
	s.log(TraceLogLevel, msg, "", meta)
}

// Will create log only if app running in debug mode,
// Same as L.Log(), but sets level to the DebugLogLevel.
func (s *Source[L]) Debug(msg string, meta structs.Meta) {
	s.log(DebugLogLevel, msg, "", meta)
}

// Same as L.Log(), but sets level to the InfoLogLevel.
func (s *Source[L]) Info(msg string, meta structs.Meta) {
	s.log(InfoLogLevel, msg, "", meta)
}

// Same as L.Log(), but sets level to the WarningLogLevel.
func (s *Source[L]) Warning(msg string, meta structs.Meta) {
	s.log(WarningLogLevel, msg, "", meta)
}

// Same as L.Log(), but sets level to the ErrorLogLevel.
func (s *Source[L]) Error(msg string, err string, meta structs.Meta) {
	s.log(ErrorLogLevel, msg, err, meta)
}

// Same as L.Log(), but sets level to the FatalLogLevel
func (s *Source[L]) Fatal(msg string, err string, meta structs.Meta) {
	s.log(FatalLogLevel, msg, err, meta)
}

// Same as L.Log(), but sets level to the PanicLogLevel
func (s *Source[L]) Panic(msg string, err string, meta structs.Meta) {
	s.log(PanicLogLevel, msg, err, meta)
}
