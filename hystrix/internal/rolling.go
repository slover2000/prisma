package internal

import (
	"sync"
	"time"	
)

const (
	defaultWindowsLength = 10
)

// Number tracks a numberBucket over a bounded number of
// time buckets. Currently the buckets are one second long and only the last 10 seconds are kept.
type Number struct {
	buckets	 map[int64]int
	windows  int
	Mutex    *sync.RWMutex
}

// NewNumber initializes a RollingNumber struct.
// @param reserved define how many values should be reserved
func NewNumber(windows int) *Number {
	if windows == 0 {
		windows = defaultWindowsLength
	}
	r := &Number{
		windows: windows,
		Mutex:   &sync.RWMutex{},
	}
	return r
}

// Increment increments the number in current timeBucket.
func (r *Number) Increment(i int) {
	now := time.Now().Unix()

	r.Mutex.Lock()
	defer r.Mutex.Unlock()
	
	bucket, ok := r.buckets[now]
	if !ok {
		r.buckets[now] = i
	} else {
		r.buckets[now] = bucket + i
	}
	r.removeOldBuckets()
}

func (r *Number) removeOldBuckets() {
	if len(r.buckets) > r.windows {
		now := time.Now().Unix() - int64(r.windows)
		for timestamp := range r.buckets {
			if timestamp <= now {
				delete(r.buckets, timestamp)
			}
		}
	}
}

// Sum sums the values over the buckets in the last 10 seconds.
func (r *Number) Sum(now time.Time) int {
	sum := 0
	r.Mutex.RLock()
	defer r.Mutex.RUnlock()
	for timestamp, bucket := range r.buckets {		
		if timestamp >= now.Unix() - int64(r.windows) {
			sum += bucket
		}
	}

	return sum
}

// Max returns the maximum value seen in the lastest seconds.
func (r *Number) Max(now time.Time) int {
	max := 0

	r.Mutex.RLock()
	defer r.Mutex.RUnlock()
	for timestamp, bucket := range r.buckets {
		if timestamp >= now.Unix() - int64(r.windows) {
			if bucket > max {
				max = bucket
			}
		}
	}

	return max
}

// Avg returns the average value seen in the lastest seconds.
func (r *Number) Avg(now time.Time) int {
	return r.Sum(now) / r.windows
}