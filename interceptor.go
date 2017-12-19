package prisma

import (
	"log"
	"sync"
	"time"
	
	"golang.org/x/net/context"
	"github.com/sirupsen/logrus"

	"github.com/slover2000/prisma/trace"
	"github.com/slover2000/prisma/logging"
	"github.com/slover2000/prisma/thirdparty"
	"github.com/slover2000/prisma/hystrix"
	"github.com/slover2000/prisma/metrics"
	"github.com/slover2000/prisma/metrics/prometheus"
)

const (
	MongoName	= "mongo"
	MysqlName 	= "mysql"
	RedisName 	= "redis"
	ElasticsearchName = "elasticsearch"

	defaultCircuitBreakerName = "default"
)

var (
	// std is the name of the standard InterceptorClient
	std *InterceptorClient
)

// InterceptorClient a client for interceptor 
type InterceptorClient struct {
	trace				*trace.Client
	log					*logging.Client
	grpcClientMetrics   metrics.ClientMetrics
	grpcServerMetrics   metrics.ClientMetrics
	httpClientMetrics	metrics.ClientMetrics
	httpServerMetrics	metrics.ClientMetrics	
	wrapperMetrics		sync.Map
	metrcsHttpServer	metrics.MetricsHttpServer
}

type wrapperSystemMetrics struct {
	wrapperClient metrics.ClientMetrics
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
	service     string
	policy      trace.SamplingPolicy
	collector   trace.Collector	
}

type metricsOptions struct {
	grpceCient	bool
	grpcServer  bool
	httpClient  bool
	httpServer  bool
	all			bool
	buckets     []float64
	listenPort  int
}

// InterceptorOption represents a interceptor option
type InterceptorOption func(*interceptorOptions)

func init() {
	std = &InterceptorClient{}
}

// EnableLoggingWithEntry config log system
func EnableLoggingWithEntry(level logging.LogLevel, entry *logrus.Entry) InterceptorOption {
	return func(i *interceptorOptions) { 
		i.logging.level = level
		i.logging.entry = entry
	}
}

// EnableLogging config log system
func EnableLogging(level logging.LogLevel) InterceptorOption {
	return func(i *interceptorOptions) { 
		i.logging.level = level
		i.logging.entry = logrus.NewEntry(logrus.StandardLogger())
	}
}

// EnableTracing config trace system
func EnableTracing(serviceName string, policy trace.SamplingPolicy, collector trace.Collector) InterceptorOption {
	return func (i *interceptorOptions) {
		i.tracing.service = serviceName
		i.tracing.policy = policy
		i.tracing.collector = collector
	}
}

// EnableTracingWithDefaultSample config trace system
func EnableTracingWithDefaultSample(serviceName string, collector trace.Collector) InterceptorOption {
	return func (i *interceptorOptions) {
		i.tracing.service = serviceName
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
// EnableAllMetrics enable all metrics including http and grpc
func EnableAllMetrics() InterceptorOption {
	return func (i *interceptorOptions) { i.metrics.all = true }
}

// EnableMetricsExportHTTPServer config metrics system
func EnableMetricsExportHTTPServer(port int) InterceptorOption {
	return func (i *interceptorOptions) { 
		i.metrics.listenPort = port
	}
}

// WithMetricsHistogramBuckets allows you to specify custom bucket ranges for histograms
func WithMetricsHistogramBuckets(buckets []float64) InterceptorOption {
	return func(i *interceptorOptions) { i.metrics.buckets = buckets }
}

// StandardInterceptorClient return standard interceptor client of package
func StandardInterceptorClient() *InterceptorClient {
	return std
}

// ConfigInterceptorClient create a new interceptor client
func ConfigInterceptorClient(ctx context.Context, options ...InterceptorOption) (*InterceptorClient, error) {
	intercepOptions := &interceptorOptions{}
	for _, option := range options {
		option(intercepOptions)
	}

	client := std
	if len(intercepOptions.tracing.service) > 0 && intercepOptions.tracing.collector != nil {
		traceClient, err := trace.NewClient(ctx, intercepOptions.tracing.service)
		if err != nil {
			log.Printf("create trace client failed:%s", err.Error())
			return nil, err
		}
		traceClient.SetSamplingPolicy(intercepOptions.tracing.policy)
		traceClient.SetCollector(intercepOptions.tracing.collector)
		client.trace = traceClient
		log.Printf("enable tracing module for service:%s", intercepOptions.tracing.service)
	} else {
		log.Println("disable tracing module")
	}

	if intercepOptions.logging.entry != nil {
		logClient, err := logging.NewClient(intercepOptions.logging.level, intercepOptions.logging.entry)
		if err != nil {
			log.Printf("create log client failed:%s", err.Error())
			return nil, err
		}
		client.log = logClient
		log.Printf("enable logging module")
	} else {
		log.Println("disable logging module")
	}
		
	if intercepOptions.metrics.all {
		intercepOptions.metrics.grpceCient = true
		intercepOptions.metrics.grpcServer = true
		intercepOptions.metrics.httpClient = true
		intercepOptions.metrics.httpServer = true
	}

	enableAnyMetric := false
	if intercepOptions.metrics.grpceCient {
		client.grpcClientMetrics = prometheus.NewGRPCClientPrometheus(intercepOptions.metrics.buckets)
		enableAnyMetric = true
	}

	if intercepOptions.metrics.grpcServer {
		client.grpcServerMetrics = prometheus.NewGRPCServerPrometheus(intercepOptions.metrics.buckets)
		enableAnyMetric = true
	}

	if intercepOptions.metrics.httpClient {
		client.httpClientMetrics = prometheus.NewHTTPClientPrometheus(intercepOptions.metrics.buckets)
		enableAnyMetric = true
	}

	if intercepOptions.metrics.httpServer {
		client.httpServerMetrics = prometheus.NewHTTPServerPrometheus(intercepOptions.metrics.buckets)
		enableAnyMetric = true
	}	

	if enableAnyMetric {
		client.metrcsHttpServer = metrics.StartPrometheusMetricsHTTPServer(intercepOptions.metrics.listenPort)
		log.Printf("enable metrics module")
	} else {
		log.Printf("disable metrics module")
	}

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

// Close interceptor client
func (c *InterceptorClient) Close() {
	if c.metrcsHttpServer != nil {
		c.metrcsHttpServer.Shutdown()
	}	
}

// EnableLoggingWithEntry config log system
func (c *InterceptorClient) EnableLoggingWithEntry(level logging.LogLevel, entry *logrus.Entry) *InterceptorClient {
	logClient, err := logging.NewClient(level, entry)
	if err != nil {
		log.Printf("create log client failed:%s", err.Error())
		return c
	}

	log.Printf("enable logging module")
	c.log = logClient
	return c
}

// EnableLogging config log system
func (c *InterceptorClient) EnableLogging(level logging.LogLevel) *InterceptorClient {
	return c.EnableLoggingWithEntry(level, logrus.NewEntry(logrus.StandardLogger()))
}

// EnableTracing config trace system
func (c *InterceptorClient) EnableTracing(serviceName string, policy trace.SamplingPolicy, collector trace.Collector) *InterceptorClient {
	traceClient, err := trace.NewClient(context.Background(), serviceName)
	if err != nil {
		log.Printf("create trace client failed:%s", err.Error())
		return c
	}

	traceClient.SetSamplingPolicy(policy)
	traceClient.SetCollector(collector)
	c.trace = traceClient
	log.Printf("enable tracing module for service:%s", serviceName)
	return c
}

// EnableTracingWithDefaultSample config trace system
func (c *InterceptorClient) EnableTracingWithDefaultSample(serviceName string, collector trace.Collector) *InterceptorClient {
	return c.EnableTracing(serviceName, nil, collector)
}

// EnableGRPCClientMetrics config metrics system
func (c *InterceptorClient) EnableGRPCClientMetrics(buckets []float64) *InterceptorClient {
	c.grpcClientMetrics = prometheus.NewGRPCClientPrometheus(buckets)
	return c
}

// EnableGRPCServerMetrics config metrics system
func (c *InterceptorClient) EnableGRPCServerMetrics(buckets []float64) *InterceptorClient {
	c.grpcServerMetrics = prometheus.NewGRPCServerPrometheus(buckets)
	return c
}

// EnableHTTPClientMetrics config metrics system
func (c *InterceptorClient) EnableHTTPClientMetrics(buckets []float64) *InterceptorClient {
	c.httpClientMetrics = prometheus.NewHTTPClientPrometheus(buckets)
	return c
}

// EnableHTTPServerMetrics config metrics system
func (c *InterceptorClient) EnableHTTPServerMetrics(buckets []float64) *InterceptorClient {
	c.httpServerMetrics = prometheus.NewHTTPServerPrometheus(buckets)
	return c
}

// Enable3rdDBMetricsWithBuckets enable thirdparty database metrics
func (c *InterceptorClient) Enable3rdDBMetricsWithBuckets(system string, buckets []float64) *InterceptorClient {
	databaseMetrics := prometheus.NewDatabaseClientPrometheus(system, buckets)
	c.wrapperMetrics.Store(system, databaseMetrics)
	return c
}

// Enable3rdDBMetrics enable thirdparty database metrics
func (c *InterceptorClient) Enable3rdDBMetrics(system string) *InterceptorClient {
	databaseMetrics := prometheus.NewDatabaseClientPrometheus(system, nil)
	c.wrapperMetrics.Store(system, databaseMetrics)
	return c
}

// Enable3rdCacheMetricsWithBuckets enable thirdparty cache metrics
func (c *InterceptorClient) Enable3rdCacheMetricsWithBuckets(system string, buckets []float64) *InterceptorClient {
	cacheMetrics := prometheus.NewCacheClientPrometheus(system, buckets)
	c.wrapperMetrics.Store(system, cacheMetrics)
	return c
}

// Enable3rdCacheMetrics enable thirdparty cache metrics
func (c *InterceptorClient) Enable3rdCacheMetrics(system string) *InterceptorClient {
	cacheMetrics := prometheus.NewCacheClientPrometheus(system, nil)
	c.wrapperMetrics.Store(system, cacheMetrics)
	return c
}

// Enable3rdSearchMetricsWithBuckets enable thirdparty cache metrics
func (c *InterceptorClient) Enable3rdSearchMetricsWithBuckets(system string, buckets []float64) *InterceptorClient {
	searchMetrics := prometheus.NewSearchClientPrometheus(system, buckets)
	c.wrapperMetrics.Store(system, searchMetrics)
	return c
}

// Enable3rdSearchMetrics enable thirdparty cache metrics
func (c *InterceptorClient) Enable3rdSearchMetrics(system string) *InterceptorClient {
	searchMetrics := prometheus.NewSearchClientPrometheus(system, nil)
	c.wrapperMetrics.Store(system, searchMetrics)
	return c
}

// EnableAllMetrics enable all metrics including http and grpc
func (c *InterceptorClient) EnableAllMetrics(buckets []float64) *InterceptorClient {
	return c.EnableGRPCClientMetrics(buckets).EnableGRPCServerMetrics(buckets).EnableHTTPClientMetrics(buckets).EnableHTTPServerMetrics(buckets)
}

// EnableMetricsExportHTTPServer config metrics system
func (c *InterceptorClient) EnableMetricsExportHTTPServer(port int) *InterceptorClient {
	c.metrcsHttpServer = metrics.StartPrometheusMetricsHTTPServer(port)
	log.Printf("enable metrics module")
	return c
}

// CloseInterceptorClient close interceptor client
func CloseInterceptorClient(c *InterceptorClient) {
	if c.metrcsHttpServer != nil {
		c.metrcsHttpServer.Shutdown()
	}
}

type actionResult struct{
	value interface{}
	err   error
}

type runFunc = func() (interface{}, error)
type fallbackFunc = func(error) (interface{}, error)

func (c *InterceptorClient) execute(ctx context.Context, run runFunc, fallback fallbackFunc) (interface{}, error) {
	if name, ok := hystrix.GetHystrixCommand(ctx); ok {
		return hystrix.Execute(ctx, name, run, fallback)
	}

	// disable hystrix module
	done := make(chan actionResult, 1)
	go func() {
		result, err := run()
		if err == nil {
			done <- actionResult{value: result, err: err}
		} else {			
			fallbackResult, fallbackErr := fallback(err)
			done <- actionResult{value: fallbackResult, err: fallbackErr}		
		}		
	}()
	
	select {
	case result, _ := <-done:
		return result.value, result.err
	case <-ctx.Done():
		fallbackResult, fallbackErr := fallback(ctx.Err())
		return fallbackResult, fallbackErr
	}
}

// Do runs your function in a synchronous manner, blocking until either your function succeeds
// or an error is returned
func (c *InterceptorClient) Do(ctx context.Context, run runFunc) (interface{}, error) {
	return c.DoWithFallback(ctx, run, nil)
}

// DoWithFallback runs your function in a synchronous manner, blocking until either your function succeeds
// or an error is returned
func (c *InterceptorClient) DoWithFallback(ctx context.Context, run runFunc, fallback fallbackFunc) (interface{}, error) {
	t := thirdparty.DetectContextValue(ctx)
	switch t {
	case thirdparty.TypeDatabaseSystem:
		return c.doDBAction(ctx, run, fallback)
	case thirdparty.TypeCacheSystem:
		return c.doCacheAction(ctx, run, fallback)
	case thirdparty.TypeSearchSystem:
		return c.doSearchAction(ctx, run, fallback)
	default:
		return c.execute(ctx, run, fallback)
	}
}

// doDBAction runs your function in a synchronous manner, blocking until either your function succeeds
// or an error is returned
func (c *InterceptorClient) doDBAction(ctx context.Context, run runFunc, fallback fallbackFunc) (value interface{}, err error) {
	params, ok := thirdparty.ParseDatabaeContextValue(ctx)
	if !ok {
		return c.execute(ctx, run, fallback)
	}
	
	span := trace.FromContext(ctx).NewDatabaseChild(params)
	defer span.Finish()
	
	startTime := time.Now()
	value, err = c.execute(ctx, run, fallback)
	// do metrics
	if m, ok := c.wrapperMetrics.Load(params.System); ok {
		metrics, _ := m.(metrics.ClientMetrics)
		metrics.CounterDatabase(params, time.Since(startTime), err)
	}

	return
}

// doCacheAction runs your function in a synchronous manner, blocking until either your function succeeds
// or an error is returned
func (c *InterceptorClient) doCacheAction(ctx context.Context, run runFunc, fallback fallbackFunc) (value interface{}, err error) {
	params, ok := thirdparty.ParseCacheContextValue(ctx)
	if !ok {		
		return c.execute(ctx, run, fallback)
	}
	
	span := trace.FromContext(ctx).NewCacheChild(params)
	defer span.Finish()
	
	startTime := time.Now()
	value, err = c.execute(ctx, run, fallback)
	// do metrics
	if m, ok := c.wrapperMetrics.Load(params.System); ok {
		metrics, _ := m.(metrics.ClientMetrics)
		metrics.CounterCache(params, time.Since(startTime), err)
	}

	return
}

// doSearchAction runs your function in a synchronous manner, blocking until either your function succeeds
// or an error is returned
func (c *InterceptorClient) doSearchAction(ctx context.Context, run runFunc, fallback fallbackFunc) (value interface{}, err error) {
	params, ok := thirdparty.ParseSearchContextValue(ctx)
	if !ok {		
		return c.execute(ctx, run, fallback)
	}
	
	span := trace.FromContext(ctx).NewSearchChild(params)
	defer span.Finish()
	
	startTime := time.Now()
	value, err = c.execute(ctx, run, fallback)
	// do metrics
	if m, ok := c.wrapperMetrics.Load(params.System); ok {
		metrics, _ := m.(metrics.ClientMetrics)
		metrics.CounterSearch(params, time.Since(startTime), err)
	}

	return
}