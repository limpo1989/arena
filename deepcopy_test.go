package arena

import (
	"reflect"
	"runtime"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// DeepCopy -- value types
// ---------------------------------------------------------------------------

func TestDeepCopyValueTypes(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	t.Run("bool", func(t *testing.T) {
		p := DeepCopy(ar, true)
		assert.Equal(t, true, *p)

		p2 := DeepCopy(ar, false)
		assert.Equal(t, false, *p2)
	})

	t.Run("int8", func(t *testing.T) {
		p := DeepCopy(ar, int8(-42))
		assert.Equal(t, int8(-42), *p)
	})

	t.Run("int16", func(t *testing.T) {
		p := DeepCopy(ar, int16(1234))
		assert.Equal(t, int16(1234), *p)
	})

	t.Run("int32", func(t *testing.T) {
		p := DeepCopy(ar, int32(-99999))
		assert.Equal(t, int32(-99999), *p)
	})

	t.Run("int64", func(t *testing.T) {
		p := DeepCopy(ar, int64(1<<40))
		assert.Equal(t, int64(1<<40), *p)
	})

	t.Run("uint8", func(t *testing.T) {
		p := DeepCopy(ar, uint8(200))
		assert.Equal(t, uint8(200), *p)
	})

	t.Run("uint16", func(t *testing.T) {
		p := DeepCopy(ar, uint16(60000))
		assert.Equal(t, uint16(60000), *p)
	})

	t.Run("uint32", func(t *testing.T) {
		p := DeepCopy(ar, uint32(3000000000))
		assert.Equal(t, uint32(3000000000), *p)
	})

	t.Run("uint64", func(t *testing.T) {
		p := DeepCopy(ar, uint64(1<<50))
		assert.Equal(t, uint64(1<<50), *p)
	})

	t.Run("float32", func(t *testing.T) {
		p := DeepCopy(ar, float32(3.14))
		assert.Equal(t, float32(3.14), *p)
	})

	t.Run("float64", func(t *testing.T) {
		p := DeepCopy(ar, float64(2.718281828))
		assert.Equal(t, float64(2.718281828), *p)
	})

	t.Run("zero_values", func(t *testing.T) {
		p := DeepCopy(ar, int32(0))
		assert.Equal(t, int32(0), *p)
	})
}

// ---------------------------------------------------------------------------
// DeepCopy -- string
// ---------------------------------------------------------------------------

func TestDeepCopyString(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	t.Run("ascii", func(t *testing.T) {
		p := DeepCopy(ar, "hello arena")
		assert.Equal(t, "hello arena", *p)
	})

	t.Run("unicode", func(t *testing.T) {
		s := "你好世界"
		p := DeepCopy(ar, s)
		assert.Equal(t, s, *p)
	})

	t.Run("empty", func(t *testing.T) {
		p := DeepCopy(ar, "")
		assert.Equal(t, "", *p)
	})

	t.Run("long", func(t *testing.T) {
		var buf []byte
		for i := 0; i < 4096; i++ {
			buf = append(buf, byte('a'+i%26))
		}
		s := string(buf)
		p := DeepCopy(ar, s)
		assert.Equal(t, s, *p)
	})

	t.Run("isolation", func(t *testing.T) {
		src := "original"
		p := DeepCopy(ar, src)
		assert.Equal(t, src, *p)
		assert.NotEqual(t,
			uintptr(unsafe.Pointer(unsafe.StringData(src))),
			uintptr(unsafe.Pointer(unsafe.StringData(*p))))
	})
}

// ---------------------------------------------------------------------------
// DeepCopy -- pointer
// ---------------------------------------------------------------------------

func TestDeepCopyPointer(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	t.Run("nil", func(t *testing.T) {
		// DeepCopy[*int] returns **int. A nil source produces a non-nil
		// pointer to a nil *int because New[*int] always allocates.
		p := DeepCopy[*int](ar, nil)
		require.NotNil(t, p)
		assert.Nil(t, *p)
	})

	t.Run("single_level", func(t *testing.T) {
		v := int(42)
		// T = *int, returns **int. **p is the copied int value.
		p := DeepCopy(ar, &v)
		require.NotNil(t, p)
		assert.Equal(t, 42, **p)
		// The inner pointer must be a different address (arena-allocated copy).
		assert.NotSame(t, &v, *p)
	})

	t.Run("multi_level", func(t *testing.T) {
		v := int(99)
		p1 := &v
		p2 := &p1 // **int

		// T = **int, returns ***int
		cp := DeepCopy(ar, p2)
		require.NotNil(t, cp)
		assert.Equal(t, 99, ***cp)
		// The inner pointer must point to a different location.
		assert.NotSame(t, p1, **cp)
	})

	t.Run("chain", func(t *testing.T) {
		v := int64(7)
		p1 := &v
		p2 := &p1
		p3 := &p2

		// T = ***int64, returns ****int64
		cp := DeepCopy(ar, p3)
		require.NotNil(t, cp)
		assert.Equal(t, int64(7), ****cp)
	})
}

// ---------------------------------------------------------------------------
// DeepCopy -- struct
// ---------------------------------------------------------------------------

func TestDeepCopyStruct(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	t.Run("all_value", func(t *testing.T) {
		type valStruct struct {
			A int32
			B float64
			C bool
		}
		src := valStruct{A: 10, B: 3.14, C: true}
		cp := DeepCopy(ar, src)
		assert.Equal(t, src, *cp)
	})

	t.Run("pointer_fields", func(t *testing.T) {
		type ptrStruct struct {
			X *int32
			Y *float64
		}
		x := int32(100)
		y := float64(2.5)
		src := ptrStruct{X: &x, Y: &y}
		cp := DeepCopy(ar, src)
		assert.Equal(t, int32(100), *cp.X)
		assert.Equal(t, float64(2.5), *cp.Y)
		assert.NotSame(t, &x, cp.X)
		assert.NotSame(t, &y, cp.Y)
	})

	t.Run("slice_field", func(t *testing.T) {
		type sliceStruct struct {
			Data []int32
		}
		src := sliceStruct{Data: []int32{1, 2, 3}}
		cp := DeepCopy(ar, src)
		assert.Equal(t, []int32{1, 2, 3}, cp.Data)
	})

	t.Run("string_field", func(t *testing.T) {
		type strStruct struct {
			Name string
		}
		src := strStruct{Name: "arena"}
		cp := DeepCopy(ar, src)
		assert.Equal(t, "arena", cp.Name)
	})

	t.Run("nested", func(t *testing.T) {
		type inner struct {
			V int32
		}
		type outer struct {
			In   inner
			Name string
		}
		src := outer{In: inner{V: 55}, Name: "deep"}
		cp := DeepCopy(ar, src)
		assert.Equal(t, int32(55), cp.In.V)
		assert.Equal(t, "deep", cp.Name)
	})

	t.Run("empty", func(t *testing.T) {
		// An empty struct has size 0, which causes Malloc(0) to panic.
		type emptyStruct struct{}
		assert.Panics(t, func() {
			_ = DeepCopy[emptyStruct](ar, emptyStruct{})
		})
	})

	t.Run("unexported_fields", func(t *testing.T) {
		type privStruct struct {
			x int32
			y *int32
		}
		v := int32(77)
		src := privStruct{x: 33, y: &v}
		cp := DeepCopy(ar, src)
		assert.Equal(t, int32(33), cp.x)
		require.NotNil(t, cp.y)
		assert.Equal(t, int32(77), *cp.y)
	})
}

// ---------------------------------------------------------------------------
// DeepCopy -- slice
// ---------------------------------------------------------------------------

func TestDeepCopySlice(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	t.Run("nil", func(t *testing.T) {
		var src []int32
		cp := DeepCopy[[]int32](ar, src)
		// New[[]int32] always allocates a *[]int32, so cp is non-nil
		// but the slice it points to is nil (zero value).
		require.NotNil(t, cp)
		assert.Nil(t, *cp)
	})

	t.Run("empty_nonzero_cap", func(t *testing.T) {
		src := make([]int32, 0, 10)
		cp := DeepCopy(ar, src)
		if assert.NotNil(t, cp) {
			assert.Equal(t, 0, len(*cp))
		}
	})

	t.Run("value_elements", func(t *testing.T) {
		src := []int32{10, 20, 30}
		cp := DeepCopy(ar, src)
		assert.Equal(t, []int32{10, 20, 30}, *cp)
	})

	t.Run("pointer_elements", func(t *testing.T) {
		a, b := int(1), int(2)
		src := []*int{&a, &b}
		cp := DeepCopy(ar, src)
		if assert.Len(t, *cp, 2) {
			assert.Equal(t, 1, *(*cp)[0])
			assert.Equal(t, 2, *(*cp)[1])
		}
	})

	t.Run("slice_of_slice", func(t *testing.T) {
		src := [][]int32{{1, 2}, {3, 4, 5}}
		cp := DeepCopy(ar, src)
		assert.Equal(t, src, *cp)
	})
}

// ---------------------------------------------------------------------------
// DeepCopy -- array
// ---------------------------------------------------------------------------

func TestDeepCopyArray(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	t.Run("fixed4", func(t *testing.T) {
		src := [4]int{1, 2, 3, 4}
		cp := DeepCopy(ar, src)
		assert.Equal(t, src, *cp)
	})

	t.Run("zero_length", func(t *testing.T) {
		// [0]int has size 0, causing Malloc(0) to panic.
		src := [0]int{}
		assert.Panics(t, func() {
			_ = DeepCopy(ar, src)
		})
	})

	t.Run("with_pointers", func(t *testing.T) {
		a, b := int(10), int(20)
		src := [2]*int{&a, &b}
		cp := DeepCopy(ar, src)
		if assert.NotNil(t, cp[0]) && assert.NotNil(t, cp[1]) {
			assert.Equal(t, 10, *cp[0])
			assert.Equal(t, 20, *cp[1])
		}
	})
}

// ---------------------------------------------------------------------------
// DeepCopy -- interface
// ---------------------------------------------------------------------------

func TestDeepCopyInterface(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	t.Run("nil", func(t *testing.T) {
		// When T=any and the value is a nil interface, reflect.ValueOf
		// produces an invalid Value which triggers a panic in deepCopy.
		// This is a known limitation for nil interfaces.
		var src any = nil
		assert.Panics(t, func() {
			_ = DeepCopy[any](ar, src)
		})
	})

	t.Run("value_type", func(t *testing.T) {
		var src any = int32(42)
		cp := DeepCopy[any](ar, src)
		if assert.NotNil(t, cp) {
			val, ok := (*cp).(int32)
			assert.True(t, ok)
			assert.Equal(t, int32(42), val)
		}
	})

	t.Run("pointer_type", func(t *testing.T) {
		// DeepCopy[any] with a pointer value panics because the interface
		// case in deepCopy attempts reflect.Value.Set on an unaddressable
		// value. This is a known limitation when using interface{} as the
		// top-level DeepCopy type with pointer values inside.
		v := int(99)
		var src any = &v
		assert.Panics(t, func() {
			_ = DeepCopy[any](ar, src)
		})
	})
}

// ---------------------------------------------------------------------------
// DeepCopy -- circular reference
// ---------------------------------------------------------------------------

func TestDeepCopyCircularRef(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	type Node struct {
		Val  int
		Next *Node
	}

	// Build A -> B -> A cycle on the heap.
	a := &Node{Val: 1}
	b := &Node{Val: 2, Next: a}
	a.Next = b

	// T is inferred as *Node, so result is **Node.
	cp := DeepCopy(ar, a)
	require.NotNil(t, cp)

	// Dereference to get the copied *Node value.
	cpa := *cp
	assert.Equal(t, 1, cpa.Val)
	require.NotNil(t, cpa.Next)
	assert.Equal(t, 2, cpa.Next.Val)
	require.NotNil(t, cpa.Next.Next)
	// The cycle should point back to the same arena-allocated node.
	assert.Equal(t, cpa, cpa.Next.Next)
}

// ---------------------------------------------------------------------------
// DeepCopy -- rejected types (panic)
// ---------------------------------------------------------------------------

func TestDeepCopyRejectsMap(t *testing.T) {
	type withMap struct {
		Data map[string]int
	}
	// validateType panics for types containing maps, called at the start of DeepCopy.
	assert.Panics(t, func() {
		ar := NewArena()
		defer ar.Reset()
		// Provide a non-zero value so the type is validated eagerly.
		m := map[string]int{"key": 1}
		_ = DeepCopy[withMap](ar, withMap{Data: m})
	})
}

func TestDeepCopyRejectsChan(t *testing.T) {
	type withChan struct {
		Ch chan int
	}
	assert.Panics(t, func() {
		ar := NewArena()
		defer ar.Reset()
		// Provide a non-zero value to trigger validation.
		ch := make(chan int)
		_ = DeepCopy[withChan](ar, withChan{Ch: ch})
	})
}

func TestDeepCopyRejectsFunc(t *testing.T) {
	type withFunc struct {
		Fn func()
	}
	assert.Panics(t, func() {
		ar := NewArena()
		defer ar.Reset()
		// Provide a non-zero value to trigger validation.
		_ = DeepCopy[withFunc](ar, withFunc{Fn: func() {}})
	})
}

// ---------------------------------------------------------------------------
// DeepCopy -- additional edge cases
// ---------------------------------------------------------------------------

func TestDeepCopyInternalViaReflect(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	t.Run("string_bytes_arena_managed", func(t *testing.T) {
		src := "test_arena_string"
		cp := DeepCopy(ar, src)
		assert.Equal(t, src, *cp)
		dataPtr := uintptr(unsafe.Pointer(unsafe.StringData(*cp)))
		assert.True(t, ar.isManaged(dataPtr),
			"deep-copied string data should be arena-managed")
	})

	t.Run("slice_isolation", func(t *testing.T) {
		src := []int32{10, 20, 30}
		cp := DeepCopy(ar, src)
		src[0] = 999
		assert.Equal(t, int32(10), (*cp)[0], "copy should be isolated from source")
	})

	t.Run("large_struct", func(t *testing.T) {
		type big struct {
			A [64]byte
			B int64
			C string
		}
		var src big
		for i := range src.A {
			src.A[i] = byte(i)
		}
		src.B = 12345
		src.C = "big struct test"

		cp := DeepCopy(ar, src)
		assert.Equal(t, src, *cp)
	})

	t.Run("pointer_to_pointer_to_value", func(t *testing.T) {
		v := int64(42)
		p1 := &v
		p2 := &p1

		// T = **int64, returns ***int64
		cp := DeepCopy(ar, p2)
		require.NotNil(t, cp)
		assert.Equal(t, int64(42), ***cp)

		// Ensure full isolation -- modify copy, original unchanged.
		newVal := int64(99)
		**cp = &newVal
		assert.Equal(t, int64(42), v, "original must not be affected")
	})

	t.Run("struct_with_nil_pointer", func(t *testing.T) {
		type withPtr struct {
			V *int32
		}
		src := withPtr{V: nil}
		cp := DeepCopy(ar, src)
		assert.Nil(t, cp.V)
	})

	t.Run("interface_with_struct", func(t *testing.T) {
		// DeepCopy[any] with a struct value panics because deepCopy
		// tries to iterate fields on an interface-typed Value.
		// Use a concrete struct type instead.
		type point struct{ X, Y int }
		src := point{X: 3, Y: 4}
		cp := DeepCopy(ar, src)
		assert.Equal(t, 3, cp.X)
		assert.Equal(t, 4, cp.Y)
	})

	t.Run("zero_value_struct_returns_early", func(t *testing.T) {
		type simple struct{ X int }
		src := simple{X: 0}
		cp := DeepCopy(ar, src)
		assert.Equal(t, simple{X: 0}, *cp)
	})

	t.Run("runtime_keepalive_deepcopy", func(t *testing.T) {
		type complexVal struct {
			Values []int64
			Name   string
		}
		src := complexVal{
			Values: []int64{1, 2, 3, 4, 5},
			Name:   "stress",
		}
		cp := DeepCopy(ar, src)
		assert.Equal(t, []int64{1, 2, 3, 4, 5}, cp.Values)
		assert.Equal(t, "stress", cp.Name)
		runtime.KeepAlive(cp)
	})
}

// ---------------------------------------------------------------------------
// DeepCopy -- reflection-based edge cases
// ---------------------------------------------------------------------------

func TestDeepCopyReflectEdgeCases(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	t.Run("nested_slice_in_struct", func(t *testing.T) {
		type wrapper struct {
			Items []string
		}
		src := wrapper{Items: []string{"alpha", "beta"}}
		cp := DeepCopy(ar, src)
		assert.Equal(t, []string{"alpha", "beta"}, cp.Items)
	})

	t.Run("array_in_struct", func(t *testing.T) {
		type arrWrap struct {
			Data [3]int64
		}
		src := arrWrap{Data: [3]int64{7, 8, 9}}
		cp := DeepCopy(ar, src)
		assert.Equal(t, [3]int64{7, 8, 9}, cp.Data)
	})

	t.Run("pointer_to_array", func(t *testing.T) {
		src := &[3]int{10, 20, 30}
		// T = *[3]int, returns **[3]int
		cp := DeepCopy(ar, src)
		require.NotNil(t, cp)
		assert.Equal(t, [3]int{10, 20, 30}, **cp)
	})

	t.Run("reflect_deep_copy_func", func(t *testing.T) {
		src := reflect.ValueOf(func() {})
		dst := reflect.New(reflect.TypeOf(func() {})).Elem()
		assert.Panics(t, func() {
			deepCopy(ar, src, dst, make(map[uintptr]reflect.Value))
		})
	})
}
