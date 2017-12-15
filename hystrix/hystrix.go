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

type runFunc func() (interface{}, error)
type fallbackFunc func(error) (interface{}, error)

func init() {
	errCircuitOpen = errors.New("circuit open")
}

// Execute runs your function in a synchronous manner, blocking until either your function succeeds
// or an error is returned, including hystrix circuit errors
func Execute(ctx context.Context, name string, run runFunc, fallback fallbackFunc) (interface{}, error) {
	done := make(chan actionResult, 1)
	eventType := successTypeError
	circuit := GetCircuit(name)
	go func() {
		var result actionResult		
		if circuit.AllowRequest() {
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
				eventType = circuitTypeError
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
		return nil, ctx.Err()
	}
}