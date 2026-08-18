package counting

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
)

// Snapshot errors.
var (
	ErrSnapshotTooShort   = errors.New("counting: snapshot too short")
	ErrSnapshotBadMagic   = errors.New("counting: snapshot bad magic")
	ErrSnapshotCRC        = errors.New("counting: snapshot CRC mismatch")
	ErrSnapshotDataLen    = errors.New("counting: snapshot data length mismatch")
)

const (
	snapshotMagic  uint32 = 0x43424631 // "CBF1"
	snapshotHeader        = 20         // magic(4) + m(4) + k(4) + counterBits(4) + count(4)
	snapshotCRC           = 4
)

// MarshalSnapshot serializes a CountingFilter to a binary snapshot with CRC.
//
// Format:
//
//	[4] magic (0x43424631)
//	[4] m (uint32 big-endian)
//	[4] k (uint32 big-endian)
//	[4] counterBits (uint32 big-endian)
//	[4] count (int32 big-endian)
//	[N] data (raw counter bytes)
//	[4] CRC32 (IEEE, over everything before CRC)
func MarshalSnapshot(cf *CountingFilter) ([]byte, error) {
	if cf == nil {
		return nil, ErrNilFilter
	}
	cf.mu.RLock()
	defer cf.mu.RUnlock()

	dataLen := len(cf.data)
	total := snapshotHeader + dataLen + snapshotCRC
	buf := make([]byte, total)

	binary.BigEndian.PutUint32(buf[0:4], snapshotMagic)
	binary.BigEndian.PutUint32(buf[4:8], uint32(cf.m))
	binary.BigEndian.PutUint32(buf[8:12], uint32(cf.k))
	binary.BigEndian.PutUint32(buf[12:16], uint32(cf.counterBits))
	binary.BigEndian.PutUint32(buf[16:20], uint32(cf.count))
	copy(buf[snapshotHeader:], cf.data)

	checksum := crc32.ChecksumIEEE(buf[:total-snapshotCRC])
	binary.BigEndian.PutUint32(buf[total-snapshotCRC:], checksum)
	return buf, nil
}

// UnmarshalSnapshot reconstructs a CountingFilter from a snapshot produced
// by MarshalSnapshot. Validates CRC integrity.
func UnmarshalSnapshot(b []byte) (*CountingFilter, error) {
	if len(b) < snapshotHeader+snapshotCRC {
		return nil, ErrSnapshotTooShort
	}
	magic := binary.BigEndian.Uint32(b[0:4])
	if magic != snapshotMagic {
		return nil, ErrSnapshotBadMagic
	}

	m := uint(binary.BigEndian.Uint32(b[4:8]))
	k := uint(binary.BigEndian.Uint32(b[8:12]))
	counterBits := uint(binary.BigEndian.Uint32(b[12:16]))
	count := int(int32(binary.BigEndian.Uint32(b[16:20])))

	expectedDataLen := (m*counterBits + 7) / 8
	if uint(len(b)) < uint(snapshotHeader)+expectedDataLen+snapshotCRC {
		return nil, ErrSnapshotDataLen
	}

	// Validate CRC
	payload := b[:snapshotHeader+int(expectedDataLen)]
	storedCRC := binary.BigEndian.Uint32(b[snapshotHeader+int(expectedDataLen):])
	computed := crc32.ChecksumIEEE(payload)
	if storedCRC != computed {
		return nil, ErrSnapshotCRC
	}

	data := make([]byte, expectedDataLen)
	copy(data, b[snapshotHeader:snapshotHeader+int(expectedDataLen)])

	return &CountingFilter{
		m:           m,
		k:           k,
		counterBits: counterBits,
		maxCount:    (1 << counterBits) - 1,
		data:        data,
		count:       count,
	}, nil
}

// Clone creates a deep copy of the CountingFilter.
func (cf *CountingFilter) Clone() *CountingFilter {
	cf.mu.RLock()
	defer cf.mu.RUnlock()
	data := make([]byte, len(cf.data))
	copy(data, cf.data)
	return &CountingFilter{
		m:           cf.m,
		k:           cf.k,
		counterBits: cf.counterBits,
		maxCount:    cf.maxCount,
		data:        data,
		count:       cf.count,
	}
}

// Merge combines another CountingFilter into this one by summing counters.
// Both filters must have identical parameters (m, k, counterBits).
// If a sum would exceed maxCount, it is clamped to maxCount.
func (cf *CountingFilter) Merge(other *CountingFilter) error {
	if other == nil {
		return ErrNilFilter
	}
	cf.mu.Lock()
	defer cf.mu.Unlock()
	other.mu.RLock()
	defer other.mu.RUnlock()

	if cf.m != other.m || cf.k != other.k || cf.counterBits != other.counterBits {
		return ErrParamMismatch
	}

	for i := uint(0); i < cf.m; i++ {
		a := cf.getCounter(i)
		b := other.getCounter(i)
		sum := uint16(a) + uint16(b)
		if sum > uint16(cf.maxCount) {
			sum = uint16(cf.maxCount)
		}
		cf.setCounter(i, uint8(sum))
	}
	cf.count += other.count
	return nil
}

// NonZeroCount returns the number of counter positions that are > 0.
func (cf *CountingFilter) NonZeroCount() int {
	cf.mu.RLock()
	defer cf.mu.RUnlock()
	n := 0
	for i := uint(0); i < cf.m; i++ {
		if cf.getCounter(i) > 0 {
			n++
		}
	}
	return n
}

// MaxObservedCount returns the highest counter value currently in the filter.
func (cf *CountingFilter) MaxObservedCount() uint8 {
	cf.mu.RLock()
	defer cf.mu.RUnlock()
	var max uint8
	for i := uint(0); i < cf.m; i++ {
		v := cf.getCounter(i)
		if v > max {
			max = v
		}
	}
	return max
}
