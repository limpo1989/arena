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
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// TestNewMap
// ---------------------------------------------------------------------------

func TestNewMap(t *testing.T) {
	t.Run("capacity 0 uses minimum 8", func(t *testing.T) {
		a := NewArena()
		defer a.Reset()
		m := NewMap[int, int](a, 0)
		assert.NotNil(t, m)
		assert.Equal(t, 0, m.Len())
	})

	t.Run("capacity 16", func(t *testing.T) {
		a := NewArena()
		defer a.Reset()
		m := NewMap[int, int](a, 16)
		assert.NotNil(t, m)
		assert.Equal(t, 0, m.Len())
	})

	t.Run("string keys int values", func(t *testing.T) {
		a := NewArena()
		defer a.Reset()
		m := NewMap[string, int](a, 8)
		assert.NotNil(t, m)
	})

	t.Run("int keys struct values", func(t *testing.T) {
		a := NewArena()
		defer a.Reset()
		type point struct{ X, Y int }
		m := NewMap[int, point](a, 8)
		assert.NotNil(t, m)
	})
}

// ---------------------------------------------------------------------------
// TestMapPut
// ---------------------------------------------------------------------------

func TestMapPut(t *testing.T) {
	t.Run("new key", func(t *testing.T) {
		a := NewArena()
		defer a.Reset()
		m := NewMap[string, int](a, 8)
		m.Put("hello", 42)
		v, ok := m.Get("hello")
		assert.True(t, ok)
		assert.Equal(t, 42, v)
	})

	t.Run("overwrite existing key", func(t *testing.T) {
		a := NewArena()
		defer a.Reset()
		m := NewMap[string, int](a, 8)
		m.Put("key", 1)
		m.Put("key", 2)
		v, ok := m.Get("key")
		assert.True(t, ok)
		assert.Equal(t, 2, v)
		assert.Equal(t, 1, m.Len())
	})

	t.Run("value with pointer fields deep-copied", func(t *testing.T) {
		a := NewArena()
		defer a.Reset()
		m := NewMap[int, *int](a, 8)

		original := 99
		m.Put(1, &original)

		// Modifying the original must NOT affect the stored value.
		original = 0
		v, ok := m.Get(1)
		assert.True(t, ok)
		assert.Equal(t, 99, *v)
	})
}

// ---------------------------------------------------------------------------
// TestMapGet
// ---------------------------------------------------------------------------

func TestMapGet(t *testing.T) {
	t.Run("existing key returns value", func(t *testing.T) {
		a := NewArena()
		defer a.Reset()
		m := NewMap[string, int](a, 8)
		m.Put("x", 10)
		v, ok := m.Get("x")
		assert.True(t, ok)
		assert.Equal(t, 10, v)
	})

	t.Run("missing key returns false", func(t *testing.T) {
		a := NewArena()
		defer a.Reset()
		m := NewMap[string, int](a, 8)
		_, ok := m.Get("nonexistent")
		assert.False(t, ok)
	})

	t.Run("zero value key returns true", func(t *testing.T) {
		a := NewArena()
		defer a.Reset()
		m := NewMap[string, int](a, 8)
		m.Put("num", 0)
		v, ok := m.Get("num")
		assert.True(t, ok)
		assert.Equal(t, 0, v)
	})
}

// ---------------------------------------------------------------------------
// TestMapRemove
// ---------------------------------------------------------------------------

func TestMapRemove(t *testing.T) {
	t.Run("existing key removed and Len decrements", func(t *testing.T) {
		a := NewArena()
		defer a.Reset()
		m := NewMap[string, int](a, 8)
		m.Put("a", 1)
		m.Put("b", 2)
		assert.Equal(t, 2, m.Len())

		m.Remove("a")
		assert.Equal(t, 1, m.Len())
		_, ok := m.Get("a")
		assert.False(t, ok)
		v, ok := m.Get("b")
		assert.True(t, ok)
		assert.Equal(t, 2, v)
	})

	t.Run("missing key no-op", func(t *testing.T) {
		a := NewArena()
		defer a.Reset()
		m := NewMap[string, int](a, 8)
		m.Put("a", 1)
		m.Remove("nonexistent")
		assert.Equal(t, 1, m.Len())
		v, ok := m.Get("a")
		assert.True(t, ok)
		assert.Equal(t, 1, v)
	})

	t.Run("re-insert after remove works", func(t *testing.T) {
		a := NewArena()
		defer a.Reset()
		m := NewMap[string, int](a, 8)
		m.Put("k", 10)
		m.Remove("k")
		_, ok := m.Get("k")
		assert.False(t, ok)

		m.Put("k", 20)
		v, ok := m.Get("k")
		assert.True(t, ok)
		assert.Equal(t, 20, v)
		assert.Equal(t, 1, m.Len())
	})

	t.Run("tombstone does not break probe chains", func(t *testing.T) {
		a := NewArena()
		defer a.Reset()
		m := NewMap[int, int](a, 8)

		for i := 0; i < 6; i++ {
			m.Put(i, i*10)
		}

		m.Remove(0)

		for i := 1; i < 6; i++ {
			v, ok := m.Get(i)
			assert.True(t, ok, "key %d should still be present", i)
			assert.Equal(t, i*10, v, "key %d value mismatch", i)
		}

		_, ok := m.Get(0)
		assert.False(t, ok)
	})
}

// ---------------------------------------------------------------------------
// TestMapAddIfAbsent
// ---------------------------------------------------------------------------

func TestMapAddIfAbsent(t *testing.T) {
	a := NewArena()
	defer a.Reset()

	m := NewMap[int, string](a, 8)

	t.Run("new key returns true", func(t *testing.T) {
		assert.True(t, m.AddIfAbsent(1, "first"))
		v, ok := m.Get(1)
		assert.True(t, ok)
		assert.Equal(t, "first", v)
	})

	t.Run("existing key returns false", func(t *testing.T) {
		assert.False(t, m.AddIfAbsent(1, "second"))
		v, ok := m.Get(1)
		assert.True(t, ok)
		assert.Equal(t, "first", v)
	})
}

// ---------------------------------------------------------------------------
// TestMapClear
// ---------------------------------------------------------------------------

func TestMapClear(t *testing.T) {
	t.Run("populated map cleared", func(t *testing.T) {
		a := NewArena()
		defer a.Reset()
		m := NewMap[int, int](a, 8)
		for i := 0; i < 20; i++ {
			m.Put(i, i)
		}
		assert.Equal(t, 20, m.Len())

		m.Clear()
		assert.Equal(t, 0, m.Len())
		for i := 0; i < 20; i++ {
			_, ok := m.Get(i)
			assert.False(t, ok)
		}
	})

	t.Run("reuse after clear works", func(t *testing.T) {
		a := NewArena()
		defer a.Reset()
		m := NewMap[int, int](a, 8)
		for i := 0; i < 10; i++ {
			m.Put(i, i)
		}
		m.Clear()

		m.Put(100, 200)
		assert.Equal(t, 1, m.Len())
		v, ok := m.Get(100)
		assert.True(t, ok)
		assert.Equal(t, 200, v)
	})
}

// ---------------------------------------------------------------------------
// TestMapLen
// ---------------------------------------------------------------------------

func TestMapLen(t *testing.T) {
	a := NewArena()
	defer a.Reset()
	m := NewMap[string, int](a, 8)

	assert.Equal(t, 0, m.Len())

	m.Put("a", 1)
	assert.Equal(t, 1, m.Len())

	m.Put("b", 2)
	assert.Equal(t, 2, m.Len())

	m.Remove("a")
	assert.Equal(t, 1, m.Len())
}

// ---------------------------------------------------------------------------
// TestMapRange
// ---------------------------------------------------------------------------

func TestMapRange(t *testing.T) {
	t.Run("full iteration count matches Len", func(t *testing.T) {
		a := NewArena()
		defer a.Reset()
		m := NewMap[int, string](a, 8)
		for i := 0; i < 10; i++ {
			m.Put(i, strconv.Itoa(i))
		}

		count := 0
		for range m.All() {
			count++
		}
		assert.Equal(t, 10, count)
	})

	t.Run("early termination", func(t *testing.T) {
		a := NewArena()
		defer a.Reset()
		m := NewMap[int, string](a, 8)
		for i := 0; i < 10; i++ {
			m.Put(i, strconv.Itoa(i))
		}

		count := 0
		for range m.All() {
			count++
			if count >= 3 {
				break
			}
		}
		assert.Equal(t, 3, count)
	})

	t.Run("empty map no calls", func(t *testing.T) {
		a := NewArena()
		defer a.Reset()
		m := NewMap[int, string](a, 8)

		called := false
		for range m.All() {
			called = true
		}
		assert.False(t, called)
	})
}

// ---------------------------------------------------------------------------
// TestMapIter
// ---------------------------------------------------------------------------

func TestMapIter(t *testing.T) {
	a := NewArena()
	defer a.Reset()
	m := NewMap[int, string](a, 8)
	for i := 0; i < 5; i++ {
		m.Put(i, strconv.Itoa(i))
	}

	visited := make(map[int]string)
	for k, v := range m.All() {
		visited[k] = v
	}
	assert.Equal(t, 5, len(visited))
	for i := 0; i < 5; i++ {
		assert.Equal(t, strconv.Itoa(i), visited[i])
	}
}

// ---------------------------------------------------------------------------
// TestMapResize
// ---------------------------------------------------------------------------

func TestMapResize(t *testing.T) {
	t.Run("insert enough to trigger resize", func(t *testing.T) {
		a := NewArena()
		defer a.Reset()
		m := NewMap[int, int](a, 8)
		for i := 0; i < 7; i++ {
			m.Put(i, i*100)
		}

		assert.Equal(t, 7, m.Len())
		for i := 0; i < 7; i++ {
			v, ok := m.Get(i)
			assert.True(t, ok, "key %d should exist after resize", i)
			assert.Equal(t, i*100, v, "key %d value mismatch after resize", i)
		}
	})
}

// ---------------------------------------------------------------------------
// TestMapDeepCopier
// ---------------------------------------------------------------------------

func TestMapDeepCopier(t *testing.T) {
	type wrapper struct {
		M *Map[string, int]
	}

	a := NewArena()
	defer a.Reset()

	original := NewMap[string, int](a, 8)
	original.Put("x", 1)
	original.Put("y", 2)

	w := wrapper{M: original}
	clone := DeepCopy[wrapper](a, w)

	assert.NotNil(t, clone)
	assert.NotNil(t, clone.M)
	assert.NotEqual(t, original, clone.M)

	assert.Equal(t, 2, clone.M.Len())
	v, ok := clone.M.Get("x")
	assert.True(t, ok)
	assert.Equal(t, 1, v)

	clone.M.Put("z", 3)
	_, ok = original.Get("z")
	assert.False(t, ok)
	assert.Equal(t, 2, original.Len())
	assert.Equal(t, 3, clone.M.Len())
}

// ---------------------------------------------------------------------------
// TestMapNoGoMap
// ---------------------------------------------------------------------------

func TestMapNoGoMap(t *testing.T) {
	a := NewArena()
	defer a.Reset()
	m := NewMap[int, int](a, 64)

	for i := 0; i < 40; i++ {
		m.Put(i, i)
	}
	for i := 0; i < 40; i++ {
		v, ok := m.Get(i)
		assert.True(t, ok)
		assert.Equal(t, i, v)
	}
	for i := 0; i < 20; i++ {
		m.Remove(i)
	}
	assert.Equal(t, 20, m.Len())
	for i := 20; i < 40; i++ {
		v, ok := m.Get(i)
		assert.True(t, ok)
		assert.Equal(t, i, v)
	}
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

func TestStdMap(t *testing.T) {
	var m = make(map[int]*largeMessage, largeSize)
	for i := 0; i < largeSize; i++ {
		m[i] = prepareArgs()
	}
	start := time.Now()
	runtime.GC()
	var memStat runtime.MemStats
	runtime.ReadMemStats(&memStat)
	t.Logf("Heap GC took time: %v, living objects: %d", time.Since(start), memStat.HeapObjects)
	runtime.KeepAlive(m)
}

func TestArenaMap(t *testing.T) {
	var allocator = NewArena(WithChunkSize(1024 * 1024))
	defer allocator.Reset()

	var m = NewMap[int, largeMessage](allocator, largeSize)
	for i := 0; i < largeSize; i++ {
		m.Put(i, *prepareArgs())
	}
	start := time.Now()
	runtime.GC()
	var memStat runtime.MemStats
	runtime.ReadMemStats(&memStat)
	t.Logf("Arena GC took time: %v, living objects: %d", time.Since(start), memStat.HeapObjects)
	runtime.KeepAlive(m)
}
