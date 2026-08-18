// Package partition provides a partitioned Bloom filter that distributes
// elements across multiple independent sub-filters. This reduces contention
// in concurrent scenarios and enables per-partition rotation/expiration.
package partition

import (
	"errors"
	"sync"

	"bloom-filter/internal/filter"
	"bloom-filter/internal/hash"
)

// Errors returned by the partition package.
var (
	ErrInvalidPartitions = errors.New("partition: partition count must be > 0")
	ErrInvalidParams     = errors.New("partition: m and k must be > 0")
	ErrNilFilter         = errors.New("partition: nil partitioned filter")
	ErrPartitionIndex    = errors.New("partition: index out of range")
)

// Filter is a partitioned Bloom filter. Elements are routed to a specific
// partition based on a routing hash, then added/tested within that partition's
// sub-filter.
type Filter struct {
	mu         sync.RWMutex
	partitions []*filter.BloomFilter
	count      int
	m          uint // bits per partition
	k          uint // hash functions per partition
}

// New creates a partitioned Bloom filter with the given number of partitions.
// Each partition has m bits and k hash functions.
func New(partitions int, m, k uint) (*Filter, error) {
	if partitions <= 0 {
		return nil, ErrInvalidPartitions
	}
	if m == 0 || k == 0 {
		return nil, ErrInvalidParams
	}
	parts := make([]*filter.BloomFilter, partitions)
	for i := range parts {
		bf, err := filter.New(m, k)
		if err != nil {
			return nil, err
		}
		parts[i] = bf
	}
	return &Filter{
		partitions: parts,
		count:      partitions,
		m:          m,
		k:          k,
	}, nil
}

// Add inserts data into the appropriate partition.
func (f *Filter) Add(data []byte) {
	idx := f.route(data)
	f.mu.Lock()
	f.partitions[idx].Add(data)
	f.mu.Unlock()
}

// Test checks whether data may be present. It checks only the routed partition.
func (f *Filter) Test(data []byte) bool {
	idx := f.route(data)
	f.mu.RLock()
	result := f.partitions[idx].Test(data)
	f.mu.RUnlock()
	return result
}

// TestAll checks all partitions for the item. Returns true if any partition
// reports the item as possibly present. This is useful after rotations where
// the item might be in a non-routed partition.
func (f *Filter) TestAll(data []byte) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, p := range f.partitions {
		if p.Test(data) {
			return true
		}
	}
	return false
}

// PartitionCount returns the number of partitions.
func (f *Filter) PartitionCount() int {
	return f.count
}

// BitsPerPartition returns m for each sub-filter.
func (f *Filter) BitsPerPartition() uint {
	return f.m
}

// HashFunctions returns k.
func (f *Filter) HashFunctions() uint {
	return f.k
}

// TotalBits returns the total number of bits across all partitions.
func (f *Filter) TotalBits() uint {
	return f.m * uint(f.count)
}

// Partition returns the sub-filter at the given index.
// Returns ErrPartitionIndex if idx is out of range.
func (f *Filter) Partition(idx int) (*filter.BloomFilter, error) {
	if idx < 0 || idx >= f.count {
		return nil, ErrPartitionIndex
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.partitions[idx], nil
}

// Reset clears all bits in all partitions.
func (f *Filter) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.partitions {
		bf, _ := filter.New(f.m, f.k)
		f.partitions[i] = bf
	}
}

// ResetPartition clears only the specified partition.
func (f *Filter) ResetPartition(idx int) error {
	if idx < 0 || idx >= f.count {
		return ErrPartitionIndex
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	bf, _ := filter.New(f.m, f.k)
	f.partitions[idx] = bf
	return nil
}

// Saturation returns the fraction of bits set for a given partition.
func (f *Filter) Saturation(idx int) (float64, error) {
	if idx < 0 || idx >= f.count {
		return 0, ErrPartitionIndex
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	bits := f.partitions[idx].Bits()
	set := countBits(bits)
	return float64(set) / float64(f.m), nil
}

// route determines which partition should handle the given data.
// Uses a single hash to pick the partition index.
func (f *Filter) route(data []byte) int {
	// Use hash function index 0 with a large modulus, then mod by partition count
	h := hash.Hash(data, 0, uint(f.count)*1000)
	return int(h) % f.count
}

func countBits(b []byte) int {
	n := 0
	for _, v := range b {
		n += popcount(v)
	}
	return n
}

func popcount(b byte) int {
	count := 0
	for b != 0 {
		b &= b - 1
		count++
	}
	return count
}
