package protowire

// EncodeVarint encodes a uint64 as a protobuf varint into dAtA.
// Returns the number of bytes written.
func EncodeVarint(dAtA []byte, v uint64) int {
	n := 0
	for v >= 0x80 {
		dAtA[n] = byte(v) | 0x80
		v >>= 7
		n++
	}
	dAtA[n] = byte(v)
	return n + 1
}

// EncodeFixed32 writes a uint32 as 4-byte little-endian into dAtA.
// Returns the number of bytes written (always 4).
func EncodeFixed32(dAtA []byte, v uint32) int {
	dAtA[0] = byte(v)
	dAtA[1] = byte(v >> 8)
	dAtA[2] = byte(v >> 16)
	dAtA[3] = byte(v >> 24)
	return 4
}

// EncodeFixed64 writes a uint64 as 8-byte little-endian into dAtA.
// Returns the number of bytes written (always 8).
func EncodeFixed64(dAtA []byte, v uint64) int {
	dAtA[0] = byte(v)
	dAtA[1] = byte(v >> 8)
	dAtA[2] = byte(v >> 16)
	dAtA[3] = byte(v >> 24)
	dAtA[4] = byte(v >> 32)
	dAtA[5] = byte(v >> 40)
	dAtA[6] = byte(v >> 48)
	dAtA[7] = byte(v >> 56)
	return 8
}
