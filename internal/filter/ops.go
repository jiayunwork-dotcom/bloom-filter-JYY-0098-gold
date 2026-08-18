package filter

import "errors"

// Errors for filter operations.
var (
	ErrNilFilter     = errors.New("filter: nil filter")
	ErrParamMismatch = errors.New("filter: filters must have same m and k")
)

// Reset clears all bits in the filter, making it empty.
func (f *BloomFilter) Reset() {
	for i := range f.bits {
		f.bits[i] = 0
	}
}

// Clone creates a deep copy of the filter.
func (f *BloomFilter) Clone() *BloomFilter {
	cp := make([]byte, len(f.bits))
	copy(cp, f.bits)
	return &BloomFilter{m: f.m, k: f.k, bits: cp}
}

// Union computes bitwise OR with another filter, modifying f in place.
// Both filters must have identical m and k.
func (f *BloomFilter) Union(other *BloomFilter) error {
	if other == nil {
		return ErrNilFilter
	}
	if f.m != other.m || f.k != other.k {
		return ErrParamMismatch
	}
	for i := range f.bits {
		f.bits[i] |= other.bits[i]
	}
	return nil
}

// Intersection computes bitwise AND with another filter, modifying f in place.
// Both filters must have identical m and k.
func (f *BloomFilter) Intersection(other *BloomFilter) error {
	if other == nil {
		return ErrNilFilter
	}
	if f.m != other.m || f.k != other.k {
		return ErrParamMismatch
	}
	for i := range f.bits {
		f.bits[i] &= other.bits[i]
	}
	return nil
}

// Equals checks if two filters have identical parameters and bit arrays.
func (f *BloomFilter) Equals(other *BloomFilter) bool {
	if other == nil {
		return false
	}
	if f.m != other.m || f.k != other.k {
		return false
	}
	if len(f.bits) != len(other.bits) {
		return false
	}
	for i := range f.bits {
		if f.bits[i] != other.bits[i] {
			return false
		}
	}
	return true
}

// IsEmpty returns true if no bits are set.
func (f *BloomFilter) IsEmpty() bool {
	for _, b := range f.bits {
		if b != 0 {
			return false
		}
	}
	return true
}

// PopCount returns the number of set bits in the filter.
func (f *BloomFilter) PopCount() int {
	count := 0
	for _, b := range f.bits {
		count += popcnt(b)
	}
	return count
}

// Saturation returns the fraction of set bits: popCount / m.
func (f *BloomFilter) Saturation() float64 {
	if f.m == 0 {
		return 0
	}
	return float64(f.PopCount()) / float64(f.m)
}

// Difference computes bitwise AND NOT (f &^ other), modifying f in place.
// Bits set in other are cleared in f.
func (f *BloomFilter) Difference(other *BloomFilter) error {
	if other == nil {
		return ErrNilFilter
	}
	if f.m != other.m || f.k != other.k {
		return ErrParamMismatch
	}
	for i := range f.bits {
		f.bits[i] &^= other.bits[i]
	}
	return nil
}

// XOR computes bitwise XOR with another filter, modifying f in place.
func (f *BloomFilter) XOR(other *BloomFilter) error {
	if other == nil {
		return ErrNilFilter
	}
	if f.m != other.m || f.k != other.k {
		return ErrParamMismatch
	}
	for i := range f.bits {
		f.bits[i] ^= other.bits[i]
	}
	return nil
}

// SetBit sets a specific bit position. No-op if pos >= m.
func (f *BloomFilter) SetBit(pos uint) {
	if pos >= f.m {
		return
	}
	f.bits[pos/8] |= 1 << (pos % 8)
}

// ClearBit clears a specific bit position. No-op if pos >= m.
func (f *BloomFilter) ClearBit(pos uint) {
	if pos >= f.m {
		return
	}
	f.bits[pos/8] &^= 1 << (pos % 8)
}

// GetBit returns whether the bit at pos is set.
func (f *BloomFilter) GetBit(pos uint) bool {
	if pos >= f.m {
		return false
	}
	return f.bits[pos/8]&(1<<(pos%8)) != 0
}

func popcnt(b byte) int {
	count := 0
	for b != 0 {
		b &= b - 1
		count++
	}
	return count
}
