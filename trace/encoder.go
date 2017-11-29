package trace

import (
	"encoding/binary"
	"encoding/json"
)

const (
	versionID    = 0
	traceIDField = 0
	spanIDField  = 1
	optsField    = 2

	traceIDLen = 16
	spanIDLen  = 8
	optsLen    = 1

	// Len represents the length of trace context.
	traceContextLen = 1 + 1 + traceIDLen + 1 + spanIDLen + 1 + optsLen
)

// EncodeTrace encodes trace ID, span ID and options into dst. The number of bytes
// written will be returned. If len(dst) isn't big enough to fit the trace context,
// a negative number is returned.
func EncodeTrace(dst []byte, traceID []byte, spanID uint64, opts byte) (n int) {
	if len(dst) < traceContextLen {
		return -1
	}
	var offset = 0
	putByte := func(b byte) { dst[offset] = b; offset++ }
	putUint64 := func(u uint64) { binary.LittleEndian.PutUint64(dst[offset:], u); offset += 8 }

	putByte(versionID)
	putByte(traceIDField)
	for _, b := range traceID {
		putByte(b)
	}
	putByte(spanIDField)
	putUint64(spanID)
	putByte(optsField)
	putByte(opts)

	return offset
}

// DecodeTrace decodes the src into a trace ID, span ID and options. If src doesn't
// contain a valid trace context, ok = false is returned.
func DecodeTrace(src []byte) (traceID []byte, spanID uint64, opts byte, ok bool) {
	if len(src) < traceContextLen {
		return traceID, spanID, 0, false
	}
	var offset = 0
	readByte := func() byte { b := src[offset]; offset++; return b }
	readUint64 := func() uint64 { v := binary.LittleEndian.Uint64(src[offset:]); offset += 8; return v }

	if readByte() != versionID {
		return traceID, spanID, 0, false
	}
	for offset < len(src) {
		switch readByte() {
		case traceIDField:
			traceID = src[offset : offset+traceIDLen]
			offset += traceIDLen
		case spanIDField:
			spanID = readUint64()
		case optsField:
			opts = readByte()
		}
	}
	return traceID, spanID, opts, true
}

// TraceEncoder encode span data
type TraceEncoder interface {
	Encode(t *trace) []byte
}

type TraceEncoderType int

const (
	// JSONEncoderType type of trace encoders
	JSONEncoderType TraceEncoderType = iota
)

func NewTraceEncoder(t EncoderType) TraceEncoder {
	switch t {
	case  JSONEncoderType:
		return &JSONEncoder{}
	default:
		return nil
	}
}

type (
// JSONEncoder encoder for json format
 	JSONEncoder struct {
	
	}
	
	JSONTraceData struct {
		ProjectID 	string 	`json:project`
		TraceID 	string	`json:trace`
		Spans		[]JSONSpanData `json:spans`
	}
	
	JSONSpanData struct {
		StartTime	int64	`json:starttime`
		EndTime 	int64	`json:endtime`
		Name 		string 	`json:name`
		Kind 		string 	`json:kind`
		SpanID 		uint64	`spanid`
		ParentID	uint64	`parentid`
		Labels		map[string]string	`labels`
	}
)

func (r *JSONEncoder) Encode(t *trace) []byte {
	data := JSONTraceData{
		ProjectID: t.client.projectID,
		TraceID: t.traceID,
		Spans: make([]JSONSpanData, len(t.spans)),
	}

	for i := range t.spans {
		s := t.spans[i]
		data.Spans[i] = JSONSpanData{
			StartTime: s.start.Unix(),
			EndTime: s.end.Unix(),
			Name: s.name,
			Kind: s.kind,
			SpanID: s.spanID,
			ParentID: s.parentSpanID,
			Labels: s.labels,
		}
	}

	result, _ := json.Marshal(data)
	return result
}