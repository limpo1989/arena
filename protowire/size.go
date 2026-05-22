package protowire

// SizeOfVarint returns the number of bytes needed to encode v as a varint.
func SizeOfVarint(v uint64) int {
	n := 1
	for v >= 0x80 {
		v >>= 7
		n++
	}
	return n
}

// SizeOfBytes returns the total wire format size for a bytes/string field:
// tag (1 byte for fieldNum 1-15) + length varint + len(data).
func SizeOfBytes(fieldNum int, data []byte) int {
	return SizeOfTag(fieldNum) + SizeOfVarint(uint64(len(data))) + len(data)
}

// SizeOfTag returns the encoded size of a field tag.
func SizeOfTag(fieldNum int) int {
	return SizeOfVarint(uint64(fieldNum<<3 | 2))
}

// SizeOfVarintField returns the total wire format size for a varint field.
func SizeOfVarintField(fieldNum int, v uint64) int {
	return SizeOfVarint(uint64(fieldNum<<3)) + SizeOfVarint(v)
}

// SizeOfFixed32Field returns the total wire format size for a fixed32 field.
func SizeOfFixed32Field(fieldNum int) int {
	return SizeOfVarint(uint64(fieldNum<<3|5)) + 4
}

// SizeOfFixed64Field returns the total wire format size for a fixed64 field.
func SizeOfFixed64Field(fieldNum int) int {
	return SizeOfVarint(uint64(fieldNum<<3|1)) + 8
}
