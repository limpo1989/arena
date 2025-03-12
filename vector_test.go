package arena

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

type player struct {
	id   int
	name string
}

func (p player) equals(dst player) bool {
	return p.id == dst.id
}

func TestNewVector(t *testing.T) {
	arena := NewArena()
	vec := NewVector[int](arena, 8)
	assert.Equal(t, 0, vec.Len())

	runtime.KeepAlive(arena)
}

func TestVector_AddIfAbsent(t *testing.T) {
	arena := NewArena()
	vec := NewVector[int](arena, 8)
	assert.Equal(t, true, vec.AddIfAbsent(1))
	assert.Equal(t, true, vec.AddIfAbsent(2))
	assert.Equal(t, true, vec.AddIfAbsent(3))
	assert.Equal(t, false, vec.AddIfAbsent(3))
	assert.Equal(t, 3, vec.Len())

	runtime.KeepAlive(arena)
}

func TestVector_Append(t *testing.T) {
	arena := NewArena()
	vec := NewVector[int](arena, 8)
	vec.Append(1, 2, 3)
	assert.Equal(t, 3, vec.Len())
	vec.Append(4)
	vec.Append(5)
	vec.Append(6)
	vec.Append(7)
	vec.Append(8)
	vec.Append(9)
	assert.Equal(t, 9, vec.Len())
	for i := 0; i < vec.Len(); i++ {
		assert.Equal(t, i+1, vec.At(i))
	}

	runtime.KeepAlive(arena)
}

func TestVector_At(t *testing.T) {
	arena := NewArena()
	vec := NewVector[int](arena, 8)
	vec.Append(0, 1, 2, 3)
	assert.Equal(t, 4, vec.Len())
	for i := 0; i < vec.Len(); i++ {
		assert.Equal(t, i, vec.At(i))
	}

	runtime.KeepAlive(arena)
}

func TestVector_Cap(t *testing.T) {
	arena := NewArena()
	vec := NewVector[int](arena, 8)
	assert.Equal(t, 8, vec.Cap())
	vec.Append(0, 1, 2, 3, 4, 5, 6, 7)
	assert.Equal(t, 8, vec.Len())
	assert.Equal(t, 8, vec.Cap())
	wantCap := calculateNewCap(vec.Len(), 3)
	vec.Append(8, 9, 10)
	assert.Equal(t, 11, vec.Len())
	assert.Equal(t, wantCap, vec.Cap())

	runtime.KeepAlive(arena)
}

func TestVector_Equatable(t *testing.T) {

	arena := NewArena()
	vec := NewVector[player](arena, 8)
	vec.Equatable(func(a, b player) bool {
		return a.equals(b)
	})

	vec.AddIfAbsent(player{id: 1, name: "111"})
	assert.Equal(t, 1, vec.Len())
	vec.AddIfAbsent(player{id: 1, name: "dup111"})
	assert.Equal(t, 1, vec.Len())
	vec.AddIfAbsent(player{id: 2, name: "2222"})
	assert.Equal(t, 2, vec.Len())

	runtime.KeepAlive(arena)
}

func TestVector_Index(t *testing.T) {
	arena := NewArena()
	vec := NewVector[int](arena, 8)
	vec.Append(0, 1, 2, 3, 4, 5, 6, 7, 8, 9)
	assert.Equal(t, 10, vec.Len())
	for i := 0; i < vec.Len(); i++ {
		assert.Equal(t, i, vec.At(i))
	}
	runtime.KeepAlive(arena)
}

func TestVector_Iter(t *testing.T) {
	arena := NewArena()
	vec := NewVector[int](arena, 8)
	vec.Append(0, 1, 2, 3, 4, 5, 6, 7, 8, 9)
	assert.Equal(t, 10, vec.Len())

	vec.Iter()(func(index int, v int) bool {
		assert.Equal(t, index, v)
		return true
	})

	runtime.KeepAlive(arena)
}

func TestVector_LastIndex(t *testing.T) {
	arena := NewArena()
	vec := NewVector[int](arena, 8)
	vec.Append(0, 1, 2, 3, 4, 5, 6, 7, 8, 9)
	assert.Equal(t, 10, vec.Len())

	for i := 0; i < vec.Len(); i++ {
		assert.Equal(t, i, vec.LastIndex(i))
	}

	runtime.KeepAlive(arena)
}

func TestVector_Len(t *testing.T) {
	arena := NewArena()
	vec := NewVector[int](arena, 8)
	vec.Append(0, 1, 2, 3, 4, 5, 6, 7, 8, 9)
	assert.Equal(t, 10, vec.Len())

	runtime.KeepAlive(arena)
}

func TestVector_Range(t *testing.T) {
	arena := NewArena()
	vec := NewVector[int](arena, 8)
	vec.Append(0, 1, 2, 3, 4, 5, 6, 7, 8, 9)
	assert.Equal(t, 10, vec.Len())

	vec.Range(func(index int, v int) bool {
		assert.Equal(t, index, v)
		return true
	})

	runtime.KeepAlive(arena)
}

func TestVector_RemoveBy(t *testing.T) {
	arena := NewArena()
	vec := NewVector[int](arena, 8)
	vec.Append(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 9, 9, 9)
	assert.Equal(t, 13, vec.Len())
	removed := vec.RemoveBy(2, func(index int, v int) bool {
		return 9 == v
	})
	assert.Equal(t, 2, removed)
	assert.Equal(t, 11, vec.Len())

	removed = vec.RemoveBy(1, func(index int, v int) bool {
		return 9 == v
	})
	assert.Equal(t, 1, removed)
	assert.Equal(t, 10, vec.Len())

	runtime.KeepAlive(arena)
}

func TestVector_RemoveIdx(t *testing.T) {
	arena := NewArena()
	vec := NewVector[int](arena, 8)
	vec.Append(0, 1, 2, 3, 4, 5, 6, 7, 8, 9)
	assert.Equal(t, 10, vec.Len())

	for vec.Len() > 0 {
		vec.RemoveIdx(0)
	}
	assert.Equal(t, 0, vec.Len())

	runtime.KeepAlive(arena)
}

func TestVector_RemoveOne(t *testing.T) {
	arena := NewArena()
	vec := NewVector[int](arena, 8)
	vec.Append(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 9)
	assert.Equal(t, 11, vec.Len())
	assert.Equal(t, false, vec.Remove(-1))
	assert.Equal(t, 11, vec.Len())
	assert.Equal(t, true, vec.Remove(9))
	assert.Equal(t, 10, vec.Len())

	runtime.KeepAlive(arena)
}
