# Arena Memory Allocator
[![Go Reference](https://pkg.go.dev/badge/github.com/limpo1989/arena.svg)](https://pkg.go.dev/github.com/limpo1989/arena)

> **WARNING: Experimental** — This project is in early development. The API is not stabilized and may change without notice. **Do not use in production.**

A high-performance memory allocator for Go that reduces garbage collection (GC) overhead by managing object lifetimes explicitly.

## Features
- **Reduces GC Pressure**: Allocates objects in contiguous chunks, minimizing GC scans.
- **Zero-Allocation APIs**: Methods like `Malloc`, `New`, and `NewSlice` avoid heap allocations.
- **Thread Safety**: Optional spinlock-based synchronization.
- **Customizable**: Configure chunk sizes, memory sources, and pooling behavior.
- **Arena-Native Containers**: `Map[K,V]` and `Vector[T]` with all data stored in arena memory.
- **Type Safety**: Compile-time type validation rejects unsupported types (map, chan, func).

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

	// Use maps
	m := arena.NewMap[string, int](ar, 8)
	m.Put("answer", 42)
	if v, ok := m.Get("answer"); ok {
		fmt.Println("print map:", v)
	}
}
```

## Memory Safety Rules

Arena memory is allocated as raw `[]byte` slices. **Go GC does not scan arena memory contents for pointers.** The following rules govern the boundary between Go heap and arena memory.

### WARNING: Never assign heap pointers to arena object fields

Arena object fields must be created through `DeepCopy` or arena APIs. Direct assignment may write a heap pointer into arena memory. Since GC cannot trace this reference, the heap target may be collected, producing a dangling pointer. This issue does not fail immediately — it causes random crashes at runtime and is extremely difficult to diagnose.

```
❌ p.Field = &heapObj          // DANGEROUS: arena holds a heap pointer
❌ p.Field = heapSlice         // DANGEROUS: backing array is on heap
❌ p.Field = &SomeType{}       // DANGEROUS: implicit heap allocation

✓ p.Field = DeepCopy(ar, val) // SAFE: value is copied into arena
✓ p.Field = ar.NewInt(42)     // SAFE: created via arena API
✓ p.Field = New[T](ar)        // SAFE: allocated in arena
```

### Rule 1: Pointers flow from Heap into Arena, never the reverse

```
              Pointer direction
  ┌───────┐   ────────▶   ┌───────┐   ────────▶   ┌───────┐
  │       │    SAFE ✓     │       │    SAFE ✓     │       │
  │ Heap  │──────────────▶│ Arena │──────────────▶│ Arena │
  │       │               │       │               │       │
  └───────┘               └───────┘               └───────┘
      ▲                       │
      └────── UNSAFE ✗ ───────┘
          Arena → Heap pointer is invisible to GC
```

### Rule 2: Arena objects must be self-contained

All data reachable from an arena object must also reside in arena memory. Use `DeepCopy` to copy values into arena instead of assigning references.

```
  Arena Memory (self-contained)
  ┌─────────────────────────────────────────────┐
  │  GameData struct                            │
  │  ┌───────────────────────────────────────┐  │
  │  │ Name string ──────▶ bytes in arena    │  │
  │  │ Tags  *Map ───────▶ Map in arena      │  │
  │  │ Items *Vector ────▶ Vector in arena   │  │
  │  └───────────────────────────────────────┘  │
  │         All references point inward          │
  └─────────────────────────────────────────────┘
```

### Rule 3: Arena struct and management data stay on Go heap

The `Arena` object, `chunkBlock` structs, and internal maps are always on the Go heap. This is necessary — they hold `[]byte` references to arena chunks, which is how GC keeps chunk data alive.

```
  Go Heap (GC-managed)
  ┌──────────────────────────────┐
  │ Arena struct                 │
  │  chunkBlocks ──▶ []byte refs │──▶ chunk data
  │  freelist    ──▶ []byte refs │
  └──────────────────────────────┘
       ▲              ▲
       │              │
  Map.allocator  Vector.allocator   ← arena→heap refs, safe because
                                        Arena always outlives its containers
```

### Rule 4: Arena lifetime governs all pointers

All pointers into arena memory become invalid after `Arena.Reset()`. Using them after Reset is undefined behavior.

```
  NewArena()              Use phase                 Reset()
     │                       │                        │
     ▼                       ▼                        ▼
  ┌──────┐             ┌──────────┐            ┌──────────────┐
  │ Init │──▶ ... ──── │ Normal   │──▶ ... ──▶│ All pointers │
  │      │             │ usage    │            │ invalidated  │
  └──────┘             └──────────┘            └──────────────┘
```

### Supported and Rejected Types

| Type | Status | Notes |
|------|--------|-------|
| `bool`, `int*`, `uint*`, `float*` | ✓ Supported | Value types, direct copy |
| `string` | ✓ Supported | Bytes copied into arena |
| `*T` (pointer) | ✓ Supported | Deep-copied recursively |
| `[]T` (slice) | ✓ Supported | Backing array in arena |
| `[N]T` (array) | ✓ Supported | Elements deep-copied |
| `struct { ... }` | ✓ Supported | Fields validated recursively |
| `*arena.Map[K,V]` | ✓ Supported | Arena-native hash map |
| `*arena.Vector[T]` | ✓ Supported | Arena-native dynamic array |
| `map[K]V` | ✗ Rejected | Use `arena.Map[K,V]` |
| `chan T` | ✗ Rejected | Not arena-compatible |
| `func(...)` | ✗ Rejected | Not arena-compatible |
| `sync.Mutex` | ✗ Rejected | Not arena-compatible |

Type validation runs automatically at API entry points (`New`, `DeepCopy`, `NewSlice`, `NewMap`, `NewVector`) with clear error messages. Use `arena.Validate[T]()` for explicit checking.

## API

### Core Allocation

```go
ar := arena.NewArena(
    arena.WithChunkSize(4096),
    arena.WithPoolSize(128),
    arena.WithEnableLock(true),
)

p := arena.New[MyStruct](ar)       // Allocate a struct
s := arena.NewSlice[int](ar, 0, 8) // Allocate a slice
s = arena.Append(ar, s, 1, 2, 3)  // Append to arena slice

cp := arena.DeepCopy(ar, original) // Deep copy into arena

ar.Free(p)  // Free individual allocation
ar.Reset()  // Free all allocations at once
```

### Map[K,V]

```go
m := arena.NewMap[string, int](ar, 16)
m.Put("key", 42)
v, ok := m.Get("key") 
m.Remove("key")
m.Len()
for k, v := range m.Iter() { /* ... */ }
m.Clear()
```

### Vector[T]

```go
v := arena.NewVector[int](ar, 8)
v.Append(1, 2, 3)
v.At(0)           // 1
v.Remove(2)       // remove by value
v.RemoveIdx(0)    // remove by index
v.Index(3)        // find by value, returns -1 if not found
v.Len(), v.Cap()
for i, val := range v.Iter() { /* ... */ }
v.Clear()
```

## Performance

Arena reduces GC pauses by:
1. Bulk Allocation: Objects are grouped in chunks, decreasing GC scan count.
2. Lifetime Control: Allocations are freed together via Reset().
3. Reduced Fragmentation: Chunk reuse minimizes heap fragmentation.

### Benchmark (vs. Go heap)

Tested on **Apple M4 Pro**, allocating 1M `largeMessage` objects (40+ fields each):

| Metric | Go Heap | Arena | Improvement |
|--------|---------|-------|-------------|
| GC Scan Time | 25.6ms | 2.7ms | **9.4x faster** |
| Living Objects | 2,500,702 | 2,288 | **1,093x fewer** |

## License

The `arena` is released under version 2.0 of the Apache License.
