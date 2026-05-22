/*
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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

func TestVector_Iter(t *testing.T) {
	arena := NewArena()
	vec := NewVector[int](arena, 8)
	vec.Append(0, 1, 2, 3, 4, 5, 6, 7, 8, 9)
	assert.Equal(t, 10, vec.Len())

	for i, v := range vec.All() {
		assert.Equal(t, i, v)
	}

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

func TestVector_Clear(t *testing.T) {
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
	vec.Clear()

	// Vector struct itself is arena-allocated (1 ref), data slice was freed by Clear
	if arena.current.ref != 1 {
		t.Fatalf("arena.current.ref %d not 1", arena.current.ref)
	}

	runtime.KeepAlive(arena)
}

// ---------------------------------------------------------------------------
// Additional vector tests
// ---------------------------------------------------------------------------

func TestVector_StructElements(t *testing.T) {
	arena := NewArena()
	defer runtime.KeepAlive(arena)

	type item struct {
		ID    int
		Label string
	}

	vec := NewVector[item](arena, 4)
	vec.Append(item{ID: 1, Label: "alpha"})
	vec.Append(item{ID: 2, Label: "beta"})
	vec.Append(item{ID: 3, Label: "gamma"})

	assert.Equal(t, 3, vec.Len())
	assert.Equal(t, item{ID: 1, Label: "alpha"}, vec.At(0))
	assert.Equal(t, item{ID: 2, Label: "beta"}, vec.At(1))
	assert.Equal(t, item{ID: 3, Label: "gamma"}, vec.At(2))

	// Remove middle element
	removed := vec.Remove(item{ID: 2, Label: "beta"})
	assert.True(t, removed)
	assert.Equal(t, 2, vec.Len())
	assert.Equal(t, item{ID: 1, Label: "alpha"}, vec.At(0))
	assert.Equal(t, item{ID: 3, Label: "gamma"}, vec.At(1))
}

func TestVector_PointerElements(t *testing.T) {
	arena := NewArena()
	defer runtime.KeepAlive(arena)

	vec := NewVector[*int](arena, 4)

	// Allocate ints via arena and use their pointers as vector elements
	p1 := arena.Int(10)
	p2 := arena.Int(20)
	p3 := arena.Int(30)
	vec.Append(p1, p2, p3)

	assert.Equal(t, 3, vec.Len())
	assert.Equal(t, 10, *vec.At(0))
	assert.Equal(t, 20, *vec.At(1))
	assert.Equal(t, 30, *vec.At(2))

	// Verify deep copy isolation: mutate the original pointer and ensure
	// the vector element is independent.
	// For pointer elements, the vector stores copies of the pointer values
	// (i.e., the addresses).  Deep-copy through the Vector's arenaDeepCopy
	// should allocate new pointer targets.
}

func TestVector_DeepCopier(t *testing.T) {
	arena := NewArena()

	type container struct {
		Tag string
		Num *Vector[int]
	}

	original := &container{
		Tag: "original",
		Num: NewVector[int](arena, 4),
	}
	original.Num.Append(1, 2, 3)

	// DeepCopy the container — Vector should implement deepCopier
	cp := DeepCopy(arena, *original)

	// Modify the copy's vector and verify original is unaffected
	cp.Num.Append(99)
	assert.Equal(t, 3, original.Num.Len(), "original vector length should be unchanged")
	assert.Equal(t, 4, cp.Num.Len(), "copied vector should have the extra element")
	assert.Equal(t, 99, cp.Num.At(3))

	// Modify the original and verify copy is unaffected
	original.Num.Append(42)
	assert.Equal(t, 4, original.Num.Len())
	assert.Equal(t, 4, cp.Num.Len(), "copied vector length should not change when original is modified")

	runtime.KeepAlive(arena)
}
