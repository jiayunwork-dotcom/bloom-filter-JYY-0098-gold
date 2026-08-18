// Package stats provides runtime statistics and metrics collection for Bloom
// filter operations. It tracks add/test counts, hit/miss ratios, latency
// percentiles, and saturation history over time.
package stats

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics holds counters for filter operations.
type Metrics struct {
	Adds       int64 // total Add calls
	Tests      int64 // total Test calls
	Hits       int64 // Test calls that returned true
	Misses     int64 // Test calls that returned false
	Removes    int64 // Remove calls (counting filters)
	Overflows  int64 // Add calls that hit counter overflow
	Underflows int64 // Remove calls that hit counter underflow
}

// HitRate returns hits / tests. Returns 0 if no tests have been performed.
func (m *Metrics) HitRate() float64 {
	total := m.Tests
	if total == 0 {
		return 0
	}
	return float64(m.Hits) / float64(total)
}

// MissRate returns misses / tests. Returns 0 if no tests have been performed.
func (m *Metrics) MissRate() float64 {
	total := m.Tests
	if total == 0 {
		return 0
	}
	return float64(m.Misses) / float64(total)
}

// Collector gathers runtime statistics for a Bloom filter.
// It is safe for concurrent use.
type Collector struct {
	adds       atomic.Int64
	tests      atomic.Int64
	hits       atomic.Int64
	misses     atomic.Int64
	removes    atomic.Int64
	overflows  atomic.Int64
	underflows atomic.Int64

	mu       sync.Mutex
	history  []Snapshot
	maxHist  int
	created  time.Time
}

// Snapshot represents a point-in-time metrics snapshot.
type Snapshot struct {
	Timestamp  time.Time
	Metrics    Metrics
	Saturation float64 // filter saturation at snapshot time
	Elapsed    time.Duration
}

// NewCollector creates a new stats collector with the given history buffer size.
// If maxHistory <= 0, defaults to 1000.
func NewCollector(maxHistory int) *Collector {
	if maxHistory <= 0 {
		maxHistory = 1000
	}
	return &Collector{
		history: make([]Snapshot, 0, maxHistory),
		maxHist: maxHistory,
		created: time.Now(),
	}
}

// RecordAdd increments the add counter.
func (c *Collector) RecordAdd() {
	c.adds.Add(1)
}

// RecordTest increments the test counter and records hit or miss.
func (c *Collector) RecordTest(hit bool) {
	c.tests.Add(1)
	if hit {
		c.hits.Add(1)
	} else {
		c.misses.Add(1)
	}
}

// RecordRemove increments the remove counter.
func (c *Collector) RecordRemove() {
	c.removes.Add(1)
}

// RecordOverflow increments the overflow counter.
func (c *Collector) RecordOverflow() {
	c.overflows.Add(1)
}

// RecordUnderflow increments the underflow counter.
func (c *Collector) RecordUnderflow() {
	c.underflows.Add(1)
}

// Metrics returns a snapshot of the current counters.
func (c *Collector) Metrics() Metrics {
	return Metrics{
		Adds:       c.adds.Load(),
		Tests:      c.tests.Load(),
		Hits:       c.hits.Load(),
		Misses:     c.misses.Load(),
		Removes:    c.removes.Load(),
		Overflows:  c.overflows.Load(),
		Underflows: c.underflows.Load(),
	}
}

// TakeSnapshot records a point-in-time snapshot with the given saturation value.
// Older snapshots are evicted when the buffer is full.
func (c *Collector) TakeSnapshot(saturation float64) Snapshot {
	m := c.Metrics()
	snap := Snapshot{
		Timestamp:  time.Now(),
		Metrics:    m,
		Saturation: saturation,
		Elapsed:    time.Since(c.created),
	}
	c.mu.Lock()
	if len(c.history) >= c.maxHist {
		// Evict oldest 10%
		evict := c.maxHist / 10
		if evict < 1 {
			evict = 1
		}
		c.history = c.history[evict:]
	}
	c.history = append(c.history, snap)
	c.mu.Unlock()
	return snap
}

// History returns all recorded snapshots.
func (c *Collector) History() []Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Snapshot, len(c.history))
	copy(out, c.history)
	return out
}

// HistorySince returns snapshots after the given time.
func (c *Collector) HistorySince(t time.Time) []Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []Snapshot
	for _, s := range c.history {
		if s.Timestamp.After(t) {
			out = append(out, s)
		}
	}
	return out
}

// Reset zeroes all counters and clears history.
func (c *Collector) Reset() {
	c.adds.Store(0)
	c.tests.Store(0)
	c.hits.Store(0)
	c.misses.Store(0)
	c.removes.Store(0)
	c.overflows.Store(0)
	c.underflows.Store(0)
	c.mu.Lock()
	c.history = c.history[:0]
	c.created = time.Now()
	c.mu.Unlock()
}

// Uptime returns the time since the collector was created (or last reset).
func (c *Collector) Uptime() time.Duration {
	return time.Since(c.created)
}
