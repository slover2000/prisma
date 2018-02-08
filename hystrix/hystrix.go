package hystrix

import (
	"errors"

	"golang.org/x/net/context"
)

type errorType = int
const (
	successTypeError errorType = iota 
	failureTypeError
	circuitTypeError
	timeoutTypeError
)

type actionResult struct{
	value interface{}
	err   error
}

var errCircuitOpen error
var errCircuitReject error
var errCircuitTimeout error

type runFunc = func() (interface{}, error)
type fallbackFunc = func(error) (interface{}, error)

type hystrixKey struct{}

func init() {
	errCircuitOpen = errors.New("circuit open")
	errCircuitReject = errors.New("circuit reject")
	errCircuitTimeout = errors.New("circuit timeout")
}

func WithGroup(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, hystrixKey{}, name)
}

func GetHystrixCommand(ctx context.Context) (string, bool) {
	v := ctx.Value(hystrixKey{})
	if v != nil {
		if name, ok := v.(string); ok {
			return name, true
		}
	}
	return "", false
}

func InitHystrix(configs map[string]CommandConfig) {
	for k, v := range configs {
		ConfigureCommand(k, v)
	}
}

func StopHystrix(timeout int) {
	stopAllCircuit(timeout)
}

// Execute runs your function in a synchronous manner, blocking until either your function succeeds
// or an error is returned, including hystrix circuit errors
func Execute(ctx context.Context, name string, run runFunc, fallback fallbackFunc) (interface{}, error) {
	eventType := successTypeError
	circuit := GetCircuit(name)
	defer circuit.ReportEvent(eventType)
	
	if circuit.AttemptExecution() {
		result := circuit.Excute(ctx, run, fallback)
		if result.err != nil {
			eventType = failureTypeError
		}
		return result.value, result.err
	} else {
		eventType = circuitTypeError
		if fallback != nil {
			return fallback(errCircuitOpen)
		} else {
			return nil, errCircuitOpen
		}		
	}
}