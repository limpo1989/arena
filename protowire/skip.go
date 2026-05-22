package protowire

import "fmt"

// SkipField skips past one unknown field in the wire format data.
// wireType must be 0 (varint), 1 (fixed64), 2 (length-delimited), or 5 (fixed32).
// Returns the number of bytes consumed.
func SkipField(data []byte, wireType int) (int, error) {
	switch wireType {
	case 0: // varint
		_, n, err := DecodeVarint(data)
		return n, err
	case 1: // fixed64
		if len(data) < 8 {
			return 0, errBufferTooSmall
		}
		return 8, nil
	case 2: // length-delimited
		length, n, err := DecodeVarint(data)
		if err != nil {
			return 0, err
		}
		total := n + int(length)
		if total > len(data) {
			return 0, errBufferTooSmall
		}
		return total, nil
	case 5: // fixed32
		if len(data) < 4 {
			return 0, errBufferTooSmall
		}
		return 4, nil
	default:
		return 0, fmt.Errorf("protowire: unknown wire type %d", wireType)
	}
}
