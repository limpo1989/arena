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
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"runtime"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
)

func TestArena(t *testing.T) {

	ar := NewArena()

	for i := 0; i < 1000; i++ {
		if "hello" != *ar.NewString("hello") {
			t.Fatalf("missmatch")
		}

		var rv = rand.Int()

		if rv > 100 != *ar.NewBool(rv > 100) {
			t.Fatalf("missmatch")
		}

		if rv != *ar.NewInt(rv) {
			t.Fatalf("missmatch")
		}

		if int8(rv) != *ar.NewInt8(int8(rv)) {
			t.Fatalf("missmatch")
		}

		if int16(rv) != *ar.NewInt16(int16(rv)) {
			t.Fatalf("missmatch")
		}

		if int32(rv) != *ar.NewInt32(int32(rv)) {
			t.Fatalf("missmatch")
		}

		if int64(rv) != *ar.NewInt64(int64(rv)) {
			t.Fatalf("missmatch")
		}

		if uint(rv) != *ar.NewUint(uint(rv)) {
			t.Fatalf("missmatch")
		}

		if uint8(rv) != *ar.NewUint8(uint8(rv)) {
			t.Fatalf("missmatch")
		}

		if uint16(rv) != *ar.NewUint16(uint16(rv)) {
			t.Fatalf("missmatch")
		}

		if uint32(rv) != *ar.NewUint32(uint32(rv)) {
			t.Fatalf("missmatch")
		}

		if uint64(rv) != *ar.NewUint64(uint64(rv)) {
			t.Fatalf("missmatch")
		}

		var s = []byte("hello arena")
		if 0 != bytes.Compare(s, ar.Bytes(s)) {
			t.Fatalf("missmatch")
		}

		var f32 = rand.Float32() * 1000.0
		if f32 != *ar.NewFloat32(f32) {
			t.Fatalf("missmatch")
		}

		var f64 = rand.Float64() * 1000.0
		if f64 != *ar.NewFloat64(f64) {
			t.Fatalf("missmatch")
		}

	}
}

func TestArenaMalloc(t *testing.T) {
	ar := NewArena()
	ptr := ar.Malloc(8)
	ar.Free(ptr)

	ptr = ar.Malloc(8)
	ar.Free(ptr)

	ptr = ar.Malloc(128)
	ar.Free(ptr)

	ptr = ar.Malloc(128)
	ar.Free(ptr)

	ptr = ar.Malloc(64)
	ar.Free(ptr)

	ptr1 := ar.Malloc(80)
	ptr2 := ar.Malloc(60)
	ar.Free(ptr1)
	ar.Free(ptr2)

	size := []uintptr{16, 32, 64, 128, 256, 512, 1024, 2048, 4098, 9012, 10240}

	for i := 0; i < 10000; i++ {
		ptr := ar.Malloc(size[i%len(size)])
		ar.Free(ptr)
	}
}

func TestArenaLifecycle(t *testing.T) {
	ar := NewArena()

	runtime.SetFinalizer(ar, func(ar *Arena) {
		fmt.Println("Arena released")
	})

	// 从Arena中分配对象
	i32 := ar.NewInt32(1001)

	// 触发Arena SetFinalizer 调用
	runtime.GC()
	// 触发GC回收Arena
	runtime.GC()
	fmt.Println("gc finished")

	// i32对象虽然是指向Arena的，由于使用unsafe的方式分配的因此是不受GC管理的对象，也即是i32不影响arena对象的生命周期
	// i32所指向的内存已经被回收到内存分配器，此处已经是未定义的行为
	//
	fmt.Println("access i32 = ", *i32)
	fmt.Println("test finished")

	// 如果想要正确使用Arena来分配对象，应该确保Arena对象的生命周期大于所分配对象的生命周期
	// 通过注释这行代码，观察输出信息变化
	runtime.KeepAlive(ar)

	// 触发Arena SetFinalizer 调用
	runtime.GC()
	// 触发GC回收Arena
	runtime.GC()
}

func TestArenaBadLifecycle(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	type subject struct {
		id  int32
		age *int32
	}

	p := New[subject](ar)
	p.age = new(int32) // 误用：持有堆内存指针
	*p.age = 100

	// 通过设置终结器可以观察堆内存指针会在Arena结束之前被回收
	//
	runtime.SetFinalizer(p.age, func(p *int32) {
		fmt.Println("subject.age released")
	})

	runtime.GC()
	fmt.Println("gc finished 1")
	runtime.GC()
	fmt.Println("gc finished 2")

	// 未定义的访问，实际上这部分内存已经被GC回收了，虽然这里也许可以访问它，但是将会产生未定义的行为
	//*p.age = 99
}

func BenchmarkArenaMallocFree(b *testing.B) {
	ar := NewArena()

	size := []uintptr{16, 32, 64, 128, 256, 512, 1024, 2048, 4098, 9012, 10240}

	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ptr := ar.Malloc(size[i%len(size)])
		ar.Free(ptr)
	}

	_ = ar
	_ = ar.chunkSize
}

type largeMessage struct {
	Field1   string
	Field9   string
	Field18  string
	Field80  *bool
	Field81  *bool
	Field2   int32
	Field3   int32
	Field280 int32
	Field6   *int32
	Field22  int64
	Field4   string
	Field5   []uint64
	Field59  *bool
	Field7   string
	Field16  int32
	Field130 *int32
	Field12  *bool
	Field17  *bool
	Field13  *bool
	Field14  *bool
	Field104 *int32
	Field100 *int32
	Field101 *int32
	Field102 string
	Field103 string
	Field29  *int32
	Field30  *bool
	Field60  *int32
	Field271 *int32
	Field272 *int32
	Field150 int32
	Field23  *int32
	Field24  *bool
	Field25  *int32
	Field78  bool
	Field67  *int32
	Field68  int32
	Field128 *int32
	Field129 *string
	Field131 *int32
}

func prepareArgs() *largeMessage {
	b := true
	var i int32 = 100000
	var s = "许多往事在眼前一幕一幕，变的那麼模糊"

	var args largeMessage

	v := reflect.ValueOf(&args).Elem()
	num := v.NumField()
	for k := 0; k < num; k++ {
		field := v.Field(k)
		if field.Type().Kind() == reflect.Ptr {
			switch v.Field(k).Type().Elem().Kind() {
			case reflect.Int, reflect.Int32, reflect.Int64:
				field.Set(reflect.ValueOf(&i))
			case reflect.Bool:
				field.Set(reflect.ValueOf(&b))
			case reflect.String:
				field.Set(reflect.ValueOf(&s))
			}
		} else {
			switch field.Kind() {
			case reflect.Int, reflect.Int32, reflect.Int64:
				field.SetInt(100000)
			case reflect.Bool:
				field.SetBool(true)
			case reflect.String:
				field.SetString(s)
			}
		}

	}
	return &args
}

func prepareArenaArgs(ar *Arena) *largeMessage {
	b := true
	var i int32 = 100000
	var s = "那画面太美，我不敢看"

	var args = New[largeMessage](ar)

	v := reflect.ValueOf(args).Elem()
	num := v.NumField()
	for k := 0; k < num; k++ {
		field := v.Field(k)
		if field.Type().Kind() == reflect.Ptr {
			switch v.Field(k).Type().Elem().Kind() {
			case reflect.Int:
				field.Set(reflect.ValueOf(ar.NewInt(int(i))))
			case reflect.Int32:
				field.Set(reflect.ValueOf(ar.NewInt32(int32(i))))
			case reflect.Int64:
				field.Set(reflect.ValueOf(ar.NewInt64(int64(i))))
			case reflect.Bool:
				field.Set(reflect.ValueOf(ar.NewBool(b)))
			case reflect.String:
				field.Set(reflect.ValueOf(ar.NewString(s)))
			}
		} else {
			switch field.Kind() {
			case reflect.Int, reflect.Int32, reflect.Int64:
				field.SetInt(100000)
			case reflect.Bool:
				field.SetBool(true)
			case reflect.String:
				field.SetString(s)
			}
		}

	}
	return args
}

const largeSize = 1000 * 1000 * 1

func TestHeapLargeObjects(t *testing.T) {
	var m = make([]*largeMessage, largeSize)
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

func TestArenaLargeObjects(t *testing.T) {
	var allocator = NewArena(WithChunkSize(1024 * 1024))
	defer allocator.Reset()

	var m = make([]*largeMessage, largeSize)
	for i := 0; i < largeSize; i++ {
		m[i] = prepareArenaArgs(allocator)
	}
	start := time.Now()
	runtime.GC()
	var memStat runtime.MemStats
	runtime.ReadMemStats(&memStat)
	t.Logf("Arena GC took time: %v, living objects: %d", time.Since(start), memStat.HeapObjects)
	runtime.KeepAlive(m)
}

// --- Additional comprehensive tests ---

func TestNewArenaOptions(t *testing.T) {
	t.Run("WithChunkSize zero", func(t *testing.T) {
		// Zero chunk size should be clamped to minimum (512) internally
		ar := NewArena(WithChunkSize(0))
		assert.NotNil(t, ar)
		ar.Reset()
	})

	t.Run("WithPoolSize zero", func(t *testing.T) {
		ar := NewArena(WithPoolSize(0))
		assert.NotNil(t, ar)
		// Allocate and free should still work; freed chunks are discarded immediately
		ptr := ar.Malloc(16)
		ar.Free(ptr)
		ar.Reset()
	})

	t.Run("WithEnableLock true", func(t *testing.T) {
		ar := NewArena(WithEnableLock(true))
		assert.NotNil(t, ar)
		// Concurrent allocations should not race
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				p := ar.NewInt(42)
				assert.Equal(t, 42, *p)
			}()
		}
		wg.Wait()
		ar.Reset()
	})

	t.Run("WithEnableLock false", func(t *testing.T) {
		ar := NewArena(WithEnableLock(false))
		assert.NotNil(t, ar)
		p := ar.NewString("test")
		assert.Equal(t, "test", *p)
		ar.Reset()
	})

	t.Run("WithMemory custom", func(t *testing.T) {
		var allocs int
		var frees int
		custom := &trackingMemory{
			allocFn: func(size uintptr) []byte {
				allocs++
				return make([]byte, size)
			},
			freeFn: func(_ []byte) {
				frees++
			},
		}
		ar := NewArena(WithMemory(custom))
		assert.NotNil(t, ar)

		// The custom allocator is used for all internal allocations.
		// Allocate an oversized block: it becomes a separate chunkBlock,
		// and when freed and the pool is exhausted, memory.Free is called.
		// Use WithPoolSize(0) so freed blocks are immediately released.
		ar2 := NewArena(WithMemory(custom), WithPoolSize(0))
		bigSize := uintptr(8192)
		ptr := ar2.Malloc(bigSize)
		assert.NotNil(t, ptr)
		ar2.Free(ptr) // With poolSize=0, the block is released via memory.Free
		ar.Reset()
		ar2.Reset()
		assert.GreaterOrEqual(t, allocs, 1, "custom allocator should have been called")
		assert.GreaterOrEqual(t, frees, 1, "custom deallocator should have been called")
	})
}

// trackingMemory is a test helper Memory implementation that tracks calls.
type trackingMemory struct {
	allocFn func(size uintptr) []byte
	freeFn  func(m []byte)
}

func (m *trackingMemory) Alloc(size uintptr) []byte {
	return m.allocFn(size)
}

func (m *trackingMemory) Free(b []byte) {
	m.freeFn(b)
}

func TestMallocZeroPanic(t *testing.T) {
	ar := NewArena()
	assert.Panics(t, func() {
		ar.Malloc(0)
	})
}

func TestMallocAlignment(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	align := unsafe.Sizeof(uintptr(0))

	for _, sz := range []uintptr{1, 3, 7, 13, 100, 255, 1023} {
		ptr := ar.Malloc(sz)
		addr := uintptr(ptr)
		assert.Equal(t, uintptr(0), addr%align, "Malloc(%d) returned misaligned pointer", sz)
		ar.Free(ptr)
	}
}

func TestMallocOversized(t *testing.T) {
	// Default chunk size is 4096 + align; allocate something larger
	ar := NewArena()
	defer ar.Reset()

	bigSize := uintptr(8192)
	ptr := ar.Malloc(bigSize)
	assert.NotNil(t, ptr)

	// Write to the entire block to verify it is usable
	slice := unsafe.Slice((*byte)(ptr), bigSize)
	for i := uintptr(0); i < bigSize; i++ {
		slice[i] = byte(i % 256)
	}
	for i := uintptr(0); i < bigSize; i++ {
		assert.Equal(t, byte(i%256), slice[i])
	}
	ar.Free(ptr)
}

func TestFreeNonArenaPointer(t *testing.T) {
	ar := NewArena()

	heapInt := new(int)
	*heapInt = 42

	assert.Panics(t, func() {
		ar.Free(heapInt)
	})
}

func TestReset(t *testing.T) {
	t.Run("clears state", func(t *testing.T) {
		ar := NewArena()
		p1 := ar.NewInt(10)
		p2 := ar.NewString("hello")
		_ = p1
		_ = p2

		ar.Reset()
		// After Reset the arena's internal state is cleared (current is nil, chunkBlocks empty).
		// A new arena should work normally afterwards.
		ar2 := NewArena()
		p3 := ar2.NewInt(99)
		assert.Equal(t, 99, *p3)
		ar2.Reset()
	})

	t.Run("multiple resets", func(t *testing.T) {
		// Each iteration creates a fresh arena, uses it, and resets it.
		for i := 0; i < 5; i++ {
			ar := NewArena()
			p := ar.NewInt(i)
			assert.Equal(t, i, *p)
			ar.Reset()
		}
	})

	t.Run("reset then allocate", func(t *testing.T) {
		ar := NewArena(WithChunkSize(512))

		// First round
		p1 := ar.Malloc(64)
		assert.NotNil(t, p1)
		ar.Reset()

		// After Reset, create a new arena to verify fresh allocation works
		ar2 := NewArena(WithChunkSize(512))
		p2 := ar2.Malloc(64)
		assert.NotNil(t, p2)
		slice := unsafe.Slice((*byte)(p2), 64)
		for i := range slice {
			slice[i] = byte(i)
		}
		for i := range slice {
			assert.Equal(t, byte(i), slice[i])
		}
		ar2.Reset()
	})
}

func TestConvenienceMethods(t *testing.T) {
	ar := NewArena()
	defer ar.Reset()

	t.Run("Bool", func(t *testing.T) {
		assert.False(t, *ar.NewBool(false))
		assert.True(t, *ar.NewBool(true))
	})

	t.Run("Int", func(t *testing.T) {
		assert.Equal(t, 0, *ar.NewInt(0))
		assert.Equal(t, -1, *ar.NewInt(-1))
		assert.Equal(t, math.MaxInt, *ar.NewInt(math.MaxInt))
	})

	t.Run("String", func(t *testing.T) {
		assert.Equal(t, "", *ar.NewString(""))
		assert.Equal(t, "hello", *ar.NewString("hello"))
		longStr := string(make([]byte, 10000))
		assert.Equal(t, longStr, *ar.NewString(longStr))
	})

	t.Run("Bytes", func(t *testing.T) {
		assert.Empty(t, ar.Bytes([]byte{}))
		assert.Equal(t, []byte{1, 2, 3}, ar.Bytes([]byte{1, 2, 3}))
		big := make([]byte, 8192)
		for i := range big {
			big[i] = byte(i % 256)
		}
		result := ar.Bytes(big)
		assert.Equal(t, big, result)
	})

	t.Run("Float32", func(t *testing.T) {
		assert.Equal(t, float32(0), *ar.NewFloat32(0))
		assert.Equal(t, float32(-1.5), *ar.NewFloat32(-1.5))
		assert.Equal(t, float32(math.MaxFloat32), *ar.NewFloat32(math.MaxFloat32))
	})

	t.Run("Float64", func(t *testing.T) {
		assert.Equal(t, float64(0), *ar.NewFloat64(0))
		assert.Equal(t, -3.14, *ar.NewFloat64(-3.14))
		assert.Equal(t, math.MaxFloat64, *ar.NewFloat64(math.MaxFloat64))
	})
}
