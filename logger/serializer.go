package logger

import (
	"bytes"
	"encoding/json"
)

type Serializer interface {
	Reset()
	WriteVal(v any) error
	Buffer() []byte
}

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
