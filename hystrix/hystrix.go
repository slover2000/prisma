package hystrix

import (
	"errors"

	"golang.org/x/net/context"
	"github.com/slover2000/prisma/metrics"
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

type runFunc = func() (interface{}, error)
type fallbackFunc = func(error) (interface{}, error)

type hystrixKey struct{}

func init() {
	errCircuitOpen = errors.New("circuit open")
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

// Execute runs your function in a synchronous manner, blocking until either your function succeeds
// or an error is returned, including hystrix circuit errors
func Execute(ctx context.Context, name string, run runFunc, fallback fallbackFunc) (interface{}, error) {
	done := make(chan actionResult, 1)
	eventType := successTypeError
	circuit := GetCircuit(name)
	go func() {
		var result actionResult		
		if circuit.AttemptExecution() {
			value, err := run()
			result.value = value
			result.err = err
			if err != nil {
				eventType = failureTypeError
			}
		} else {
			if fallback != nil {
				value, err := fallback(errCircuitOpen)
				result.value = value
				result.err = err
			} else {
				result.err = errCircuitOpen
			}
			eventType = circuitTypeError
		}
		done <- result
	}()
	defer circuit.ReportEvent(eventType)

	select {
	case result, _ := <-done:
		return result.value, result.err
	case <-ctx.Done():
		eventType = timeoutTypeError
		if fallback != nil {
			return fallback(ctx.Err())
		}
		return nil, ctx.Err()
	}
}