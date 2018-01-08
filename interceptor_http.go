package prisma

import (
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/slover2000/prisma/trace"
)

// Transport is an http.RoundTripper that traces the outgoing requests.
//
// Transport is safe for concurrent usage.
type Transport struct {
	Client *InterceptorClient
	Base   http.RoundTripper
}

// RoundTrip creates a trace.Span and inserts it into the outgoing request's headers.
// The created span can follow a parent span, if a parent is presented in
// the request's context.
func (t Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	span := trace.FromContext(req.Context()).NewRemoteChild(req)
	startTime := time.Now()
	resp, err := t.base().RoundTrip(req)

	// TODO(jbd): Is it possible to defer the span.Finish?
	// In cases where RoundTrip panics, we still can finish the span.
	span.Finish(trace.WithResponse(resp))

	// log http request
	statusCode := 500
	if resp != nil {
		statusCode = resp.StatusCode
	}

	// do metrics
	if t.Client != nil && t.Client.httpClientMetrics != nil {
		t.Client.httpClientMetrics.CounterHTTP(req, time.Since(startTime), statusCode)
	}

	// log request
	if t.Client != nil && t.Client.log != nil {
		msg := fmt.Sprintf("%s %s %s %d", req.Method, req.URL.String(), req.Proto, statusCode)
		t.Client.log.LogHttpClientLine(req, startTime, statusCode, msg)
	}
	return resp, err
}

// CancelRequest cancels an in-flight request by closing its connection.
func (t Transport) CancelRequest(req *http.Request) {
	type canceler interface {
		CancelRequest(*http.Request)
	}
	if cr, ok := t.base().(canceler); ok {
		cr.CancelRequest(req)
	}
}

func (t Transport) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}

type withStatusCodeResponseWriter struct {
	writer     http.ResponseWriter
	statusCode int
}

func (w *withStatusCodeResponseWriter) Header() http.Header {
	return w.writer.Header()
}

func (w *withStatusCodeResponseWriter) Write(bytes []byte) (int, error) {
	return w.writer.Write(bytes)
}

func (w *withStatusCodeResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.writer.WriteHeader(code)
}

// define http filter function
type FilterFunc func(w http.ResponseWriter, r *http.Request) bool

// define http handler options
type httpFilterOptions struct {
	filters map[string]FilterFunc
}

type HTTPFilterOption func(*httpFilterOptions)

// UsingFilter config customer filter
func UsingFilter(pattern string, filter FilterFunc) HTTPFilterOption {
	return func(o *httpFilterOptions) {
		o.filters[pattern] = filter
	}
}

// HTTPHandler returns a http.Handler from the given handler
// that is aware of the incoming request's span.
// The span can be extracted from the incoming request in handler
// functions from incoming request's context:
//
//    span := trace.FromContext(r.Context())
//
// The span will be auto finished by the handler.
func (c *InterceptorClient) HTTPHandler(h http.Handler, options ...HTTPFilterOption) http.Handler {
	if c == nil {
		return h
	}

	o := &httpFilterOptions{
		filters: make(map[string]FilterFunc),
	}
	for _, option := range options {
		option(o)
	}

	filters := make([]*httpFilter, len(o.filters))
	index := 0
	for k, v := range o.filters {
		f := &httpFilter{
			pattern:    k,
			filterFunc: v,
			regexps:    regexp.MustCompile(k),
		}
		filters[index] = f
		index += 1
	}
	return &httpHandler{client: c, handler: h, filters: filters}
}

func (c *InterceptorClient) BeegoMiddleware(h http.Handler) http.Handler {
	if c == nil {
		return h
	}

	return &httpHandler{client: c, handler: h, filters: make([]*httpFilter, 0)}
}

type httpFilter struct {
	pattern    string
	regexps    *regexp.Regexp
	filterFunc FilterFunc
}

type httpHandler struct {
	client  *InterceptorClient
	handler http.Handler
	filters []*httpFilter
}

func (h *httpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if len(h.filters) != 0 {
		for _, filter := range h.filters {
			if filter.regexps.MatchString(r.URL.Path) {
				isContinue := filter.filterFunc(w, r)
				if !isContinue {
					return
				}
			}
		}
	}

	var span *trace.Span
	if h.client != nil && h.client.trace != nil {
		span = h.client.trace.SpanFromRequestOrNot(r)
	}
	defer span.Finish()

	r = r.WithContext(trace.NewContext(r.Context(), span))
	// if ok && !optionsOk {
	// 	// Inject the trace context back to the response with the sampling options.
	// 	// TODO(jbd): Remove when there is a better way to report the client's sampling.
	// 	w.Header().Set(httpTraceHeader, spanHeader(traceID, parentSpanID, span.trace.localOptions))
	// }

	rw := &withStatusCodeResponseWriter{writer: w}
	startTime := time.Now()
	h.handler.ServeHTTP(rw, r)

	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}

	// do metrics
	if h.client != nil && h.client.httpServerMetrics != nil {
		h.client.httpServerMetrics.CounterHTTP(r, time.Since(startTime), rw.statusCode)
	}

	// log request
	if h.client != nil && h.client.log != nil {
		msg := fmt.Sprintf("%s %s %s %d", r.Method, r.URL.String(), r.Proto, rw.statusCode)
		h.client.log.LogHttpServerLine(r, startTime, rw.statusCode, msg)
	}
}
