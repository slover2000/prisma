package prisma

import (
	"log"
	
	"golang.org/x/net/context"
	"github.com/sirupsen/logrus"

	"github.com/slover2000/prisma/trace"
	"github.com/slover2000/prisma/logging"
)

// InterceptorClient a client for interceptor 
type InterceptorClient struct {
	trace		*trace.Client
	log			*logging.Client
}

type interceptorOptions struct {
	tracing		tracingOptions
	logging		loggingOptions
}

type loggingOptions struct {
	level				logging.LogLevel
	entry 			*logrus.Entry
}

type tracingOptions struct {
	projectID 	string
	policy    	trace.SamplingPolicy
	collector		trace.Collector	
}

// InterceptorOption represents a interceptor option
type InterceptorOption func(*interceptorOptions)

// WithLogging config log system
func WithLogging(level logging.LogLevel, entry *logrus.Entry) InterceptorOption {
	return func(i *interceptorOptions) { 
		i.logging.level = level
		i.logging.entry = entry
	}
}

// WithTracing config trace system
func WithTracing(project string, policy trace.SamplingPolicy, collector trace.Collector) InterceptorOption {
	return func (i *interceptorOptions) {
		i.tracing.projectID = project
		i.tracing.policy = policy
		i.tracing.collector = collector
	}
}

// NewInterceptorClient create a new interceptor client
func NewInterceptorClient(ctx context.Context, options ...InterceptorOption) (*InterceptorClient, error) {
	intercepOptions := &interceptorOptions{}
	for _, option := range options {
		option(intercepOptions)
	}

	client := &InterceptorClient{}
	
	if len(intercepOptions.tracing.projectID) > 0 {
		traceClient, err := trace.NewClient(ctx, intercepOptions.tracing.projectID)
		if err != nil {
			log.Printf("create trace client failed:%s", err.Error())
			return nil, err
		}
		traceClient.SetSamplingPolicy(intercepOptions.tracing.policy)
		traceClient.SetCollector(intercepOptions.tracing.collector)
		client.trace = traceClient
	}

	if intercepOptions.logging.entry != nil {
		logClient, err := logging.NewClient(intercepOptions.logging.level, intercepOptions.logging.entry)
		if err != nil {
			log.Printf("create log client failed:%s", err.Error())
			return nil, err
		}
		client.log = logClient
	}
	
	return client, nil
}