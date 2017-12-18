package hystrix

import (
	"sync/atomic"
	"log"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type StatusType = int32
const (
	Closed StatusType = iota
	Open
	HalfOpen
)

// CircuitBreaker is created for each ExecutorPool to track whether requests
// should be attempted, or rejected if the Health of the circuit is too low.
type CircuitBreaker struct {
	name           string
	status         StatusType
	forceOpen      bool
	openedTime 	   int64
	metrics        MetricCollector
	limiter        *rate.Limiter
}

var circuitBreakers	sync.Map

// GetCircuit returns the circuit for the given command and whether this call created it.
func GetCircuit(name string) *CircuitBreaker {
	if v, ok := circuitBreakers.Load(name); ok {
		circuit := v.(*CircuitBreaker)
		return circuit
	}

	circuit := newCircuitBreaker(name)
	circuitBreakers.Store(name, circuit)
	return circuit
}

// newCircuitBreaker creates a CircuitBreaker with associated Health
func newCircuitBreaker(name string) *CircuitBreaker {
	config := getSettings(name)
	c := &CircuitBreaker{
		name: name,
		status: Closed,
		metrics: newMetricCollector(name, config.RollingWindows),
	}
	if config.MaxQPS > 0 {
		c.limiter = rate.NewLimiter(rate.Limit(config.MaxQPS), 1 + config.MaxQPS)
	}

	return c
}

// toggleForceOpen allows manually causing the fallback logic for all instances
// of a given command.
func (circuit *CircuitBreaker) toggleForceOpen(toggle bool) {
	circuit = GetCircuit(circuit.name)
	circuit.forceOpen = toggle
	return
}

// IsOpen Whether the circuit is currently open (tripped).
// @return boolean state of circuit breaker
func (circuit *CircuitBreaker) IsOpen() bool {
	if circuit.forceOpen {
		return true
	}

	if circuit.limiter != nil {
		now := time.Now()
		if !circuit.limiter.AllowN(now, 1) {
			return true
		}
	}

	return atomic.LoadInt64(&circuit.openedTime) > 0
}

// AllowRequest requests asks this if it is allowed to proceed or not.  It is idempotent and does
// not modify any internal state, and takes into account the half-open logic which allows some requests through
// after the circuit has been opened
// @return boolean whether a request should be permitted
func (circuit *CircuitBreaker) AllowRequest() bool {
	if circuit.forceOpen {
		return false
	}

	if circuit.limiter != nil {
		now := time.Now()
		if !circuit.limiter.AllowN(now, 1) {
			return false
		}
	}

	if atomic.LoadInt64(&circuit.openedTime) == 0 {
		return true
	} else {
		if atomic.LoadInt32(&circuit.status) == HalfOpen {
			return false
		} else {
			return circuit.isAfterSleepWindow()
		}
	}
}

func (circuit *CircuitBreaker) isAfterSleepWindow() bool {
	circuitOpenTime := atomic.LoadInt64(&circuit.openedTime)
	currentTime := time.Now().UnixNano()
	config := getSettings(circuit.name)
	return currentTime > circuitOpenTime + config.SleepWindow.Nanoseconds()
}

// AttemptExecution invoked at start of command execution to attempt an execution.  This is non-idempotent - it may modify internal state
func (circuit *CircuitBreaker) AttemptExecution() bool {
	if circuit.forceOpen {
		return false
	}

	if circuit.limiter != nil {
		now := time.Now()
		if !circuit.limiter.AllowN(now, 1) {
			return false
		}
	}

	if atomic.LoadInt64(&circuit.openedTime) == 0 {
		return true
	} else {
		if circuit.isAfterSleepWindow() {
			//only the first request after sleep window should execute
			//if the executing command succeeds, the status will transition to CLOSED
			//if the executing command fails, the status will transition to OPEN
			//if the executing command gets unsubscribed, the status will transition to OPEN
			if atomic.CompareAndSwapInt32(&circuit.status, Open, HalfOpen) {
				log.Printf("hystrix: try circuit %s with half-open", circuit.name)
				return true
			} else {
				return false
			}
		} else {
			return false;
		}
	}	
}

// Invoked on successful executions from {@link HystrixCommand} as part of feedback mechanism when in a half-open state.
func (circuit *CircuitBreaker) markSuccess() {
	if atomic.CompareAndSwapInt32(&circuit.status, HalfOpen, Closed) {
		//This goroutine wins the race to close the circuit - it resets the stream to start it over from 0
		circuit.metrics.Reset()
		atomic.StoreInt64(&circuit.openedTime, 0)		
		log.Printf("hystrix: closing circuit %s", circuit.name)
	}
}

// Invoked on unsuccessful executions from {@link HystrixCommand} as part of feedback mechanism when in a half-open state.
func (circuit *CircuitBreaker) markNonSuccess() {
	if atomic.CompareAndSwapInt32(&circuit.status, HalfOpen, Open) {
		//This goroutine wins the race to re-open the circuit - it resets the start time for the sleep window
		atomic.StoreInt64(&circuit.openedTime, time.Now().UnixNano())
		log.Printf("hystrix: opening circuit %s", circuit.name)
	}
}

// ReportEvent records command metrics for tracking recent error rates
func (circuit *CircuitBreaker) ReportEvent(t errorType) {
	switch t {
	case successTypeError:
		circuit.metrics.IncrementAttempts()
		circuit.metrics.IncrementSuccesses()
		circuit.markSuccess()
	case failureTypeError:
		circuit.metrics.IncrementAttempts()
		circuit.metrics.IncrementErrors()
		circuit.markNonSuccess()
	case circuitTypeError:
		circuit.metrics.IncrementAttempts()
		circuit.metrics.IncrementFailures()
	case timeoutTypeError:
		circuit.metrics.IncrementAttempts()
		circuit.metrics.IncrementErrors()
		circuit.markNonSuccess()
	}

	// update health counter
	now := time.Now()
	if circuit.metrics.NumRequests().Sum(now) < getSettings(circuit.name).RequestVolumeThreshold {
		// we are not past the minimum volume threshold for the stat window,
		// so no change to circuit status.
		// if it was CLOSED, it stays CLOSED
		// if it was half-open, we need to wait for a successful command execution
		// if it was open, we need to wait for sleep window to elapse
	} else {
		if circuit.metrics.IsHealthy(now) {
			//we are not past the minimum error threshold for the stat window,
			// so no change to circuit status.
			// if it was CLOSED, it stays CLOSED
			// if it was half-open, we need to wait for a successful command execution
			// if it was open, we need to wait for sleep window to elapse
		} else {
			// our failure rate is too high, we need to set the state to OPEN
			if atomic.CompareAndSwapInt32(&circuit.status, Closed, Open) {
				atomic.StoreInt64(&circuit.openedTime, now.UnixNano())
				log.Printf("hystrix: opening circuit %s due to failure rate is too high", circuit.name)
			}
		}
	}
}