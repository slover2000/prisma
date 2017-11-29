package trace

import (
	"log"
)

// TraceRecorder ...
type TraceRecorder interface {
	Record(t *trace)
}

type TraceLogRecorder struct {
	encoder TraceEncoder
}

func (r *TraceLogRecorder) Record(t *trace) {
	log.Println(string(r.encoder.Encode(t)))
}

func NewLogRecorder() TraceRecorder {
	return &TraceLogRecorder{encoder: NewTraceEncoder(JSONEncoderType)}
}