package prometheus

import (
	"time"
	"fmt"
	"net/http"

	"google.golang.org/grpc"

	m "github.com/slover2000/prisma/metrics"
	"github.com/slover2000/prisma/utils"
	prom "github.com/prometheus/client_golang/prometheus"	
)

const (
	defaultProjectName	= "default_project"
	defaultReportInterval = 60
)

type prometheusClient struct {
	qpsCounter        *prom.CounterVec
	latencyHistogram  *prom.HistogramVec
	host              string
}

// NewGRPCClientPrometheus returns a ClientMetrics object. Use a new instance of
// ClientMetrics when not using the default Prometheus metrics registry, for
// example when wanting to control which metrics are added to a registry as
// opposed to automatically adding metrics via init functions.
func NewGRPCClientPrometheus(project string) m.ClientMetrics {
	host := "unknown"
	ips, err := utils.LocalIPs()
	if err == nil || len(ips) > 0 {
		host = ips[0]
	}

	client := &prometheusClient{
		host: host,
		qpsCounter: prom.NewCounterVec(
			prom.CounterOpts{
				Name: fmt.Sprintf("%s_grpc_client_qps", project),
				Help: fmt.Sprintf("%s: total number of RPCs completed by the client, regardless of success or failure.", project),
			}, []string{"host", "service", "method", "code"}),

		latencyHistogram: prom.NewHistogramVec(
			prom.HistogramOpts{
				Name: fmt.Sprintf("%s_grpc_client_latency", project),
				Help: fmt.Sprintf("%s: histogram of response latency (milliseconds) of the gRPC until it is finished by the application.", project),
				Buckets: prom.DefBuckets,
			},
			[]string{"host", "service", "method", "code"},
		),
	}
	prom.MustRegister(client.qpsCounter)
	prom.MustRegister(client.latencyHistogram)

	return client
}

// NewGRPCServerPrometheus returns a ServerMetrics object. Use a new instance of
// ServerMetrics when not using the default Prometheus metrics registry, for
// example when wanting to control which metrics are added to a registry as
// opposed to automatically adding metrics via init functions.
func NewGRPCServerPrometheus(project string) m.ClientMetrics {
	host := "unknown"
	ips, err := utils.LocalIPs()
	if err == nil || len(ips) > 0 {
		host = ips[0]
	}

	client := &prometheusClient{
		host: host,
		qpsCounter: prom.NewCounterVec(
			prom.CounterOpts{
				Name: fmt.Sprintf("%s_grpc_server_qps", project),
				Help: fmt.Sprintf("%s: total number of RPCs completed on the server, regardless of success or failure.", project),
			}, []string{"host", "service", "method", "code"}),

		latencyHistogram: prom.NewHistogramVec(
			prom.HistogramOpts{
				Name: fmt.Sprintf("%s_grpc_server_latency", project),
				Help: fmt.Sprintf("%s: histogram of response latency (seconds) of gRPC that had been application-level handled by the server.", project),
				Buckets: prom.DefBuckets,
			},
			[]string{"host", "service", "method", "code"},
		),
	}
	prom.MustRegister(client.qpsCounter)
	prom.MustRegister(client.latencyHistogram)

	return client
}

// NewHTTPClientPrometheus returns a ServerMetrics object.
func NewHTTPClientPrometheus(project string) m.ClientMetrics {
	host := "unknown"
	ips, err := utils.LocalIPs()
	if err == nil || len(ips) > 0 {
		host = ips[0]
	}

	client := &prometheusClient{
		host: host,
		qpsCounter: prom.NewCounterVec(
			prom.CounterOpts{
				Name: fmt.Sprintf("%s_http_client_qps", project),
				Help: fmt.Sprintf("%s: total number of http completed by the client, regardless of success or failure.", project),
			}, []string{"host", "domain", "path", "method", "code"}),

		latencyHistogram: prom.NewHistogramVec(
			prom.HistogramOpts{
				Name: fmt.Sprintf("%s_http_client_latency", project),
				Help: fmt.Sprintf("%s: histogram of response latency (milliseconds) of the http until it is finished by the application.", project),
				Buckets: prom.DefBuckets,
			},
			[]string{"host", "domain", "path", "method", "code"},
		),
	}
	prom.MustRegister(client.qpsCounter)
	prom.MustRegister(client.latencyHistogram)

	return client
}

// NewHTTPServerPrometheus returns a ServerMetrics object.
func NewHTTPServerPrometheus(project string) m.ClientMetrics {
	host := "unknown"
	ips, err := utils.LocalIPs()
	if err == nil || len(ips) > 0 {
		host = ips[0]
	}

	client := &prometheusClient{
		host: host,
		qpsCounter: prom.NewCounterVec(
			prom.CounterOpts{
				Name: fmt.Sprintf("%s_http_server_qps", project),
				Help: fmt.Sprintf("%s: total number of http completed by the server, regardless of success or failure.", project),
			}, []string{"host", "domain", "path", "method", "code"}),

		latencyHistogram: prom.NewHistogramVec(
			prom.HistogramOpts{
				Name: fmt.Sprintf("%s_http_server_latency", project),
				Help: fmt.Sprintf("%s: histogram of response latency (milliseconds) of the http until it is finished by the application.", project),
				Buckets: prom.DefBuckets,
			},
			[]string{"host", "domain", "path", "method", "code"},
		),
	}
	prom.MustRegister(client.qpsCounter)
	prom.MustRegister(client.latencyHistogram)

	return client
}

func (c *prometheusClient) CounterGRPC(name string, duration time.Duration, err error) {
	if c == nil {
		return
	}

	code := grpc.Code(err)
	serviceName, methodName := m.SplitGRPCMethodName(name)
	// 记录Counter, like QPS
	c.qpsCounter.WithLabelValues(c.host, serviceName, methodName, code.String()).Inc()

	// 记录Histogram, in millisecond, measure cost time of every method
	ms := duration.Nanoseconds() / int64(time.Millisecond)
	if ms == 0 {
		ms = 1
	}
	c.latencyHistogram.WithLabelValues(c.host, serviceName, methodName, code.String()).Observe(float64(ms))
}

func (c *prometheusClient) CounterHTTP(req *http.Request, duration time.Duration, code int) {
	if c == nil {
		return
	}
		
	// 记录Counter, like QPS
	c.qpsCounter.WithLabelValues(c.host, req.URL.Host, req.URL.Path, req.Method, fmt.Sprintf("%d", code)).Inc()

	// 记录Histogram, in millisecond, measure cost time of every method
	ms := duration.Nanoseconds() / int64(time.Millisecond)
	if ms == 0 {
		ms = 1
	}
	c.latencyHistogram.WithLabelValues(c.host, req.URL.Host, req.URL.Path, req.Method, fmt.Sprintf("%d", code)).Observe(float64(ms))
}

func (c *prometheusClient) Close() error {
	return nil
}