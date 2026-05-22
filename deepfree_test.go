package arena

import (
	"reflect"
	"runtime"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// refCount returns the ref count of the arena's current chunk block.
// This is only accessible because we are in the same package (internal test).
func currentRefCount(ar *Arena) int64 {
	return ar.current.ref
}

// ---------------------------------------------------------------------------
// deepFree — value types (struct with only value fields)
// ---------------------------------------------------------------------------

func TestDeepFreeValueTypes(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	type valOnly struct {
		A int32
		B float64
	}

	refBefore := currentRefCount(ar)
	p := New[valOnly](ar)
	p.A = 100
	p.B = 2.5
	refAfter := currentRefCount(ar)
	assert.Equal(t, refBefore+1, refAfter, "one allocation should increase ref by 1")

	ar.Free(p)
	assert.Equal(t, refBefore, currentRefCount(ar), "freeing value-only struct should return ref to prior level")
}

// ---------------------------------------------------------------------------
// deepFree — pointer
// ---------------------------------------------------------------------------

func TestDeepFreePointer(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	p := ar.Int(42)
	refAfter := currentRefCount(ar)

	ar.Free(p)
	assert.LessOrEqual(t, currentRefCount(ar), refAfter,
		"freeing a pointer should decrement ref count")
}

// ---------------------------------------------------------------------------
// deepFree — struct with multiple pointer fields
// ---------------------------------------------------------------------------

func TestDeepFreeStruct(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	type multiPtr struct {
		X *int32
		Y *float64
	}

	p := New[multiPtr](ar)
	p.X = ar.Int32(10)
	p.Y = ar.Float64(3.14)

	// p itself is one allocation, plus X and Y are two more.
	refBefore := currentRefCount(ar)

	ar.Free(p)

	// After freeing the struct, all three allocations should be freed.
	// The ref count on the current block should have decreased.
	assert.LessOrEqual(t, currentRefCount(ar), refBefore,
		"freeing struct with pointer fields should decrement refs")
}

// ---------------------------------------------------------------------------
// deepFree — slice of pointers
// ---------------------------------------------------------------------------

func TestDeepFreeSlice(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	// Create a slice of pointers.
	s := NewSlice[*int](ar, 3, 3)
	s[0] = ar.Int(1)
	s[1] = ar.Int(2)
	s[2] = ar.Int(3)

	refBefore := currentRefCount(ar)

	ar.Free(s)

	assert.LessOrEqual(t, currentRefCount(ar), refBefore,
		"freeing slice of pointers should free all element pointers + slice data")
}

// ---------------------------------------------------------------------------
// deepFree — string
// ---------------------------------------------------------------------------

func TestDeepFreeString(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	s := ar.String("hello arena copy")
	require.NotNil(t, s)
	assert.Equal(t, "hello arena copy", *s)

	// The string data is backed by arena memory (from Bytes).
	dataPtr := uintptr(unsafe.Pointer(unsafe.StringData(*s)))
	assert.True(t, ar.isManaged(dataPtr),
		"string data should be arena-managed")

	refBefore := currentRefCount(ar)
	ar.Free(s)
	assert.LessOrEqual(t, currentRefCount(ar), refBefore,
		"freeing string should release the underlying byte data")
}

// ---------------------------------------------------------------------------
// deepFree — circular reference (no double-free)
// ---------------------------------------------------------------------------

func TestDeepFreeCircularRef(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	type Node struct {
		Val  int
		Next *Node
	}

	// Build a cycle: a -> b -> a
	a := New[Node](ar)
	b := New[Node](ar)
	a.Val = 1
	a.Next = b
	b.Val = 2
	b.Next = a

	// Free must not double-free due to the visited map in deepFree.
	assert.NotPanics(t, func() {
		ar.Free(a)
	}, "freeing a circular structure must not panic")
}

// ---------------------------------------------------------------------------
// deepFree — verify ref counts across multiple objects
// ---------------------------------------------------------------------------

func TestDeepFreeVerifyRefCount(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	// Allocate several independent objects.
	p1 := ar.Int(10)
	p2 := ar.Int(20)
	p3 := ar.Int(30)

	// Record ref count after all allocations.
	refAll := currentRefCount(ar)

	// Free only p2; ref count should drop by 1.
	ar.Free(p2)
	refAfterP2 := currentRefCount(ar)
	assert.LessOrEqual(t, refAfterP2, refAll,
		"freeing one of three objects should decrement ref")

	// p1 and p3 should still be valid.
	assert.Equal(t, 10, *p1)
	assert.Equal(t, 30, *p3)

	// Free the remaining objects.
	ar.Free(p1)
	ar.Free(p3)

	assert.LessOrEqual(t, currentRefCount(ar), refAfterP2,
		"freeing remaining objects should further decrement ref")
}

// ---------------------------------------------------------------------------
// deepFree — struct with slice field
// ---------------------------------------------------------------------------

func TestDeepFreeStructWithSlice(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	type withSlice struct {
		Items []int32
	}

	p := New[withSlice](ar)
	items := NewSlice[int32](ar, 3, 3)
	items[0] = 10
	items[1] = 20
	items[2] = 30
	p.Items = items

	refBefore := currentRefCount(ar)

	ar.Free(p)

	assert.LessOrEqual(t, currentRefCount(ar), refBefore,
		"freeing struct with slice should release slice data")
}

// ---------------------------------------------------------------------------
// deepFree — nil pointer (no-op)
// ---------------------------------------------------------------------------

func TestDeepFreeNilPointer(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	type withPtr struct {
		P *int32
	}

	p := New[withPtr](ar)
	p.P = nil

	refBefore := currentRefCount(ar)
	ar.Free(p)
	assert.LessOrEqual(t, currentRefCount(ar), refBefore,
		"freeing struct with nil pointer should not error")
}

// ---------------------------------------------------------------------------
// deepFree — nil slice (no-op for slice data)
// ---------------------------------------------------------------------------

func TestDeepFreeNilSlice(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	type withNilSlice struct {
		Data []int32
	}

	p := New[withNilSlice](ar)
	p.Data = nil

	refBefore := currentRefCount(ar)
	ar.Free(p)
	assert.LessOrEqual(t, currentRefCount(ar), refBefore)
}

// ---------------------------------------------------------------------------
// deepFree — array of value types (no individual frees needed)
// ---------------------------------------------------------------------------

func TestDeepFreeArrayOfValues(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	type withArr struct {
		Data [4]int32
	}

	p := New[withArr](ar)
	p.Data = [4]int32{1, 2, 3, 4}

	refBefore := currentRefCount(ar)
	ar.Free(p)
	assert.LessOrEqual(t, currentRefCount(ar), refBefore)
}

// ---------------------------------------------------------------------------
// deepFree — array of pointers
// ---------------------------------------------------------------------------

func TestDeepFreeArrayOfPointers(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	type withPtrArr struct {
		Ptrs [3]*int32
	}

	p := New[withPtrArr](ar)
	p.Ptrs[0] = ar.Int32(10)
	p.Ptrs[1] = ar.Int32(20)
	p.Ptrs[2] = ar.Int32(30)

	refBefore := currentRefCount(ar)
	ar.Free(p)
	assert.LessOrEqual(t, currentRefCount(ar), refBefore,
		"freeing array of pointers should free each element")
}

// ---------------------------------------------------------------------------
// deepFree — interface with pointer value
// ---------------------------------------------------------------------------

func TestDeepFreeInterface(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	type ifaceWrap struct {
		Val any
	}

	v := ar.Int(42)
	p := New[ifaceWrap](ar)
	p.Val = v // interface wrapping *int allocated in arena

	refBefore := currentRefCount(ar)
	ar.Free(p)
	assert.LessOrEqual(t, currentRefCount(ar), refBefore,
		"freeing struct with interface wrapping arena pointer should free it")
}

// ---------------------------------------------------------------------------
// deepFree — nil interface (no-op)
// ---------------------------------------------------------------------------

func TestDeepFreeNilInterface(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	type ifaceWrap struct {
		Val any
	}

	p := New[ifaceWrap](ar)
	p.Val = nil

	refBefore := currentRefCount(ar)
	ar.Free(p)
	assert.LessOrEqual(t, currentRefCount(ar), refBefore)
}

// ---------------------------------------------------------------------------
// deepFree — deep-copied object can be freed
// ---------------------------------------------------------------------------

func TestDeepFreeDeepCopiedObject(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	type obj struct {
		Name  string
		Value int32
		Next  *obj
	}

	src := obj{Name: "root", Value: 1}
	cp := DeepCopy(ar, src)

	refBefore := currentRefCount(ar)
	ar.Free(cp)
	assert.LessOrEqual(t, currentRefCount(ar), refBefore,
		"freeing a DeepCopy-ed object should work correctly")
}

// ---------------------------------------------------------------------------
// deepFree — string isolation (deep copy then free)
// ---------------------------------------------------------------------------

func TestDeepFreeStringAfterDeepCopy(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	s := DeepCopy(ar, "arena string")
	require.NotNil(t, s)
	assert.Equal(t, "arena string", *s)

	ar.Free(s)

	// After freeing, the arena ref count should be reduced.
	// The arena itself should still be functional for new allocations.
	p := ar.Int(100)
	assert.Equal(t, 100, *p)
	runtime.KeepAlive(p)
}

// ---------------------------------------------------------------------------
// deepFree — reflect edge cases
// ---------------------------------------------------------------------------

func TestDeepFreeReflectEdgeCases(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	t.Run("deep_free_func_panics", func(t *testing.T) {
		src := reflect.ValueOf(func() {})
		assert.Panics(t, func() {
			deepFree(ar, src, make(map[uintptr]struct{}))
		})
	})

	t.Run("deep_free_map_panics", func(t *testing.T) {
		src := reflect.ValueOf(map[string]int{})
		assert.Panics(t, func() {
			deepFree(ar, src, make(map[uintptr]struct{}))
		})
	})

	t.Run("deep_free_chan_panics", func(t *testing.T) {
		src := reflect.ValueOf(make(chan int))
		assert.Panics(t, func() {
			deepFree(ar, src, make(map[uintptr]struct{}))
		})
	})
}

// ---------------------------------------------------------------------------
// deepFree — complex nested structure
// ---------------------------------------------------------------------------

func TestDeepFreeComplexNested(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	type leaf struct {
		Value int32
	}

	type branch struct {
		Children []*leaf
		Name     string
	}

	root := New[branch](ar)
	root.Name = *ar.String("root")

	c1 := New[leaf](ar)
	c1.Value = 10
	c2 := New[leaf](ar)
	c2.Value = 20

	root.Children = NewSlice[*leaf](ar, 2, 2)
	root.Children[0] = c1
	root.Children[1] = c2

	refBefore := currentRefCount(ar)

	ar.Free(root)

	assert.LessOrEqual(t, currentRefCount(ar), refBefore,
		"freeing complex nested structure should release all sub-allocations")
}
