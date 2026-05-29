package arena

import (
	"testing"
	"unsafe"
)

// ============================================================================
// Types for DeepCopy benchmarks
// ============================================================================

// 1. Single value — measures per-call overhead (visited map, reflect)
type flatValue struct {
	X int32
}

// 2. 3-field pure value struct — measures reflect per-field walk
type flatStruct3 struct {
	X, Y, Z int32
}

// 3. Single string — measures string deep copy cost
type stringStruct struct {
	Name string
}

// 4. Single pointer — measures pointer allocation + copy
type ptrStruct struct {
	Val *int32
}

// 5. Slice of int32 — measures slice allocation + element copy
type sliceIntStruct struct {
	Items []int32
}

// 6. Slice of string — measures slice + per-element string copy
type sliceStrStruct struct {
	Items []string
}

// 7. Nested 2 levels
type nestedL1 struct {
	Name  string
	Child *nestedL2
}
type nestedL2 struct {
	Value int32
	Tag   string
}

// 8. Nested 3 levels
type deepOuter struct {
	Middle *deepMiddle
}
type deepMiddle struct {
	Inner *deepInner
}
type deepInner struct {
	Value int32
}

// 9. 20-field pure value struct — measures per-field reflect scaling
type wideStruct struct {
	F0, F1, F2, F3, F4      int32
	F5, F6, F7, F8, F9      int32
	F10, F11, F12, F13, F14 int32
	F15, F16, F17, F18, F19 int32
}

// 10. Circular reference (linked list)
type linkedNode struct {
	Value int32
	Next  *linkedNode
}

// 11. Mixed struct — string + pointer + slice + nested
type mixedStruct struct {
	Name   string
	Age    *int32
	Tags   []string
	Detail *nestedL2
}

// 12. Deeply nested with strings at each level
type treeL3 struct {
	Label string
	Value int32
}
type treeL2 struct {
	Label string
	Left  *treeL3
	Right *treeL3
}
type treeL1 struct {
	Root *treeL2
}

// ============================================================================
// Helpers
// ============================================================================

func manualStringCopy(ar *Arena, s string) string {
	if len(s) == 0 {
		return ""
	}
	b := ar.Bytes([]byte(s))
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// ============================================================================
// 1. flatValue — single int32
// ============================================================================

func BenchmarkDeepCopy_FlatValue(b *testing.B) {
	src := flatValue{X: 42}

	b.Run("DeepCopy", func(b *testing.B) {
		ar := NewArena()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = DeepCopy(ar, src)
		}
	})

	b.Run("ManualCopy", func(b *testing.B) {
		ar := NewArena()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p := New[flatValue](ar)
			p.X = src.X
			_ = p
		}
	})
}

// ============================================================================
// 2. flatStruct3 — 3 int32 fields, no pointers/strings/slices
// ============================================================================

func BenchmarkDeepCopy_FlatStruct3(b *testing.B) {
	src := flatStruct3{X: 1, Y: 2, Z: 3}

	b.Run("DeepCopy", func(b *testing.B) {
		ar := NewArena()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = DeepCopy(ar, src)
		}
	})

	b.Run("ManualCopy", func(b *testing.B) {
		ar := NewArena()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p := New[flatStruct3](ar)
			p.X = src.X
			p.Y = src.Y
			p.Z = src.Z
			_ = p
		}
	})
}

// ============================================================================
// 3. stringStruct — single string field
// ============================================================================

func BenchmarkDeepCopy_StringStruct(b *testing.B) {
	src := stringStruct{Name: "hello arena benchmark string"}

	b.Run("DeepCopy", func(b *testing.B) {
		ar := NewArena()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = DeepCopy(ar, src)
		}
	})

	b.Run("ManualCopy", func(b *testing.B) {
		ar := NewArena()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p := New[stringStruct](ar)
			p.Name = manualStringCopy(ar, src.Name)
			_ = p
		}
	})
}

// ============================================================================
// 4. ptrStruct — single *int32 field
// ============================================================================

func BenchmarkDeepCopy_PtrStruct(b *testing.B) {
	val := int32(42)
	src := ptrStruct{Val: &val}

	b.Run("DeepCopy", func(b *testing.B) {
		ar := NewArena()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = DeepCopy(ar, src)
		}
	})

	b.Run("ManualCopy", func(b *testing.B) {
		ar := NewArena()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p := New[ptrStruct](ar)
			p.Val = New[int32](ar)
			*p.Val = *src.Val
			_ = p
		}
	})
}

// ============================================================================
// 5. sliceIntStruct — []int32 field
// ============================================================================

func BenchmarkDeepCopy_SliceIntStruct(b *testing.B) {
	src := sliceIntStruct{Items: []int32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}}

	b.Run("DeepCopy", func(b *testing.B) {
		ar := NewArena()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = DeepCopy(ar, src)
		}
	})

	b.Run("ManualCopy", func(b *testing.B) {
		ar := NewArena()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p := New[sliceIntStruct](ar)
			p.Items = NewSlice[int32](ar, len(src.Items), cap(src.Items))
			copy(p.Items, src.Items)
			_ = p
		}
	})
}

// ============================================================================
// 6. sliceStrStruct — []string field
// ============================================================================

func BenchmarkDeepCopy_SliceStrStruct(b *testing.B) {
	src := sliceStrStruct{
		Items: []string{"alpha", "bravo", "charlie", "delta", "echo"},
	}

	b.Run("DeepCopy", func(b *testing.B) {
		ar := NewArena()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = DeepCopy(ar, src)
		}
	})

	b.Run("ManualCopy", func(b *testing.B) {
		ar := NewArena()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p := New[sliceStrStruct](ar)
			p.Items = NewSlice[string](ar, len(src.Items), cap(src.Items))
			for j, s := range src.Items {
				p.Items[j] = manualStringCopy(ar, s)
			}
			_ = p
		}
	})
}

// ============================================================================
// 7. nestedL1 — 2-level nesting
// ============================================================================

func BenchmarkDeepCopy_Nested2(b *testing.B) {
	v := int32(42)
	src := nestedL1{
		Name: "level one",
		Child: &nestedL2{
			Value: 42,
			Tag:   "level two",
		},
	}

	b.Run("DeepCopy", func(b *testing.B) {
		ar := NewArena()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = DeepCopy(ar, src)
		}
	})

	b.Run("ManualCopy", func(b *testing.B) {
		_ = v
		ar := NewArena()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p := New[nestedL1](ar)
			p.Name = manualStringCopy(ar, src.Name)
			p.Child = New[nestedL2](ar)
			p.Child.Value = src.Child.Value
			p.Child.Tag = manualStringCopy(ar, src.Child.Tag)
			_ = p
		}
	})
}

// ============================================================================
// 8. deepOuter — 3-level nesting
// ============================================================================

func BenchmarkDeepCopy_Nested3(b *testing.B) {
	src := deepOuter{
		Middle: &deepMiddle{
			Inner: &deepInner{Value: 99},
		},
	}

	b.Run("DeepCopy", func(b *testing.B) {
		ar := NewArena()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = DeepCopy(ar, src)
		}
	})

	b.Run("ManualCopy", func(b *testing.B) {
		ar := NewArena()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p := New[deepOuter](ar)
			p.Middle = New[deepMiddle](ar)
			p.Middle.Inner = New[deepInner](ar)
			p.Middle.Inner.Value = src.Middle.Inner.Value
			_ = p
		}
	})
}

// ============================================================================
// 9. wideStruct — 20 int32 fields
// ============================================================================

func BenchmarkDeepCopy_WideStruct(b *testing.B) {
	src := wideStruct{
		F0: 1, F1: 2, F2: 3, F3: 4, F4: 5,
		F5: 6, F6: 7, F7: 8, F8: 9, F9: 10,
		F10: 11, F11: 12, F12: 13, F13: 14, F14: 15,
		F15: 16, F16: 17, F17: 18, F18: 19, F19: 20,
	}

	b.Run("DeepCopy", func(b *testing.B) {
		ar := NewArena()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = DeepCopy(ar, src)
		}
	})

	b.Run("ManualCopy", func(b *testing.B) {
		ar := NewArena()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p := New[wideStruct](ar)
			p.F0 = src.F0
			p.F1 = src.F1
			p.F2 = src.F2
			p.F3 = src.F3
			p.F4 = src.F4
			p.F5 = src.F5
			p.F6 = src.F6
			p.F7 = src.F7
			p.F8 = src.F8
			p.F9 = src.F9
			p.F10 = src.F10
			p.F11 = src.F11
			p.F12 = src.F12
			p.F13 = src.F13
			p.F14 = src.F14
			p.F15 = src.F15
			p.F16 = src.F16
			p.F17 = src.F17
			p.F18 = src.F18
			p.F19 = src.F19
			_ = p
		}
	})
}

// ============================================================================
// 10. linkedNode — circular reference
// ============================================================================

func BenchmarkDeepCopy_LinkedList(b *testing.B) {
	// Build a 10-element linked list
	head := &linkedNode{Value: 0}
	cur := head
	for i := 1; i < 10; i++ {
		cur.Next = &linkedNode{Value: int32(i)}
		cur = cur.Next
	}

	b.Run("DeepCopy", func(b *testing.B) {
		ar := NewArena()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = DeepCopy(ar, head)
		}
	})

	b.Run("ManualCopy", func(b *testing.B) {
		ar := NewArena()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			newHead := New[linkedNode](ar)
			newHead.Value = head.Value
			dst := newHead
			for src := head.Next; src != nil; src = src.Next {
				dst.Next = New[linkedNode](ar)
				dst.Next.Value = src.Value
				dst = dst.Next
			}
			_ = newHead
		}
	})
}

// ============================================================================
// 11. mixedStruct — string + pointer + slice + nested
// ============================================================================

func BenchmarkDeepCopy_MixedStruct(b *testing.B) {
	age := int32(30)
	src := mixedStruct{
		Name: "benchmark user with a long name",
		Age:  &age,
		Tags: []string{"go", "arena", "performance"},
		Detail: &nestedL2{
			Value: 42,
			Tag:   "nested detail",
		},
	}

	b.Run("DeepCopy", func(b *testing.B) {
		ar := NewArena()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = DeepCopy(ar, src)
		}
	})

	b.Run("ManualCopy", func(b *testing.B) {
		_ = age
		ar := NewArena()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p := New[mixedStruct](ar)
			p.Name = manualStringCopy(ar, src.Name)
			p.Age = New[int32](ar)
			*p.Age = *src.Age
			p.Tags = NewSlice[string](ar, len(src.Tags), cap(src.Tags))
			for j, s := range src.Tags {
				p.Tags[j] = manualStringCopy(ar, s)
			}
			p.Detail = New[nestedL2](ar)
			p.Detail.Value = src.Detail.Value
			p.Detail.Tag = manualStringCopy(ar, src.Detail.Tag)
			_ = p
		}
	})
}

// ============================================================================
// 12. treeL1 — 3-level tree with strings at each node
// ============================================================================

func BenchmarkDeepCopy_TreeStruct(b *testing.B) {
	src := treeL1{
		Root: &treeL2{
			Label: "root",
			Left:  &treeL3{Label: "left child", Value: 10},
			Right: &treeL3{Label: "right child", Value: 20},
		},
	}

	b.Run("DeepCopy", func(b *testing.B) {
		ar := NewArena()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = DeepCopy(ar, src)
		}
	})

	b.Run("ManualCopy", func(b *testing.B) {
		ar := NewArena()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p := New[treeL1](ar)
			p.Root = New[treeL2](ar)
			p.Root.Label = manualStringCopy(ar, src.Root.Label)
			p.Root.Left = New[treeL3](ar)
			p.Root.Left.Label = manualStringCopy(ar, src.Root.Left.Label)
			p.Root.Left.Value = src.Root.Left.Value
			p.Root.Right = New[treeL3](ar)
			p.Root.Right.Label = manualStringCopy(ar, src.Root.Right.Label)
			p.Root.Right.Value = src.Root.Right.Value
			_ = p
		}
	})
}
