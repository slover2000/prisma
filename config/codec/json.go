package codec

import (
	"encoding/json"
)

// JSONCodec define json codec
type JSONCodec struct {
}

// NewJSONCodec create a instance of json codec
func NewJSONCodec() Codec {
	return &JSONCodec{}
}

// Unmarshal convert bytes to object
func (c *JSONCodec) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// Marshal convert object to bytes
func (c *JSONCodec) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
