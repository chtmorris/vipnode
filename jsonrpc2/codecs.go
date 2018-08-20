package jsonrpc2

import (
	"encoding/json"
	"io"
)

// TODO: Does WriteMessage need to be mutexed?

// Codec is an straction for receiving and sending JSONRPC messages.
type Codec interface {
	ReadMessage() (*Message, error)
	WriteMessage(*Message) error
	Close() error
}

var _ Codec = &jsonCodec{}

// IOCodec returns a Codec that wraps JSON encoding and decoding over IO.
func IOCodec(rwc io.ReadWriteCloser) *jsonCodec {
	return &jsonCodec{
		dec:    json.NewDecoder(rwc),
		enc:    json.NewEncoder(rwc),
		closer: rwc,
	}
}

type jsonCodec struct {
	dec    *json.Decoder
	enc    *json.Encoder
	closer io.Closer
}

func (codec *jsonCodec) ReadMessage() (*Message, error) {
	var msg Message
	err := codec.dec.Decode(&msg)
	return &msg, err
}

func (codec *jsonCodec) WriteMessage(msg *Message) error {
	return codec.enc.Encode(msg)
}

func (codec *jsonCodec) Close() error {
	return codec.closer.Close()
}
