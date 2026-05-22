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
	"unsafe"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// 4.1 Clean arena object — no violations
// ---------------------------------------------------------------------------

func TestAudit_CleanObject(t *testing.T) {
	ar := NewArena()
	defer runtime.KeepAlive(ar)

	type Inner struct {
		Name string
	}

	type Outer struct {
		Inner *Inner
		Tags  []string
	}

	inner := DeepCopy(ar, Inner{Name: "hello"})
	outer := New[Outer](ar)
	outer.Inner = inner
	outer.Tags = NewSlice[string](ar, 0, 4)
	outer.Tags = Append(ar, outer.Tags, *DeepCopy(ar, "world"))

	violations := ar.AuditPointers(outer)
	assert.Empty(t, violations)
}

// ---------------------------------------------------------------------------
// 4.2 Heap pointer field
// ---------------------------------------------------------------------------

func TestAudit_HeapPointer(t *testing.T) {
	ar := NewArena()
	defer runtime.KeepAlive(ar)

	type Holder struct {
		Ptr *int
	}

	heapVal := new(int)
	*heapVal = 42

	holder := New[Holder](ar)
	holder.Ptr = heapVal

	violations := ar.AuditPointers(holder)
	assert.Len(t, violations, 1)
	assert.Equal(t, ViolationPointer, violations[0].Kind)
	assert.Equal(t, "Ptr", violations[0].Path)
	assert.NotEqual(t, 0, violations[0].Address)
}

// ---------------------------------------------------------------------------
// 4.3 Heap slice backing array
// ---------------------------------------------------------------------------

func TestAudit_HeapSlice(t *testing.T) {
	ar := NewArena()
	defer runtime.KeepAlive(ar)

	type Holder struct {
		Items []int
	}

	holder := New[Holder](ar)
	holder.Items = []int{1, 2, 3}

	violations := ar.AuditPointers(holder)
	assert.Len(t, violations, 1)
	assert.Equal(t, ViolationSlice, violations[0].Kind)
	assert.Equal(t, "Items", violations[0].Path)
}

// ---------------------------------------------------------------------------
// 4.4 Non-arena string data
// ---------------------------------------------------------------------------

func TestAudit_NonArenaString(t *testing.T) {
	ar := NewArena()
	defer runtime.KeepAlive(ar)

	type Holder struct {
		Label string
	}

	holder := New[Holder](ar)
	holder.Label = "hello world"

	violations := ar.AuditPointers(holder)
	assert.Len(t, violations, 1)
	assert.Equal(t, ViolationString, violations[0].Kind)
	assert.Equal(t, "Label", violations[0].Path)
	assert.Contains(t, violations[0].Hint, "string literal")
}

// ---------------------------------------------------------------------------
// 4.5 Func field violation
// ---------------------------------------------------------------------------

func TestAudit_FuncField(t *testing.T) {
	ar := NewArena()
	defer runtime.KeepAlive(ar)

	type FuncHolder struct {
		Handler func() int
	}

	rawPtr := ar.Malloc(uintptr(unsafe.Sizeof(FuncHolder{})))
	holder := (*FuncHolder)(rawPtr)
	holder.Handler = func() int { return 42 }

	violations := ar.AuditPointers(holder)
	assert.Len(t, violations, 1)
	assert.Equal(t, ViolationFunc, violations[0].Kind)
	assert.Equal(t, "Handler", violations[0].Path)
	assert.Contains(t, violations[0].Hint, "pure function")
}

// ---------------------------------------------------------------------------
// 4.6 Nil and zero values — no violations
// ---------------------------------------------------------------------------

func TestAudit_NilAndZero(t *testing.T) {
	ar := NewArena()
	defer runtime.KeepAlive(ar)

	type Holder struct {
		Ptr   *int
		Items []string
		Label string
		Num   int
	}

	holder := New[Holder](ar)

	violations := ar.AuditPointers(holder)
	assert.Empty(t, violations)
}

// ---------------------------------------------------------------------------
// 4.7 *Arena field skipped
// ---------------------------------------------------------------------------

func TestAudit_ArenaFieldSkipped(t *testing.T) {
	ar := NewArena()
	defer runtime.KeepAlive(ar)

	m := NewMap[string, int](ar, 8)
	m.Put("key", 42)

	violations := ar.AuditPointers(m)
	for _, v := range violations {
		assert.NotContains(t, v.Path, "allocator",
			"*Arena field should not produce violations")
	}
}

// ---------------------------------------------------------------------------
// 4.8 arena:"safe" tagged field skipped
// ---------------------------------------------------------------------------

func TestAudit_SafeTagSkipped(t *testing.T) {
	ar := NewArena()
	defer runtime.KeepAlive(ar)

	vec := NewVector[int](ar, 4)
	vec.Equatable(func(a, b int) bool { return a == b })

	violations := ar.AuditPointers(vec)
	for _, v := range violations {
		assert.NotEqual(t, "equatable", v.Path,
			"arena:\"safe\" tagged field should not produce violations")
	}
}

// ---------------------------------------------------------------------------
// 4.9 Nested struct path building
// ---------------------------------------------------------------------------

func TestAudit_NestedPath(t *testing.T) {
	ar := NewArena()
	defer runtime.KeepAlive(ar)

	type Inner struct {
		Value *string
	}

	type Outer struct {
		Inner *Inner
	}

	heapStr := "heap-value"
	inner := New[Inner](ar)
	inner.Value = &heapStr

	outer := New[Outer](ar)
	outer.Inner = inner

	violations := ar.AuditPointers(outer)
	assert.Len(t, violations, 1)
	assert.Equal(t, "Inner.Value", violations[0].Path)
	assert.Equal(t, ViolationPointer, violations[0].Kind)
}

func TestAudit_SliceElementPath(t *testing.T) {
	ar := NewArena()
	defer runtime.KeepAlive(ar)

	type Item struct {
		Ref *int
	}

	// Use NewSlice + direct assignment to bypass deep copy
	items := NewSlice[Item](ar, 1, 1)
	heapVal := new(int)
	*heapVal = 7
	items[0].Ref = heapVal // direct assignment — heap pointer in arena

	type Holder struct {
		Items []Item
	}
	holder := New[Holder](ar)
	holder.Items = items

	violations := ar.AuditPointers(holder)
	found := false
	for _, v := range violations {
		if v.Kind == ViolationPointer {
			found = true
			assert.Contains(t, v.Path, "Items[")
		}
	}
	assert.True(t, found, "should find heap pointer inside slice element")
}

// ---------------------------------------------------------------------------
// 4.10 Map scans only valid slots
// ---------------------------------------------------------------------------

func TestAudit_MapValidSlots(t *testing.T) {
	ar := NewArena()
	defer runtime.KeepAlive(ar)

	m := NewMap[int, string](ar, 8)
	m.Put(1, *DeepCopy(ar, "one"))
	m.Put(2, *DeepCopy(ar, "two"))
	m.Put(3, *DeepCopy(ar, "three"))
	m.Remove(2)

	violations := ar.AuditPointers(m)
	for _, v := range violations {
		assert.NotContains(t, v.Path, "keys[2]", "deleted slot should not be scanned")
		assert.NotContains(t, v.Path, "values[2]", "deleted slot should not be scanned")
	}
}

// ---------------------------------------------------------------------------
// 4.11 Vector scans only len elements
// ---------------------------------------------------------------------------

func TestAudit_VectorLenOnly(t *testing.T) {
	ar := NewArena()
	defer runtime.KeepAlive(ar)

	vec := NewVector[int](ar, 16)
	vec.Append(1, 2, 3)

	violations := ar.AuditPointers(vec)
	assert.Empty(t, violations)
}

// ---------------------------------------------------------------------------
// 4.12 Cycle detection
// ---------------------------------------------------------------------------

func TestAudit_CycleDetection(t *testing.T) {
	ar := NewArena()
	defer runtime.KeepAlive(ar)

	type Node struct {
		Next *Node
	}

	a := New[Node](ar)
	b := New[Node](ar)
	a.Next = b
	b.Next = a

	violations := ar.AuditPointers(a)
	assert.Empty(t, violations)
}

// ---------------------------------------------------------------------------
// 4.13 Violation stops recursion
// ---------------------------------------------------------------------------

func TestAudit_ViolationStopsRecursion(t *testing.T) {
	ar := NewArena()
	defer runtime.KeepAlive(ar)

	type Deep struct {
		Value int
	}

	type Holder struct {
		Ptr *Deep
	}

	heapDeep := &Deep{Value: 42}
	holder := New[Holder](ar)
	holder.Ptr = heapDeep

	violations := ar.AuditPointers(holder)
	assert.Len(t, violations, 1)
	assert.Equal(t, "Ptr", violations[0].Path)
	assert.Equal(t, ViolationPointer, violations[0].Kind)
}

// ---------------------------------------------------------------------------
// 4.14 Deep container scan — Map value with heap pointer
// ---------------------------------------------------------------------------

func TestAudit_MapValueHeapPointer(t *testing.T) {
	ar := NewArena()
	defer runtime.KeepAlive(ar)

	type Score struct {
		Ref *int
	}

	m := NewMap[int, Score](ar, 8)

	// Bypass deep copy: directly manipulate Map's internal values array
	heapInt := new(int)
	*heapInt = 99

	// Use Put with a Score that has a heap Ref
	// Since Map.Put does deepCopyValue, the heap pointer would get moved to arena.
	// Instead, directly set a value in the Map's internal array after allocation.
	m.Put(1, Score{}) // puts empty score at some index
	// Find which index key 1 landed at
	for i := 0; i < m.capacity; i++ {
		if m.ctrl[i] > ctrlDeleted && m.keys[i] == 1 {
			m.values[i].Ref = heapInt // direct write bypasses deepCopyValue
			break
		}
	}

	violations := ar.AuditPointers(m)
	found := false
	for _, v := range violations {
		if v.Kind == ViolationPointer {
			assert.Contains(t, v.Path, "values[")
			assert.Contains(t, v.Path, "Ref")
			found = true
		}
	}
	assert.True(t, found, "should detect heap pointer inside Map value")
}

// ---------------------------------------------------------------------------
// Additional: Vector with heap pointer element
// ---------------------------------------------------------------------------

func TestAudit_VectorHeapPointer(t *testing.T) {
	ar := NewArena()
	defer runtime.KeepAlive(ar)

	type Item struct {
		Data *string
	}

	vec := NewVector[Item](ar, 4)

	// Append an item, then directly set a heap pointer
	// (bypassing deepCopy by writing directly into the backing array)
	vec.Append(Item{})
	heapStr := "heap"
	// Direct write into the vector's backing array
	vec.vec[0].Data = &heapStr

	violations := ar.AuditPointers(vec)
	found := false
	for _, v := range violations {
		if v.Kind == ViolationPointer {
			assert.Contains(t, v.Path, "vec[0]")
			found = true
		}
	}
	assert.True(t, found, "should detect heap pointer inside Vector element")
}
