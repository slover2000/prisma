package prisma

import (
	"fmt"
	"io"
	"sync"
	"encoding/hex"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/slover2000/prisma/trace"
)

const grpcMetadataKey = "x-trace-bin"

// GRPCUnaryClientInterceptor returns a grpc.UnaryClientInterceptor that traces all outgoing requests from a gRPC client.
// The calling context should already have a *trace.Span; a child span will be
// created for the outgoing gRPC call. If the calling context doesn't have a span,
// the call will not be traced. If the client is nil, then the interceptor just
// passes through the request.
//
// The functionality in gRPC that this feature relies on is currently experimental.
func (c *InterceptorClient) GRPCUnaryClientInterceptor() grpc.UnaryClientInterceptor {
	if c == nil {
		return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			return invoker(ctx, method, req, reply, cc, opts...)
		}
	}
	return grpc.UnaryClientInterceptor(c.grpcUnaryInterceptor)
}

func (c *InterceptorClient) grpcUnaryInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	span := trace.FromContext(ctx).NewChild(method)
	if span == nil {
		span = c.trace.NewClientKindSpan(method)
	}
	defer span.Finish()
	
	outgoingCtx := buildClientOutgoingContext(ctx, span)
	err := invoker(outgoingCtx, method, req, reply, cc, opts...)
	if err != nil {
		span.SetLabel(trace.LabelError, err.Error())
	}

	// log unary client grpc	
	c.log.LogGrpcClientLine(outgoingCtx, method, span.Start(), err, fmt.Sprintf("finished client call %s.", method))
	return err
}

// GRPCStreamClientInterceptor returns a grpc.StreamClientInterceptor that traces all outgoing requests from a gRPC client.
func (c *InterceptorClient) GRPCStreamClientInterceptor() grpc.StreamClientInterceptor {
	if c == nil {
		return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			return streamer(ctx, desc, cc, method, opts...)
		}
	}

	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		span := trace.FromContext(ctx).NewChild(method)
		if span == nil {
			span = c.trace.NewClientKindSpan(method)
		}

		outgoingCtx := buildClientOutgoingContext(ctx, span)
		clientStream, err := streamer(outgoingCtx, desc, cc, method, opts...)
		if err != nil {		
			finishClientSpan(outgoingCtx, c, span, err)
			return nil, err
		}
		return &tracedClientStream{ClientStream: clientStream, client: c, clientSpan: span}, nil
	}
}

func buildClientOutgoingContext(parentCtx context.Context, span *trace.Span) context.Context {
	// traceID is a hex-encoded 128-bit value.
	// TODO(jbd): Decode trace IDs upon arrival and
	// represent trace IDs with 16 bytes internally.
	tid, err := hex.DecodeString(span.TraceID())
	if err != nil {
		return parentCtx		
	}

	traceContext := make([]byte, trace.TraceContextLen)
	trace.PackTrace(traceContext, tid, span.SpanID(), byte(span.TraceGloablOptions()))
	md, ok := metadata.FromOutgoingContext(parentCtx)
	if !ok {
		md = metadata.Pairs(grpcMetadataKey, string(traceContext))
	} else {
		md = md.Copy() // metadata is immutable, copy.
		md[grpcMetadataKey] = []string{string(traceContext)}
	}
	ctx := metadata.NewOutgoingContext(parentCtx, md)
	
	return ctx
}

// type serverStreamingRetryingStream is the implementation of grpc.ClientStream that acts as a
// proxy to the underlying call. If any of the RecvMsg() calls fail, it will try to reestablish
// a new ClientStream according to the retry policy.
type tracedClientStream struct {
	grpc.ClientStream
	mu              	sync.Mutex
	alreadyFinished 	bool
	client 						*InterceptorClient
	clientSpan				*trace.Span
}

func (s *tracedClientStream) Header() (metadata.MD, error) {
	h, err := s.ClientStream.Header()
	if err != nil {
		s.finishClientSpan(err)
	}
	return h, err
}

func (s *tracedClientStream) SendMsg(m interface{}) error {
	err := s.ClientStream.SendMsg(m)
	if err != nil {
		s.finishClientSpan(err)
	}
	return err
}

func (s *tracedClientStream) CloseSend() error {
	err := s.ClientStream.CloseSend()
	if err != nil {
		s.finishClientSpan(err)
	}
	return err
}

func (s *tracedClientStream) RecvMsg(m interface{}) error {
	err := s.ClientStream.RecvMsg(m)
	if err != nil {
		s.finishClientSpan(err)
	}
	return err
}

func (s *tracedClientStream) finishClientSpan(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.alreadyFinished {
		finishClientSpan(s.Context(), s.client, s.clientSpan, err)
		s.alreadyFinished = true
	}
}

func finishClientSpan(ctx context.Context, client *InterceptorClient, clientSpan *trace.Span, err error) {
	// log stream client grpc
	method := clientSpan.Name()
	var logErr error
	if err != nil && err != io.EOF {
		logErr = err
	}
	client.log.LogGrpcClientLine(ctx, method, clientSpan.Start(), logErr, fmt.Sprintf("finished client call %s.", method))

	if err != nil && err != io.EOF {
		clientSpan.SetLabel(trace.LabelError, err.Error())
	}
	clientSpan.Finish()
}

// GRPCUnaryServerInterceptor returns a grpc.UnaryServerInterceptor that enables the tracing of the incoming
// gRPC calls. Incoming call's context can be used to extract the span on servers that enabled this option:
//
//	span := trace.FromContext(ctx)
//
// If the client is nil, then the interceptor just invokes the handler.
//
// The functionality in gRPC that this feature relies on is currently experimental.
func (c *InterceptorClient) GRPCUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	if c == nil {
		return func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
			return handler(ctx, req)
		}
	}

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		md, _ := metadata.FromIncomingContext(ctx)
		var span *trace.Span
		if header, ok := md[grpcMetadataKey]; ok {
			span = c.trace.SpanFromContext(info.FullMethod, header[0])
			defer span.Finish()
		}
		ctx = trace.NewContext(ctx, span)
		resp, err = handler(ctx, req)

		// log unary server grpc
		c.log.LogGrpcClientLine(ctx, info.FullMethod, span.Start(), err, fmt.Sprintf("finished service %s.", info.FullMethod))
		return
	}
}

// WrappedServerStream is a thin wrapper around grpc.ServerStream that allows modifying context.
type WrappedServerStream struct {
	grpc.ServerStream
	// WrappedContext is the wrapper's own Context. You can assign it.
	WrappedContext context.Context
}

// Context returns the wrapper's WrappedContext, overwriting the nested grpc.ServerStream.Context()
func (w *WrappedServerStream) Context() context.Context {
	return w.WrappedContext
}

// WrapServerStream returns a ServerStream that has the ability to overwrite context.
func wrapServerStream(stream grpc.ServerStream) *WrappedServerStream {
	if existing, ok := stream.(*WrappedServerStream); ok {
		return existing
	}
	return &WrappedServerStream{ServerStream: stream, WrappedContext: stream.Context()}
}

// GRPStreamServerInterceptor returns a new streaming server interceptor for OpenTracing.
func (c *InterceptorClient) GRPStreamServerInterceptor() grpc.StreamServerInterceptor {
	if c == nil {
		return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			return handler(srv, stream)
		}
	}

	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := stream.Context()
		md, _ := metadata.FromIncomingContext(ctx)
		var span *trace.Span
		if header, ok := md[grpcMetadataKey]; ok {
			span = c.trace.SpanFromContext(info.FullMethod, header[0])
			defer span.Finish()
		}		
		
		ctx = trace.NewContext(ctx, span)
		wrappedStream := wrapServerStream(stream)
		wrappedStream.WrappedContext = ctx
		err := handler(srv, stream)

		// log stream server grpc
		c.log.LogGrpcClientLine(ctx, info.FullMethod, span.Start(), err, fmt.Sprintf("finished service %s.", info.FullMethod))
		return err
	}
}