package hash

import (
	"encoding/binary"
	"hash/fnv"
)

// Strategy enumerates the available hash strategies.
type Strategy int

const (
	// StrategyFNV uses the default FNV-1a / FNV-1 double hashing.
	StrategyFNV Strategy = iota
	// StrategyMurmur3 uses MurmurHash3-128 double hashing.
	StrategyMurmur3
	// StrategyEnhancedDouble uses enhanced double hashing with a quadratic term.
	StrategyEnhancedDouble
	// StrategyTriple uses triple hashing: h1 + i*h2 + i^2*h3.
	StrategyTriple
)

// HashWithStrategy returns the i-th bit index using the specified strategy.
func HashWithStrategy(data []byte, i, m uint, s Strategy) uint {
	switch s {
	case StrategyMurmur3:
		return Murmur3Hash(data, i, m)
	case StrategyEnhancedDouble:
		return EnhancedDoubleHash(data, i, m)
	case StrategyTriple:
		return TripleHash(data, i, m)
	default:
		return Hash(data, i, m)
	}
}

// EnhancedDoubleHash implements enhanced double hashing with a quadratic
// correction term that improves bit dispersion for large k values:
//
//	idx = (h1 + i*h2 + (i*(i-1))/2) mod m
//
// This variant was proposed by Dillinger & Manolios (2004) and reduces the
// correlation between positions compared to standard double hashing.
func EnhancedDoubleHash(data []byte, i, m uint) uint {
	if m == 0 {
		return 0
	}
	h1 := fnv64a(data)
	h2 := fnv64(data)
	// Enhanced: add quadratic correction (i*(i-1))/2
	correction := uint64(i * (i - 1) / 2)
	idx := (h1 + uint64(i)*h2 + correction) % uint64(m)
	return uint(idx)
}

// TripleHash uses three independent hash functions to compute bit positions:
//
//	idx = (h1 + i*h2 + i^2*h3) mod m
//
// h1 and h2 come from FNV-1a/FNV-1 as usual; h3 is derived from a seeded
// FNV-1a (data prepended with a fixed salt byte).
func TripleHash(data []byte, i, m uint) uint {
	if m == 0 {
		return 0
	}
	h1 := fnv64a(data)
	h2 := fnv64(data)
	h3 := seededFNV64a(data, 0x9E)
	idx := (h1 + uint64(i)*h2 + uint64(i)*uint64(i)*h3) % uint64(m)
	return uint(idx)
}

// seededFNV64a computes FNV-1a with a leading salt byte to produce a hash
// independent from the standard fnv64a.
func seededFNV64a(data []byte, seed byte) uint64 {
	h := fnv.New64a()
	h.Write([]byte{seed})
	h.Write(data)
	return h.Sum64()
}

// MultiHash computes k hash indices at once and returns them as a slice.
// This is more efficient than calling Hash k times when you need all positions.
func MultiHash(data []byte, k, m uint, s Strategy) []uint {
	if m == 0 || k == 0 {
		return nil
	}
	positions := make([]uint, k)
	for i := uint(0); i < k; i++ {
		positions[i] = HashWithStrategy(data, i, m, s)
	}
	return positions
}

// IndependentHashes returns multiple fully independent hash values by varying
// a uint64 seed appended to the data. Slower but provides better independence
// guarantees for analysis/benchmarking.
func IndependentHashes(data []byte, k, m uint) []uint {
	if m == 0 || k == 0 {
		return nil
	}
	positions := make([]uint, k)
	buf := make([]byte, len(data)+8)
	copy(buf, data)
	for i := uint(0); i < k; i++ {
		binary.LittleEndian.PutUint64(buf[len(data):], uint64(i))
		h := fnv.New64a()
		h.Write(buf)
		positions[i] = uint(h.Sum64() % uint64(m))
	}
	return positions
}

// Dispersion computes how well-spread the k positions are for the given data.
// Returns a value in [0, 1]: 1.0 means perfectly uniform distribution,
// lower values indicate clustering. Uses coefficient of variation of gaps.
func Dispersion(data []byte, k, m uint, s Strategy) float64 {
	if k <= 1 || m == 0 {
		return 1.0
	}
	positions := MultiHash(data, k, m, s)
	// Sort positions (insertion sort — k is small)
	for i := 1; i < len(positions); i++ {
		key := positions[i]
		j := i - 1
		for j >= 0 && positions[j] > key {
			positions[j+1] = positions[j]
			j--
		}
		positions[j+1] = key
	}
	// Compute gaps (circular)
	gaps := make([]float64, k)
	for i := uint(0); i < k-1; i++ {
		gaps[i] = float64(positions[i+1] - positions[i])
	}
	gaps[k-1] = float64(m-positions[k-1]) + float64(positions[0])

	// Ideal gap
	idealGap := float64(m) / float64(k)
	if idealGap == 0 {
		return 1.0
	}

	// Coefficient of variation: lower CV = better dispersion
	var sumSqDiff float64
	for _, g := range gaps {
		diff := g - idealGap
		sumSqDiff += diff * diff
	}
	variance := sumSqDiff / float64(k)
	stddev := sqrt(variance)
	cv := stddev / idealGap
	// Map to [0,1]: 1 is perfect, 0 is worst
	score := 1.0 / (1.0 + cv)
	return score
}

// sqrt computes square root via Newton's method (avoids importing math).
func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	guess := x / 2
	for i := 0; i < 20; i++ {
		guess = (guess + x/guess) / 2
	}
	return guess
}
