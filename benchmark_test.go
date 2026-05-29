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
	"fmt"
	"runtime"
	"strconv"
	"testing"
)

// ============================================================================
// Arena primitives
// ============================================================================

func BenchmarkArenaMallocFreeSizes(b *testing.B) {
	sizes := []uintptr{16, 64, 128, 512, 1024, 4096}
	for _, sz := range sizes {
		b.Run(fmt.Sprintf("size_%d", sz), func(b *testing.B) {
			ar := NewArena()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ptr := ar.Malloc(sz)
				ar.Free(ptr)
			}
		})
	}
}

func BenchmarkArenaNewInt(b *testing.B) {
	ar := NewArena()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = New[int](ar)
	}
}

// ============================================================================
// Map[int,int] — value types, no deep copy overhead
// ============================================================================

func BenchmarkMapPutIntInt(b *testing.B) {
	b.Run("ArenaMap", func(b *testing.B) {
		ar := NewArena()
		m := NewMap[int, int](ar, b.N)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			m.Put(i, i)
		}
		runtime.KeepAlive(ar)
	})

	b.Run("GoMap", func(b *testing.B) {
		m := make(map[int]int, b.N)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			m[i] = i
		}
	})
}

func BenchmarkMapGetIntInt(b *testing.B) {
	const mapSize = 100000
	ar := NewArena()
	m := NewMap[int, int](ar, mapSize)
	for i := 0; i < mapSize; i++ {
		m.Put(i, i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.Get(i % mapSize)
	}
	runtime.KeepAlive(ar)
}

func BenchmarkGoMapGetIntInt(b *testing.B) {
	const mapSize = 100000
	m := make(map[int]int, mapSize)
	for i := 0; i < mapSize; i++ {
		m[i] = i
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m[i%mapSize]
	}
}

func BenchmarkMapPutRemoveIntInt(b *testing.B) {
	const mapSize = 100000
	b.Run("ArenaMap", func(b *testing.B) {
		ar := NewArena()
		m := NewMap[int, int](ar, mapSize)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			k := i % mapSize
			m.Put(k, i)
			if i%3 == 0 {
				m.Remove(k)
			}
		}
		runtime.KeepAlive(ar)
	})

	b.Run("GoMap", func(b *testing.B) {
		m := make(map[int]int, mapSize)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			k := i % mapSize
			m[k] = i
			if i%3 == 0 {
				delete(m, k)
			}
		}
	})
}

func BenchmarkMapIterIntInt(b *testing.B) {
	const mapSize = 100000
	ar := NewArena()
	m := NewMap[int, int](ar, mapSize)
	for i := 0; i < mapSize; i++ {
		m.Put(i, i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for range m.All() {
		}
	}
	runtime.KeepAlive(ar)
}

func BenchmarkGoMapIterIntInt(b *testing.B) {
	const mapSize = 100000
	m := make(map[int]int, mapSize)
	for i := 0; i < mapSize; i++ {
		m[i] = i
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for range m {
		}
	}
}

// ============================================================================
// Map[int,string] — pre-sized to avoid resize, measures deep copy overhead
// ============================================================================

func BenchmarkMapPutIntString(b *testing.B) {
	b.Run("ArenaMap", func(b *testing.B) {
		ar := NewArena()
		m := NewMap[int, string](ar, b.N)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			m.Put(i, strconv.Itoa(i%1000))
		}
		runtime.KeepAlive(ar)
	})

	b.Run("GoMap", func(b *testing.B) {
		m := make(map[int]string, b.N)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			m[i] = strconv.Itoa(i % 1000)
		}
	})
}

func BenchmarkMapGetIntString(b *testing.B) {
	const mapSize = 10000
	ar := NewArena()
	m := NewMap[int, string](ar, mapSize)
	for i := 0; i < mapSize; i++ {
		m.Put(i, strconv.Itoa(i))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.Get(i % mapSize)
	}
	runtime.KeepAlive(ar)
}

func BenchmarkGoMapGetIntString(b *testing.B) {
	const mapSize = 10000
	m := make(map[int]string, mapSize)
	for i := 0; i < mapSize; i++ {
		m[i] = strconv.Itoa(i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m[i%mapSize]
	}
}

// ============================================================================
// Vector Append
// ============================================================================

func BenchmarkVectorAppendInt(b *testing.B) {
	ar := NewArena()
	vec := NewVector[int](ar, b.N)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vec.Append(i)
	}
	runtime.KeepAlive(ar)
}

func BenchmarkGoSliceAppendInt(b *testing.B) {
	s := make([]int, 0, b.N)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s = append(s, i)
	}
}

func BenchmarkVectorAppendString(b *testing.B) {
	ar := NewArena()
	vec := NewVector[string](ar, b.N)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vec.Append(strconv.Itoa(i % 1000))
	}
	runtime.KeepAlive(ar)
}

func BenchmarkGoSliceAppendString(b *testing.B) {
	s := make([]string, 0, b.N)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s = append(s, strconv.Itoa(i%1000))
	}
}

// ============================================================================
// Vector iteration
// ============================================================================

func BenchmarkVectorIterInt(b *testing.B) {
	const vecSize = 100000
	ar := NewArena()
	vec := NewVector[int](ar, vecSize)
	for i := 0; i < vecSize; i++ {
		vec.Append(i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for range vec.All() {
		}
	}
	runtime.KeepAlive(ar)
}

func BenchmarkGoSliceIterInt(b *testing.B) {
	const vecSize = 100000
	s := make([]int, vecSize)
	for i := 0; i < vecSize; i++ {
		s[i] = i
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for range s {
		}
	}
}

// ============================================================================
// DeepCopy
// ============================================================================

type benchPerson struct {
	Name    string
	Age     int32
	Active  bool
	Scores  []float64
	Details *benchDetails
}

type benchDetails struct {
	Email string
	Phone string
}

func newBenchPerson() benchPerson {
	return benchPerson{
		Name:   "benchmark user with a reasonably long name",
		Age:    42,
		Active: true,
		Scores: []float64{98.5, 87.3, 76.1},
		Details: &benchDetails{
			Email: "user@example.com",
			Phone: "+1-555-0123",
		},
	}
}

func BenchmarkDeepCopyStruct(b *testing.B) {
	src := newBenchPerson()

	b.Run("ArenaDeepCopy", func(b *testing.B) {
		ar := NewArena()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = DeepCopy(ar, src)
		}
		runtime.KeepAlive(ar)
	})

	b.Run("GoHeapCopy", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			d := benchPerson{
				Name:    src.Name,
				Age:     src.Age,
				Active:  src.Active,
				Details: &benchDetails{Email: src.Details.Email, Phone: src.Details.Phone},
			}
			d.Scores = make([]float64, len(src.Scores))
			copy(d.Scores, src.Scores)
			_ = d
		}
	})
}

// ============================================================================
// GC scan cost: measure how long GC takes with arena vs heap
// ============================================================================

func BenchmarkGCScanCost(b *testing.B) {
	type entry struct {
		Key   int
		Value string
		Next  *entry
	}

	sizes := []int{10000, 100000, 1000000}

	for _, n := range sizes {
		b.Run(fmt.Sprintf("Arena_%d", n), func(b *testing.B) {
			ar := NewArena(WithChunkSize(1024 * 1024))
			for j := 0; j < n; j++ {
				e := New[entry](ar)
				e.Key = j
				e.Value = strconv.Itoa(j)
				_ = e
			}
			runtime.GC()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				runtime.GC()
			}
			runtime.KeepAlive(ar)
		})

		b.Run(fmt.Sprintf("GoHeap_%d", n), func(b *testing.B) {
			objects := make([]*entry, n)
			for j := 0; j < n; j++ {
				objects[j] = &entry{Key: j, Value: strconv.Itoa(j)}
			}
			runtime.GC()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				runtime.GC()
			}
			runtime.KeepAlive(objects)
		})
	}
}

func BenchmarkGCSweepWithMap(b *testing.B) {
	const mapSize = 100000

	b.Run("ArenaMap_GC", func(b *testing.B) {
		ar := NewArena(WithChunkSize(1024 * 1024))
		m := NewMap[int, int](ar, mapSize)
		for i := 0; i < mapSize; i++ {
			m.Put(i, i)
		}
		runtime.GC()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			runtime.GC()
		}
		runtime.KeepAlive(ar)
	})

	b.Run("GoMap_GC", func(b *testing.B) {
		m := make(map[int]int, mapSize)
		for i := 0; i < mapSize; i++ {
			m[i] = i
		}
		runtime.GC()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			runtime.GC()
		}
	})
}

func BenchmarkGCSweepWithVector(b *testing.B) {
	const vecSize = 1000000

	b.Run("ArenaVector_GC", func(b *testing.B) {
		ar := NewArena(WithChunkSize(1024 * 1024))
		vec := NewVector[int](ar, vecSize)
		for i := 0; i < vecSize; i++ {
			vec.Append(i)
		}
		runtime.GC()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			runtime.GC()
		}
		runtime.KeepAlive(ar)
	})

	b.Run("GoSlice_GC", func(b *testing.B) {
		s := make([]int, vecSize)
		for i := 0; i < vecSize; i++ {
			s[i] = i
		}
		runtime.GC()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			runtime.GC()
		}
	})
}
