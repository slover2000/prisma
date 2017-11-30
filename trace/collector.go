package trace

import (
	"log"
)

// TraceCollector represents a trace collector, which is probably a set of
// remote endpoints.
type TraceCollector interface {
	Collect(*trace) error
	Close() error	
}

// NopCollector implements Collector but performs no work.
type NopCollector struct{}

// Collect implements Collector.
func (*NopCollector) Collect(*trace) error { return nil }

// Close implements Collector.
func (*NopCollector) Close() error { return nil }

func NewNopCollector() TraceCollector {
	return &NopCollector{}
}

type LogCollector struct {
	encoder TraceEncoder
}

func (r *LogCollector) Collect(t *trace) error {
	log.Println(string(r.encoder.Encode(t)))
	return nil
}

func (r *LogCollector) Close() error {
	return nil
}

func NewLogCollector() TraceCollector {
	return &LogCollector{encoder: NewTraceEncoder(JSONEncoderType)}
}