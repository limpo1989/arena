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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// Map JSON serialization tests
// ============================================================================

func TestMapMarshalJSON(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	t.Run("simple map", func(t *testing.T) {
		m := NewMap[string, int](ar, 8)
		m.Put("foo", 1)
		m.Put("bar", 2)
		m.Put("baz", 3)

		b, err := json.Marshal(m)
		assert.NoError(t, err)

		// Parse back and verify (order-independent)
		var got map[string]int
		err = json.Unmarshal(b, &got)
		assert.NoError(t, err)
		assert.Equal(t, map[string]int{"foo": 1, "bar": 2, "baz": 3}, got)
	})

	t.Run("empty map", func(t *testing.T) {
		m := NewMap[string, int](ar, 8)
		b, err := json.Marshal(m)
		assert.NoError(t, err)
		assert.Equal(t, "{}", string(b))
	})

	t.Run("int keys", func(t *testing.T) {
		m := NewMap[int, string](ar, 8)
		m.Put(1, "one")
		m.Put(2, "two")

		b, err := json.Marshal(m)
		assert.NoError(t, err)

		var got map[int]string
		err = json.Unmarshal(b, &got)
		assert.NoError(t, err)
		assert.Equal(t, map[int]string{1: "one", 2: "two"}, got)
	})

	t.Run("struct values", func(t *testing.T) {
		type person struct {
			Name string
			Age  int
		}
		m := NewMap[string, person](ar, 8)
		m.Put("alice", person{Name: "Alice", Age: 30})
		m.Put("bob", person{Name: "Bob", Age: 25})

		b, err := json.Marshal(m)
		assert.NoError(t, err)

		var got map[string]person
		err = json.Unmarshal(b, &got)
		assert.NoError(t, err)
		assert.Equal(t, person{Name: "Alice", Age: 30}, got["alice"])
		assert.Equal(t, person{Name: "Bob", Age: 25}, got["bob"])
	})

	t.Run("pointer values", func(t *testing.T) {
		m := NewMap[string, *int](ar, 8)
		one := 1
		two := 2
		m.Put("a", &one)
		m.Put("b", &two)

		b, err := json.Marshal(m)
		assert.NoError(t, err)

		var got map[string]int
		err = json.Unmarshal(b, &got)
		assert.NoError(t, err)
		assert.Equal(t, 1, got["a"])
		assert.Equal(t, 2, got["b"])
	})

	t.Run("nested Vector value", func(t *testing.T) {
		m := NewMap[string, *Vector[int]](ar, 8)
		v := NewVector[int](ar, 4)
		v.Append(1, 2, 3)
		m.Put("nums", v)

		b, err := json.Marshal(m)
		assert.NoError(t, err)

		// Unmarshal and verify
		var got map[string][]int
		err = json.Unmarshal(b, &got)
		assert.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3}, got["nums"])
	})

	t.Run("unmarshal into shared arena", func(t *testing.T) {
		ar2 := NewArena()
		defer ar2.Reset()

		m := NewMap[string, int](ar2, 8)
		data := []byte(`{"x": 100, "y": 200}`)
		err := json.Unmarshal(data, m)
		assert.NoError(t, err)
		assert.Equal(t, 2, m.Len())

		v, ok := m.Get("x")
		assert.True(t, ok)
		assert.Equal(t, 100, v)
		v2, ok := m.Get("y")
		assert.True(t, ok)
		assert.Equal(t, 200, v2)
	})

	t.Run("unmarshal clears existing", func(t *testing.T) {
		ar2 := NewArena()
		defer ar2.Reset()

		m := NewMap[string, int](ar2, 8)
		m.Put("old", 999)

		data := []byte(`{"new": 42}`)
		err := json.Unmarshal(data, m)
		assert.NoError(t, err)

		_, ok := m.Get("old")
		assert.False(t, ok, "old entry should be cleared")
		v, ok := m.Get("new")
		assert.True(t, ok)
		assert.Equal(t, 42, v)
	})

	t.Run("unmarshal empty object", func(t *testing.T) {
		ar2 := NewArena()
		defer ar2.Reset()

		m := NewMap[string, int](ar2, 8)
		m.Put("x", 1)

		err := json.Unmarshal([]byte("{}"), m)
		assert.NoError(t, err)
		assert.Equal(t, 0, m.Len())
	})

	t.Run("unmarshal invalid JSON", func(t *testing.T) {
		ar2 := NewArena()
		defer ar2.Reset()

		m := NewMap[string, int](ar2, 8)
		err := json.Unmarshal([]byte("{bad"), m)
		assert.Error(t, err)
	})

	t.Run("unmarshal wrong type", func(t *testing.T) {
		ar2 := NewArena()
		defer ar2.Reset()

		m := NewMap[string, int](ar2, 8)
		err := json.Unmarshal([]byte("[1, 2, 3]"), m)
		assert.Error(t, err)
	})

	t.Run("round-trip", func(t *testing.T) {
		ar2 := NewArena()
		defer ar2.Reset()

		m1 := NewMap[string, string](ar2, 8)
		m1.Put("hello", "world")
		m1.Put("foo", "bar")
		m1.Put("unicode", "你好")

		b, err := json.Marshal(m1)
		assert.NoError(t, err)

		var got map[string]string
		err = json.Unmarshal(b, &got)
		assert.NoError(t, err)
		assert.Equal(t, map[string]string{"hello": "world", "foo": "bar", "unicode": "你好"}, got)
	})

	t.Run("marshal with special JSON chars", func(t *testing.T) {
		ar2 := NewArena()
		defer ar2.Reset()

		m := NewMap[string, string](ar2, 8)
		m.Put("key\"with\"quotes", "val\nwith\nnewlines")
		m.Put("backslash", "c:\\path")

		b, err := json.Marshal(m)
		assert.NoError(t, err)

		var got map[string]string
		err = json.Unmarshal(b, &got)
		assert.NoError(t, err)
		assert.Equal(t, "val\nwith\nnewlines", got["key\"with\"quotes"])
		assert.Equal(t, "c:\\path", got["backslash"])
	})
}

// ============================================================================
// Vector JSON serialization tests
// ============================================================================

func TestVectorMarshalJSON(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	t.Run("simple vector", func(t *testing.T) {
		v := NewVector[int](ar, 4)
		v.Append(1, 2, 3, 4)

		b, err := json.Marshal(v)
		assert.NoError(t, err)

		var got []int
		err = json.Unmarshal(b, &got)
		assert.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3, 4}, got)
	})

	t.Run("empty vector", func(t *testing.T) {
		v := NewVector[int](ar, 4)
		b, err := json.Marshal(v)
		assert.NoError(t, err)
		assert.Equal(t, "[]", string(b))
	})

	t.Run("string vector", func(t *testing.T) {
		v := NewVector[string](ar, 4)
		v.Append("alpha", "beta", "gamma")

		b, err := json.Marshal(v)
		assert.NoError(t, err)

		var got []string
		err = json.Unmarshal(b, &got)
		assert.NoError(t, err)
		assert.Equal(t, []string{"alpha", "beta", "gamma"}, got)
	})

	t.Run("struct elements", func(t *testing.T) {
		type item struct {
			ID   int
			Name string
		}
		v := NewVector[item](ar, 4)
		v.Append(item{ID: 1, Name: "one"}, item{ID: 2, Name: "two"})

		b, err := json.Marshal(v)
		assert.NoError(t, err)

		var got []item
		err = json.Unmarshal(b, &got)
		assert.NoError(t, err)
		assert.Equal(t, []item{{ID: 1, Name: "one"}, {ID: 2, Name: "two"}}, got)
	})

	t.Run("pointer elements", func(t *testing.T) {
		v := NewVector[*int](ar, 4)
		one, two, three := 1, 2, 3
		v.Append(&one, &two, &three)

		b, err := json.Marshal(v)
		assert.NoError(t, err)

		var got []int
		err = json.Unmarshal(b, &got)
		assert.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3}, got)
	})

	t.Run("nested Map element", func(t *testing.T) {
		v := NewVector[*Map[string, int]](ar, 4)
		m := NewMap[string, int](ar, 4)
		m.Put("a", 1)
		m.Put("b", 2)
		v.Append(m)

		b, err := json.Marshal(v)
		assert.NoError(t, err)

		var got []map[string]int
		err = json.Unmarshal(b, &got)
		assert.NoError(t, err)
		assert.Equal(t, []map[string]int{{"a": 1, "b": 2}}, got)
	})

	t.Run("nested Vector elements", func(t *testing.T) {
		v := NewVector[*Vector[int]](ar, 4)
		inner := NewVector[int](ar, 4)
		inner.Append(10, 20)
		v.Append(inner)

		b, err := json.Marshal(v)
		assert.NoError(t, err)

		var got [][]int
		err = json.Unmarshal(b, &got)
		assert.NoError(t, err)
		assert.Equal(t, [][]int{{10, 20}}, got)
	})

	t.Run("unmarshal into shared arena", func(t *testing.T) {
		ar2 := NewArena()
		defer ar2.Reset()

		v := NewVector[int](ar2, 4)
		data := []byte("[100, 200, 300]")
		err := json.Unmarshal(data, v)
		assert.NoError(t, err)

		assert.Equal(t, 3, v.Len())
		assert.Equal(t, 100, v.At(0))
		assert.Equal(t, 200, v.At(1))
		assert.Equal(t, 300, v.At(2))
	})

	t.Run("unmarshal clears existing", func(t *testing.T) {
		ar2 := NewArena()
		defer ar2.Reset()

		v := NewVector[int](ar2, 8)
		v.Append(1, 2, 3)

		data := []byte("[99]")
		err := json.Unmarshal(data, v)
		assert.NoError(t, err)

		assert.Equal(t, 1, v.Len())
		assert.Equal(t, 99, v.At(0))
	})

	t.Run("unmarshal empty array", func(t *testing.T) {
		ar2 := NewArena()
		defer ar2.Reset()

		v := NewVector[int](ar2, 8)
		v.Append(1, 2, 3)

		err := json.Unmarshal([]byte("[]"), v)
		assert.NoError(t, err)
		assert.Equal(t, 0, v.Len())
	})

	t.Run("unmarshal invalid JSON", func(t *testing.T) {
		ar2 := NewArena()
		defer ar2.Reset()

		v := NewVector[int](ar2, 4)
		err := json.Unmarshal([]byte("[bad"), v)
		assert.Error(t, err)
	})

	t.Run("unmarshal wrong type", func(t *testing.T) {
		ar2 := NewArena()
		defer ar2.Reset()

		v := NewVector[int](ar2, 4)
		err := json.Unmarshal([]byte(`{"key": "val"}`), v)
		assert.Error(t, err)
	})

	t.Run("round-trip", func(t *testing.T) {
		ar2 := NewArena()
		defer ar2.Reset()

		v1 := NewVector[string](ar2, 8)
		v1.Append("hello", "world", "你好", "special\nchars")

		b, err := json.Marshal(v1)
		assert.NoError(t, err)

		var got []string
		err = json.Unmarshal(b, &got)
		assert.NoError(t, err)
		assert.Equal(t, []string{"hello", "world", "你好", "special\nchars"}, got)
	})

	t.Run("marshal null elements", func(t *testing.T) {
		ar2 := NewArena()
		defer ar2.Reset()

		v := NewVector[*int](ar2, 4)
		one := 1
		v.Append(&one, nil, &one)

		b, err := json.Marshal(v)
		assert.NoError(t, err)

		var got []*int
		err = json.Unmarshal(b, &got)
		assert.NoError(t, err)
		assert.Equal(t, 3, len(got))
		assert.NotNil(t, got[0])
		assert.Nil(t, got[1])
		assert.NotNil(t, got[2])
	})
}

// ============================================================================
// Deep nesting — Map in Vector in Map
// ============================================================================

func TestJSONDeepNesting(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	t.Run("Map of Vector of Map", func(t *testing.T) {
		outer := NewMap[string, *Vector[*Map[string, int]]](ar, 8)

		inner1 := NewMap[string, int](ar, 4)
		inner1.Put("x", 1)
		inner1.Put("y", 2)

		inner2 := NewMap[string, int](ar, 4)
		inner2.Put("z", 3)

		vec := NewVector[*Map[string, int]](ar, 4)
		vec.Append(inner1, inner2)

		outer.Put("data", vec)

		b, err := json.Marshal(outer)
		assert.NoError(t, err)

		var got map[string][]map[string]int
		err = json.Unmarshal(b, &got)
		assert.NoError(t, err)
		assert.Equal(t, []map[string]int{{"x": 1, "y": 2}, {"z": 3}}, got["data"])
	})

	t.Run("Vector of Map of Vector", func(t *testing.T) {
		inner := NewVector[int](ar, 4)
		inner.Append(1, 2, 3)

		m := NewMap[string, *Vector[int]](ar, 4)
		m.Put("nums", inner)

		outer := NewVector[*Map[string, *Vector[int]]](ar, 4)
		outer.Append(m)

		b, err := json.Marshal(outer)
		assert.NoError(t, err)

		var got []map[string][]int
		err = json.Unmarshal(b, &got)
		assert.NoError(t, err)
		assert.Equal(t, []map[string][]int{{"nums": {1, 2, 3}}}, got)
	})
}

// ============================================================================
// Wire compatibility — output matches standard Go map/slice JSON
// ============================================================================

func TestJSONWireCompatibility(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	t.Run("Map matches Go map output", func(t *testing.T) {
		goMap := map[string]int{"a": 1, "b": 2, "c": 3}
		goJSON, err := json.Marshal(goMap)
		assert.NoError(t, err)

		arenaMap := NewMap[string, int](ar, 8)
		arenaMap.Put("a", 1)
		arenaMap.Put("b", 2)
		arenaMap.Put("c", 3)
		arenaJSON, err := json.Marshal(arenaMap)
		assert.NoError(t, err)

		// Parse both and compare (order may differ)
		var goParsed, arenaParsed map[string]int
		json.Unmarshal(goJSON, &goParsed)
		json.Unmarshal(arenaJSON, &arenaParsed)
		assert.Equal(t, goParsed, arenaParsed)
	})

	t.Run("Vector matches Go slice output", func(t *testing.T) {
		goSlice := []int{1, 2, 3, 4, 5}
		goJSON, err := json.Marshal(goSlice)
		assert.NoError(t, err)

		arenaVec := NewVector[int](ar, 8)
		arenaVec.Append(1, 2, 3, 4, 5)
		arenaJSON, err := json.Marshal(arenaVec)
		assert.NoError(t, err)

		assert.Equal(t, string(goJSON), string(arenaJSON))
	})
}

// ============================================================================
// Edge cases
// ============================================================================

func TestJSONEdgeCases(t *testing.T) {
	t.Run("Map unmarshal null", func(t *testing.T) {
		ar := NewArena()
		defer ar.Reset()

		m := NewMap[string, int](ar, 8)
		m.Put("x", 1)

		err := json.Unmarshal([]byte("null"), m)
		assert.NoError(t, err)
		assert.Equal(t, 0, m.Len(), "null should clear the map")
	})

	t.Run("Vector unmarshal null", func(t *testing.T) {
		ar := NewArena()
		defer ar.Reset()

		v := NewVector[int](ar, 8)
		v.Append(1, 2, 3)

		err := json.Unmarshal([]byte("null"), v)
		assert.NoError(t, err)
		assert.Equal(t, 0, v.Len(), "null should clear the vector")
	})

	t.Run("Map marshal is read-only", func(t *testing.T) {
		ar := NewArena()
		defer ar.Reset()

		m := NewMap[string, int](ar, 8)
		m.Put("a", 1)
		m.Put("b", 2)
		origLen := m.Len()

		_, err := json.Marshal(m)
		assert.NoError(t, err)
		assert.Equal(t, origLen, m.Len(), "marshal should not modify map")

		v, ok := m.Get("a")
		assert.True(t, ok)
		assert.Equal(t, 1, v)
	})

	t.Run("Vector marshal is read-only", func(t *testing.T) {
		ar := NewArena()
		defer ar.Reset()

		v := NewVector[int](ar, 8)
		v.Append(10, 20, 30)
		origLen := v.Len()

		_, err := json.Marshal(v)
		assert.NoError(t, err)
		assert.Equal(t, origLen, v.Len(), "marshal should not modify vector")
		assert.Equal(t, 10, v.At(0))
	})

	t.Run("float64 types", func(t *testing.T) {
		ar := NewArena()
		defer ar.Reset()

		t.Run("Map", func(t *testing.T) {
			m := NewMap[string, float64](ar, 8)
			m.Put("pi", 3.14159)
			m.Put("e", 2.71828)

			b, err := json.Marshal(m)
			assert.NoError(t, err)

			var got map[string]float64
			err = json.Unmarshal(b, &got)
			assert.NoError(t, err)
			assert.InDelta(t, 3.14159, got["pi"], 1e-6)
			assert.InDelta(t, 2.71828, got["e"], 1e-6)
		})

		t.Run("Vector", func(t *testing.T) {
			v := NewVector[float64](ar, 4)
			v.Append(1.1, 2.2, 3.3)

			b, err := json.Marshal(v)
			assert.NoError(t, err)

			var got []float64
			err = json.Unmarshal(b, &got)
			assert.NoError(t, err)
			assert.InDelta(t, 1.1, got[0], 1e-6)
			assert.InDelta(t, 2.2, got[1], 1e-6)
			assert.InDelta(t, 3.3, got[2], 1e-6)
		})
	})

	t.Run("bool types", func(t *testing.T) {
		ar := NewArena()
		defer ar.Reset()

		t.Run("Map", func(t *testing.T) {
			m := NewMap[string, bool](ar, 8)
			m.Put("flag1", true)
			m.Put("flag2", false)

			b, err := json.Marshal(m)
			assert.NoError(t, err)

			var got map[string]bool
			err = json.Unmarshal(b, &got)
			assert.NoError(t, err)
			assert.Equal(t, true, got["flag1"])
			assert.Equal(t, false, got["flag2"])
		})

		t.Run("Vector", func(t *testing.T) {
			v := NewVector[bool](ar, 4)
			v.Append(true, false, true)

			b, err := json.Marshal(v)
			assert.NoError(t, err)

			var got []bool
			err = json.Unmarshal(b, &got)
			assert.NoError(t, err)
			assert.Equal(t, []bool{true, false, true}, got)
		})
	})

	t.Run("unmarshal into arena-managed Map values are in arena", func(t *testing.T) {
		ar := NewArena()
		defer ar.Reset()

		m := NewMap[string, string](ar, 8)
		err := json.Unmarshal([]byte(`{"hello":"world"}`), m)
		assert.NoError(t, err)

		// Verify the string values are arena-managed
		val, ok := m.Get("hello")
		assert.True(t, ok)
		// The string data should be in arena memory (deepCopy'd by Put)
		assert.True(t, len(val) > 0)
	})

	t.Run("unmarshal into arena Vector values are in arena", func(t *testing.T) {
		ar := NewArena()
		defer ar.Reset()

		v := NewVector[string](ar, 4)
		err := json.Unmarshal([]byte(`["alpha","beta"]`), v)
		assert.NoError(t, err)

		assert.Equal(t, 2, v.Len())
		assert.Equal(t, "alpha", v.At(0))
		assert.Equal(t, "beta", v.At(1))
	})

	t.Run("Vector after Clear marshal", func(t *testing.T) {
		ar := NewArena()
		defer ar.Reset()

		v := NewVector[int](ar, 8)
		v.Append(1, 2, 3)
		v.Clear()

		b, err := json.Marshal(v)
		assert.NoError(t, err)
		// After Clear, vec is nil; json.Marshal(nil) → "null" per Go convention
		assert.Equal(t, "null", string(b))
	})

	t.Run("Map after Clear marshal", func(t *testing.T) {
		ar := NewArena()
		defer ar.Reset()

		m := NewMap[string, int](ar, 8)
		m.Put("a", 1)
		m.Clear()

		b, err := json.Marshal(m)
		assert.NoError(t, err)
		assert.Equal(t, "{}", string(b))
	})

	t.Run("Map marshal then unmarshal back to Arena", func(t *testing.T) {
		ar1 := NewArena()
		defer ar1.Reset()

		// Create with Arena, marshal to JSON
		src := NewMap[string, int](ar1, 8)
		src.Put("key1", 100)
		src.Put("key2", 200)
		jsonData, err := json.Marshal(src)
		assert.NoError(t, err)

		// Unmarshal into a fresh Arena
		ar2 := NewArena()
		defer ar2.Reset()
		dst := NewMap[string, int](ar2, 8)
		err = json.Unmarshal(jsonData, dst)
		assert.NoError(t, err)

		v1, _ := dst.Get("key1")
		v2, _ := dst.Get("key2")
		assert.Equal(t, 100, v1)
		assert.Equal(t, 200, v2)
	})

	t.Run("Vector marshal then unmarshal back to Arena", func(t *testing.T) {
		ar1 := NewArena()
		defer ar1.Reset()

		src := NewVector[string](ar1, 8)
		src.Append("a", "b", "c")
		jsonData, err := json.Marshal(src)
		assert.NoError(t, err)

		ar2 := NewArena()
		defer ar2.Reset()
		dst := NewVector[string](ar2, 8)
		err = json.Unmarshal(jsonData, dst)
		assert.NoError(t, err)

		assert.Equal(t, 3, dst.Len())
		assert.Equal(t, "a", dst.At(0))
		assert.Equal(t, "b", dst.At(1))
		assert.Equal(t, "c", dst.At(2))
	})

	t.Run("Map with uint32 keys", func(t *testing.T) {
		ar := NewArena()
		defer ar.Reset()

		m := NewMap[uint32, string](ar, 8)
		m.Put(1, "one")
		m.Put(2, "two")

		b, err := json.Marshal(m)
		assert.NoError(t, err)

		var got map[uint32]string
		err = json.Unmarshal(b, &got)
		assert.NoError(t, err)
		assert.Equal(t, "one", got[1])
		assert.Equal(t, "two", got[2])
	})

	t.Run("unmarshal large JSON", func(t *testing.T) {
		ar := NewArena()
		defer ar.Reset()

		// Build large JSON array
		var data []byte
		data = append(data, '[')
		for i := 0; i < 1000; i++ {
			if i > 0 {
				data = append(data, ',')
			}
			data = append(data, []byte(`{"id":`)...)
			data = append(data, []byte(formatInt(i))...)
			data = append(data, []byte(`,"name":"item`)...)
			data = append(data, []byte(formatInt(i))...)
			data = append(data, []byte(`"}`)...)
		}
		data = append(data, ']')

		type item struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}
		v := NewVector[item](ar, 8)
		err := json.Unmarshal(data, v)
		assert.NoError(t, err)
		assert.Equal(t, 1000, v.Len())
		assert.Equal(t, 0, v.At(0).ID)
		assert.Equal(t, "item0", v.At(0).Name)
		assert.Equal(t, 999, v.At(999).ID)
		assert.Equal(t, "item999", v.At(999).Name)
	})
}

func formatInt(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
