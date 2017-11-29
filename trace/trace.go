package trace

import (
	"fmt"
	"time"
	"net/http"
	"crypto/rand"
	"strings"
	"strconv"
	"sync"
	"sync/atomic"
	"encoding/binary"	

	"golang.org/x/net/context"
)

const (
	httpTraceHeader 	= `x-trace-ctx`
	spanKindClient      = `RPC_CLIENT`
	spanKindServer      = `RPC_SERVER`
	spanKindUnspecified = `SPAN_KIND_UNSPECIFIED`
	defaultSampleRate	= 1e-4
	defaultMaxQPS		= 10
)

// Stackdriver Trace API predefined labels.
const (
	LabelAgent              = `trace/agent`
	LabelComponent          = `trace/component`
	LabelErrorMessage       = `trace/error/message`
	LabelErrorName          = `trace/error/name`
	LabelHTTPClientCity     = `trace/http/client_city`
	LabelHTTPClientCountry  = `trace/http/client_country`
	LabelHTTPClientProtocol = `trace/http/client_protocol`
	LabelHTTPClientRegion   = `trace/http/client_region`
	LabelHTTPHost           = `trace/http/host`
	LabelHTTPMethod         = `trace/http/method`
	LabelHTTPRedirectedURL  = `trace/http/redirected_url`
	LabelHTTPRequestSize    = `trace/http/request/size`
	LabelHTTPResponseSize   = `trace/http/response/size`
	LabelHTTPStatusCode     = `trace/http/status_code`
	LabelHTTPURL            = `trace/http/url`
	LabelHTTPUserAgent      = `trace/http/user_agent`
	LabelPID                = `trace/pid`
	LabelSamplingPolicy     = `trace/sampling_policy`
	LabelSamplingWeight     = `trace/sampling_weight`
	LabelStackTrace         = `trace/stacktrace`
	LabelTID                = `trace/tid`
)

var (
	spanIDCounter   uint64
	spanIDIncrement uint64
)

type contextKey struct{}

func init() {
	// Set spanIDCounter and spanIDIncrement to random values.  nextSpanID will
	// return an arithmetic progression using these values, skipping zero.  We set
	// the LSB of spanIDIncrement to 1, so that the cycle length is 2^64.
	binary.Read(rand.Reader, binary.LittleEndian, &spanIDCounter)
	binary.Read(rand.Reader, binary.LittleEndian, &spanIDIncrement)
	spanIDIncrement |= 1
}

// nextSpanID returns a new span ID.  It will never return zero.
func nextSpanID() uint64 {
	var id uint64
	for id == 0 {
		id = atomic.AddUint64(&spanIDCounter, spanIDIncrement)
	}
	return id
}

// nextTraceID returns a new trace ID.
func nextTraceID() string {
	id1 := nextSpanID()
	id2 := nextSpanID()
	return fmt.Sprintf("%016x%016x", id1, id2)
}

// Client is a client for uploading traces to the Trace service.
// A nil Client will no-op for all of its methods.
type Client struct {
	projectID 	string
	policy    	SamplingPolicy
}

// NewClient creates a new Google Stackdriver Trace client.
func NewClient(ctx context.Context, projectID string) (*Client, error) {
	policy, _ := NewLimitedSampler(defaultSampleRate, defaultMaxQPS)
	c := &Client{
		projectID: projectID,
		policy: policy,
	}
	return c, nil
}

// SetSamplingPolicy sets the SamplingPolicy that determines how often traces
// are initiated by this client.
func (c *Client) SetSamplingPolicy(p SamplingPolicy) {
	if c != nil {
		c.policy = p
	}
}

// SpanFromHeader returns a new trace span based on a provided request header
// value or nil iff the client is nil.
//
// The trace information and identifiers will be read from the header value.
// Otherwise, a new trace ID is made and the parent span ID is zero.
//
// The name of the new span is provided as an argument.
//
// If a non-nil sampling policy has been set in the client, it can override
// the options set in the header and choose whether to trace the request.
//
// If the header doesn't have existing tracing information, then a *Span is
// returned anyway, but it will not be uploaded to the server, just as when
// calling SpanFromRequest on an untraced request.
//
// Most users using HTTP should use SpanFromRequest, rather than
// SpanFromHeader, since it provides additional functionality for HTTP
// requests. In particular, it will set various pieces of request information
// as labels on the *Span, which is not available from the header alone.
func (c *Client) SpanFromHeader(name string, header string) *Span {
	if c == nil {
		return nil
	}
	traceID, parentSpanID, options, _, ok := traceInfoFromHeader(header)
	if !ok {
		traceID = nextTraceID()
	}
	t := &trace{
		traceID:       traceID,
		client:        c,
		globalOptions: options,
		localOptions:  options,
	}
	span := startNewChild(name, t, parentSpanID)
	span.kind = spanKindServer
	span.rootSpan = true
	configureSpanFromPolicy(span, c.policy, ok)
	return span
}

// SpanFromRequest returns a new trace span for an HTTP request or nil
// if the client is nil.
//
// If the incoming HTTP request contains a trace context header, the trace ID,
// parent span ID, and tracing options will be read from that header.
// Otherwise, a new trace ID is made and the parent span ID is zero.
//
// If a non-nil sampling policy has been set in the client, it can override the
// options set in the header and choose whether to trace the request.
//
// If the request is not being traced, then a *Span is returned anyway, but it
// will not be uploaded to the server -- it is only useful for propagating
// trace context to child requests and for getting the TraceID.  All its
// methods can still be called -- the Finish, FinishWait, and SetLabel methods
// do nothing.  NewChild does nothing, and returns the same *Span.  TraceID
// works as usual.
func (c *Client) SpanFromRequest(r *http.Request) *Span {
	if c == nil {
		return nil
	}
	traceID, parentSpanID, options, _, ok := traceInfoFromHeader(r.Header.Get(httpTraceHeader))
	if !ok {
		traceID = nextTraceID()
	}
	t := &trace{
		traceID:       traceID,
		client:        c,
		globalOptions: options,
		localOptions:  options,
	}
	span := startNewChildWithRequest(r, t, parentSpanID)
	span.kind = spanKindServer
	span.rootSpan = true
	configureSpanFromPolicy(span, c.policy, ok)
	return span
}

// SpanFromMetadata returns a new trace span from a metadata or nil
// if the client is nil.
//
func (c *Client) SpanFromMetadata(name string, header string) *Span {
	if c == nil {
		return nil
	}
	traceID, parentSpanID, options, _, ok := traceInfoFromHeader(header)
	if !ok {
		traceID = nextTraceID()
	}
	t := &trace{
		traceID:       traceID,
		client:        c,
		globalOptions: options,
		localOptions:  options,
	}
	span := startNewChild(name, t, parentSpanID)
	span.kind = spanKindServer
	span.rootSpan = true
	configureSpanFromPolicy(span, c.policy, ok)
	return span
}

// NewSpan returns a new trace span with the given name or nil if the
// client is nil.
//
// A new trace and span ID is generated to trace the span.
// Returned span need to be finished by calling Finish or FinishWait.
func (c *Client) NewSpan(name string) *Span {
	if c == nil {
		return nil
	}
	t := &trace{
		traceID:       nextTraceID(),
		client:        c,
		localOptions:  optionTrace,
		globalOptions: optionTrace,
	}
	span := startNewChild(name, t, 0)
	span.kind = spanKindUnspecified
	span.rootSpan = true
	configureSpanFromPolicy(span, c.policy, false)
	return span
}

func configureSpanFromPolicy(s *Span, p SamplingPolicy, ok bool) {
	if p == nil {
		return
	}
	d := p.Sample(Parameters{HasTraceHeader: ok})
	if d.Trace {
		// Turn on tracing locally, and in child requests.
		s.trace.localOptions |= optionTrace
		s.trace.globalOptions |= optionTrace
	} else {
		// Turn off tracing locally.
		s.trace.localOptions = 0
		return
	}
	if d.Sample {
		// This trace is in the random sample, so set the labels.
		s.SetLabel(LabelSamplingPolicy, d.Policy)
		s.SetLabel(LabelSamplingWeight, fmt.Sprint(d.Weight))
	}
}

// NewContext returns a derived context containing the span.
func NewContext(ctx context.Context, s *Span) context.Context {
	if s == nil {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, s)
}

// FromContext returns the span contained in the context, or nil.
func FromContext(ctx context.Context) *Span {
	s, _ := ctx.Value(contextKey{}).(*Span)
	return s
}

func traceInfoFromHeader(h string) (traceID string, spanID uint64, options optionFlags, optionsOk bool, ok bool) {
	// See https://cloud.google.com/trace/docs/faq for the header format.
	// Return if the header is empty or missing, or if the header is unreasonably
	// large, to avoid making unnecessary copies of a large string.
	if h == "" || len(h) > 200 {
		return "", 0, 0, false, false

	}

	// Parse the trace id field.
	slash := strings.Index(h, `/`)
	if slash == -1 {
		return "", 0, 0, false, false

	}
	traceID, h = h[:slash], h[slash+1:]

	// Parse the span id field.
	spanstr := h
	semicolon := strings.Index(h, `;`)
	if semicolon != -1 {
		spanstr, h = h[:semicolon], h[semicolon+1:]
	}
	spanID, err := strconv.ParseUint(spanstr, 10, 64)
	if err != nil {
		return "", 0, 0, false, false

	}

	// Parse the options field, options field is optional.
	if !strings.HasPrefix(h, "o=") {
		return traceID, spanID, 0, false, true

	}
	o, err := strconv.ParseUint(h[2:], 10, 64)
	if err != nil {
		return "", 0, 0, false, false

	}
	options = optionFlags(o)
	return traceID, spanID, options, true, true
}

type optionFlags uint32

const (
	optionTrace optionFlags = 1 << iota
	optionStack
)

type trace struct {
	mu            sync.Mutex
	client        *Client
	traceID       string
	globalOptions optionFlags // options that will be passed to any child requests
	localOptions  optionFlags // options applied in this server
	spans         []*Span     // finished spans for this trace.
}

// finish appends s to t.spans.  If s is the root span, uploads the trace to the
// server.
func (t *trace) finish(s *Span, wait bool, opts ...FinishOption) error {
	for _, o := range opts {
		o.modifySpan(s)
	}
	s.end = time.Now()
	t.mu.Lock()
	t.spans = append(t.spans, s)
	spans := t.spans
	t.mu.Unlock()
	if s.rootSpan {
		if wait {
			//return t.client.upload([]*api.Trace{t.constructTrace(spans)})
		}
		go func() {
			t.constructTrace(spans)
			// err := t.client.bundler.Add(tr, 1+len(spans))
			// if err == bundler.ErrOversizedItem {
			// 	err = t.client.upload([]*api.Trace{tr})
			// }
			// if err != nil {
			// 	log.Println("error uploading trace:", err)
			// }
		}()
	}
	return nil
}

func (t *trace) constructTrace(spans []*Span) string {
	// apiSpans := make([]*api.TraceSpan, len(spans))
	// for i, sp := range spans {
	// 	sp.span.StartTime = sp.start.In(time.UTC).Format(time.RFC3339Nano)
	// 	sp.span.EndTime = sp.end.In(time.UTC).Format(time.RFC3339Nano)
	// 	if t.localOptions&optionStack != 0 {
	// 		sp.setStackLabel()
	// 	}
	// 	if sp.host != "" {
	// 		sp.SetLabel(LabelHTTPHost, sp.host)
	// 	}
	// 	if sp.url != "" {
	// 		sp.SetLabel(LabelHTTPURL, sp.url)
	// 	}
	// 	if sp.method != "" {
	// 		sp.SetLabel(LabelHTTPMethod, sp.method)
	// 	}
	// 	if sp.statusCode != 0 {
	// 		sp.SetLabel(LabelHTTPStatusCode, strconv.Itoa(sp.statusCode))
	// 	}
	// 	apiSpans[i] = &sp.span
	// }

	// return &api.Trace{
	// 	ProjectId: t.client.projectID,
	// 	TraceId:   t.traceID,
	// 	Spans:     apiSpans,
	// }
	return ""
}

// Span contains information about one span of a trace.
type Span struct {
	trace *trace

	spanMu sync.Mutex // guards span.Labels
	name 	   		string
	kind 			string
	spanID	   		uint64
	parentSpanID   	uint64

	start      time.Time
	end        time.Time
	rootSpan   bool
	host       string
	method     string
	url        string
	statusCode int
	labels 	   map[string]string
}

// Traced reports whether the current span is sampled to be traced.
func (s *Span) Traced() bool {
	if s == nil {
		return false
	}
	return s.trace.localOptions & optionTrace != 0
}

// NewChild creates a new span with the given name as a child of s.
// If s is nil, does nothing and returns nil.
func (s *Span) NewChild(name string) *Span {
	if s == nil {
		return nil
	}
	if !s.Traced() {
		// TODO(jbd): Document this behavior in godoc here and elsewhere.
		return s
	}
	return startNewChild(name, s.trace, s.spanID)
}

// NewRemoteChild creates a new span as a child of s.
func (s *Span) NewRemoteChild(r *http.Request) *Span {
	if s == nil {
		return nil
	}
	if !s.Traced() {
		r.Header[httpTraceHeader] = []string{spanHeader(s.trace.traceID, s.parentSpanID, s.trace.globalOptions)}
		return s
	}
	newSpan := startNewChildWithRequest(r, s.trace, s.spanID)
	r.Header[httpTraceHeader] = []string{spanHeader(s.trace.traceID, newSpan.spanID, s.trace.globalOptions)}
	return newSpan
}

// Header returns the value of the X-Cloud-Trace-Context header that
// should be used to propagate the span.  This is the inverse of
// SpanFromHeader.
//
// Most users should use NewRemoteChild unless they have specific
// propagation needs or want to control the naming of their span.
// Header() does not create a new span.
func (s *Span) Header() string {
	if s == nil {
		return ""
	}
	return spanHeader(s.trace.traceID, s.spanID, s.trace.globalOptions)
}

func startNewChildWithRequest(r *http.Request, trace *trace, parentSpanID uint64) *Span {
	name := r.URL.Host + r.URL.Path // drop scheme and query params
	newSpan := startNewChild(name, trace, parentSpanID)
	if r.Host == "" {
		newSpan.host = r.URL.Host
	} else {
		newSpan.host = r.Host
	}
	newSpan.method = r.Method
	newSpan.url = r.URL.String()
	return newSpan
}

func startNewChild(name string, trace *trace, parentSpanID uint64) *Span {
	spanID := nextSpanID()
	for spanID == parentSpanID {
		spanID = nextSpanID()
	}
	newSpan := &Span{
		trace: trace,
		name: name,
		kind: spanKindClient,
		parentSpanID: parentSpanID,
		spanID: spanID,
		start: time.Now(),
	}
	// if trace.localOptions & optionStack != 0 {
	// 	_ = runtime.Callers(1, newSpan.stack[:])
	// }
	return newSpan
}

// TraceID returns the ID of the trace to which s belongs.
func (s *Span) TraceID() string {
	if s == nil {
		return ""
	}
	return s.trace.traceID
}

// SetLabel sets the label for the given key to the given value.
// If the value is empty, the label for that key is deleted.
// If a label is given a value automatically and by SetLabel, the
// automatically-set value is used.
// If s is nil, does nothing.
//
// SetLabel shouldn't be called after Finish or FinishWait.
func (s *Span) SetLabel(key, value string) {
	if s == nil {
		return
	}
	if !s.Traced() {
		return
	}
	s.spanMu.Lock()
	defer s.spanMu.Unlock()

	if value == "" {
		if s.labels != nil {
			delete(s.labels, key)
		}
		return
	}
	if s.labels == nil {
		s.labels = make(map[string]string)
	}
	s.labels[key] = value
}

// Finish declares that the span has finished.
//
// If s is nil, Finish does nothing and returns nil.
//
// If the option trace.WithResponse(resp) is passed, then some labels are set
// for s using information in the given *http.Response.  This is useful when the
// span is for an outgoing http request; s will typically have been created by
// NewRemoteChild in this case.
//
// If s is a root span (one created by SpanFromRequest) then s, and all its
// descendant spans that have finished, are uploaded to the Google Stackdriver
// Trace server asynchronously.
func (s *Span) Finish(opts ...FinishOption) {
	if s == nil {
		return
	}
	if !s.Traced() {
		return
	}
	s.trace.finish(s, false, opts...)
}

// FinishWait is like Finish, but if s is a root span, it waits until uploading
// is finished, then returns an error if one occurred.
func (s *Span) FinishWait(opts ...FinishOption) error {
	if s == nil {
		return nil
	}
	if !s.Traced() {
		return nil
	}
	return s.trace.finish(s, true, opts...)
}

// FinishOption ...
type FinishOption interface {
	modifySpan(s *Span)
}

type withResponse struct {
	*http.Response
}

// WithResponse returns an option that can be passed to Finish that indicates
// that some labels for the span should be set using the given *http.Response.
func WithResponse(resp *http.Response) FinishOption {
	return withResponse{resp}
}

func (u withResponse) modifySpan(s *Span) {
	if u.Response != nil {
		s.statusCode = u.StatusCode
	}
}

func spanHeader(traceID string, spanID uint64, options optionFlags) string {
	// See https://cloud.google.com/trace/docs/faq for the header format.
	return fmt.Sprintf("%s/%d;o=%d", traceID, spanID, options)
}