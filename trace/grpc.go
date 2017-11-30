package trace

import (
	"io"
	"sync"
	"encoding/hex"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const grpcMetadataKey = "x-trace-bin"

// GRPCUnaryClientInterceptor returns a grpc.UnaryClientInterceptor that traces all outgoing requests from a gRPC client.
// The calling context should already have a *trace.Span; a child span will be
// created for the outgoing gRPC call. If the calling context doesn't have a span,
// the call will not be traced. If the client is nil, then the interceptor just
// passes through the request.
//
// The functionality in gRPC that this feature relies on is currently experimental.
func (c *Client) GRPCUnaryClientInterceptor() grpc.UnaryClientInterceptor {
	if c == nil {
		return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			return invoker(ctx, method, req, reply, cc, opts...)
		}
	}
	return grpc.UnaryClientInterceptor(c.grpcUnaryInterceptor)
}

func (c *Client) grpcUnaryInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	// TODO: also intercept streams.
	span := FromContext(ctx).NewChild(method)
	if span == nil {
		span = c.NewSpan(method)
	}
	defer span.Finish()
	
	outgoingCtx, err := buildClientOutgoingContext(ctx, span)
	if err != nil {
		return invoker(ctx, method, req, reply, cc, opts...)
	}

	err = invoker(outgoingCtx, method, req, reply, cc, opts...)
	if err != nil {
		// TODO: standardize gRPC label names?
		span.SetLabel("error", err.Error())
	}
	return err
}

// GRPCStreamClientInterceptor returns a grpc.StreamClientInterceptor that traces all outgoing requests from a gRPC client.
func (c *Client) GRPCStreamClientInterceptor() grpc.StreamClientInterceptor {
	if c == nil {
		return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			return streamer(ctx, desc, cc, method, opts...)
		}
	}

	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		span := FromContext(ctx).NewChild(method)
		if span == nil {
			span = c.NewSpan(method)
		}

		outgoingCtx, err := buildClientOutgoingContext(ctx, span)
		if err != nil {
			return streamer(ctx, desc, cc, method, opts...)
		}

		clientStream, err := streamer(outgoingCtx, desc, cc, method, opts...)
		if err != nil {
			// TODO: standardize gRPC label names?
			span.SetLabel("error", err.Error())
			finishClientSpan(span, err)
			return nil, err
		}
		return &tracedClientStream{ClientStream: clientStream, clientSpan: span}, nil
	}
}

func buildClientOutgoingContext(parentCtx context.Context, span *Span) (context.Context, error) {
	// traceID is a hex-encoded 128-bit value.
	// TODO(jbd): Decode trace IDs upon arrival and
	// represent trace IDs with 16 bytes internally.
	tid, err := hex.DecodeString(span.trace.traceID)
	if err != nil {
		return nil, err		
	}

	traceContext := make([]byte, traceContextLen)
	packTrace(traceContext, tid, span.spanID, byte(span.trace.globalOptions))
	md, ok := metadata.FromOutgoingContext(parentCtx)
	if !ok {
		md = metadata.Pairs(grpcMetadataKey, string(traceContext))
	} else {
		md = md.Copy() // metadata is immutable, copy.
		md[grpcMetadataKey] = []string{string(traceContext)}
	}
	ctx := metadata.NewOutgoingContext(parentCtx, md)

	return ctx, nil
}

// type serverStreamingRetryingStream is the implementation of grpc.ClientStream that acts as a
// proxy to the underlying call. If any of the RecvMsg() calls fail, it will try to reestablish
// a new ClientStream according to the retry policy.
type tracedClientStream struct {
	grpc.ClientStream
	mu              	sync.Mutex
	alreadyFinished 	bool
	clientSpan			*Span
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
		finishClientSpan(s.clientSpan, err)
		s.alreadyFinished = true
	}
}

func finishClientSpan(clientSpan *Span, err error) {
	if err != nil && err != io.EOF {
		// TODO: standardize gRPC label names?
		clientSpan.SetLabel("error", err.Error())		
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
func (c *Client) GRPCUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	if c == nil {
		return func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
			return handler(ctx, req)
		}
	}

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		md, _ := metadata.FromIncomingContext(ctx)
		var span *Span
		if header, ok := md[grpcMetadataKey]; ok {
			span = c.SpanFromContext(info.FullMethod, header[0])
			defer span.Finish()
		}		
		ctx = NewContext(ctx, span)
		return handler(ctx, req)
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
func (c *Client) GRPStreamServerInterceptor() grpc.StreamServerInterceptor {
	if c == nil {
		return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			return handler(srv, stream)
		}
	}

	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := stream.Context()
		md, _ := metadata.FromIncomingContext(ctx)
		var span *Span
		if header, ok := md[grpcMetadataKey]; ok {
			span = c.SpanFromContext(info.FullMethod, header[0])
			defer span.Finish()
		}		
		
		ctx = NewContext(ctx, span)
		wrappedStream := wrapServerStream(stream)
		wrappedStream.WrappedContext = ctx
		err := handler(srv, stream)
		return err
	}
}