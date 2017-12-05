package prisma

import (
	"fmt"
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
	metrcsHttpServer	metrics.MetricsHttpServer
}

type interceptorOptions struct {
	project	    string
	tracing		tracingOptions
	logging		loggingOptions
	metrics     metricsOptions
}

type loggingOptions struct {
	level        logging.LogLevel
	entry        *logrus.Entry
}

type tracingOptions struct {	
	policy      trace.SamplingPolicy
	collector   trace.Collector	
}

type metricsOptions struct {
	grpceCient	bool
	grpcServer  bool
	httpClient  bool
	httpServer  bool
	listenPort  int
}

// InterceptorOption represents a interceptor option
type InterceptorOption func(*interceptorOptions)

// WithProject set project ID
func WithProject(project string) InterceptorOption {
	return func (i *interceptorOptions) { i.project = project }
}

// EnableLogging config log system
func EnableLogging(level logging.LogLevel, entry *logrus.Entry) InterceptorOption {
	return func(i *interceptorOptions) { 
		i.logging.level = level
		i.logging.entry = entry
	}
}

// EnableTracing config trace system
func EnableTracing(policy trace.SamplingPolicy, collector trace.Collector) InterceptorOption {
	return func (i *interceptorOptions) {
		i.tracing.policy = policy
		i.tracing.collector = collector
	}
}

// EnableTracingWithDefaultSample config trace system
func EnableTracingWithDefaultSample(collector trace.Collector) InterceptorOption {
	return func (i *interceptorOptions) {		
		i.tracing.collector = collector
	}
}

// EnableGRPCClientMetrics config metrics system
func EnableGRPCClientMetrics() InterceptorOption {
	return func (i *interceptorOptions) { i.metrics.grpceCient = true }
}

// EnableGRPCServerMetrics config metrics system
func EnableGRPCServerMetrics() InterceptorOption {
	return func (i *interceptorOptions) { i.metrics.grpcServer = true }
}

// EnableHTTPClientMetrics config metrics system
func EnableHTTPClientMetrics() InterceptorOption {
	return func (i *interceptorOptions) { i.metrics.httpClient = true }
}

// EnableHTTPServerMetrics config metrics system
func EnableHTTPServerMetrics() InterceptorOption {
	return func (i *interceptorOptions) { i.metrics.httpServer = true }
}

// EnableMetricsExportServer config metrics http server listen port
func EnableMetricsExportServer(port int) InterceptorOption {
	return func (i *interceptorOptions) {
		i.metrics.listenPort = port 
	}
}

// NewInterceptorClient create a new interceptor client
func NewInterceptorClient(ctx context.Context, options ...InterceptorOption) (*InterceptorClient, error) {
	intercepOptions := &interceptorOptions{}
	for _, option := range options {
		option(intercepOptions)
	}

	if len(intercepOptions.project) == 0 {
		return nil, fmt.Errorf("the project name must be provided")
	}

	client := &InterceptorClient{}
	
	if intercepOptions.tracing.collector != nil {
		traceClient, err := trace.NewClient(ctx, intercepOptions.project)
		if err != nil {
			log.Printf("create trace client failed:%s", err.Error())
			return nil, err
		}
		if intercepOptions.tracing.policy != nil {
			traceClient.SetSamplingPolicy(intercepOptions.tracing.policy)
		}
		traceClient.SetCollector(intercepOptions.tracing.collector)
		client.trace = traceClient
		log.Printf("enable tracing for project:%s", intercepOptions.project)
	} else {
		log.Println("no tracing collector, so logging disabled")
	}

	if intercepOptions.logging.entry != nil {
		logClient, err := logging.NewClient(intercepOptions.logging.level, intercepOptions.logging.entry)
		if err != nil {
			log.Printf("create log client failed:%s", err.Error())
			return nil, err
		}
		client.log = logClient
		log.Printf("enable logging for project:%s", intercepOptions.project)
	} else {
		log.Println("no log entry, so logging disabled")
	}
	
	if intercepOptions.metrics.grpceCient {
		client.grpcClientMetrics = prometheus.NewGRPCClientPrometheus(intercepOptions.project)
	}

	if intercepOptions.metrics.grpcServer {
		client.grpcServerMetrics = prometheus.NewGRPCServerPrometheus(intercepOptions.project)
	}

	if intercepOptions.metrics.httpClient {
		client.httpClientMetrics = prometheus.NewHTTPClientPrometheus(intercepOptions.project)
	}

	if intercepOptions.metrics.httpServer {
		client.httpServerMetrics = prometheus.NewHTTPServerPrometheus(intercepOptions.project)
	}
	client.metrcsHttpServer = metrics.StartPrometheusMetricsHTTPServer(intercepOptions.metrics.listenPort)
	log.Printf("enable metrics for project:%s", intercepOptions.project)

	return client, nil
}

// TraceClient return tracing client
func (c *InterceptorClient) TraceClient() *trace.Client {
	return c.trace
}

// LoggingClient return logging client
func (c *InterceptorClient) LoggingClient() *logging.Client {
	return c.log
}

// Close close interceptor client
func (c *InterceptorClient) Close() {
	if c.metrcsHttpServer != nil {
		c.metrcsHttpServer.Shutdown()
	}	
}
 
// CloseInterceptorClient close interceptor client
func CloseInterceptorClient(c *InterceptorClient) {
	if c.metrcsHttpServer != nil {
		c.metrcsHttpServer.Shutdown()
	}
} 