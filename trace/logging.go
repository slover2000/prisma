package trace

import (
	"time"
	"path"

	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

const (
	SystemField 		= "system"
	KindField 			= "span.kind"
	GRPCServiceField	= "grpc.service"
	GRPCMethodField		= "grpc.method"
	GRPCCodeField		= "grpc.code"
	GRPCDurationField	= "grpc.duration"
)

type LogLevel = int
const (
	DebugLevel LogLevel = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
)

type loggingOptions struct {
	level			LogLevel
	entry 			*logrus.Entry
}

var (
	defaultLoggingOptions = &loggingOptions{
		level:    	InfoLevel,
	}
)

type LoggingOption func(*loggingOptions)

func LoggingLevel(level LogLevel) LoggingOption {
	return func(o *loggingOptions) { o.level = level }
}

func LoggingEntry(e *logrus.Entry) LoggingOption {
	return func(o *loggingOptions) { o.entry = e }
}

func grpcErrorToCode(err error) codes.Code {
	return grpc.Code(err)
}

func logGrpcClientLine(options *loggingOptions, ctx context.Context, fullMethodString string, startTime time.Time, err error, msg string) {
	if options.entry == nil {
		return
	}

	code := grpc.Code(err)
	level := grpcCodeToLogrusLevel(code)
	if level < loglevelToLogusLevel(options.level) {
		return
	}

	durVal := time.Now().Sub(startTime)	
	fields := newGrpcClientLoggerFields(ctx, fullMethodString)
	fields[GRPCCodeField] = code.String()
	fields[GRPCDurationField] = durVal
	if err != nil {
		fields[logrus.ErrorKey] = err
	}

	logMessageWithLevel(options.entry.WithFields(fields), level, msg)
}

func newGrpcClientLoggerFields(ctx context.Context, fullMethodString string) logrus.Fields {
	service := path.Dir(fullMethodString)[1:]
	method := path.Base(fullMethodString)
	return logrus.Fields{
		SystemField:    	"grpc",
		KindField:      	SpanKindClient,
		GRPCServiceField: 	service,
		GRPCMethodField:  	method,
	}
}

func logMessageWithLevel(entry *logrus.Entry, level logrus.Level, msg string) {
	switch level {
	case logrus.DebugLevel:
		entry.Debug(msg)
	case logrus.InfoLevel:
		entry.Info(msg)
	case logrus.WarnLevel:
		entry.Warn(msg)
	case logrus.ErrorLevel:
		entry.Error(msg)
	case logrus.FatalLevel:
		entry.Fatal(msg)
	}
}

func loglevelToLogusLevel(lvl LogLevel) logrus.Level {
	switch lvl {
	case DebugLevel:
		return logrus.DebugLevel
	case InfoLevel:
		return logrus.InfoLevel
	case WarnLevel:
		return logrus.WarnLevel
	case ErrorLevel:
		return logrus.ErrorLevel
	case FatalLevel:
		return logrus.FatalLevel
	default:
		return logrus.InfoLevel
	}
}

// grpcCodeToLogrusLevel is the default implementation of gRPC return codes to log levels for server side.
func grpcCodeToLogrusLevel(code codes.Code) logrus.Level {
	switch code {
	case codes.OK:
		return logrus.InfoLevel
	case codes.Canceled:
		return logrus.InfoLevel
	case codes.Unknown:
		return logrus.ErrorLevel
	case codes.InvalidArgument:
		return logrus.InfoLevel
	case codes.DeadlineExceeded:
		return logrus.WarnLevel
	case codes.NotFound:
		return logrus.InfoLevel
	case codes.AlreadyExists:
		return logrus.InfoLevel
	case codes.PermissionDenied:
		return logrus.WarnLevel
	case codes.Unauthenticated:
		return logrus.InfoLevel // unauthenticated requests can happen
	case codes.ResourceExhausted:
		return logrus.WarnLevel
	case codes.FailedPrecondition:
		return logrus.WarnLevel
	case codes.Aborted:
		return logrus.WarnLevel
	case codes.OutOfRange:
		return logrus.WarnLevel
	case codes.Unimplemented:
		return logrus.ErrorLevel
	case codes.Internal:
		return logrus.ErrorLevel
	case codes.Unavailable:
		return logrus.WarnLevel
	case codes.DataLoss:
		return logrus.ErrorLevel
	default:
		return logrus.ErrorLevel
	}
}