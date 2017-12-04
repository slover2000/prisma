package prisma

import (
	"log"
	
	"golang.org/x/net/context"
	"github.com/sirupsen/logrus"

	"github.com/slover2000/prisma/trace"
	"github.com/slover2000/prisma/logging"
	"github.com/slover2000/prisma/metrics"
	"github.com/slover2000/prisma/metrics/prometheus"
)

// InterceptorClient a client for interceptor 
type InterceptorClient struct {
	trace				*trace.Client
	log					*logging.Client
	grpcClientMetrics   metrics.ClientMetrics
	grpcServerMetrics   metrics.ClientMetrics
	httpClientMetrics	metrics.ClientMetrics
	httpServerMetrics	metrics.ClientMetrics
}

type interceptorOptions struct {
	tracing		tracingOptions
	logging		loggingOptions
	metrics     metricsOptions
}

type loggingOptions struct {
	level        logging.LogLevel
	entry        *logrus.Entry
}

type tracingOptions struct {
	projectID 	string
	policy      trace.SamplingPolicy
	collector   trace.Collector	
}

type metricsOptions struct {
	projectID 	string
	exposeEp	string
	grpceCient	bool
	grpcServer  bool
	httpClient  bool
	httpServer  bool
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

// WithMetricsProject config metrics system
func WithMetricsProject(project string) InterceptorOption {
	return func (i *interceptorOptions) { i.metrics.projectID = project }
}

// WithMetricseExposeEndpoint config expose endpoint
func WithMetricseExposeEndpoint(endpoint string) InterceptorOption {
	return func (i *interceptorOptions) { i.metrics.exposeEp = endpoint }
}

// EnableGRPCClientMetrics config metrics system
func EnableGRPCClientMetrics(b bool) InterceptorOption {
	return func (i *interceptorOptions) { i.metrics.grpceCient = b }
}

// EnableGRPCServerMetrics config metrics system
func EnableGRPCServerMetrics(b bool) InterceptorOption {
	return func (i *interceptorOptions) { i.metrics.grpcServer = b }
}

// EnableHTTPClientMetrics config metrics system
func EnableHTTPClientMetrics(b bool) InterceptorOption {
	return func (i *interceptorOptions) { i.metrics.httpClient = b }
}

// EnableHTTPServerMetrics config metrics system
func EnableHTTPServerMetrics(b bool) InterceptorOption {
	return func (i *interceptorOptions) { i.metrics.httpServer = b }
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
	
	if len(intercepOptions.metrics.projectID) > 0 {
		if intercepOptions.metrics.grpceCient {
			client.grpcClientMetrics = prometheus.NewGRPCClientPrometheus(intercepOptions.metrics.projectID)
			client.grpcClientMetrics.RegisterHttpHandler(intercepOptions.metrics.exposeEp)
		}

		if intercepOptions.metrics.grpcServer {
			client.grpcServerMetrics = prometheus.NewGRPCServerPrometheus(intercepOptions.metrics.projectID)
			client.grpcServerMetrics.RegisterHttpHandler(intercepOptions.metrics.exposeEp)
		}

		if intercepOptions.metrics.httpClient {
			client.httpClientMetrics = prometheus.NewHTTPClientPrometheus(intercepOptions.metrics.projectID)
			client.grpcServerMetrics.RegisterHttpHandler(intercepOptions.metrics.exposeEp)
		}

		if intercepOptions.metrics.httpServer {
			client.httpServerMetrics = prometheus.NewHTTPServerPrometheus(intercepOptions.metrics.projectID)
			client.httpServerMetrics.RegisterHttpHandler(intercepOptions.metrics.exposeEp)
		}
	}

	return client, nil
}