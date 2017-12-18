package hystrix

import (
	//"fmt"
	"log"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// CircuitBreaker is created for each ExecutorPool to track whether requests
// should be attempted, or rejected if the Health of the circuit is too low.
type CircuitBreaker struct {
	Name           string
	open           bool
	forceOpen      bool
	openedTime 	   int64
	metrics        MetricCollector
	*rate.Limiter
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
		Name: name,
		metrics: newMetricCollector(name, config.RollingWindows),
		Limiter: rate.NewLimiter(rate.Limit(config.MaxQPS), 1 + config.MaxQPS),
	}

	return c
}

// toggleForceOpen allows manually causing the fallback logic for all instances
// of a given command.
func (circuit *CircuitBreaker) toggleForceOpen(toggle bool) {
	circuit = GetCircuit(circuit.Name)
	circuit.forceOpen = toggle
	return
}

// IsOpen is called before any Command execution to check whether or
// not it should be attempted. An "open" circuit means it is disabled.
func (circuit *CircuitBreaker) IsOpen() bool {
	now := time.Now()
	config := getSettings(circuit.Name)
	
	if circuit.forceOpen {
		return true
	}

	if circuit.open && (now.UnixNano() - circuit.openedTime) <= config.SleepWindow.Nanoseconds() {
		return true
	}
	
	if !circuit.AllowN(now, 1) {
		return true
	}

	if circuit.metrics.NumRequests().Sum(now) < getSettings(circuit.Name).RequestVolumeThreshold {
		return false
	}

	if !circuit.metrics.IsHealthy(now) {
		// too many failures, open the circuit
		circuit.setOpen()
		return true
	}

	return false
}

// AllowRequest is checked before a command executes, ensuring that circuit state and metric health allow it.
// When the circuit is open, this call will occasionally return true to measure whether the external service
// has recovered.
func (circuit *CircuitBreaker) AllowRequest() bool {
	return !circuit.IsOpen()
}

func (circuit *CircuitBreaker) setOpen() {
	if circuit.open {
		return
	}

	log.Printf("hystrix: opening circuit %v", circuit.Name)

	circuit.openedTime = time.Now().UnixNano()
	circuit.open = true
}

func (circuit *CircuitBreaker) setClose() {
	if !circuit.open {
		return
	}

	log.Printf("hystrix: closing circuit %v", circuit.Name)
	circuit.openedTime = 0
	circuit.open = false
}

// ReportEvent records command metrics for tracking recent error rates
func (circuit *CircuitBreaker) ReportEvent(t errorType) {
	switch t {
	case successTypeError:
		circuit.metrics.IncrementAttempts()
		circuit.metrics.IncrementSuccesses()
		if circuit.open {
			circuit.setClose()
		}
	case failureTypeError:
		circuit.metrics.IncrementAttempts()
		circuit.metrics.IncrementFailures()
	case circuitTypeError:
		circuit.metrics.IncrementAttempts()
		circuit.metrics.IncrementErrors()
	case timeoutTypeError:
		circuit.metrics.IncrementAttempts()
		circuit.metrics.IncrementErrors()
	}
}