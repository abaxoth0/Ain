package logger

import (
	"bytes"
	"encoding/json"
)

// Interface for serializing log entries.
// Implementations must be safe for concurrent use from multiple goroutines
// since a single instance may be retrieved from a pool and used concurrently.
type Serializer interface {
	// Clears the serializer's internal buffer, preparing it for a new entry.
	Reset()
	// Serializes the given value to the internal buffer.
	WriteVal(v any) error
	// Returns the serialized data.
	Buffer() []byte
}

// Default Serializer implementation using encoding/json.
// It uses a bytes.Buffer internally which is reset and reused via sync.Pool
// to reduce allocations.
type JSONSerializer struct {
	buffer  *bytes.Buffer
	encoder *json.Encoder
}

func NewJSONSerializer() *JSONSerializer {
	buf := new(bytes.Buffer)
	return &JSONSerializer{
		buffer:  buf,
		encoder: json.NewEncoder(buf),
	}
}

func (s *JSONSerializer) Reset() {
	s.buffer.Reset()
}

func (s *JSONSerializer) Buffer() []byte {
	return s.buffer.Bytes()
}

func (s *JSONSerializer) WriteVal(v any) error {
	if err := s.encoder.Encode(v); err != nil {
		return err
	}
	// json.Encoder.Encode() appends \n at the end, need to trim it
	s.buffer.Truncate(s.buffer.Len() - 1)
	return nil
}
