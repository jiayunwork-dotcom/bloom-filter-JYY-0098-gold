package codec

import (
	"encoding/binary"
	"fmt"
)

// FormatInfo describes the wire format of an encoded Bloom filter blob.
type FormatInfo struct {
	Version    int
	HeaderSize int
	HasCRC     bool
	M          uint
	K          uint
	BitsLen    int
	TotalLen   int
}

// Inspect examines raw bytes and returns metadata about the encoded filter
// without fully deserializing it. Useful for tooling and debugging.
func Inspect(b []byte) (FormatInfo, error) {
	if len(b) < 4 {
		return FormatInfo{}, ErrTooShort
	}
	magic := binary.BigEndian.Uint32(b[0:4])
	switch magic {
	case magicV1:
		return inspectV1(b)
	case magicV2:
		return inspectV2(b)
	default:
		return FormatInfo{}, ErrBadMagic
	}
}

func inspectV1(b []byte) (FormatInfo, error) {
	if len(b) < headerSizeV1 {
		return FormatInfo{}, ErrTooShort
	}
	m := uint(binary.BigEndian.Uint32(b[4:8]))
	k := uint(binary.BigEndian.Uint32(b[8:12]))
	bitsLen := len(b) - headerSizeV1
	return FormatInfo{
		Version:    1,
		HeaderSize: headerSizeV1,
		HasCRC:     false,
		M:          m,
		K:          k,
		BitsLen:    bitsLen,
		TotalLen:   len(b),
	}, nil
}

func inspectV2(b []byte) (FormatInfo, error) {
	if len(b) < headerSizeV2+trailerV2 {
		return FormatInfo{}, ErrTooShort
	}
	m := uint(binary.BigEndian.Uint32(b[5:9]))
	k := uint(binary.BigEndian.Uint32(b[9:13]))
	bitsLen := int((m + 7) / 8)
	return FormatInfo{
		Version:    2,
		HeaderSize: headerSizeV2,
		HasCRC:     true,
		M:          m,
		K:          k,
		BitsLen:    bitsLen,
		TotalLen:   len(b),
	}, nil
}

// FormatString returns a human-readable summary of the encoded filter.
func FormatString(b []byte) string {
	info, err := Inspect(b)
	if err != nil {
		return fmt.Sprintf("invalid: %v", err)
	}
	return fmt.Sprintf("v%d m=%d k=%d bits=%d bytes=%d crc=%v",
		info.Version, info.M, info.K, info.BitsLen, info.TotalLen, info.HasCRC)
}

// Validate checks whether the raw bytes represent a valid encoded filter.
// Returns nil if valid, or an error describing the problem.
func Validate(b []byte) error {
	_, err := Unmarshal(b)
	return err
}

// ConvertV1ToV2 upgrades a v1 encoded filter to v2 format (adding CRC).
// Returns the original bytes unchanged if already v2.
func ConvertV1ToV2(b []byte) ([]byte, error) {
	if len(b) < 4 {
		return nil, ErrTooShort
	}
	magic := binary.BigEndian.Uint32(b[0:4])
	if magic == magicV2 {
		return b, nil // already v2
	}
	if magic != magicV1 {
		return nil, ErrBadMagic
	}
	f, err := unmarshalV1(b)
	if err != nil {
		return nil, err
	}
	return Marshal(f), nil
}
