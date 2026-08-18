// Package counting implements a Counting Bloom Filter (CBF) that supports
// element deletion. Instead of single bits, each position holds a multi-bit
// counter (default 4 bits, allowing counts up to 15). Adding increments the
// counters; removing decrements them. This enables approximate set membership
// queries with removal capability at the cost of higher memory usage.
package counting

import (
	"errors"
	"sync"

	"bloom-filter/internal/hash"
)

// Errors returned by the counting package.
var (
	ErrInvalidParams  = errors.New("counting: m, k, and counterBits must be > 0")
	ErrCounterBits    = errors.New("counting: counterBits must be 1-8")
	ErrOverflow       = errors.New("counting: counter overflow (max reached)")
	ErrUnderflow      = errors.New("counting: counter underflow (already zero)")
	ErrNilFilter      = errors.New("counting: nil filter")
	ErrParamMismatch  = errors.New("counting: filters must have same parameters")
	ErrSizeMismatch   = errors.New("counting: data length mismatch")
)

// CountingFilter is a Bloom filter variant that uses multi-bit counters
// instead of single bits, enabling element removal.
type CountingFilter struct {
	mu          sync.RWMutex
	m           uint   // number of counter positions
	k           uint   // number of hash functions
	counterBits uint   // bits per counter (1-8)
	maxCount    uint8  // (1 << counterBits) - 1
	data        []byte // packed counters
	count       int    // number of elements added (approx)
}

// New creates a CountingFilter with m positions, k hash functions, and
// counterBits bits per counter. counterBits must be between 1 and 8.
// A counterBits of 4 gives a max count of 15, which is standard.
func New(m, k, counterBits uint) (*CountingFilter, error) {
	if m == 0 || k == 0 || counterBits == 0 {
		return nil, ErrInvalidParams
	}
	if counterBits > 8 {
		return nil, ErrCounterBits
	}
	totalBits := m * counterBits
	byteLen := (totalBits + 7) / 8
	return &CountingFilter{
		m:           m,
		k:           k,
		counterBits: counterBits,
		maxCount:    (1 << counterBits) - 1,
		data:        make([]byte, byteLen),
		count:       0,
	}, nil
}

// M returns the number of counter positions.
func (cf *CountingFilter) M() uint { return cf.m }

// K returns the number of hash functions.
func (cf *CountingFilter) K() uint { return cf.k }

// CounterBits returns the bits per counter.
func (cf *CountingFilter) CounterBits() uint { return cf.counterBits }

// Count returns the approximate number of elements added.
func (cf *CountingFilter) Count() int {
	cf.mu.RLock()
	defer cf.mu.RUnlock()
	return cf.count
}

// MaxCount returns the maximum value a single counter can hold.
func (cf *CountingFilter) MaxCount() uint8 { return cf.maxCount }

// Add inserts data into the filter by incrementing the k relevant counters.
// Returns ErrOverflow if any counter would exceed its maximum.
func (cf *CountingFilter) Add(data []byte) error {
	cf.mu.Lock()
	defer cf.mu.Unlock()

	// First check all counters to ensure no overflow
	positions := cf.positions(data)
	for _, pos := range positions {
		v := cf.getCounter(pos)
		if v >= cf.maxCount {
			return ErrOverflow
		}
	}
	// Increment all
	for _, pos := range positions {
		v := cf.getCounter(pos)
		cf.setCounter(pos, v+1)
	}
	cf.count++
	return nil
}

// Remove removes data from the filter by decrementing the k relevant counters.
// Returns ErrUnderflow if any counter is already at zero (item was never added
// or was already removed).
func (cf *CountingFilter) Remove(data []byte) error {
	cf.mu.Lock()
	defer cf.mu.Unlock()

	positions := cf.positions(data)
	// Check for underflow
	for _, pos := range positions {
		v := cf.getCounter(pos)
		if v == 0 {
			return ErrUnderflow
		}
	}
	// Decrement all
	for _, pos := range positions {
		v := cf.getCounter(pos)
		cf.setCounter(pos, v-1)
	}
	cf.count--
	return nil
}

// Test checks if data may be present. Returns true if all k counters are > 0.
func (cf *CountingFilter) Test(data []byte) bool {
	cf.mu.RLock()
	defer cf.mu.RUnlock()

	for _, pos := range cf.positions(data) {
		if cf.getCounter(pos) == 0 {
			return false
		}
	}
	return true
}

// CountAt returns the counter value at a specific position.
func (cf *CountingFilter) CountAt(pos uint) uint8 {
	cf.mu.RLock()
	defer cf.mu.RUnlock()
	if pos >= cf.m {
		return 0
	}
	return cf.getCounter(pos)
}

// Reset clears all counters and resets the element count.
func (cf *CountingFilter) Reset() {
	cf.mu.Lock()
	defer cf.mu.Unlock()
	for i := range cf.data {
		cf.data[i] = 0
	}
	cf.count = 0
}

// Data returns a copy of the raw counter data for serialization.
func (cf *CountingFilter) Data() []byte {
	cf.mu.RLock()
	defer cf.mu.RUnlock()
	out := make([]byte, len(cf.data))
	copy(out, cf.data)
	return out
}

// Saturation returns the fraction of non-zero counters.
func (cf *CountingFilter) Saturation() float64 {
	cf.mu.RLock()
	defer cf.mu.RUnlock()
	nonZero := 0
	for i := uint(0); i < cf.m; i++ {
		if cf.getCounter(i) > 0 {
			nonZero++
		}
	}
	return float64(nonZero) / float64(cf.m)
}

// positions returns the k hash positions for data.
func (cf *CountingFilter) positions(data []byte) []uint {
	pos := make([]uint, cf.k)
	for i := uint(0); i < cf.k; i++ {
		pos[i] = hash.Hash(data, i, cf.m)
	}
	return pos
}

// getCounter reads the counter value at position pos.
func (cf *CountingFilter) getCounter(pos uint) uint8 {
	bitOffset := pos * cf.counterBits
	return cf.readBits(bitOffset, cf.counterBits)
}

// setCounter writes a counter value at position pos.
func (cf *CountingFilter) setCounter(pos uint, val uint8) {
	bitOffset := pos * cf.counterBits
	cf.writeBits(bitOffset, cf.counterBits, val)
}

// readBits reads n bits starting at bitOffset from the data array.
func (cf *CountingFilter) readBits(bitOffset, n uint) uint8 {
	var result uint8
	for i := uint(0); i < n; i++ {
		byteIdx := (bitOffset + i) / 8
		bitIdx := (bitOffset + i) % 8
		if byteIdx < uint(len(cf.data)) && cf.data[byteIdx]&(1<<bitIdx) != 0 {
			result |= 1 << i
		}
	}
	return result
}

// writeBits writes n bits starting at bitOffset in the data array.
func (cf *CountingFilter) writeBits(bitOffset, n uint, val uint8) {
	for i := uint(0); i < n; i++ {
		byteIdx := (bitOffset + i) / 8
		bitIdx := (bitOffset + i) % 8
		if byteIdx >= uint(len(cf.data)) {
			break
		}
		if val&(1<<i) != 0 {
			cf.data[byteIdx] |= 1 << bitIdx
		} else {
			cf.data[byteIdx] &^= 1 << bitIdx
		}
	}
}
