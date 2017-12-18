package hystrix

import (
	"sync"
	"time"

	"github.com/slover2000/prisma/hystrix/internal"
)

// MetricCollector represents the contract that all collectors must fulfill to gather circuit statistics.
// Implementations of this interface do not have to maintain locking around thier data stores so long as
// they are not modified outside of the hystrix context.
type MetricCollector interface {
	// IncrementAttempts increments the number of updates.
	IncrementAttempts()
	// IncrementErrors increments the number of unsuccessful attempts.
	// Attempts minus Errors will equal successes within a time range.
	// Errors are any result from an attempt that is not a success.
	IncrementErrors()
	// IncrementSuccesses increments the number of requests that succeed.
	IncrementSuccesses()
	// IncrementFailures increments the number of requests that fail.
	IncrementFailures()

	NumRequests() *internal.Number
	Errors() *internal.Number
	Successes() *internal.Number
	Failures() *internal.Number
	IsHealthy(now time.Time) bool
	Reset()
}

type metricCollector struct {
	name        string
	windows     int
	mutex       *sync.RWMutex
	numRequests *internal.Number
	errors      *internal.Number

	successes   *internal.Number
	failures    *internal.Number
}

func newMetricCollector(name string, windows int) MetricCollector {
	m := &metricCollector{
		name: name,
		windows: windows,
		mutex: &sync.RWMutex{},
		numRequests: internal.NewNumber(windows),
		errors: internal.NewNumber(windows),
		successes: internal.NewNumber(windows),
		failures: internal.NewNumber(windows),
	}
	return m
}

// NumRequests returns the rolling number of requests
func (d *metricCollector) NumRequests() *internal.Number {
	d.mutex.RLock()
	defer d.mutex.RUnlock()	
	return d.numRequests
}

// Errors returns the rolling number of errors
func (d *metricCollector) Errors() *internal.Number {
	d.mutex.RLock()
	defer d.mutex.RUnlock()	
	return d.errors
}

// Successes returns the rolling number of successes
func (d *metricCollector) Successes() *internal.Number {
	d.mutex.RLock()
	defer d.mutex.RUnlock()	
	return d.successes
}

// Failures returns the rolling number of failures
func (d *metricCollector) Failures() *internal.Number {
	d.mutex.RLock()
	defer d.mutex.RUnlock()	
	return d.failures
}

// IncrementAttempts increments the number of requests seen in the latest time bucket.
func (d *metricCollector) IncrementAttempts() {
	d.mutex.RLock()
	defer d.mutex.RUnlock()	
	d.numRequests.Increment(1)
}

// IncrementErrors increments the number of errors seen in the latest time bucket.
// Errors are any result from an attempt that is not a success.
func (d *metricCollector) IncrementErrors() {
	d.mutex.RLock()
	defer d.mutex.RUnlock()		
	d.errors.Increment(1)
}

// IncrementSuccesses increments the number of successes seen in the latest time bucket.
func (d *metricCollector) IncrementSuccesses() {
	d.mutex.RLock()
	defer d.mutex.RUnlock()		
	d.successes.Increment(1)
}

// IncrementFailures increments the number of failures seen in the latest time bucket.
func (d *metricCollector) IncrementFailures() {
	d.mutex.RLock()
	defer d.mutex.RUnlock()		
	d.failures.Increment(1)
}

func (d *metricCollector) errorPercent(now time.Time) int {
	reqs := d.NumRequests().Sum(now)	
	if reqs > 0 {
		errs := d.Errors().Sum(now)
		errPct := (float64(errs) / float64(reqs)) * 100
		return int(errPct + 0.5)
	}

	return 0
}

func (d *metricCollector) IsHealthy(now time.Time) bool {
	return d.errorPercent(now) < getSettings(d.name).ErrorPercentThreshold
}

func (d *metricCollector) Reset() {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.numRequests = internal.NewNumber(d.windows)
	d.errors = internal.NewNumber(d.windows)
	d.successes = internal.NewNumber(d.windows)
	d.failures = internal.NewNumber(d.windows)
}