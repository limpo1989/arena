package protowire

import "errors"

var (
	errBufferTooSmall  = errors.New("protowire: buffer too small")
	errTruncatedVarint = errors.New("protowire: varint truncated")
)

// DecodeVarint decodes a protobuf varint from data.
// Returns the value, number of bytes consumed, and any error.
func DecodeVarint(data []byte) (uint64, int, error) {
	var v uint64
	var n int
	for shift := uint(0); shift < 64; shift += 7 {
		if n >= len(data) {
			return 0, 0, errTruncatedVarint
		}
		b := data[n]
		v |= uint64(b&0x7f) << shift
		n++
		if b < 0x80 {
			return v, n, nil
		}
	}
	return 0, 0, errTruncatedVarint
}

// DecodeFixed32 reads a 4-byte little-endian uint32 from data.
// Returns the value, number of bytes consumed (always 4), and any error.
func DecodeFixed32(data []byte) (uint32, int, error) {
	if len(data) < 4 {
		return 0, 0, errBufferTooSmall
	}
	return uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24, 4, nil
}

// DecodeFixed64 reads an 8-byte little-endian uint64 from data.
// Returns the value, number of bytes consumed (always 8), and any error.
func DecodeFixed64(data []byte) (uint64, int, error) {
	if len(data) < 8 {
		return 0, 0, errBufferTooSmall
	}
	return uint64(data[0]) | uint64(data[1])<<8 | uint64(data[2])<<16 | uint64(data[3])<<24 |
		uint64(data[4])<<32 | uint64(data[5])<<40 | uint64(data[6])<<48 | uint64(data[7])<<56, 8, nil
}
