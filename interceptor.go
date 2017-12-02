package prisma

import (
	"golang.org/x/net/context"
	"github.com/sirupsen/logrus"
	"github.com/slover2000/prisma/trace"
)

// InterceptorClient a client for interceptor 
type InterceptorClient struct {
	logging		loggingOptions
	tracing		tracingOptions
}

type loggingOptions struct {
	level				trace.LogLevel
	entry 			*logrus.Entry
}

type tracingOptions struct {
	projectID 	string
	policy    	trace.SamplingPolicy
	collector		trace.Collector	
}

// InterceptorOption represents a interceptor option
type InterceptorOption func(*InterceptorClient)

// WithLogging config log system
func WithLogging(level trace.LogLevel, entry *logrus.Entry) InterceptorOption {
	return func(i *InterceptorClient) { 
		i.logging.level = level
		i.logging.entry = entry
	}
}

// WithTracing config trace system
func WithTracing(project string, policy trace.SamplingPolicy, collector trace.Collector) InterceptorOption {
	return func (i *InterceptorClient) {
		i.tracing.projectID = project
		i.tracing.policy = policy
		i.tracing.collector = collector
	}
}

// NewInterceptorClient create a new interceptor client
func NewInterceptorClient(ctx context.Context) (*InterceptorClient, error) {
	client := &InterceptorClient{
		
	}

	return nil, nil
}