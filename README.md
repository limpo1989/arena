# Arena Memory Allocator
[![Go Reference](https://pkg.go.dev/badge/github.com/limpo1989/arena.svg)](https://pkg.go.dev/github.com/limpo1989/arena)

A high-performance memory allocator for Go that reduces garbage collection (GC) overhead by managing object lifetimes explicitly.

## Features
- **Reduces GC Pressure**: Allocates objects in contiguous chunks, minimizing GC scans.
- **Zero-Allocation APIs**: Methods like `Malloc`, `New`, and `NewSlice` avoid heap allocations.
- **Thread Safety**: Optional spinlock-based synchronization.
- **Customizable**: Configure chunk sizes, memory sources, and pooling behavior.
- **Rich Utilities**: Includes type-safe vectors (`Vector[T]`) and deep-copy helpers.

## Use Cases
- High-throughput services with frequent small object allocations.
- Long-lived objects that benefit from bulk deallocation.
- Latency-sensitive applications needing predictable GC behavior.

## Installation
```bash
go get github.com/limpo1989/arena
```

## Quick Start

```go
package main

import (
	"fmt"

	"github.com/limpo1989/arena"
)

func main() {
	ar := arena.NewArena()
	defer ar.Reset()

	// Allocate primitives
	num := arena.New[int](ar)
	*num = 42
	fmt.Println("print num:", *num)

	// Allocate slices
	slice := arena.NewSlice[string](ar, 0, 10)
	slice = arena.Append(ar, slice, "hello", "world")
	fmt.Println("print slice:", slice)

	// Use vectors
	vec := arena.NewVector[int](ar, 8)
	vec.Append(1, 2, 3)
	fmt.Println("print vec:", vec.At(0), vec.At(1), vec.At(2))
}

// Output:
// print num: 42
// print slice: [hello world]
// print vec: 1 2 3
```

## Performance

Arena reduces GC pauses by:
1. Bulk Allocation: Objects are grouped in chunks, decreasing GC scan count.
2. Lifetime Control: Allocations are freed together via Reset().
3. Reduced Fragmentation: Chunk reuse minimizes heap fragmentation.

### Benchmark (vs. Go heap):

```
TestHeapLargeObjects    Heap GC took time: 121.2361ms, living objects: 13108293
TestArenaLargeObjects   Arena GC took time: 13.0534ms, living objects: 8031
```

## Caveats
* Manual Management: You must call Free or Reset to reclaim memory.
* No GC Integration: Arena-allocated objects are ignored by Go's GC.
* Pointer Safety: Mixing Arena and heap pointers may cause leaks/errors.
* Unsupported Types: map, channel, func

### Bad Case

**Mixing Arena and heap pointers**

```go
func main() {
    ar := NewArena()
    defer ar.Reset()
    
    type subject struct {
        id  int32
        age *int32
    }
    
    p := New[subject](ar)
    p.age = new(int32) // Bad：store heap pointer
    *p.age = 100
    
    // You can setting a finalizer, it is possible to observe that the heap memory pointer will be reclaimed before the Arena ends
    //
    runtime.SetFinalizer(p.age, func (p *int32) {
        fmt.Println("subject.age released")
    })
    
    runtime.GC()
    fmt.Println("gc finished 1")
    runtime.GC()
    fmt.Println("gc finished 2")
    
    // Undefined access, in fact, this part of memory has already been reclaimed by GC.
    // Although it might be possible to access it here, it will result in undefined behavior
    //
    *p.age = 99 // Undefined access 
}
```

### License

The `arena` is released under version 2.0 of the Apache License.