// Package merge provides bitwise operations for combining Bloom filters.
// Two filters can be merged (union) or intersected when they share the same
// parameters (m and k). This is useful for distributed systems where each
// node maintains its own filter and results need to be combined.
package merge

import (
	"errors"
	"math"

	"bloom-filter/internal/filter"
)

// Errors returned by the merge package.
var (
	// ErrNilFilter is returned when a nil filter is passed.
	ErrNilFilter = errors.New("merge: nil filter")
	// ErrParamMismatch is returned when two filters have different m or k.
	ErrParamMismatch = errors.New("merge: filters must have same m and k")
	// ErrEmptyList is returned when an empty list of filters is provided.
	ErrEmptyList = errors.New("merge: empty filter list")
)

// Union combines two Bloom filters by bitwise OR. The resulting filter will
// report "present" for any item that was added to either filter.
// Both filters must have identical m and k parameters.
// Returns a new filter; the inputs are not modified.
func Union(a, b *filter.BloomFilter) (*filter.BloomFilter, error) {
	if a == nil || b == nil {
		return nil, ErrNilFilter
	}
	if a.M() != b.M() || a.K() != b.K() {
		return nil, ErrParamMismatch
	}
	bitsA := a.Bits()
	bitsB := b.Bits()
	merged := make([]byte, len(bitsA))
	for i := range merged {
		merged[i] = bitsA[i] | bitsB[i]
	}
	return filter.NewFromParts(a.M(), a.K(), merged)
}

// Intersection combines two Bloom filters by bitwise AND. The resulting filter
// will only report "present" for items where both filters agree. Note that this
// does NOT guarantee the item was in both original sets — it only means both
// filters have the relevant bits set (which can happen due to false positives).
// Both filters must have identical m and k parameters.
func Intersection(a, b *filter.BloomFilter) (*filter.BloomFilter, error) {
	if a == nil || b == nil {
		return nil, ErrNilFilter
	}
	if a.M() != b.M() || a.K() != b.K() {
		return nil, ErrParamMismatch
	}
	bitsA := a.Bits()
	bitsB := b.Bits()
	merged := make([]byte, len(bitsA))
	for i := range merged {
		merged[i] = bitsA[i] & bitsB[i]
	}
	return filter.NewFromParts(a.M(), a.K(), merged)
}

// Difference computes the bits present in a but not in b (bitwise AND NOT).
// This is an approximation — clearing bits that happen to overlap with b may
// cause false negatives for items that were legitimately in a.
// Use with care; primarily useful for estimating set difference cardinality.
func Difference(a, b *filter.BloomFilter) (*filter.BloomFilter, error) {
	if a == nil || b == nil {
		return nil, ErrNilFilter
	}
	if a.M() != b.M() || a.K() != b.K() {
		return nil, ErrParamMismatch
	}
	bitsA := a.Bits()
	bitsB := b.Bits()
	result := make([]byte, len(bitsA))
	for i := range result {
		result[i] = bitsA[i] &^ bitsB[i]
	}
	return filter.NewFromParts(a.M(), a.K(), result)
}

// UnionMany merges multiple filters into one via bitwise OR.
// All filters must share the same m and k. Returns a new filter.
func UnionMany(filters []*filter.BloomFilter) (*filter.BloomFilter, error) {
	if len(filters) == 0 {
		return nil, ErrEmptyList
	}
	if filters[0] == nil {
		return nil, ErrNilFilter
	}
	m := filters[0].M()
	k := filters[0].K()
	byteLen := len(filters[0].Bits())
	merged := make([]byte, byteLen)
	copy(merged, filters[0].Bits())

	for i := 1; i < len(filters); i++ {
		f := filters[i]
		if f == nil {
			return nil, ErrNilFilter
		}
		if f.M() != m || f.K() != k {
			return nil, ErrParamMismatch
		}
		bits := f.Bits()
		for j := range merged {
			merged[j] |= bits[j]
		}
	}
	return filter.NewFromParts(m, k, merged)
}

// IntersectionMany intersects multiple filters via bitwise AND.
// All filters must share the same m and k. Returns a new filter.
func IntersectionMany(filters []*filter.BloomFilter) (*filter.BloomFilter, error) {
	if len(filters) == 0 {
		return nil, ErrEmptyList
	}
	if filters[0] == nil {
		return nil, ErrNilFilter
	}
	m := filters[0].M()
	k := filters[0].K()
	byteLen := len(filters[0].Bits())
	merged := make([]byte, byteLen)
	copy(merged, filters[0].Bits())

	for i := 1; i < len(filters); i++ {
		f := filters[i]
		if f == nil {
			return nil, ErrNilFilter
		}
		if f.M() != m || f.K() != k {
			return nil, ErrParamMismatch
		}
		bits := f.Bits()
		for j := range merged {
			merged[j] &= bits[j]
		}
	}
	return filter.NewFromParts(m, k, merged)
}

// Similarity computes the Jaccard similarity between two filters:
// |A ∩ B| / |A ∪ B| where |·| counts set bits.
// Returns 0.0 for empty filters.
func Similarity(a, b *filter.BloomFilter) (float64, error) {
	if a == nil || b == nil {
		return 0, ErrNilFilter
	}
	if a.M() != b.M() || a.K() != b.K() {
		return 0, ErrParamMismatch
	}
	bitsA := a.Bits()
	bitsB := b.Bits()
	var andCount, orCount int
	for i := range bitsA {
		andCount += popcount(bitsA[i] & bitsB[i])
		orCount += popcount(bitsA[i] | bitsB[i])
	}
	if orCount == 0 {
		return 0, nil
	}
	return float64(andCount) / float64(orCount), nil
}

// HammingDistance returns the number of bit positions where two filters differ.
func HammingDistance(a, b *filter.BloomFilter) (int, error) {
	if a == nil || b == nil {
		return 0, ErrNilFilter
	}
	if a.M() != b.M() || a.K() != b.K() {
		return 0, ErrParamMismatch
	}
	bitsA := a.Bits()
	bitsB := b.Bits()
	dist := 0
	for i := range bitsA {
		dist += popcount(bitsA[i] ^ bitsB[i])
	}
	return dist, nil
}

// EstimateUnionCardinality estimates the number of distinct items in the union
// of two filters using the formula: n ≈ -(m/k) * ln(1 - bitsSet/m).
func EstimateUnionCardinality(a, b *filter.BloomFilter) (int, error) {
	u, err := Union(a, b)
	if err != nil {
		return 0, err
	}
	bitsSet := 0
	for _, v := range u.Bits() {
		bitsSet += popcount(v)
	}
	return estimateN(u.M(), u.K(), bitsSet), nil
}

// EstimateIntersectionCardinality estimates |A ∩ B| using inclusion-exclusion:
// |A ∩ B| ≈ |A| + |B| - |A ∪ B|
func EstimateIntersectionCardinality(a, b *filter.BloomFilter) (int, error) {
	if a == nil || b == nil {
		return 0, ErrNilFilter
	}
	if a.M() != b.M() || a.K() != b.K() {
		return 0, ErrParamMismatch
	}
	nA := countAndEstimate(a)
	nB := countAndEstimate(b)
	nUnion, err := EstimateUnionCardinality(a, b)
	if err != nil {
		return 0, err
	}
	nIntersect := nA + nB - nUnion
	if nIntersect < 0 {
		nIntersect = 0
	}
	return nIntersect, nil
}

func countAndEstimate(f *filter.BloomFilter) int {
	set := 0
	for _, v := range f.Bits() {
		set += popcount(v)
	}
	return estimateN(f.M(), f.K(), set)
}

func estimateN(m, k uint, bitsSet int) int {
	if bitsSet == 0 {
		return 0
	}
	mf := float64(m)
	kf := float64(k)
	x := float64(bitsSet)
	if x >= mf {
		return int(mf / kf)
	}
	n := -(mf / kf) * math.Log(1-x/mf)
	if n < 0 {
		return 0
	}
	return int(n)
}

func popcount(b byte) int {
	count := 0
	for b != 0 {
		b &= b - 1
		count++
	}
	return count
}
