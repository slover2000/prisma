package metrics

import (
	"time"
	"strings"
	"net/http"
)

type grpcType = string
const (
	Unary        grpcType = "unary"
	ClientStream grpcType = "client_stream"
	ServerStream grpcType = "server_stream"
	BidiStream   grpcType = "bidi_stream"
)

// ClientMetrics represent a metrics client
type ClientMetrics interface {
	CounterGRPC(name string, duration time.Duration, err error)
	CounterHTTP(req *http.Request, duration time.Duration, code int)
	RegisterHttpHandler(endpoint string)
	Close() error
}

func SplitGRPCMethodName(fullMethodName string) (string, string) {
	fullMethodName = strings.TrimPrefix(fullMethodName, "/") // remove leading slash
	if i := strings.Index(fullMethodName, "/"); i >= 0 {
		return fullMethodName[:i], fullMethodName[i+1:]
	}
	return "unknown", "unknown"
}

func ConvertMethodName(fullMethodName string) string {
	fullMethodName = strings.TrimPrefix(fullMethodName, "/") // remove leading slash
	return strings.Replace(fullMethodName, "/", ".", -1)
}