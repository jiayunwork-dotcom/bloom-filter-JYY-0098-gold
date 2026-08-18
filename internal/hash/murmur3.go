package hash

// murmur3_128 implements a simplified MurmurHash3 128-bit variant for x86_64.
// This is a self-contained implementation (no external dependencies) used as
// an alternative hash family for enhanced double hashing.

// Murmur3Hash returns the i-th bit index in a filter of m bits using
// MurmurHash3-128 double hashing: idx = (h1 + i*h2) mod m.
func Murmur3Hash(data []byte, i, m uint) uint {
	if m == 0 {
		return 0
	}
	h1, h2 := murmur3Sum128(data, 0)
	return uint((h1 + uint64(i)*h2) % uint64(m))
}

// Murmur3Sum128 exposes the raw 128-bit hash for external use.
func Murmur3Sum128(data []byte, seed uint64) (uint64, uint64) {
	return murmur3Sum128(data, seed)
}

const (
	c1_128 uint64 = 0x87c37b91114253d5
	c2_128 uint64 = 0x4cf5ad432745937f
)

func murmur3Sum128(data []byte, seed uint64) (uint64, uint64) {
	h1 := seed
	h2 := seed
	length := len(data)

	// Process 16-byte blocks
	nblocks := length / 16
	for i := 0; i < nblocks; i++ {
		k1 := getBlock64(data, i*2)
		k2 := getBlock64(data, i*2+1)

		k1 *= c1_128
		k1 = rotl64(k1, 31)
		k1 *= c2_128
		h1 ^= k1
		h1 = rotl64(h1, 27)
		h1 += h2
		h1 = h1*5 + 0x52dce729

		k2 *= c2_128
		k2 = rotl64(k2, 33)
		k2 *= c1_128
		h2 ^= k2
		h2 = rotl64(h2, 31)
		h2 += h1
		h2 = h2*5 + 0x38495ab5
	}

	// Process remaining bytes (tail)
	tail := data[nblocks*16:]
	var k1, k2 uint64

	switch len(tail) & 15 {
	case 15:
		k2 ^= uint64(tail[14]) << 48
		fallthrough
	case 14:
		k2 ^= uint64(tail[13]) << 40
		fallthrough
	case 13:
		k2 ^= uint64(tail[12]) << 32
		fallthrough
	case 12:
		k2 ^= uint64(tail[11]) << 24
		fallthrough
	case 11:
		k2 ^= uint64(tail[10]) << 16
		fallthrough
	case 10:
		k2 ^= uint64(tail[9]) << 8
		fallthrough
	case 9:
		k2 ^= uint64(tail[8])
		k2 *= c2_128
		k2 = rotl64(k2, 33)
		k2 *= c1_128
		h2 ^= k2
		fallthrough
	case 8:
		k1 ^= uint64(tail[7]) << 56
		fallthrough
	case 7:
		k1 ^= uint64(tail[6]) << 48
		fallthrough
	case 6:
		k1 ^= uint64(tail[5]) << 40
		fallthrough
	case 5:
		k1 ^= uint64(tail[4]) << 32
		fallthrough
	case 4:
		k1 ^= uint64(tail[3]) << 24
		fallthrough
	case 3:
		k1 ^= uint64(tail[2]) << 16
		fallthrough
	case 2:
		k1 ^= uint64(tail[1]) << 8
		fallthrough
	case 1:
		k1 ^= uint64(tail[0])
		k1 *= c1_128
		k1 = rotl64(k1, 31)
		k1 *= c2_128
		h1 ^= k1
	}

	// Finalization
	h1 ^= uint64(length)
	h2 ^= uint64(length)

	h1 += h2
	h2 += h1

	h1 = fmix64(h1)
	h2 = fmix64(h2)

	h1 += h2
	h2 += h1

	return h1, h2
}

func rotl64(x uint64, r uint) uint64 {
	return (x << r) | (x >> (64 - r))
}

func fmix64(k uint64) uint64 {
	k ^= k >> 33
	k *= 0xff51afd7ed558ccd
	k ^= k >> 33
	k *= 0xc4ceb9fe1a85ec53
	k ^= k >> 33
	return k
}

func getBlock64(data []byte, idx int) uint64 {
	off := idx * 8
	if off+8 > len(data) {
		return 0
	}
	return uint64(data[off]) |
		uint64(data[off+1])<<8 |
		uint64(data[off+2])<<16 |
		uint64(data[off+3])<<24 |
		uint64(data[off+4])<<32 |
		uint64(data[off+5])<<40 |
		uint64(data[off+6])<<48 |
		uint64(data[off+7])<<56
}
