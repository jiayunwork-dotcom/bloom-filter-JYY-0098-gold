package partition

import (
	"errors"
	"sync"
	"time"

	"bloom-filter/internal/filter"
)

// Rotation errors.
var (
	ErrRotationStopped = errors.New("partition: rotation already stopped")
	ErrNoPartitions    = errors.New("partition: no partitions to rotate")
)

// RotationStrategy defines how partitions are cycled.
type RotationStrategy int

const (
	// StrategyRoundRobin resets the oldest partition on each rotation tick.
	StrategyRoundRobin RotationStrategy = iota
	// StrategySaturation resets the most saturated partition.
	StrategySaturation
)

// RotationConfig configures automatic partition rotation.
type RotationConfig struct {
	Interval        time.Duration
	Strategy        RotationStrategy
	SaturationLimit float64 // only for StrategySaturation: trigger at this fill ratio
}

// DefaultRotationConfig returns sensible defaults: rotate every 60s, round-robin.
func DefaultRotationConfig() RotationConfig {
	return RotationConfig{
		Interval:        60 * time.Second,
		Strategy:        StrategyRoundRobin,
		SaturationLimit: 0.5,
	}
}

// RotatingFilter wraps a partitioned Filter with automatic rotation.
type RotatingFilter struct {
	mu      sync.Mutex
	pf      *Filter
	cfg     RotationConfig
	current int // next partition index to reset (round-robin)
	stop    chan struct{}
	stopped bool
	history []RotationEvent
}

// RotationEvent records when and why a partition was reset.
type RotationEvent struct {
	Timestamp  time.Time
	Partition  int
	Strategy   RotationStrategy
	Saturation float64 // saturation before reset
}

// NewRotatingFilter creates a RotatingFilter. Does NOT start automatic rotation;
// call Start() for that, or call Rotate() manually.
func NewRotatingFilter(partitions int, m, k uint, cfg RotationConfig) (*RotatingFilter, error) {
	pf, err := New(partitions, m, k)
	if err != nil {
		return nil, err
	}
	return &RotatingFilter{
		pf:      pf,
		cfg:     cfg,
		current: 0,
		stop:    make(chan struct{}),
		history: make([]RotationEvent, 0, 64),
	}, nil
}

// Add inserts data.
func (rf *RotatingFilter) Add(data []byte) {
	rf.pf.Add(data)
}

// Test checks membership.
func (rf *RotatingFilter) Test(data []byte) bool {
	return rf.pf.TestAll(data)
}

// Rotate performs a single manual rotation according to the configured strategy.
func (rf *RotatingFilter) Rotate() error {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.rotateOnce()
}

// Start begins automatic periodic rotation in a background goroutine.
func (rf *RotatingFilter) Start() {
	go rf.runLoop()
}

// Stop halts the automatic rotation goroutine.
func (rf *RotatingFilter) Stop() error {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.stopped {
		return ErrRotationStopped
	}
	rf.stopped = true
	close(rf.stop)
	return nil
}

// History returns the rotation event log.
func (rf *RotatingFilter) History() []RotationEvent {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	out := make([]RotationEvent, len(rf.history))
	copy(out, rf.history)
	return out
}

// Filter returns the underlying partitioned filter.
func (rf *RotatingFilter) Filter() *Filter {
	return rf.pf
}

func (rf *RotatingFilter) runLoop() {
	ticker := time.NewTicker(rf.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-rf.stop:
			return
		case <-ticker.C:
			rf.mu.Lock()
			_ = rf.rotateOnce()
			rf.mu.Unlock()
		}
	}
}

func (rf *RotatingFilter) rotateOnce() error {
	n := rf.pf.PartitionCount()
	if n == 0 {
		return ErrNoPartitions
	}

	var idx int
	var sat float64

	switch rf.cfg.Strategy {
	case StrategySaturation:
		idx, sat = rf.mostSaturated()
		if sat < rf.cfg.SaturationLimit {
			// Not saturated enough; skip
			return nil
		}
	default: // StrategyRoundRobin
		idx = rf.current
		sat, _ = rf.pf.Saturation(idx)
		rf.current = (rf.current + 1) % n
	}

	rf.history = append(rf.history, RotationEvent{
		Timestamp:  time.Now(),
		Partition:  idx,
		Strategy:   rf.cfg.Strategy,
		Saturation: sat,
	})

	_ = rf.pf.ResetPartition(idx)
	return nil
}

func (rf *RotatingFilter) mostSaturated() (int, float64) {
	maxSat := 0.0
	maxIdx := 0
	for i := 0; i < rf.pf.PartitionCount(); i++ {
		s, _ := rf.pf.Saturation(i)
		if s > maxSat {
			maxSat = s
			maxIdx = i
		}
	}
	return maxIdx, maxSat
}

// Snapshot exports raw bits from all partitions for persistence.
func (rf *RotatingFilter) Snapshot() [][]byte {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	n := rf.pf.PartitionCount()
	out := make([][]byte, n)
	for i := 0; i < n; i++ {
		p, _ := rf.pf.Partition(i)
		bits := p.Bits()
		cp := make([]byte, len(bits))
		copy(cp, bits)
		out[i] = cp
	}
	return out
}

// Restore rebuilds partitions from a snapshot produced by Snapshot.
func (rf *RotatingFilter) Restore(snap [][]byte) error {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	n := rf.pf.PartitionCount()
	if len(snap) != n {
		return ErrInvalidPartitions
	}
	for i := 0; i < n; i++ {
		bf, err := filter.NewFromParts(rf.pf.m, rf.pf.k, snap[i])
		if err != nil {
			return err
		}
		rf.pf.mu.Lock()
		rf.pf.partitions[i] = bf
		rf.pf.mu.Unlock()
	}
	return nil
}
