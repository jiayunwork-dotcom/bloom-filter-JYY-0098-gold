package stats

import (
	"sync"
	"time"
)

// RateTracker tracks operation rates over sliding time windows.
// It counts events in fixed-size buckets and computes per-second rates.
type RateTracker struct {
	mu         sync.Mutex
	buckets    []bucket
	bucketSize time.Duration
	window     time.Duration
	numBuckets int
}

type bucket struct {
	start time.Time
	count int64
}

// NewRateTracker creates a tracker with the given window duration and bucket
// granularity. For example, window=60s and bucketSize=1s gives 60 buckets.
// If bucketSize > window or either is <= 0, sensible defaults are applied.
func NewRateTracker(window, bucketSize time.Duration) *RateTracker {
	if window <= 0 {
		window = 60 * time.Second
	}
	if bucketSize <= 0 || bucketSize > window {
		bucketSize = time.Second
	}
	numBuckets := int(window / bucketSize)
	if numBuckets < 1 {
		numBuckets = 1
	}
	buckets := make([]bucket, numBuckets)
	now := time.Now()
	for i := range buckets {
		buckets[i].start = now
	}
	return &RateTracker{
		buckets:    buckets,
		bucketSize: bucketSize,
		window:     window,
		numBuckets: numBuckets,
	}
}

// Record adds n events at the current time.
func (rt *RateTracker) Record(n int64) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	now := time.Now()
	idx := rt.bucketIndex(now)
	b := &rt.buckets[idx]
	if now.Sub(b.start) >= rt.bucketSize {
		// This bucket has expired; reset it
		b.start = now.Truncate(rt.bucketSize)
		b.count = n
	} else {
		b.count += n
	}
}

// Rate returns the average events per second over the configured window.
func (rt *RateTracker) Rate() float64 {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-rt.window)
	var total int64
	var activeDuration time.Duration

	for i := range rt.buckets {
		b := &rt.buckets[i]
		if b.start.After(cutoff) || b.start.Equal(cutoff) {
			total += b.count
			elapsed := now.Sub(b.start)
			if elapsed > rt.bucketSize {
				elapsed = rt.bucketSize
			}
			activeDuration += elapsed
		}
	}

	if activeDuration <= 0 {
		return 0
	}
	return float64(total) / activeDuration.Seconds()
}

// Total returns the sum of all events in the current window.
func (rt *RateTracker) Total() int64 {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-rt.window)
	var total int64
	for i := range rt.buckets {
		if rt.buckets[i].start.After(cutoff) || rt.buckets[i].start.Equal(cutoff) {
			total += rt.buckets[i].count
		}
	}
	return total
}

// Reset clears all buckets.
func (rt *RateTracker) Reset() {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	now := time.Now()
	for i := range rt.buckets {
		rt.buckets[i].start = now
		rt.buckets[i].count = 0
	}
}

func (rt *RateTracker) bucketIndex(t time.Time) int {
	ns := t.UnixNano()
	bucketNs := int64(rt.bucketSize)
	idx := int((ns / bucketNs) % int64(rt.numBuckets))
	if idx < 0 {
		idx += rt.numBuckets
	}
	return idx
}

// WindowSummary returns a summary of the rate tracker state.
type WindowSummary struct {
	Window       time.Duration
	BucketSize   time.Duration
	NumBuckets   int
	ActiveTotal  int64
	RatePerSec   float64
}

// Summary returns the current window summary.
func (rt *RateTracker) Summary() WindowSummary {
	return WindowSummary{
		Window:      rt.window,
		BucketSize:  rt.bucketSize,
		NumBuckets:  rt.numBuckets,
		ActiveTotal: rt.Total(),
		RatePerSec:  rt.Rate(),
	}
}
