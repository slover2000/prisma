package prometheus

import (
	"time"
	"fmt"
	"net/http"

	"google.golang.org/grpc"

	m "github.com/slover2000/prisma/metrics"
	prom "github.com/prometheus/client_golang/prometheus"	
)

const (
	defaultProjectName	= "default_project"
	defaultReportInterval = 60
)

type prometheusClient struct {
	totalCounter       *prom.CounterVec
	errorCounter       *prom.CounterVec		
	durationHistogram  *prom.HistogramVec
}

// NewGRPCClientPrometheus returns a ClientMetrics object. Use a new instance of
// ClientMetrics when not using the default Prometheus metrics registry, for
// example when wanting to control which metrics are added to a registry as
// opposed to automatically adding metrics via init functions.
func NewGRPCClientPrometheus(project string) m.ClientMetrics {
	client := &prometheusClient{
		totalCounter: prom.NewCounterVec(
			prom.CounterOpts{
				Name: fmt.Sprintf("%s_grpc_request_total", project),
				Help: fmt.Sprintf("%s: total number of RPCs completed by the client, regardless of success or failure.", project),
			}, []string{"service", "method", "code"}),
			
		errorCounter: prom.NewCounterVec(
			prom.CounterOpts{
				Name: fmt.Sprintf("%s_grpc_request_failures_total", project),
				Help: fmt.Sprintf("%s: total number of RPCs failed by the client.", project),
			}, []string{"service", "method", "code"}),			

		durationHistogram: prom.NewHistogramVec(
			prom.HistogramOpts{
				Name: fmt.Sprintf("%s_grpc_request_duration_ms", project),
				Help: fmt.Sprintf("%s: histogram of response latency (milliseconds) of the gRPC until it is finished by the application.", project),
				Buckets: prom.DefBuckets,
			},
			[]string{"service", "method", "code"},
		),
	}
	prom.MustRegister(client.totalCounter)
	prom.MustRegister(client.errorCounter)
	prom.MustRegister(client.durationHistogram)

	return client
}

// NewGRPCServerPrometheus returns a ServerMetrics object. Use a new instance of
// ServerMetrics when not using the default Prometheus metrics registry, for
// example when wanting to control which metrics are added to a registry as
// opposed to automatically adding metrics via init functions.
func NewGRPCServerPrometheus(project string) m.ClientMetrics {
	client := &prometheusClient{
		totalCounter: prom.NewCounterVec(
			prom.CounterOpts{
				Name: fmt.Sprintf("%s_grpc_service_total", project),
				Help: fmt.Sprintf("%s: total number of RPCs completed on the server, regardless of success or failure.", project),
			}, []string{"service", "method", "code"}),

		errorCounter: prom.NewCounterVec(
			prom.CounterOpts{
				Name: fmt.Sprintf("%s_grpc_service_failures_total", project),
				Help: fmt.Sprintf("%s: total number of RPCs failed by the client.", project),
			}, []string{"service", "method", "code"}),			

		durationHistogram: prom.NewHistogramVec(
			prom.HistogramOpts{
				Name: fmt.Sprintf("%s_grpc_service_duration_ms", project),
				Help: fmt.Sprintf("%s: histogram of response latency (seconds) of gRPC that had been application-level handled by the server.", project),
				Buckets: prom.DefBuckets,
			},
			[]string{"service", "method", "code"},
		),
	}
	prom.MustRegister(client.totalCounter)
	prom.MustRegister(client.errorCounter)
	prom.MustRegister(client.durationHistogram)

	return client
}

// NewHTTPClientPrometheus returns a ServerMetrics object.
func NewHTTPClientPrometheus(project string) m.ClientMetrics {
	client := &prometheusClient{
		totalCounter: prom.NewCounterVec(
			prom.CounterOpts{
				Name: fmt.Sprintf("%s_http_request_total", project),
				Help: fmt.Sprintf("%s: total number of http completed by the client, regardless of success or failure.", project),
			}, []string{"domain", "path", "method", "code"}),

		errorCounter: prom.NewCounterVec(
			prom.CounterOpts{
				Name: fmt.Sprintf("%s_http_request_failures_total", project),
				Help: fmt.Sprintf("%s: total number of http failed by the client.", project),
			}, []string{"domain", "path", "method", "code"}),

		durationHistogram: prom.NewHistogramVec(
			prom.HistogramOpts{
				Name: fmt.Sprintf("%s_http_request_duration_ms", project),
				Help: fmt.Sprintf("%s: histogram of response latency (milliseconds) of the http until it is finished by the application.", project),
				Buckets: prom.DefBuckets,
			},
			[]string{ "domain", "path", "method", "code"},
		),
	}
	prom.MustRegister(client.totalCounter)
	prom.MustRegister(client.errorCounter)
	prom.MustRegister(client.durationHistogram)

	return client
}

// NewHTTPServerPrometheus returns a ServerMetrics object.
func NewHTTPServerPrometheus(project string) m.ClientMetrics {
	client := &prometheusClient{
		totalCounter: prom.NewCounterVec(
			prom.CounterOpts{
				Name: fmt.Sprintf("%s_http_service_total", project),
				Help: fmt.Sprintf("%s: total number of http completed by the server, regardless of success or failure.", project),
			}, []string{"domain", "path", "method", "code"}),

		errorCounter: prom.NewCounterVec(
			prom.CounterOpts{
				Name: fmt.Sprintf("%s_http_service_failures_total", project),
				Help: fmt.Sprintf("%s: total number of http failed by the server.", project),
			}, []string{"domain", "path", "method", "code"}),			

		durationHistogram: prom.NewHistogramVec(
			prom.HistogramOpts{
				Name: fmt.Sprintf("%s_http_service_duration_ms", project),
				Help: fmt.Sprintf("%s: histogram of response latency (milliseconds) of the http until it is finished by the application.", project),
				Buckets: prom.DefBuckets,
			},
			[]string{"domain", "path", "method", "code"},
		),
	}
	prom.MustRegister(client.totalCounter)
	prom.MustRegister(client.errorCounter)
	prom.MustRegister(client.durationHistogram)

	return client
}

func (c *prometheusClient) CounterGRPC(name string, duration time.Duration, err error) {
	if c == nil {
		return
	}

	code := grpc.Code(err)
	serviceName, methodName := m.SplitGRPCMethodName(name)
	// 记录total counter
	c.totalCounter.WithLabelValues(serviceName, methodName, code.String()).Inc()

	// 记录failurs counter
	if err != nil {
		c.errorCounter.WithLabelValues(serviceName, methodName, code.String()).Inc()
	}

	// 记录Histogram, in millisecond, measure cost time of every method
	ms := duration.Nanoseconds() / int64(time.Millisecond)
	if ms == 0 {
		ms = 1
	}
	c.durationHistogram.WithLabelValues(serviceName, methodName, code.String()).Observe(float64(ms))
}

func (c *prometheusClient) CounterHTTP(req *http.Request, duration time.Duration, code int) {
	if c == nil {
		return
	}
		
	// 记录total counter, like QPS
	c.totalCounter.WithLabelValues(req.URL.Host, req.URL.Path, req.Method, fmt.Sprintf("%d", code)).Inc()

	// 记录failurs counter
	if code >= 400 {
		c.errorCounter.WithLabelValues(req.URL.Host, req.URL.Path, req.Method, fmt.Sprintf("%d", code)).Inc()
	}	

	// 记录Histogram, in millisecond, measure cost time of every method
	ms := duration.Nanoseconds() / int64(time.Millisecond)
	if ms == 0 {
		ms = 1
	}
	c.durationHistogram.WithLabelValues(req.URL.Host, req.URL.Path, req.Method, fmt.Sprintf("%d", code)).Observe(float64(ms))
}

func (c *prometheusClient) Close() error {
	return nil
}