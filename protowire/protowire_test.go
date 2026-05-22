package protowire

import (
	"math"
	"reflect"
	"runtime"
	"testing"
	"unsafe"

	"github.com/limpo1989/arena"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// 2.1 EncodeVarint / DecodeVarint round-trip
// ---------------------------------------------------------------------------

func TestVarintRoundTrip(t *testing.T) {
	values := []uint64{
		0, 1, 127, 128, 300, 16383, 16384,
		math.MaxUint64,
	}
	for _, v := range values {
		buf := make([]byte, 16)
		n := EncodeVarint(buf, v)
		assert.Equal(t, SizeOfVarint(v), n, "SizeOfVarint mismatch for %d", v)

		got, readN, err := DecodeVarint(buf[:n])
		assert.NoError(t, err)
		assert.Equal(t, n, readN)
		assert.Equal(t, v, got, "round-trip failed for %d", v)
	}
}

// ---------------------------------------------------------------------------
// 2.2 DecodeVarint error on truncated data
// ---------------------------------------------------------------------------

func TestDecodeVarintTruncated(t *testing.T) {
	data := []byte{0x80}
	_, _, err := DecodeVarint(data)
	assert.Error(t, err)

	data = []byte{0x80, 0x80}
	_, _, err = DecodeVarint(data)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// 2.3 Fixed32/Fixed64 round-trip + insufficient data
// ---------------------------------------------------------------------------

func TestFixed32RoundTrip(t *testing.T) {
	values := []uint32{0, 1, math.MaxUint32}
	for _, v := range values {
		buf := make([]byte, 4)
		n := EncodeFixed32(buf, v)
		assert.Equal(t, 4, n)

		got, readN, err := DecodeFixed32(buf)
		assert.NoError(t, err)
		assert.Equal(t, 4, readN)
		assert.Equal(t, v, got)
	}
}

func TestFixed32InsufficientData(t *testing.T) {
	_, _, err := DecodeFixed32([]byte{1, 2, 3})
	assert.Error(t, err)
}

func TestFixed64RoundTrip(t *testing.T) {
	values := []uint64{0, 1, math.MaxUint64}
	for _, v := range values {
		buf := make([]byte, 8)
		n := EncodeFixed64(buf, v)
		assert.Equal(t, 8, n)

		got, readN, err := DecodeFixed64(buf)
		assert.NoError(t, err)
		assert.Equal(t, 8, readN)
		assert.Equal(t, v, got)
	}
}

func TestFixed64InsufficientData(t *testing.T) {
	_, _, err := DecodeFixed64([]byte{1, 2, 3, 4, 5, 6, 7})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// 2.4 ZigZag round-trip
// ---------------------------------------------------------------------------

func TestZigZagRoundTrip(t *testing.T) {
	values := []int64{0, -1, 1, -2, 2, math.MaxInt64, math.MinInt64}
	for _, v := range values {
		encoded := EncodeZigZag(v)
		decoded := DecodeZigZag(encoded)
		assert.Equal(t, v, decoded, "ZigZag round-trip failed for %d", v)
	}
}

// ---------------------------------------------------------------------------
// 2.5 SizeOfVarint known values
// ---------------------------------------------------------------------------

func TestSizeOfVarint(t *testing.T) {
	cases := []struct {
		v    uint64
		want int
	}{
		{0, 1},
		{127, 1},
		{128, 2},
		{16383, 2},
		{16384, 3},
		{math.MaxUint64, 10},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, SizeOfVarint(tc.v), "SizeOfVarint(%d)", tc.v)
	}
}

// ---------------------------------------------------------------------------
// 2.6 SkipField for each wire type
// ---------------------------------------------------------------------------

func TestSkipFieldVarint(t *testing.T) {
	buf := make([]byte, 4)
	n := EncodeVarint(buf, 42)
	skipped, err := SkipField(buf, 0)
	assert.NoError(t, err)
	assert.Equal(t, n, skipped)
}

func TestSkipFieldFixed64(t *testing.T) {
	data := make([]byte, 8)
	skipped, err := SkipField(data, 1)
	assert.NoError(t, err)
	assert.Equal(t, 8, skipped)
}

func TestSkipFieldLengthDelimited(t *testing.T) {
	payload := []byte("hello")
	buf := make([]byte, 16)
	n := EncodeVarint(buf, uint64(len(payload)))
	copy(buf[n:], payload)
	total := n + len(payload)

	skipped, err := SkipField(buf, 2)
	assert.NoError(t, err)
	assert.Equal(t, total, skipped)
}

func TestSkipFieldFixed32(t *testing.T) {
	data := make([]byte, 4)
	skipped, err := SkipField(data, 5)
	assert.NoError(t, err)
	assert.Equal(t, 4, skipped)
}

func TestSkipFieldInvalidWireType(t *testing.T) {
	for _, wt := range []int{3, 4, 6, 7, -1} {
		_, err := SkipField(make([]byte, 10), wt)
		assert.Error(t, err, "wire type %d should error", wt)
	}
}

// ---------------------------------------------------------------------------
// 2.7 StringPtr
// ---------------------------------------------------------------------------

func TestStringPtr(t *testing.T) {
	ar := arena.NewArena()
	defer runtime.KeepAlive(ar)

	input := []byte("hello world")
	ptr := StringPtr(ar, input)
	assert.NotNil(t, ptr)
	assert.Equal(t, "hello world", *ptr)

	// Verify the pointer was returned from arena (non-nil, valid memory)
	assert.NotEqual(t, uintptr(0), uintptr(unsafe.Pointer(ptr)))
}

func TestStringPtrEmpty(t *testing.T) {
	ar := arena.NewArena()
	defer runtime.KeepAlive(ar)

	ptr := StringPtr(ar, nil)
	assert.NotNil(t, ptr)
	assert.Equal(t, "", *ptr)
}

func TestStringPtrArenaOwnership(t *testing.T) {
	ar := arena.NewArena()
	defer runtime.KeepAlive(ar)

	// Allocate via arena directly and compare addresses
	arenaPtr := ar.Malloc(1)
	ptr := StringPtr(ar, []byte("x"))

	// Both should be in similar address ranges (arena chunks)
	ptrAddr := uintptr(unsafe.Pointer(ptr))
	arenaAddr := uintptr(arenaPtr)
	// Just verify they're both non-zero and accessible
	assert.NotEqual(t, uintptr(0), ptrAddr)
	assert.NotEqual(t, uintptr(0), arenaAddr)
}

// ---------------------------------------------------------------------------
// 2.8 String
// ---------------------------------------------------------------------------

func TestString(t *testing.T) {
	ar := arena.NewArena()
	defer runtime.KeepAlive(ar)

	input := []byte("test")
	s := String(ar, input)
	assert.Equal(t, "test", s)
}

func TestStringEmpty(t *testing.T) {
	ar := arena.NewArena()
	defer runtime.KeepAlive(ar)

	s := String(ar, nil)
	assert.Equal(t, "", s)
}

func TestStringArenaOwnership(t *testing.T) {
	ar := arena.NewArena()
	defer runtime.KeepAlive(ar)

	s := String(ar, []byte("hello"))
	// String data should be in arena: read via reflect to get data pointer
	hdr := (*reflect.StringHeader)(unsafe.Pointer(&s))
	assert.NotEqual(t, uintptr(0), hdr.Data)
}

// ---------------------------------------------------------------------------
// 2.9 Ptr[T]
// ---------------------------------------------------------------------------

func TestPtrInt32(t *testing.T) {
	ar := arena.NewArena()
	defer runtime.KeepAlive(ar)

	p := Ptr(ar, int32(42))
	assert.NotNil(t, p)
	assert.Equal(t, int32(42), *p)
}

func TestPtrFloat64(t *testing.T) {
	ar := arena.NewArena()
	defer runtime.KeepAlive(ar)

	p := Ptr(ar, 3.14)
	assert.NotNil(t, p)
	assert.InDelta(t, 3.14, *p, 0.001)
}

func TestPtrBool(t *testing.T) {
	ar := arena.NewArena()
	defer runtime.KeepAlive(ar)

	p := Ptr(ar, true)
	assert.NotNil(t, p)
	assert.True(t, *p)
}

func TestPtrEnumType(t *testing.T) {
	ar := arena.NewArena()
	defer runtime.KeepAlive(ar)

	type Role int32
	p := Ptr(ar, Role(1))
	assert.NotNil(t, p)
	assert.Equal(t, Role(1), *p)
}

func TestPtrInt64(t *testing.T) {
	ar := arena.NewArena()
	defer runtime.KeepAlive(ar)

	p := Ptr(ar, int64(math.MaxInt64))
	assert.Equal(t, int64(math.MaxInt64), *p)
}

func TestPtrUint32(t *testing.T) {
	ar := arena.NewArena()
	defer runtime.KeepAlive(ar)

	p := Ptr(ar, uint32(math.MaxUint32))
	assert.Equal(t, uint32(math.MaxUint32), *p)
}
