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
	"reflect"
	"sync"
	"unsafe"

	"github.com/limpo1989/arena/internal"
)

const __align = unsafe.Sizeof(uintptr(0))

// chunkBlock represents a contiguous memory block managed by the Arena.
type chunkBlock struct {
	ref int64
	off uintptr
	mem []byte
}

// arenaOptions holds configuration settings for the Arena allocator
type arenaOptions struct {
	chunkSize uintptr
	poolSize  int
	locker    sync.Locker
	memory    Memory
}

// Option defines a function type for configuring Arena parameters
type Option func(*arenaOptions)

// WithChunkSize sets the base allocation size for memory chunks.
// Larger values reduce allocation frequency but may increase waste.
// Minimum size is automatically aligned to system pointer size.
func WithChunkSize(chunkSize uintptr) Option {
	return func(o *arenaOptions) {
		o.chunkSize = chunkSize
	}
}

// WithPoolSize configures the maximum number of reusable chunks retained in the free list.
// Higher values improve reuse at the cost of increased memory retention.
func WithPoolSize(poolSize int) Option {
	return func(o *arenaOptions) {
		o.poolSize = poolSize
	}
}

// WithEnableLock enables thread-safe operation using a spinlock.
// Required when the Arena is accessed concurrently.
// When enabled, adds ~5-15ns overhead per allocation.
func WithEnableLock(enableLock bool) Option {
	return func(o *arenaOptions) {
		o.locker = new(internal.SpinLock)
	}
}

// WithMemory specifies a custom memory allocator implementing the Memory interface.
// Allows integration with alternative memory sources (e.g., mmap, cgo, shm).
// Default: heapMemory (standard Go allocations).
func WithMemory(memory Memory) Option {
	return func(o *arenaOptions) {
		o.memory = memory
	}
}

// Memory represents a alternative memory sources (e.g., mmap, cgo, shm).
type Memory interface {
	Alloc(size uintptr) []byte
	Free(m []byte)
}

// Arena manages memory chunks and provides allocation services with reduced GC overhead.
// It maintains reusable memory blocks and handles alignment automatically.
type Arena struct {
	_           [0]sync.Mutex // NoCopy
	locker      sync.Locker
	memory      Memory
	chunkSize   uintptr
	minHoleSize uintptr
	poolSize    int
	chunkBlocks map[uintptr]*chunkBlock
	current     *chunkBlock
	freelist    []*chunkBlock
}

// NewArena creates a new Arena instance with customizable options.
// Options can configure chunk size, memory pool size, locking behavior, and memory source.
func NewArena(ops ...Option) *Arena {
	var opts = arenaOptions{
		chunkSize: 1024,
		poolSize:  64,
		locker:    nopLocker{},
		memory:    heapMemory{},
	}
	for _, op := range ops {
		op(&opts)
	}

	ar := &Arena{}
	ar.locker = opts.locker
	ar.memory = opts.memory
	ar.chunkSize = fixSize(max(512, opts.chunkSize)) + __align
	ar.minHoleSize = fixSize(max(256, ar.chunkSize/5)) + __align
	ar.poolSize = opts.poolSize
	ar.current = ar.malloc(ar.chunkSize)
	ar.chunkBlocks = make(map[uintptr]*chunkBlock, 8)
	return ar
}

// Reset clears all allocated chunks and resets the Arena to its initial state.
// Existing pointers become invalid after this operation.
func (ar *Arena) Reset() {
	ar.locker.Lock()
	defer ar.locker.Unlock()

	for _, block := range ar.chunkBlocks {
		ar.memory.Free(block.mem)
	}

	ar.chunkBlocks = make(map[uintptr]*chunkBlock)
	ar.freelist = nil
	ar.current = nil
}

// Free releases a previously allocated memory block.
// The pointer must belong to this Arena.
func (ar *Arena) Free(ptrT any) {
	ar.locker.Lock()
	defer ar.locker.Unlock()

	pVal := reflect.ValueOf(ptrT)

	visited := make(map[uintptr]struct{})
	deepFree(ar, pVal, visited)
}

func (ar *Arena) freePointer(ptr unsafe.Pointer) {
	if !ar.isManaged(uintptr(ptr)) {
		panic("ptr must be managed by arena")
	}

	ptr = unsafe.Pointer(uintptr(ptr) - __align) // 后退一个指针大小
	chunkPtr := *(*uintptr)(ptr)                 // 获取存储的源地址
	block := ar.current
	if (uintptr)(unsafe.Pointer(unsafe.SliceData(ar.current.mem))) != chunkPtr {
		var ok bool
		if block, ok = ar.chunkBlocks[chunkPtr]; !ok {
			panic(fmt.Errorf("pointer not malloc from Arena: %p", ptr))
		}
	}

	// 判断是否还有引用
	if block.ref--; block.ref == 0 {
		block.off = 0
		if ar.current != block {
			delete(ar.chunkBlocks, chunkPtr)
			// 追加到可用列表
			if len(ar.freelist) < ar.poolSize {
				ar.freelist = append(ar.freelist, block)
			} else {
				// 释放内存
				ar.memory.Free(block.mem)
				// 丢弃的块，清理底层数组指针
				block.mem = nil
			}
		}
	}
}

// Malloc allocates a memory block of the given size. Returned pointer is aligned.
// Panics if size is zero.
func (ar *Arena) Malloc(sz uintptr) unsafe.Pointer {
	if sz == 0 {
		panic("malloc size must be positive")
	}

	ar.locker.Lock()
	defer ar.locker.Unlock()

	// 计算可用长度
	availableBytes := uintptr(cap(ar.current.mem)) - ar.current.off
	// 计算实际需要申请的长度
	requiredBytes := fixSize(sz) + __align

	// 请求大小超过初始块大小的直接分配
	// 当前块剩余可用大小超过允许浪费的字节 并且小于需求大小的也直接分配 (将剩余可用空间大于浪费大小块那个块留作后用)
	if requiredBytes > ar.chunkSize || (availableBytes < requiredBytes && availableBytes >= ar.minHoleSize) {
		block := ar.malloc(requiredBytes)
		block.ref = 1
		ptr := unsafe.Pointer(unsafe.SliceData(block.mem))
		*(*uintptr)(ptr) = (uintptr)(ptr)
		ar.chunkBlocks[(uintptr)(ptr)] = block
		return unsafe.Add(ptr, __align)
	}

	if availableBytes < requiredBytes {
		// 此时current剩余字节将被浪费，浪费的字节数最多不超过 ar.minHoleSize
		ptr := unsafe.Pointer(unsafe.SliceData(ar.current.mem))
		ar.chunkBlocks[(uintptr)(ptr)] = ar.current
		ar.current = ar.malloc(ar.chunkSize)
	}

	offset := ar.current.off
	// mark alloc
	ar.current.off += requiredBytes
	ar.current.ref++

	if ar.current.off > uintptr(cap(ar.current.mem)) {
		panic("pointer out of bounds")
	}

	start := unsafe.Pointer(unsafe.SliceData(ar.current.mem))
	ptr := unsafe.Add(start, offset)
	*(*uintptr)(ptr) = (uintptr)(start)
	return unsafe.Add(ptr, __align)
}

func (ar *Arena) malloc(sz uintptr) *chunkBlock {
	// 优先复用内存
	if len(ar.freelist) > 0 {
		if chunk := ar.selectChunk(sz); nil != chunk {
			return chunk
		}
	}

	m := ar.memory.Alloc(sz)
	if nil == m || uintptr(cap(m)) < sz {
		return nil
	}
	return &chunkBlock{off: 0, ref: 0, mem: m}
}

func (ar *Arena) selectChunk(sz uintptr) *chunkBlock {

	// 选择最佳大小的块
	// freelist 通常不会设置太大，这里直接遍历查找
	var selected *chunkBlock
	var idx = -1
	for i, block := range ar.freelist {
		if cap(block.mem) >= int(sz+__align) && (nil == selected || cap(block.mem) < cap(selected.mem)) {
			selected = block
			idx = i
		}
	}

	// 没有选中任何可用块
	if -1 == idx || nil == selected {
		return nil
	}

	// fast-remove
	var lastIdx = len(ar.freelist) - 1
	ar.freelist[idx], ar.freelist[lastIdx] = ar.freelist[lastIdx], ar.freelist[idx]
	ar.freelist[lastIdx] = nil
	ar.freelist = ar.freelist[:lastIdx]
	return selected
}

// Bool allocates a boolean in the Arena and initializes it with the given value.
func (ar *Arena) Bool(v bool) *bool {
	p := New[bool](ar)
	*p = v
	return p
}

// Int allocates an integer in the Arena and initializes it with the given value.
func (ar *Arena) Int(v int) *int {
	p := New[int](ar)
	*p = v
	return p
}

// Uint allocates an uint in the Arena and initializes it with the given value.
func (ar *Arena) Uint(v uint) *uint {
	p := New[uint](ar)
	*p = v
	return p
}

// Int8 allocates an int8 in the Arena and initializes it with the given value.
func (ar *Arena) Int8(v int8) *int8 {
	p := New[int8](ar)
	*p = v
	return p
}

// Uint8 allocates an uint8 in the Arena and initializes it with the given value.
func (ar *Arena) Uint8(v uint8) *uint8 {
	p := New[uint8](ar)
	*p = v
	return p
}

// Int16 allocates an int16 in the Arena and initializes it with the given value.
func (ar *Arena) Int16(v int16) *int16 {
	p := New[int16](ar)
	*p = v
	return p
}

// Uint16 allocates an uint16 in the Arena and initializes it with the given value.
func (ar *Arena) Uint16(v uint16) *uint16 {
	p := New[uint16](ar)
	*p = v
	return p
}

// Int32 allocates an int32 in the Arena and initializes it with the given value.
func (ar *Arena) Int32(v int32) *int32 {
	p := New[int32](ar)
	*p = v
	return p
}

// Uint32 allocates an uint32 in the Arena and initializes it with the given value.
func (ar *Arena) Uint32(v uint32) *uint32 {
	p := New[uint32](ar)
	*p = v
	return p
}

// Int64 allocates an int64 in the Arena and initializes it with the given value.
func (ar *Arena) Int64(v int64) *int64 {
	p := New[int64](ar)
	*p = v
	return p
}

// Uint64 allocates an uint64 in the Arena and initializes it with the given value.
func (ar *Arena) Uint64(v uint64) *uint64 {
	p := New[uint64](ar)
	*p = v
	return p
}

// Float32 allocates an float32 in the Arena and initializes it with the given value.
func (ar *Arena) Float32(v float32) *float32 {
	p := New[float32](ar)
	*p = v
	return p
}

// Float64 allocates an float64 in the Arena and initializes it with the given value.
func (ar *Arena) Float64(v float64) *float64 {
	p := New[float64](ar)
	*p = v
	return p
}

// String allocates and initializes a string in the Arena by copying the input.
func (ar *Arena) String(v string) (s *string) {
	return DeepCopy(ar, v)
}

// Bytes allocates a byte slice in the Arena and copies the input data.
func (ar *Arena) Bytes(v []byte) (b []byte) {
	// 分配内存
	b = NewSlice[byte](ar, len(v), cap(v))
	// 拷贝数据
	copy(b[:], v)
	return
}

func (ar *Arena) isManaged(ptr uintptr) bool {
	ar.locker.Lock()
	defer ar.locker.Unlock()

	start := (uintptr)(unsafe.Pointer(unsafe.SliceData(ar.current.mem)))
	end := start + uintptr(cap(ar.current.mem))
	if ptr >= start && ptr < end {
		return true
	}

	for _, chunk := range ar.chunkBlocks {
		start = (uintptr)(unsafe.Pointer(unsafe.SliceData(chunk.mem)))
		end = start + uintptr(cap(chunk.mem))
		if ptr >= start && ptr < end {
			return true
		}
	}
	return false
}

func isArenaManagedSlice[T any](ar *Arena, s []T) bool {
	// 获取底层数组指针
	dataPtr := uintptr(unsafe.Pointer(unsafe.SliceData(s)))
	return ar.isManaged(dataPtr)
}

// DeepCopy allocates an object of type T from the Arena and performs a deep copy of the source object v into it.
// The underlying memory is managed by the Arena.
func DeepCopy[T any](ar *Arena, v T) *T {
	p := New[T](ar)
	src := reflect.ValueOf(v)
	dst := reflect.ValueOf(p).Elem()
	visited := make(map[uintptr]reflect.Value)
	deepCopy(ar, src, dst, visited)
	return p
}

// NewSlice allocates a slice of type T with the specified length and capacity.
// The underlying memory is managed by the Arena.
func NewSlice[T any](ar *Arena, length, capacity int) (result []T) {
	if length > capacity {
		capacity = length
	}

	if capacity < 0 {
		panic("invalid capacity")
	}

	if capacity == 0 {
		return
	}

	// 计算总需要内存 头 + 数据长度
	elemSize := sizeOf[T]()
	sz := elemSize * uintptr(capacity)
	ptr := ar.Malloc(sz)

	// 初始化slice数据结构
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&result))
	sh.Data = uintptr(ptr)
	sh.Len = length
	sh.Cap = capacity
	return
}

// Append extends a slice managed by the Arena, reallocating if necessary.
// The underlying memory is managed by the Arena.
func Append[T any](ar *Arena, src []T, values ...T) []T {
	// 1. 验证源切片是否来自arena
	if nil != src {
		if !isArenaManagedSlice(ar, src) {
			panic("source slice must be managed by arena")
		}
	}

	// 2. 计算需要扩容的情况
	required := len(src) + len(values)
	if required <= cap(src) {
		// 直接追加到现有容量（深拷贝）
		newSlice := src[:len(src)]
		return deepCopySliceArena(ar, newSlice, values)
	}

	defer func() {
		// 针对原有的数据区进行回收
		if nil != src {
			ar.Free(src)
		}
	}()

	// 3. 执行扩容策略
	newCap := calculateNewCap(len(src), len(values))
	newSlice := NewSlice[T](ar, len(src), newCap)

	// 4. 深拷贝原始数据
	if len(src) > 0 {
		deepCopySliceArena(ar, newSlice[:0], src)
	}

	// 5. 追加新元素（深拷贝）
	return deepCopySliceArena(ar, newSlice, values)
}

// New allocates memory for a type T and returns a pointer to it.
// The object's lifetime is tied to the Arena.
func New[T any](ar *Arena) *T {
	size := sizeOf[T]()
	return (*T)(ar.Malloc(size))
}

func sizeOf[T any]() uintptr {
	var zero T
	return unsafe.Sizeof(zero)
}

func calculateNewCap(current, appendLen int) int {
	minGrowth := current + appendLen
	newCap := current * 5 / 4 // 25% growth
	if newCap < minGrowth {
		newCap = minGrowth
	}
	return newCap
}

func deepCopySliceArena[T any](ar *Arena, dst []T, values []T) []T {
	if n := cap(dst) - len(dst); n < len(values) {
		panic(fmt.Errorf("no available capacity to copied: cap: %d, len: %d, avaliable: %d, required: %d", cap(dst), len(dst), n, len(values)))
	}
	visited := make(map[uintptr]reflect.Value)
	idx := len(dst)
	newDst := dst[:idx+len(values)]
	for i := range values {
		srcVal := reflect.ValueOf(values[i])
		dstVal := reflect.ValueOf(&newDst[idx+i]).Elem()
		deepCopy(ar, srcVal, dstVal, visited)
	}
	return newDst
}

func deepCopy(
	ar *Arena,
	src, dst reflect.Value,
	visited map[uintptr]reflect.Value,
) {
	switch src.Kind() {
	case reflect.Map, reflect.Chan, reflect.Func:
		panic(fmt.Sprintf("unsupported type: %s", src.Type()))
	default:
		if !src.CanSet() {
			src = patchValue(src)
		}

		if !dst.CanSet() {
			dst = patchValue(dst)
		}

		if src.IsZero() {
			return
		}
	}

	// 处理循环引用
	if src.Kind() == reflect.Ptr && src.CanAddr() {
		addr := src.Pointer()
		if exist, ok := visited[addr]; ok {
			dst.Set(exist)
			return
		}
		visited[addr] = dst
	}

	switch src.Kind() {
	case reflect.Ptr:
		if src.IsNil() {
			return
		}

		// 创建新指针并递归拷贝
		elemType := src.Type().Elem()
		newPtr := ar.Malloc(elemType.Size())
		dst.Set(reflect.NewAt(elemType, newPtr))
		deepCopy(ar, src.Elem(), dst.Elem(), visited)
	case reflect.Array:
		for i := 0; i < src.Len(); i++ {
			deepCopy(ar, src.Index(i), dst.Index(i), visited)
		}
	case reflect.Slice:
		if src.IsNil() {
			return
		}

		// 创建新切片
		elemType := src.Type().Elem()
		elemSize := elemType.Size()
		length := src.Len()
		capacity := src.Cap()

		// 分配内存并构建切片头
		ptr := ar.Malloc(uintptr(capacity) * elemSize)
		sh := reflect.SliceHeader{
			Data: (uintptr)(ptr),
			Len:  length,
			Cap:  capacity,
		}

		newSlice := reflect.NewAt(src.Type(), unsafe.Pointer(&sh)).Elem()
		dst.Set(newSlice)

		// 递归拷贝元素
		for i := 0; i < length; i++ {
			deepCopy(ar,
				src.Index(i),
				newSlice.Index(i),
				visited,
			)
		}

	case reflect.String:
		// 将字符串内容拷贝到Arena
		str := src.String()
		b := ar.Bytes([]byte(str))
		dst.SetString(unsafe.String(unsafe.SliceData(b), len(b)))

	case reflect.Struct:
		// 递归处理结构体字段
		for i := 0; i < src.NumField(); i++ {
			srcField := src.Field(i)
			dstField := dst.Field(i)

			deepCopy(ar,
				srcField,
				dstField,
				visited,
			)
		}

	case reflect.Interface:
		if src.IsNil() {
			return
		}
		// 创建接口值的拷贝
		elem := src.Elem()
		newElem := reflect.New(elem.Type()).Elem()
		deepCopy(ar, elem, newElem, visited)
		dst.Set(newElem)

	default:
		// 直接拷贝值类型
		dst.Set(src)
	}
}

func deepFree(
	ar *Arena,
	src reflect.Value,
	visited map[uintptr]struct{},
) {
	switch src.Kind() {
	case reflect.Map, reflect.Chan, reflect.Func:
		panic(fmt.Sprintf("unsupported type: %s", src.Type()))
	default:
		if !src.CanSet() {
			src = patchValue(src)
		}

		if src.IsZero() {
			return
		}
	}

	// 处理循环引用
	if src.Kind() == reflect.Ptr {
		addr := src.Pointer()
		if _, exists := visited[addr]; exists {
			return
		}
		visited[addr] = struct{}{}
	}

	switch src.Kind() {
	case reflect.Ptr:
		if src.IsNil() {
			return
		}

		// 先递归处理指针指向的值
		deepFree(ar, src.Elem(), visited)

		// 释放自身
		ar.freePointer(unsafe.Pointer(src.Pointer()))

	case reflect.Struct:
		for i := 0; i < src.NumField(); i++ {
			field := src.Field(i)
			deepFree(ar, field, visited)
		}

	case reflect.Slice:
		if src.IsNil() {
			return
		}

		// 处理每个元素
		for i := 0; i < src.Len(); i++ {
			deepFree(ar, src.Index(i), visited)
		}

		// 处理底层数组指针
		ar.freePointer(unsafe.Pointer(src.Pointer()))

	case reflect.Array:
		for i := 0; i < src.Len(); i++ {
			deepFree(ar, src.Index(i), visited)
		}
	case reflect.String:
		// 处理底层数组指针
		ar.freePointer(unsafe.Pointer(src.Pointer()))

	case reflect.Interface:
		if src.IsNil() {
			return
		}
		// 递归处理接口值
		deepFree(ar, src.Elem(), visited)
	case reflect.UnsafePointer:
		// 释放指针
		ar.freePointer(unsafe.Pointer(src.Pointer()))
	default:
		// 其他基础类型无需处理
	}
}

func deepEqual[T any](a, b T) bool {
	return reflect.DeepEqual(a, b)
}

func fixSize(sz uintptr) uintptr {
	return (sz + __align - 1) &^ (__align - 1)
}

type heapMemory struct{}

func (h heapMemory) Alloc(size uintptr) []byte {
	return make([]byte, size)
}

func (h heapMemory) Free(m []byte) {
}

type nopLocker struct{}

func (n nopLocker) Lock() {
}

func (n nopLocker) Unlock() {
}
