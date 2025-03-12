package arena

import (
	"bytes"
	"fmt"
	"math/rand"
	"reflect"
	"runtime"
	"testing"
	"time"
)

func TestArena(t *testing.T) {

	ar := NewArena()

	for i := 0; i < 1000; i++ {
		if "hello" != *ar.String("hello") {
			t.Fatalf("missmatch")
		}

		var rv = rand.Int()

		if rv > 100 != *ar.Bool(rv > 100) {
			t.Fatalf("missmatch")
		}

		if rv != *ar.Int(rv) {
			t.Fatalf("missmatch")
		}

		if int8(rv) != *ar.Int8(int8(rv)) {
			t.Fatalf("missmatch")
		}

		if int16(rv) != *ar.Int16(int16(rv)) {
			t.Fatalf("missmatch")
		}

		if int32(rv) != *ar.Int32(int32(rv)) {
			t.Fatalf("missmatch")
		}

		if int64(rv) != *ar.Int64(int64(rv)) {
			t.Fatalf("missmatch")
		}

		if uint(rv) != *ar.Uint(uint(rv)) {
			t.Fatalf("missmatch")
		}

		if uint8(rv) != *ar.Uint8(uint8(rv)) {
			t.Fatalf("missmatch")
		}

		if uint16(rv) != *ar.Uint16(uint16(rv)) {
			t.Fatalf("missmatch")
		}

		if uint32(rv) != *ar.Uint32(uint32(rv)) {
			t.Fatalf("missmatch")
		}

		if uint64(rv) != *ar.Uint64(uint64(rv)) {
			t.Fatalf("missmatch")
		}

		var s = []byte("hello arena")
		if 0 != bytes.Compare(s, ar.Bytes(s)) {
			t.Fatalf("missmatch")
		}

		var f32 = rand.Float32() * 1000.0
		if f32 != *ar.Float32(f32) {
			t.Fatalf("missmatch")
		}

		var f64 = rand.Float64() * 1000.0
		if f64 != *ar.Float64(f64) {
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
	i32 := ar.Int32(1001)

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
				field.Set(reflect.ValueOf(ar.Int(int(i))))
			case reflect.Int32:
				field.Set(reflect.ValueOf(ar.Int32(int32(i))))
			case reflect.Int64:
				field.Set(reflect.ValueOf(ar.Int64(int64(i))))
			case reflect.Bool:
				field.Set(reflect.ValueOf(ar.Bool(b)))
			case reflect.String:
				field.Set(reflect.ValueOf(ar.String(s)))
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

const largeSize = 1024 * 1024 * 5

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
