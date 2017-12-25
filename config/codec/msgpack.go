package codec

import (
	"github.com/vmihailenco/msgpack"
)

// MsgpackCodec define msgpack codec
type MsgpackCodec struct {
}

// NewMsgpackCodec create a instance of msgpack codec
func NewMsgpackCodec() Codec {
	return &MsgpackCodec{}
}

// Unmarshal convert bytes to object
func (c *MsgpackCodec) Unmarshal(data []byte, v interface{}) error {
	return msgpack.Unmarshal(data, v)
}

// Marshal returns the MessagePack encoding of v.
func (c *MsgpackCodec) Marshal(v interface{}) ([]byte, error) {
	return msgpack.Marshal(v)
}
