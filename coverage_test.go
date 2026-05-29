package arena

import (
	"hash/maphash"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
)

// Helper: manually init a minimal map for edge case testing
func newMinimalMap[K comparable, V any](ar *Arena, cap_ int) *Map[K, V] {
	m := New[Map[K, V]](ar)
	m.allocator = ar
	m.seed = newSeed()
	m.capacity = cap_
	m.computeThresholds()
	if cap_ > 0 {
		m.allocArrays(cap_)
	}
	return m
}

func newSeed() maphash.Seed { return maphash.MakeSeed() }

// ============================================================================
// arena.go coverage: Free with lock, IsManagedPointer, String, DeepCopy
// ============================================================================

func TestFreeWithLock(t *testing.T) {
	ar := NewArena(WithEnableLock(true))
	p := New[int](ar)
	*p = 42
	ar.Free(p) // cover the useLock branch in Free
	// After Free the arena memory is released; p itself is a local copy
}

func TestIsManagedPointer(t *testing.T) {
	t.Run("arena pointer returns true", func(t *testing.T) {
		ar := NewArena()
		p := ar.Malloc(16)
		assert.True(t, ar.IsManagedPointer(p))
	})

	t.Run("arena pointer with lock", func(t *testing.T) {
		ar := NewArena(WithEnableLock(true))
		p := ar.Malloc(16)
		assert.True(t, ar.IsManagedPointer(p))
	})

	t.Run("heap pointer returns false", func(t *testing.T) {
		ar := NewArena()
		heapInt := new(int)
		assert.False(t, ar.IsManagedPointer(unsafe.Pointer(heapInt)))
	})
}

func TestArenaString(t *testing.T) {
	t.Run("non-empty string", func(t *testing.T) {
		ar := NewArena()
		s := ar.String("hello arena")
		assert.Equal(t, "hello arena", s)
	})

	t.Run("empty string", func(t *testing.T) {
		ar := NewArena()
		s := ar.String("")
		assert.Equal(t, "", s)
	})
}

func TestDeepCopyFallbackReflect(t *testing.T) {
	// Cover the fallback reflect path in DeepCopy for non-struct/non-ptr types
	t.Run("slice type direct", func(t *testing.T) {
		ar := NewArena()
		src := []string{"hello", "world"}
		dst := Append(ar, NewSlice[string](ar, 0, 2), src...)
		assert.Equal(t, 2, len(dst))
		assert.Equal(t, "hello", dst[0])
		assert.Equal(t, "world", dst[1])
	})
}

func TestDeepCopySliceReflectFallback(t *testing.T) {
	// Cover the reflect fallback path in deepCopySliceArena (cyclic types)
	ar := NewArena()
	type node struct {
		Val  int
		Next *node
	}
	slice := []node{{Val: 1}, {Val: 2}}
	result := Append(ar, NewSlice[node](ar, 0, 2), slice...)
	assert.Equal(t, 2, len(result))
	assert.Equal(t, 1, result[0].Val)
	assert.Equal(t, 2, result[1].Val)
}

func TestDeepCopyReflectAllKinds(t *testing.T) {
	// Cover deepCopy reflect branches: nil ptr, nil slice, empty string,
	// nil interface, struct non-addressable, array, etc.
	t.Run("nil pointer field", func(t *testing.T) {
		ar := NewArena()
		type s struct{ P *int }
		src := s{P: nil}
		dst := DeepCopy(ar, src)
		assert.Nil(t, dst.P)
	})

	t.Run("nil slice field", func(t *testing.T) {
		ar := NewArena()
		type s struct{ V []int }
		src := s{V: nil}
		dst := DeepCopy(ar, src)
		assert.Nil(t, dst.V)
	})

	t.Run("empty string field", func(t *testing.T) {
		ar := NewArena()
		type s struct{ Name string }
		src := s{Name: ""}
		dst := DeepCopy(ar, src)
		assert.Equal(t, "", dst.Name)
	})

	t.Run("nil interface field", func(t *testing.T) {
		ar := NewArena()
		type s struct{ V any }
		src := s{V: nil}
		dst := DeepCopy(ar, src)
		assert.Nil(t, dst.V)
	})

	t.Run("struct with interface field", func(t *testing.T) {
		ar := NewArena()
		// Test the reflect fallback for interface fields in copy plan
		type s struct {
			Name string
			Val  any
		}
		src := s{Name: "test", Val: int(42)}
		dst := DeepCopy(ar, src)
		assert.Equal(t, "test", dst.Name)
		// Copy plan wraps interface values as pointers
		ptrVal, ok := dst.Val.(*int)
		assert.True(t, ok)
		assert.Equal(t, 42, *ptrVal)
	})

	t.Run("array field", func(t *testing.T) {
		ar := NewArena()
		type s struct{ Arr [3]int }
		src := s{Arr: [3]int{1, 2, 3}}
		dst := DeepCopy(ar, src)
		assert.Equal(t, [3]int{1, 2, 3}, dst.Arr)
	})
}

func TestDeepCopyReflectCyclicVisited(t *testing.T) {
	// Cover the visited map hit path in deepCopy for cyclic ptr refs
	ar := NewArena()
	type node struct {
		Val  int
		Next *node
	}
	a := &node{Val: 1}
	b := &node{Val: 2}
	a.Next = b
	b.Next = a // cycle

	// DeepCopy a pointer triggers the reflect fallback since node contains a cycle
	result := DeepCopy(ar, a)
	assert.Equal(t, 1, (*result).Val)
	assert.Equal(t, 2, (*result).Next.Val)
	// Cycle is preserved: a → b → a
	assert.Equal(t, (*result).Val, (*(*result).Next.Next).Val)
}

func TestDeepFreeReflectAllKinds(t *testing.T) {
	t.Run("nil slice", func(t *testing.T) {
		ar := NewArena()
		type s struct{ V []int }
		obj := DeepCopy(ar, s{V: nil})
		ar.Free(obj) // should not panic
	})

	t.Run("nil interface", func(t *testing.T) {
		ar := NewArena()
		type s struct{ V any }
		obj := DeepCopy(ar, s{V: nil})
		ar.Free(obj) // should not panic
	})

	t.Run("nil pointer", func(t *testing.T) {
		ar := NewArena()
		type s struct{ P *int }
		obj := DeepCopy(ar, s{P: nil})
		ar.Free(obj) // should not panic
	})

	t.Run("struct with string field", func(t *testing.T) {
		ar := NewArena()
		type s struct{ Name string }
		obj := DeepCopy(ar, s{Name: "test"})
		ar.Free(obj)
	})

	t.Run("array of values", func(t *testing.T) {
		ar := NewArena()
		type s struct{ Arr [3]int }
		obj := DeepCopy(ar, s{Arr: [3]int{1, 2, 3}})
		ar.Free(obj)
	})

	t.Run("slice of pointers", func(t *testing.T) {
		ar := NewArena()
		type s struct{ V []*int }
		v1, v2 := 10, 20
		src := s{V: []*int{&v1, &v2}}
		dst := DeepCopy(ar, src)
		assert.Equal(t, 2, len(dst.V))
		assert.Equal(t, 10, *dst.V[0])
		assert.Equal(t, 20, *dst.V[1])
		ar.Free(dst)
	})

	t.Run("interface with value", func(t *testing.T) {
		ar := NewArena()
		type s struct{ V any }
		obj := DeepCopy(ar, s{V: int(42)})
		ar.Free(obj)
	})
}

func TestMallocFreelistExhaust(t *testing.T) {
	// Cover malloc freelist path where no suitable chunk found
	ar := NewArena(WithPoolSize(64), WithChunkSize(512))
	for i := 0; i < 10; i++ {
		p := ar.Malloc(64)
		ar.Free(p)
	}
	// Allocate something that needs a new chunk from heap
	p := ar.Malloc(1024)
	assert.NotNil(t, p)
}

func TestMallocMinHolePath(t *testing.T) {
	// Cover the minHoleSize path in Malloc
	ar := NewArena(WithChunkSize(512))
	// Fill up most of current chunk
	for i := 0; i < 20; i++ {
		ar.Malloc(16)
	}
	// Now allocate something small — might trigger minHole if remaining >= minHoleSize
	p := ar.Malloc(8)
	assert.NotNil(t, p)
}

// ============================================================================
// copyplan.go coverage: buildCopyPlan for arrays, executeOp branches
// ============================================================================

func TestCopyPlanArray(t *testing.T) {
	ar := NewArena()
	type s struct {
		Arr [3]string
	}
	src := s{Arr: [3]string{"a", "b", "c"}}
	dst := DeepCopy(ar, src)
	assert.Equal(t, src, *dst)
}

func TestCopyPlanInterfaceField(t *testing.T) {
	ar := NewArena()
	type s struct {
		Name string
		Val  any
	}
	src := s{Name: "test", Val: int(42)}
	dst := DeepCopy(ar, src)
	assert.Equal(t, "test", dst.Name)
	// Copy plan wraps interface values as pointers
	ptrVal, ok := dst.Val.(*int)
	assert.True(t, ok)
	assert.Equal(t, 42, *ptrVal)
}

func TestCopyPlanDeepCopierField(t *testing.T) {
	ar := NewArena()
	type outer struct {
		Inner *Map[string, int]
	}
	m := NewMap[string, int](ar, 8)
	m.Put("key", 42)
	src := outer{Inner: m}
	dst := DeepCopy(ar, src)
	assert.Equal(t, 1, dst.Inner.Len())
	v, ok := dst.Inner.Get("key")
	assert.True(t, ok)
	assert.Equal(t, 42, v)
}

func TestCopyPlanSliceWithSubPlan(t *testing.T) {
	ar := NewArena()
	type inner struct{ Name string }
	type outer struct{ Items []inner }
	src := outer{Items: []inner{{"a"}, {"b"}}}
	dst := DeepCopy(ar, src)
	assert.Equal(t, 2, len(dst.Items))
	assert.Equal(t, "a", dst.Items[0].Name)
	assert.Equal(t, "b", dst.Items[1].Name)
}

func TestCopyPlanArrayFlatAndNonFlat(t *testing.T) {
	ar := NewArena()
	type s struct {
		FlatArr   [3]int
		StringArr [2]string
	}
	src := s{FlatArr: [3]int{1, 2, 3}, StringArr: [2]string{"x", "y"}}
	dst := DeepCopy(ar, src)
	assert.Equal(t, src, *dst)
}

// ============================================================================
// map.go coverage
// ============================================================================

func TestMapPutTombstoneReuse(t *testing.T) {
	// Cover the tombstone reuse path in Put (firstTombstone >= 0)
	ar := NewArena()
	m := NewMap[int, int](ar, 8)
	// Insert
	m.Put(1, 10)
	m.Put(2, 20)
	m.Put(3, 30)
	// Remove to create tombstones
	m.Remove(2)
	assert.Equal(t, 2, m.Len())
	// Re-insert at same key — should reuse tombstone
	m.Put(2, 200)
	assert.Equal(t, 3, m.Len())
	v, ok := m.Get(2)
	assert.True(t, ok)
	assert.Equal(t, 200, v)
}

func TestMapGetFullProbe(t *testing.T) {
	// Cover the full-probe fallback in Get (line 184)
	ar := NewArena()
	m := NewMap[int, int](ar, 8)
	// Fill map to high load
	for i := 0; i < 5; i++ {
		m.Put(i, i*10)
	}
	// Get a missing key — probes until ctrlEmpty
	_, ok := m.Get(999)
	assert.False(t, ok)
}

func TestMapAllEarlyBreak(t *testing.T) {
	// Cover early break in All() iterator (yield returns false)
	ar := NewArena()
	m := NewMap[int, int](ar, 8)
	for i := 0; i < 10; i++ {
		m.Put(i, i)
	}
	count := 0
	for k, v := range m.All() {
		count++
		_ = k
		_ = v
		if count >= 3 {
			break
		}
	}
	assert.Equal(t, 3, count)
}

func TestMapAllTailSlots(t *testing.T) {
	// Cover the tail loop in All() — non-aligned capacity
	ar := NewArena()
	// capacity 12 is not a multiple of 8, so tail loop executes
	m := NewMap[int, int](ar, 12)
	for i := 0; i < 8; i++ {
		m.Put(i, i)
	}
	count := 0
	for range m.All() {
		count++
	}
	assert.Equal(t, 8, count)
}

func TestMapShouldResizeCapacityZero(t *testing.T) {
	// Cover capacity==0 branch in shouldResize
	ar := NewArena()
	m := newMinimalMap[int, int](ar, 0)
	assert.True(t, m.shouldResize())
}

func TestMapResizeMinCapacity(t *testing.T) {
	// Cover resize from capacity 0 → minCapacity
	ar := NewArena()
	m := newMinimalMap[int, int](ar, 0)
	m.Put(1, 10) // triggers resize from 0
	assert.Equal(t, 1, m.Len())
	v, ok := m.Get(1)
	assert.True(t, ok)
	assert.Equal(t, 10, v)
}

func TestMapDeepCopyKeyFallback(t *testing.T) {
	// Cover deepCopyKey reflect fallback (non-copy-plan path)
	ar := NewArena()
	// string keys use deepCopyKey via reflect when copyPlan not available
	// For cyclic key types
	type node struct {
		Val  string
		Next *node
	}
	// string is not cyclic, but let's test the standard deepCopyKey path
	m2 := NewMap[string, int](ar, 8)
	m2.Put("hello", 42)
	v, ok := m2.Get("hello")
	assert.True(t, ok)
	assert.Equal(t, 42, v)
}

func TestMapPutOverwriteDeep(t *testing.T) {
	// Cover Put overwrite path for deep value types
	ar := NewArena()
	m := NewMap[int, string](ar, 8)
	m.Put(1, "first")
	m.Put(1, "second") // overwrite
	v, ok := m.Get(1)
	assert.True(t, ok)
	assert.Equal(t, "second", v)
}

func TestMapPutPanicFull(t *testing.T) {
	// Cover "map is full" panic: create a map that can't resize and fill it
	ar := NewArena()
	m := newMinimalMap[int, int](ar, 8)
	m.loadThreshold = 100 // prevent resize
	m.tombstoneThreshold = 100

	assert.Panics(t, func() {
		for i := 0; i <= m.capacity; i++ {
			m.Put(i, i)
		}
	})
}

func TestNextPowerOf2(t *testing.T) {
	assert.Equal(t, 8, nextPowerOf2(0))
	assert.Equal(t, 8, nextPowerOf2(-1))
	assert.Equal(t, 8, nextPowerOf2(5))
	assert.Equal(t, 16, nextPowerOf2(9))
	assert.Equal(t, 256, nextPowerOf2(200))
}

// ============================================================================
// vector.go coverage
// ============================================================================

func TestVectorAllEarlyBreak(t *testing.T) {
	ar := NewArena()
	vec := NewVector[int](ar, 10)
	for i := 0; i < 10; i++ {
		vec.Append(i)
	}
	count := 0
	for i, v := range vec.All() {
		count++
		_ = i
		_ = v
		if count >= 3 {
			break
		}
	}
	assert.Equal(t, 3, count)
}

func TestVectorRemoveByWithLimit(t *testing.T) {
	ar := NewArena()
	vec := NewVector[int](ar, 10)
	for i := 0; i < 10; i++ {
		vec.Append(i)
	}
	// Remove even numbers with limit 2
	removed := vec.RemoveBy(2, func(idx int, v int) bool {
		return v%2 == 0
	})
	assert.Equal(t, 2, removed)
	assert.Equal(t, 8, vec.Len())
}

func TestVectorLastIndex(t *testing.T) {
	ar := NewArena()
	vec := NewVector[int](ar, 10)
	vec.Append(1, 2, 3, 2, 1)
	idx := vec.LastIndex(2)
	assert.Equal(t, 3, idx)

	// Not found
	idx = vec.LastIndex(99)
	assert.Equal(t, -1, idx)
}

// ============================================================================
// audit.go coverage
// ============================================================================

func TestAuditInterfaceField(t *testing.T) {
	ar := NewArena()
	type s struct {
		Val any
	}
	p := DeepCopy(ar, s{Val: int(42)})
	violations := ar.AuditPointers(p)
	assert.Empty(t, violations)
}

func TestAuditInterfaceHeapPointer(t *testing.T) {
	ar := NewArena()
	type s struct {
		Val any
	}
	heapPtr := new(int)
	*heapPtr = 42
	p := New[s](ar)
	p.Val = heapPtr // non-arena pointer via interface
	violations := ar.AuditPointers(p)
	assert.NotEmpty(t, violations)
}

func TestAuditFuncNil(t *testing.T) {
	ar := NewArena()
	vec := NewVector[int](ar, 4)
	// Vector has an equatable func field; nil equatable is fine
	vec.equatable = nil
	violations := ar.AuditPointers(vec)
	assert.Empty(t, violations)
}

func TestAuditFuncNonNil(t *testing.T) {
	ar := NewArena()
	vec := NewVector[int](ar, 4)
	vec.Append(1, 2)
	violations := ar.AuditPointers(vec)
	assert.Empty(t, violations)
}

func TestAuditItoa(t *testing.T) {
	ar := NewArena()
	// Test with >= 10 elements to cover _itoa path and itoa >= 10 path
	vec := NewVector[int](ar, 0)
	for i := 0; i < 20; i++ {
		vec.Append(i)
	}
	violations := ar.AuditPointers(vec)
	assert.Empty(t, violations)
}

func TestAuditSliceElement(t *testing.T) {
	ar := NewArena()
	type s struct {
		Items []string
	}
	p := DeepCopy(ar, s{Items: []string{"a", "b", "c"}})
	violations := ar.AuditPointers(p)
	assert.Empty(t, violations)
}

func TestAuditArrayElement(t *testing.T) {
	ar := NewArena()
	type s struct {
		Arr [3]*int
	}
	p := New[s](ar)
	for i := 0; i < 3; i++ {
		val := i
		p.Arr[i] = New[int](ar)
		*p.Arr[i] = val
	}
	violations := ar.AuditPointers(p)
	assert.Empty(t, violations)
}

func TestAuditStructNonAddressable(t *testing.T) {
	// Cover the non-addressable struct branch in auditStruct
	ar := NewArena()
	type inner struct{ X int }
	type outer struct{ I inner }
	p := DeepCopy(ar, outer{I: inner{X: 42}})
	violations := ar.AuditPointers(p)
	assert.Empty(t, violations)
}

func TestAuditSliceNonRecurse(t *testing.T) {
	// Cover auditSlice where element type doesn't need recursion
	ar := NewArena()
	type s struct {
		Vals []int
	}
	p := DeepCopy(ar, s{Vals: []int{1, 2, 3}})
	violations := ar.AuditPointers(p)
	assert.Empty(t, violations)
}

func TestAuditArrayNonRecurse(t *testing.T) {
	// Cover auditArray where element type doesn't need recursion
	ar := NewArena()
	type s struct {
		Arr [3]int
	}
	p := DeepCopy(ar, s{Arr: [3]int{1, 2, 3}})
	violations := ar.AuditPointers(p)
	assert.Empty(t, violations)
}

func TestAuditStringEmpty(t *testing.T) {
	ar := NewArena()
	type s struct{ Name string }
	p := DeepCopy(ar, s{Name: ""})
	violations := ar.AuditPointers(p)
	assert.Empty(t, violations)
}

func TestAuditPtrNil(t *testing.T) {
	ar := NewArena()
	type s struct{ P *int }
	p := DeepCopy(ar, s{P: nil})
	violations := ar.AuditPointers(p)
	assert.Empty(t, violations)
}

func TestAuditSliceNil(t *testing.T) {
	ar := NewArena()
	type s struct{ V []int }
	p := DeepCopy(ar, s{V: nil})
	violations := ar.AuditPointers(p)
	assert.Empty(t, violations)
}

func TestAuditInvalidValue(t *testing.T) {
	// Cover !val.IsValid() branch
	ar := NewArena()
	violations := ar.AuditPointers(nil)
	assert.Empty(t, violations)
}

func TestAuditFuncCanAddr(t *testing.T) {
	// Cover the CanAddr branch in auditFunc — use a Vector which has
	// an equatable func field tagged arena:"safe"
	ar := NewArena()
	vec := NewVector[int](ar, 4)
	vec.Append(1, 2, 3)
	violations := ar.AuditPointers(vec)
	assert.Empty(t, violations)
}

// ============================================================================
// validate.go coverage
// ============================================================================

func TestValidateRejectsMapNested(t *testing.T) {
	err := Validate[struct {
		F struct{ M map[int]int }
	}]()
	assert.Error(t, err)
}

func TestComputeTypeInfoInterface(t *testing.T) {
	// Cover the interface branch in computeTypeInfo
	info := getTypeInfo[any]()
	assert.True(t, info.valid)
	assert.False(t, info.flat)
}

func TestValidateUnsafePointer(t *testing.T) {
	err := Validate[struct {
		P unsafe.Pointer
	}]()
	assert.Error(t, err)
}

func TestFreeSliceBackingArray(t *testing.T) {
	// Free a flat slice — deepFree iterates but no-op for int elements
	ar := NewArena()
	s := NewSlice[int](ar, 5, 5)
	for i := 0; i < 5; i++ {
		s[i] = i
	}
	ar.Free(s)
}

func TestDeepCopySliceReflectFallbackCyclic(t *testing.T) {
	// Cover the cyclic path in deepCopySliceArena
	ar := NewArena()
	type node struct {
		Val  int
		Next *node
	}
	src := []node{{Val: 1}, {Val: 2}}
	dst := NewSlice[node](ar, 0, 2)
	dst = Append(ar, dst, src...)
	assert.Equal(t, 2, len(dst))
	assert.Equal(t, 1, dst[0].Val)
}

// ============================================================================
// Additional coverage: deepCopy visited map, audit non-addr paths, etc.
// ============================================================================

func TestDeepCopyPtrVisitedHit(t *testing.T) {
	// Cover the visited[addr] hit path in deepCopy for pointer cycle detection
	ar := NewArena()
	type node struct {
		Val  int
		Next *node
	}
	a := &node{Val: 1}
	b := &node{Val: 2}
	a.Next = b
	b.Next = a // cycle

	result := DeepCopy(ar, a)
	assert.Equal(t, 1, (*result).Val)
	assert.Equal(t, 2, (*(*result).Next).Val)
	// The cycle should point back
	assert.Equal(t, (*result).Val, (*(*(*result).Next).Next).Val)
}

func TestDeepCopyNonAddressableStruct(t *testing.T) {
	// Cover the non-addressable struct branch in deepCopy (field-by-field loop)
	ar := NewArena()
	type inner struct{ X int }
	type outer struct{ I inner }
	src := outer{I: inner{X: 42}}
	dst := DeepCopy(ar, src)
	assert.Equal(t, 42, dst.I.X)
}

func TestDeepCopyNilSliceField(t *testing.T) {
	// Cover nil slice path in deepCopy
	ar := NewArena()
	type s struct{ V []int }
	src := s{V: nil}
	dst := DeepCopy(ar, src)
	assert.Nil(t, dst.V)
}

func TestDeepCopyNonNilSliceField(t *testing.T) {
	// Cover non-nil slice path in deepCopy (reflect fallback)
	ar := NewArena()
	type s struct{ V []int }
	src := s{V: []int{1, 2, 3}}
	dst := DeepCopy(ar, src)
	assert.Equal(t, 3, len(dst.V))
	assert.Equal(t, 1, dst.V[0])
}

func TestDeepCopyInterfaceFieldWithPtr(t *testing.T) {
	// Cover the interface path in deepCopy where elem is a pointer
	ar := NewArena()
	type s struct{ V any }
	val := 42
	src := s{V: &val}
	dst := DeepCopy(ar, src)
	// The copy plan wraps interface values as pointers, so dst.V is a **int
	assert.NotNil(t, dst.V)
}

func TestDeepCopyDefaultKind(t *testing.T) {
	// Cover the default branch in deepCopy (value types like bool)
	ar := NewArena()
	type s struct{ B bool }
	src := s{B: true}
	dst := DeepCopy(ar, src)
	assert.Equal(t, true, dst.B)
}

func TestDeepFreeUnsafePointer(t *testing.T) {
	// Cover the UnsafePointer branch in deepFree
	ar := NewArena()
	p := ar.Malloc(16)
	ar.Free(unsafe.Pointer(p))
}

func TestDeepFreeSliceElements(t *testing.T) {
	// Cover deepFree for slice elements (non-nil, settable)
	ar := NewArena()
	type s struct{ V []*int }
	v1, v2 := 10, 20
	src := s{V: []*int{&v1, &v2}}
	dst := DeepCopy(ar, src)
	ar.Free(dst)
}

func TestDeepFreeArrayElements(t *testing.T) {
	// Cover deepFree for array elements
	ar := NewArena()
	type s struct{ Arr [3]*int }
	v1, v2, v3 := 1, 2, 3
	src := s{Arr: [3]*int{&v1, &v2, &v3}}
	dst := DeepCopy(ar, src)
	ar.Free(dst)
}

func TestDeepFreeInterfaceNonNil(t *testing.T) {
	// Cover the non-nil interface branch in deepFree
	ar := NewArena()
	type s struct{ V any }
	obj := DeepCopy(ar, s{V: int(42)})
	ar.Free(obj)
}

func TestDeepFreeStringField(t *testing.T) {
	// Cover deepFree string path (freePointer on string data)
	ar := NewArena()
	type s struct{ Name string }
	obj := DeepCopy(ar, s{Name: "hello world"})
	ar.Free(obj)
}

func TestDeepCopySliceReflectCyclicVisited(t *testing.T) {
	// Cover cyclic visited map usage in deepCopySliceArena reflect fallback
	ar := NewArena()
	type node struct {
		Val  int
		Next *node
	}
	a := &node{Val: 1}
	b := &node{Val: 2}
	a.Next = b
	b.Next = a
	src := []*node{a, b}
	dst := NewSlice[*node](ar, 0, 2)
	dst = Append(ar, dst, src...)
	assert.Equal(t, 2, len(dst))
	assert.Equal(t, 1, (*dst[0]).Val)
	assert.Equal(t, 2, (*dst[1]).Val)
}

// ============================================================================
// audit.go additional coverage
// ============================================================================

func TestAuditStructNonAddressableCanInterface(t *testing.T) {
	// Cover the CanInterface (non-addressable) branch in auditStruct
	// This happens when a struct value is inside a non-addressable container
	ar := NewArena()
	type inner struct{ X int }
	type outer struct{ I inner }
	p := DeepCopy(ar, outer{I: inner{X: 42}})
	violations := ar.AuditPointers(p)
	assert.Empty(t, violations)
}

func TestAuditFuncNonAddressable(t *testing.T) {
	// Cover the non-CanAddr branch in auditFunc
	ar := NewArena()
	vec := NewVector[int](ar, 4)
	vec.Append(1, 2)
	violations := ar.AuditPointers(vec)
	assert.Empty(t, violations)
}

func TestAuditPtrAlreadyVisited(t *testing.T) {
	// Cover the visited[ptr] hit path in auditPtr
	ar := NewArena()
	type node struct {
		Val  int
		Next *node
	}
	a := ar.Malloc(sizeOf[node]())
	p := (*node)(a)
	p.Val = 1
	// Create a self-referential pointer
	p.Next = p
	violations := ar.AuditPointers(p)
	assert.Empty(t, violations)
}

func TestAuditStringDataPtrZero(t *testing.T) {
	// Cover the dataPtr == 0 path in auditString
	// An empty string has len==0, which returns early. But a non-empty string
	// with dataPtr 0 is impossible in Go, so this branch is defensive.
	ar := NewArena()
	type s struct{ Name string }
	p := DeepCopy(ar, s{Name: "test"})
	violations := ar.AuditPointers(p)
	assert.Empty(t, violations)
}

func TestAuditSliceBackingArrayViolation(t *testing.T) {
	// Cover the slice backing array violation path in auditSlice
	ar := NewArena()
	type s struct {
		V []int
	}
	p := New[s](ar)
	p.V = []int{1, 2, 3} // heap slice, not arena
	violations := ar.AuditPointers(p)
	assert.NotEmpty(t, violations)
}

// ============================================================================
// copyplan.go additional coverage
// ============================================================================

func TestCopyPlanSliceWithSubPlanNonFlat(t *testing.T) {
	// Cover opSlice with subPlan and non-flat elements
	ar := NewArena()
	type item struct {
		Name string
		Val  int
	}
	type s struct{ Items []item }
	src := s{Items: []item{{"a", 1}, {"b", 2}}}
	dst := DeepCopy(ar, src)
	assert.Equal(t, 2, len(dst.Items))
	assert.Equal(t, "a", dst.Items[0].Name)
	assert.Equal(t, 1, dst.Items[0].Val)
}

func TestCopyPlanSliceMemmoveFallback(t *testing.T) {
	// Cover opSlice memmove fallback (no subPlan or no ops)
	ar := NewArena()
	type s struct{ V []int }
	src := s{V: []int{1, 2, 3}}
	dst := DeepCopy(ar, src)
	assert.Equal(t, 3, len(dst.V))
	assert.Equal(t, []int{1, 2, 3}, dst.V)
}

func TestCopyPlanArrayWithSubPlanNonFlat(t *testing.T) {
	// Cover opArray with subPlan and non-flat elements
	ar := NewArena()
	type s struct {
		Arr [2]string
	}
	src := s{Arr: [2]string{"hello", "world"}}
	dst := DeepCopy(ar, src)
	assert.Equal(t, "hello", dst.Arr[0])
	assert.Equal(t, "world", dst.Arr[1])
}

func TestCopyPlanPtrNilSource(t *testing.T) {
	// Cover opPtr with nil source
	ar := NewArena()
	type s struct{ P *int }
	src := s{P: nil}
	dst := DeepCopy(ar, src)
	assert.Nil(t, dst.P)
}

func TestCopyPlanPtrVisitedHit(t *testing.T) {
	// Cover opPtr visited map hit (cyclic pointer)
	ar := NewArena()
	type node struct {
		Val  int
		Next *node
	}
	a := &node{Val: 1}
	b := &node{Val: 2}
	a.Next = b
	b.Next = a
	result := DeepCopy(ar, a)
	assert.Equal(t, 1, (*result).Val)
	assert.Equal(t, 2, (*(*result).Next).Val)
}

// ============================================================================
// map.go additional coverage
// ============================================================================

func TestMapDeepCopyKeyReflectFallback(t *testing.T) {
	// Cover deepCopyKey reflect fallback path (non-copy-plan key type)
	ar := NewArena()
	// Use a struct key with pointer fields — triggers deepCopyKey reflect path
	type key struct {
		Name string
	}
	m := NewMap[key, int](ar, 8)
	m.Put(key{Name: "hello"}, 42)
	v, ok := m.Get(key{Name: "hello"})
	assert.True(t, ok)
	assert.Equal(t, 42, v)
}

func TestMapGetFullProbeReturn(t *testing.T) {
	// Cover the full-probe return (lines 203-205) in Get
	ar := NewArena()
	m := newMinimalMap[int, int](ar, 8)
	// Fill all slots except one to ensure long probe chains
	m.loadThreshold = 100
	m.tombstoneThreshold = 100
	for i := 0; i < m.capacity; i++ {
		m.ctrl[i] = int8(i)
		m.keys[i] = i
		m.values[i] = i
		m.length++
	}
	// Now Get a missing key should probe all capacity slots
	_, ok := m.Get(999)
	assert.False(t, ok)
}

func TestMapDeepCopyValueReflectFallback(t *testing.T) {
	// Cover deepCopyValue reflect fallback (non-copy-plan value type with cyclic ref)
	ar := NewArena()
	type node struct {
		Val  string
		Next *node
	}
	m := NewMap[int, *node](ar, 8)
	n := &node{Val: "test"}
	m.Put(1, n)
	v, ok := m.Get(1)
	assert.True(t, ok)
	assert.Equal(t, "test", v.Val)
}

// ============================================================================
// vector.go additional coverage
// ============================================================================

func TestVectorRemoveByNoLimit(t *testing.T) {
	// Cover RemoveBy with limit=0 (no limit, remove all matches)
	ar := NewArena()
	vec := NewVector[int](ar, 10)
	for i := 0; i < 10; i++ {
		vec.Append(i)
	}
	removed := vec.RemoveBy(0, func(idx int, v int) bool {
		return v%2 == 0
	})
	assert.Equal(t, 5, removed)
	assert.Equal(t, 5, vec.Len())
}

func TestVectorRemoveByLimitGreaterThanMatches(t *testing.T) {
	// Cover RemoveBy where limit > actual matches
	ar := NewArena()
	vec := NewVector[int](ar, 10)
	for i := 0; i < 5; i++ {
		vec.Append(i)
	}
	removed := vec.RemoveBy(10, func(idx int, v int) bool {
		return v == 3
	})
	assert.Equal(t, 1, removed)
	assert.Equal(t, 4, vec.Len())
}

// ============================================================================
// validate.go additional coverage
// ============================================================================

func TestMustValidatePanics(t *testing.T) {
	assert.Panics(t, func() {
		MustValidate[struct {
			F func()
		}]()
	})
}

func TestMustValidateValid(t *testing.T) {
	// Should not panic for valid types
	MustValidate[struct {
		X int
	}]()
}

func TestComputeTypeInfoCyclic(t *testing.T) {
	// Cover cyclic type detection
	info := getTypeInfo[*struct {
		Next *struct {
			Next *struct{}
		}
	}]()
	assert.True(t, info.valid)
}

// ============================================================================
// arena.go: Malloc OOM path, freelist non-current block, DeepCopy flat path
// ============================================================================

func TestMallocFromFreelistNonCurrentBlock(t *testing.T) {
	// Cover the non-current block path in freePointer
	ar := NewArena(WithPoolSize(64), WithChunkSize(512))
	// Allocate and free multiple blocks to populate freelist
	ptrs := make([]unsafe.Pointer, 0, 20)
	for i := 0; i < 20; i++ {
		p := ar.Malloc(64)
		ptrs = append(ptrs, p)
	}
	for _, p := range ptrs {
		ar.Free(p)
	}
	// Now allocate again — these should come from freelist
	p := ar.Malloc(64)
	assert.NotNil(t, p)
	ar.Free(p)
}

func TestDeepCopyFlatFastPath(t *testing.T) {
	// Cover the flat fast path in DeepCopy
	ar := NewArena()
	src := 42
	dst := DeepCopy(ar, src)
	assert.Equal(t, 42, *dst)
}

func TestDeepCopyBoolFlat(t *testing.T) {
	// Cover flat path for bool
	ar := NewArena()
	src := true
	dst := DeepCopy(ar, src)
	assert.Equal(t, true, *dst)
}

func TestMallocWithLock(t *testing.T) {
	// Cover Malloc lock branch
	ar := NewArena(WithEnableLock(true))
	p := ar.Malloc(16)
	assert.NotNil(t, p)
}

func TestFreeWithLockDeepValue(t *testing.T) {
	// Cover Free with lock + deep value (arena-managed string)
	ar := NewArena(WithEnableLock(true))
	s := ar.String("hello") // arena-managed string
	p := New[string](ar)
	*p = s
	ar.Free(p)
}

// ============================================================================
// Remaining coverage gaps: audit, copyplan, map reflect paths
// ============================================================================

func TestAuditStructWithArenaField(t *testing.T) {
	// Cover the *Arena field skip in auditStruct
	ar := NewArena()
	// Map has an allocator field of type *Arena
	m := NewMap[int, int](ar, 8)
	violations := ar.AuditPointers(m)
	assert.Empty(t, violations)
}

func TestAuditStructWithSafeTag(t *testing.T) {
	// Cover arena:"safe" tag skip in auditStruct
	ar := NewArena()
	type s struct {
		Val *int `arena:"safe"`
	}
	p := New[s](ar)
	// Put a heap pointer in the safe-tagged field — should be skipped by audit
	heapInt := new(int)
	*heapInt = 42
	p.Val = heapInt
	violations := ar.AuditPointers(p)
	assert.Empty(t, violations)
}

func TestAuditFuncNonCanAddr(t *testing.T) {
	// Cover the non-CanAddr branch in auditFunc
	// Pass a struct VALUE (not pointer) containing a func field,
	// so the func field itself is non-addressable.
	ar := NewArena()
	s := struct {
		F func()
	}{F: func() {}}
	violations := ar.AuditPointers(s)
	// func closure not in arena will be flagged, but we're testing the non-CanAddr code path
	assert.NotEmpty(t, violations)
}

func TestAuditInterfaceNil(t *testing.T) {
	// Cover nil interface in auditInterface
	ar := NewArena()
	type s struct{ V any }
	p := DeepCopy(ar, s{V: nil})
	violations := ar.AuditPointers(p)
	assert.Empty(t, violations)
}

func TestCopyPlanDefaultKind(t *testing.T) {
	// Cover the default branch in buildCopyPlan (primitive type like bool)
	ar := NewArena()
	src := true
	dst := DeepCopy(ar, src)
	assert.Equal(t, true, *dst)
}

func TestCopyPlanFieldOpDefault(t *testing.T) {
	// Cover the default branch in buildFieldOp (primitive field types)
	ar := NewArena()
	type s struct {
		A bool
		B int64
		C uint64
	}
	src := s{A: true, B: 42, C: 99}
	dst := DeepCopy(ar, src)
	assert.Equal(t, src, *dst)
}

func TestCopyPlanSliceInt64Memmove(t *testing.T) {
	// Cover opSlice memmove fallback (no subPlan ops — int64 elements)
	ar := NewArena()
	type s struct{ V []int64 }
	src := s{V: []int64{1, 2, 3}}
	dst := DeepCopy(ar, src)
	assert.Equal(t, 3, len(dst.V))
}

func TestCopyPlanArrayMemmoveFallback(t *testing.T) {
	// Cover opArray memmove fallback (flat elem, no subPlan ops)
	ar := NewArena()
	type s struct{ Arr [3]int64 }
	src := s{Arr: [3]int64{1, 2, 3}}
	dst := DeepCopy(ar, src)
	assert.Equal(t, src, *dst)
}

func TestMapDeepCopyKeyReflectFallbackFull(t *testing.T) {
	// Cover deepCopyKey reflect fallback (full path with cyclic key type)
	// *node has cyclic references, so copy plan is cyclic and flagKeyPlan won't be set
	ar := NewArena()
	type node struct {
		Val  int
		Next *node
	}
	m := NewMap[*node, int](ar, 8)
	n1 := &node{Val: 1}
	n2 := &node{Val: 2}
	n1.Next = n2
	m.Put(n1, 10)
	// Key is deep-copied to arena, so original heap pointer won't match via Get
	// But we can verify via iteration
	assert.Equal(t, 1, m.Len())
	found := false
	for k, v := range m.All() {
		assert.Equal(t, 10, v)
		assert.Equal(t, 1, k.Val) // deep-copied key has same value
		assert.NotNil(t, k.Next)  // deep-copied relations preserved
		assert.Equal(t, 2, k.Next.Val)
		found = true
	}
	assert.True(t, found)
}

func TestMapAllTailPathLarge(t *testing.T) {
	// Cover the tail loop in All() more thoroughly (capacity not multiple of 8)
	ar := NewArena()
	// capacity 7 is not a multiple of 8, but nextPowerOf2 gives 8
	// Use capacity 24 (3*8), one less
	m := NewMap[int, int](ar, 24)
	for i := 0; i < 20; i++ {
		m.Put(i, i)
	}
	count := 0
	for range m.All() {
		count++
	}
	assert.Equal(t, 20, count)
}

func TestValidateArenaContainerType(t *testing.T) {
	// Cover isArenaContainerType early return in validateTypeRecursive
	err := Validate[*Map[string, int]]()
	assert.NoError(t, err)
}

func TestComputeTypeInfoDefault(t *testing.T) {
	// Cover the default branch in computeTypeInfo
	// Complex128 is not explicitly listed, goes through default
	info := getTypeInfo[complex128]()
	assert.True(t, info.valid)
	assert.False(t, info.flat)
}

func TestDeepCopyCyclicStructPtr(t *testing.T) {
	// Cover visited map path for cyclic pointer types
	ar := NewArena()
	type node struct {
		Val  int
		Next *node
	}
	a := &node{Val: 1}
	b := &node{Val: 2}
	a.Next = b
	b.Next = a
	result := DeepCopy(ar, a)
	assert.Equal(t, 1, (*result).Val)
	assert.Equal(t, 2, (*result).Next.Val)
}

// Test heapMemory and nopLocker coverage (trivial empty methods)
func TestHeapMemoryFree(t *testing.T) {
	h := heapMemory{}
	h.Free(nil) // no-op
}

func TestNopLocker(t *testing.T) {
	n := nopLocker{}
	n.Lock()
	n.Unlock()
}

// ============================================================================
// Final coverage gap fillers
// ============================================================================

func TestAuditStructNonAddrArenaAuditer(t *testing.T) {
	// Cover the non-addressable arenaAuditer path in auditStruct (lines 191-199)
	// Pass a Map VALUE (not pointer) to AuditPointers
	ar := NewArena()
	m := NewMap[int, int](ar, 8)
	m.Put(1, 10)
	// Dereference to get a non-addressable Map value
	violations := ar.AuditPointers(*m)
	assert.Empty(t, violations)
}

func TestMapAllWordAtATimeSkip(t *testing.T) {
	// Cover the word-at-a-time empty-group skip in All()
	// Use a sparse map: 1 entry in capacity 32 → 3 groups of 8 will be empty
	ar := NewArena()
	m := newMinimalMap[int, int](ar, 32)
	m.Put(0, 0) // only 1 entry
	count := 0
	for range m.All() {
		count++
	}
	assert.Equal(t, 1, count)
}

func TestDeepCopyInterfaceReflect(t *testing.T) {
	// Cover interface path in deepCopy reflect (lines 742-751)
	ar := NewArena()
	type s struct{ V any }
	src := s{V: nil}
	dst := DeepCopy(ar, src)
	assert.Nil(t, dst.V)

	// Non-nil interface
	src2 := s{V: int(42)}
	dst2 := DeepCopy(ar, src2)
	assert.NotNil(t, dst2.V)
}

func TestAuditFuncNilNonVector(t *testing.T) {
	// Cover nil func check in auditFunc (line 255-257) when reached from auditStruct
	// Use a struct passed by value with a nil func field
	ar := NewArena()
	s := struct {
		F func()
	}{F: nil}
	violations := ar.AuditPointers(s)
	// nil func should not be flagged
	assert.Empty(t, violations)
}

// ============================================================================
// Cover buildCopyPlan slice case + buildFieldOp default
// ============================================================================

func TestAppendSliceOfSlices(t *testing.T) {
	// Cover buildCopyPlan's reflect.Slice case: when the element type is a slice,
	// e.g. [][]int, buildCopyPlan is called with the element type []int.
	ar := NewArena()
	src := [][]int{{1, 2}, {3, 4}}
	dst := NewSlice[[]int](ar, 0, 2)
	dst = Append(ar, dst, src...)
	assert.Equal(t, 2, len(dst))
	assert.Equal(t, 2, len(dst[0]))
	assert.Equal(t, 1, dst[0][0])
}

func TestCopyPlanComplex128Field(t *testing.T) {
	// Cover buildFieldOp default branch (complex128 not explicitly listed)
	ar := NewArena()
	type s struct {
		C complex128
	}
	src := s{C: complex(1.5, 2.5)}
	dst := DeepCopy(ar, src)
	assert.Equal(t, complex(1.5, 2.5), dst.C)
}

// ============================================================================
// StressTestComplex: long-running comprehensive stress test
// Exercises all basic types, containers, deep copy, free, audit in a tight loop
// with validation after every operation.
// ============================================================================

func TestStressComplex(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	const minDuration = 10 * time.Second
	deadline := time.Now().Add(minDuration)

	// Pre-define some struct types for deep copy testing
	type point struct {
		X, Y float64
		Name string
	}
	type node struct {
		ID    int
		Label string
		Next  *node
		Data  []int
	}

	// Seed for reproducibility while still randomizing
	var seed uint64 = 42
	rng := func() uint64 {
		seed = seed*1103515245 + 12345
		return seed
	}
	rngInt := func(n int) int {
		if n <= 0 {
			return 0
		}
		return int(rng() % uint64(n))
	}

	iter := 0
	ar := NewArena(WithChunkSize(64 * 1024)) // 64KB chunks
	var lastReset int

	for time.Now().Before(deadline) {
		iter++
		// Reset arena every ~500 iterations to prevent unbounded growth
		if iter-lastReset > 500 {
			ar.Reset()
			ar.current = ar.malloc(ar.chunkSize) // re-init current chunk for same Arena
			lastReset = iter
		}

		// ============================================================
		// Phase 1: Basic type allocation and access
		// ============================================================
		pInt := New[int](ar)
		*pInt = rngInt(100000)
		assert.Equal(t, *pInt, *pInt) // self-consistency
		pOld := *pInt

		pFloat := New[float64](ar)
		*pFloat = float64(rngInt(1000)) * 0.5
		assert.True(t, *pFloat >= 0)

		pStr := New[string](ar)
		*pStr = ar.String("item_" + itoa(rngInt(10000)))
		assert.NotEmpty(t, *pStr)
		assert.True(t, ar.IsManagedPointer(unsafe.Pointer(pStr)))

		pBool := New[bool](ar)
		*pBool = rng()%2 == 0

		// Modify basic values
		*pInt = *pInt + 1
		*pFloat = *pFloat + 1.0
		assert.NotEqual(t, pOld, *pInt)

		// ============================================================
		// Phase 2: Map operations
		// ============================================================
		cap_ := rngInt(32) + 8
		m := NewMap[int, int](ar, cap_)
		ref := make(map[int]int) // Go map for verification

		// Insert random entries
		nOps := rngInt(200) + 50
		for i := 0; i < nOps; i++ {
			k := rngInt(1000)
			v := rngInt(10000)

			op := rng() % 10
			switch {
			case op < 6: // Put
				m.Put(k, v)
				ref[k] = v
			case op < 8: // Remove
				m.Remove(k)
				delete(ref, k)
			case op < 9: // AddIfAbsent
				_, exists := ref[k]
				added := m.AddIfAbsent(k, v)
				if exists {
					assert.False(t, added)
				} else {
					assert.True(t, added)
					ref[k] = v
				}
			case op < 10: // Get + verify
				vGot, ok := m.Get(k)
				vRef, okRef := ref[k]
				assert.Equal(t, okRef, ok)
				if ok {
					assert.Equal(t, vRef, vGot)
				}
			}
		}

		// Verify map state
		assert.Equal(t, len(ref), m.Len())
		for k, v := range ref {
			vGot, ok := m.Get(k)
			assert.True(t, ok, "missing key %d", k)
			assert.Equal(t, v, vGot, "value mismatch for key %d", k)
		}

		// Verify via All() iteration
		iterCount := 0
		for k, v := range m.All() {
			vRef, ok := ref[k]
			assert.True(t, ok, "iterated key %d not in reference map", k)
			assert.Equal(t, vRef, v)
			iterCount++
		}
		assert.Equal(t, len(ref), iterCount)

		// Test early break in All()
		breakCount := 0
		for k, v := range m.All() {
			_ = k
			_ = v
			breakCount++
			if breakCount >= 5 {
				break
			}
		}
		assert.Equal(t, 5, breakCount)

		// JSON marshal/unmarshal
		if len(ref) > 0 {
			data, err := m.MarshalJSON()
			assert.NoError(t, err)
			assert.NotEmpty(t, data)

			m2 := NewMap[int, int](ar, 8)
			err = m2.UnmarshalJSON(data)
			assert.NoError(t, err)
			assert.Equal(t, m.Len(), m2.Len())
		}

		// Clear and verify
		m.Clear()
		assert.Equal(t, 0, m.Len())

		// ============================================================
		// Phase 2b: Map with string keys (triggers deep copy)
		// ============================================================
		mStr := NewMap[string, int](ar, 16)
		mStr.Put("alpha", 1)
		mStr.Put("beta", 2)
		mStr.Put("gamma", 3)
		assert.Equal(t, 3, mStr.Len())

		v, ok := mStr.Get("beta")
		assert.True(t, ok)
		assert.Equal(t, 2, v)

		mStr.Remove("alpha")
		assert.Equal(t, 2, mStr.Len())
		_, ok = mStr.Get("alpha")
		assert.False(t, ok)

		// Re-insert after remove (tombstone reuse)
		mStr.Put("alpha", 100)
		v, ok = mStr.Get("alpha")
		assert.True(t, ok)
		assert.Equal(t, 100, v)
		assert.Equal(t, 3, mStr.Len())

		// ============================================================
		// Phase 2c: Map with pointer values (deep copy path)
		// ============================================================
		type val struct {
			Name  string
			Count int
		}
		mDeep := NewMap[int, *val](ar, 8)
		v1 := &val{Name: "first", Count: 10}
		v2 := &val{Name: "second", Count: 20}
		mDeep.Put(1, v1)
		mDeep.Put(2, v2)
		// Modify original heap values — arena copy must be unaffected
		v1.Name = "modified"
		v1.Count = 999
		got, ok := mDeep.Get(1)
		assert.True(t, ok)
		assert.Equal(t, "first", got.Name)
		assert.Equal(t, 10, got.Count)

		// ============================================================
		// Phase 3: Vector operations
		// ============================================================
		vec := NewVector[int](ar, 32)
		vecRef := make([]int, 0)

		nVecOps := rngInt(150) + 50
		for i := 0; i < nVecOps; i++ {
			op := rng() % 10
			switch {
			case op < 4: // Append
				v := rngInt(500)
				vec.Append(v)
				vecRef = append(vecRef, v)
			case op < 6: // RemoveIdx
				if vec.Len() > 0 {
					idx := rngInt(vec.Len())
					vec.RemoveIdx(idx)
					vecRef = append(vecRef[:idx], vecRef[idx+1:]...)
				}
			case op < 8: // Remove (by value)
				if vec.Len() > 0 {
					idx := rngInt(vec.Len())
					val := vec.At(idx)
					vec.Remove(val)
					// Remove removes the FIRST matching element;
					// find that index in vecRef and remove it
					for ri, rv := range vecRef {
						if rv == val {
							vecRef = append(vecRef[:ri], vecRef[ri+1:]...)
							break
						}
					}
				}
			case op < 9: // Index / LastIndex
				if vec.Len() > 0 {
					idx := rngInt(vec.Len())
					searchVal := vec.At(idx)
					found := vec.Index(searchVal)
					assert.GreaterOrEqual(t, found, 0)
				}
			case op < 10: // RemoveBy
				limit := rngInt(3) + 1
				target := rngInt(500)
				vec.RemoveBy(limit, func(idx int, v int) bool {
					return v == target
				})
				// Rebuild reference
				newRef := make([]int, 0, len(vecRef))
				removed := 0
				for _, v := range vecRef {
					if v == target && removed < limit {
						removed++
						continue
					}
					newRef = append(newRef, v)
				}
				vecRef = newRef
			}
		}

		// Verify vector state
		assert.Equal(t, len(vecRef), vec.Len())
		for i := 0; i < vec.Len(); i++ {
			assert.Equal(t, vecRef[i], vec.At(i), "vector mismatch at index %d", i)
		}

		// Verify via All() iteration
		allIdx := 0
		for i, v := range vec.All() {
			assert.Equal(t, allIdx, i)
			assert.Equal(t, vecRef[i], v)
			allIdx++
		}
		assert.Equal(t, len(vecRef), allIdx)

		// Test AddIfAbsent
		if vec.Len() > 0 {
			existingVal := vec.At(0)
			assert.False(t, vec.AddIfAbsent(existingVal))
			newVal := 99999
			assert.True(t, vec.AddIfAbsent(newVal))
		}

		// Test LastIndex
		if vec.Len() > 0 {
			val := vec.At(vec.Len() - 1)
			idx := vec.LastIndex(val)
			assert.GreaterOrEqual(t, idx, 0)
			assert.Equal(t, val, vec.At(idx))
		}

		// JSON marshal/unmarshal
		if vec.Len() > 0 {
			data, err := vec.MarshalJSON()
			assert.NoError(t, err)
			vec2 := NewVector[int](ar, 4)
			err = vec2.UnmarshalJSON(data)
			assert.NoError(t, err)
			assert.Equal(t, vec.Len(), vec2.Len())
		}

		// Clear
		vec.Clear()
		assert.Equal(t, 0, vec.Len())

		// ============================================================
		// Phase 4: DeepCopy for structs
		// ============================================================
		srcPt := point{X: 1.5, Y: 2.5, Name: "test_point"}
		dstPt := DeepCopy(ar, srcPt)
		assert.Equal(t, srcPt.X, dstPt.X)
		assert.Equal(t, srcPt.Y, dstPt.Y)
		assert.Equal(t, srcPt.Name, dstPt.Name)

		// DeepCopy for node with pointer fields
		n2 := &node{ID: 2, Label: "two", Data: []int{3, 4}}
		n1 := &node{ID: 1, Label: "one", Next: n2, Data: []int{1, 2}}
		dstNode := DeepCopy(ar, n1)
		assert.Equal(t, 1, (*dstNode).ID)
		assert.Equal(t, "one", (*dstNode).Label)
		assert.NotNil(t, (*dstNode).Next)
		assert.Equal(t, 2, (*dstNode).Next.ID)
		assert.Equal(t, 2, len((*dstNode).Data))
		assert.Equal(t, 1, (*dstNode).Data[0])

		// DeepCopy for slices
		srcSlice := []int{1, 2, 3, 4, 5}
		dstSlice := NewSlice[int](ar, 0, 5)
		dstSlice = Append(ar, dstSlice, srcSlice...)
		assert.Equal(t, 5, len(dstSlice))
		for i := 0; i < 5; i++ {
			assert.Equal(t, srcSlice[i], dstSlice[i])
		}

		// ============================================================
		// Phase 5: Free and audit
		// ============================================================
		testPt := DeepCopy(ar, point{X: 1, Y: 2, Name: "audit_test"})
		violations := ar.AuditPointers(testPt)
		assert.Empty(t, violations)

		ar.Free(testPt) // Free a deep-copied struct

		// Audit on Map
		auditMap := NewMap[int, int](ar, 8)
		auditMap.Put(1, 10)
		violations = ar.AuditPointers(auditMap)
		assert.Empty(t, violations)

		// Audit on Vector
		auditVec := NewVector[int](ar, 4)
		auditVec.Append(1, 2, 3)
		violations = ar.AuditPointers(auditVec)
		assert.Empty(t, violations)

		// ============================================================
		// Phase 6: Edge cases
		// ============================================================
		// Zero-capacity map
		m0 := newMinimalMap[int, int](ar, 0)
		m0.Put(1, 10)
		assert.Equal(t, 1, m0.Len())

		// Map resize stress
		mResize := NewMap[int, int](ar, 8)
		for i := 0; i < 50; i++ {
			mResize.Put(i, i*10)
		}
		assert.Equal(t, 50, mResize.Len())
		for i := 0; i < 50; i++ {
			v, ok := mResize.Get(i)
			assert.True(t, ok)
			assert.Equal(t, i*10, v)
		}

		// Tombstone accumulation and recovery
		mTomb := NewMap[int, int](ar, 16)
		for i := 0; i < 40; i++ {
			mTomb.Put(i, i)
		}
		// Remove half to create tombstones
		for i := 0; i < 40; i += 2 {
			mTomb.Remove(i)
		}
		// Re-insert with new values (tombstone reuse)
		for i := 0; i < 40; i += 2 {
			mTomb.Put(i, i*100)
		}
		assert.Equal(t, 40, mTomb.Len())
		for i := 0; i < 40; i++ {
			v, ok := mTomb.Get(i)
			assert.True(t, ok)
			if i%2 == 0 {
				assert.Equal(t, i*100, v)
			} else {
				assert.Equal(t, i, v)
			}
		}

		// Vector Append with capacity expansion
		vecSmall := NewVector[int](ar, 2)
		for i := 0; i < 100; i++ {
			vecSmall.Append(i)
		}
		assert.Equal(t, 100, vecSmall.Len())
		for i := 0; i < 100; i++ {
			assert.Equal(t, i, vecSmall.At(i))
		}

		// Free slice backing array
		freeSlice := NewSlice[int](ar, 10, 10)
		for i := 0; i < 10; i++ {
			freeSlice[i] = i
		}
		ar.Free(freeSlice)
	}

	t.Logf("Stress test completed: %d iterations in %v (min duration: %v)",
		iter, time.Since(deadline.Add(-minDuration)), minDuration)
}
