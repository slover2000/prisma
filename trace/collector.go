package trace

import (
	"log"
)

// Collector represents a trace collector, which is probably a set of
// remote endpoints.
type Collector interface {
	Collect(*Span) error
	Close() error
}

// NopCollector implements Collector but performs no work.
type NopCollector struct{}

// Collect implements Collector.
func (*NopCollector) Collect(*Span) error { return nil }

// Close implements Collector.
func (*NopCollector) Close() error { return nil }

func NewNopCollector() Collector {
	return &NopCollector{}
}

type LogCollector struct {
	encoder TraceEncoder
}

func (r *LogCollector) Collect(span *Span) error {
	log.Println(string(r.encoder.Encode(span)))
	return nil
}

func (r *LogCollector) Close() error {
	return nil
}

func NewLogCollector() Collector {
	return &LogCollector{encoder: NewTraceEncoder(JSONEncoderType)}
}