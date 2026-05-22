package arena

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
)

func TestNewSliceNormal(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	s := NewSlice[int](ar, 5, 10)
	assert.NotNil(t, s)
	assert.Equal(t, 5, len(s))
	assert.Equal(t, 10, cap(s))

	// Elements should be zero-valued
	for i := 0; i < len(s); i++ {
		assert.Equal(t, 0, s[i])
	}

	// Writing should work
	for i := 0; i < len(s); i++ {
		s[i] = i * 10
	}
	for i := 0; i < len(s); i++ {
		assert.Equal(t, i*10, s[i])
	}
}

func TestNewSliceZeroLenCap(t *testing.T) {
	ar := NewArena()

	s := NewSlice[int](ar, 0, 0)
	assert.Nil(t, s)
	assert.Equal(t, 0, len(s))
	assert.Equal(t, 0, cap(s))
}

func TestNewSliceLenGreaterThanCap(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	// len > cap should auto-correct: cap becomes len
	s := NewSlice[int](ar, 10, 5)
	assert.NotNil(t, s)
	assert.Equal(t, 10, len(s))
	assert.Equal(t, 10, cap(s))
}

func TestNewSliceNegativeCapPanic(t *testing.T) {
	ar := NewArena()

	// When both length and capacity are negative and equal, length > capacity is false,
	// so capacity stays negative and triggers the "invalid capacity" panic.
	assert.Panics(t, func() {
		NewSlice[int](ar, -1, -1)
	})
}

func TestAppendNilSlice(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	var s []int
	s = Append(ar, s, 1, 2, 3)
	assert.Equal(t, []int{1, 2, 3}, s)

	s = Append(ar, s, 4, 5)
	assert.Equal(t, []int{1, 2, 3, 4, 5}, s)
}

func TestAppendWithinCapacity(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	s := NewSlice[int](ar, 3, 10)
	s[0] = 10
	s[1] = 20
	s[2] = 30

	// Append within existing capacity -- no reallocation needed
	s2 := Append(ar, s, 40, 50)
	assert.Equal(t, []int{10, 20, 30, 40, 50}, s2)
	// The new slice should use the same underlying capacity
	assert.Equal(t, 10, cap(s2))
}

func TestAppendTriggersExpansion(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	s := NewSlice[int](ar, 2, 2)
	s[0] = 100
	s[1] = 200

	// Appending beyond capacity triggers expansion
	s2 := Append(ar, s, 300, 400, 500)
	assert.Equal(t, []int{100, 200, 300, 400, 500}, s2)
	assert.GreaterOrEqual(t, cap(s2), 5)
}

func TestAppendNonArenaSlicePanic(t *testing.T) {
	ar := NewArena()

	// A heap-allocated slice is not managed by the arena
	s := make([]int, 2, 10)
	s[0] = 1
	s[1] = 2

	assert.Panics(t, func() {
		Append(ar, s, 3)
	})
}

func TestAppendEmptyValues(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	s := NewSlice[int](ar, 2, 5)
	s[0] = 42
	s[1] = 99

	// Append with no new values should return an equivalent slice
	s2 := Append(ar, s)
	assert.Equal(t, []int{42, 99}, s2)
}

func TestNewSliceDifferentTypes(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	// string slice
	ss := NewSlice[string](ar, 2, 4)
	assert.Equal(t, 2, len(ss))
	assert.Equal(t, 4, cap(ss))

	// byte slice
	bs := NewSlice[byte](ar, 0, 32)
	assert.Equal(t, 0, len(bs))
	assert.Equal(t, 32, cap(bs))
}

func TestNewSliceMemoryIsArenaManaged(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	s := NewSlice[int64](ar, 4, 4)
	dataPtr := uintptr(unsafe.Pointer(unsafe.SliceData(s)))
	assert.True(t, ar.isManaged(dataPtr))
}
