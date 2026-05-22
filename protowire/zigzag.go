package protowire

// EncodeZigZag encodes a signed integer as a ZigZag-encoded uint64.
func EncodeZigZag(v int64) uint64 {
	return uint64(v<<1) ^ uint64(v>>63)
}

// DecodeZigZag decodes a ZigZag-encoded uint64 back to a signed integer.
func DecodeZigZag(v uint64) int64 {
	return int64((v >> 1) ^ -(v & 1))
}
