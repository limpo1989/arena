# protoc-gen-arena

> **WARNING: Experimental** — This project is in early development. The API is not stabilized and may change without notice. **Do not use in production.**

A [protoc](https://grpc.io/docs/protoc-installation/) plugin that generates Go code which allocates all Protocol Buffer objects in arena memory, eliminating garbage collection overhead for protobuf workloads.

Binary-compatible with standard protobuf — arena-generated types can interoperate with `google.golang.org/protobuf` on the wire.

## How It Works

```
                    protoc pipeline
  ┌──────────┐    ┌──────────────────┐    ┌─────────────────────────┐
  │  .proto   │───▶│  protoc          │───▶│  xxx_arena.go           │
  │ (proto2)  │    │  --arena_out=... │    │  ├─ Plain Go structs    │
  └──────────┘    └──────────────────┘    │  ├─ Size()              │
                                          │  ├─ Marshal/MarshalTo   │
                                          │  └─ UnmarshalXxx(ar)    │
                                          └─────────────────────────┘
                                                 │
                              imports             │
                              ┌───────────────────┘
                              ▼
              ┌───────────────────────────────┐
              │  github.com/limpo1989/arena   │
              │  ├─ arena.go  (allocator)     │
              │  ├─ map.go    (arena.Map)     │
              │  └─ protowire/ (codec)        │
              └───────────────────────────────┘
```

All unmarshalled objects, strings, bytes, and sub-messages are allocated inside an [`arena.Arena`](https://pkg.go.dev/github.com/limpo1989/arena). When you call `ar.Reset()`, every object is freed at once — no individual GC per object.

## Installation

```bash
go install github.com/limpo1989/arena/cmd/protoc-gen-arena@latest
```

## Quick Start

### 1. Write a proto2 schema

```protobuf
syntax = "proto2";
package example;
option go_package = "github.com/yourorg/yourproject/pb";

message Player {
  optional string name = 1;
  optional int32  score = 2;
  repeated string tags = 3;
  map<string, int32> attributes = 4;
}
```

### 2. Generate Go code

```bash
protoc \
  --arena_out=. \
  --arena_opt=paths=source_relative \
  player.proto
```

This produces `player_arena.go` in the same directory.

### 3. Use the generated code

```go
package main

import (
    "fmt"
    "github.com/limpo1989/arena"
    pb "github.com/yourorg/yourproject/pb"
)

func main() {
    ar := arena.NewArena()
    defer ar.Reset()

    // Unmarshal from bytes (e.g., received over network)
    data := []byte{ /* protobuf-encoded bytes */ }
    player, err := pb.UnmarshalPlayer(data, ar)
    if err != nil {
        panic(err)
    }

    // Direct field access
    fmt.Println(player.GetName())
    fmt.Println(player.GetScore())

    // Marshal back to bytes
    out, err := player.Marshal()
    if err != nil {
        panic(err)
    }
    fmt.Printf("encoded %d bytes\n", len(out))
}
```

## Generated Code

For each message in the proto file, the plugin generates:

### Flat Go Structs

Fields are plain Go types — no `proto.Message` interface, no reflection overhead.

```go
// Proto:  optional string name = 1;
// Go:
Name *string

// Proto:  repeated string tags = 3;
// Go:
Tags []string

// Proto:  map<string, int32> attributes = 4;
// Go:
Attributes *arena.Map[string, int32]

// Proto:  optional Inner nested = 5;
// Go:
Nested *Inner
```

### Getters & Has Methods

Each `optional` field gets a `GetXxx()` method that returns the zero value when unset, and a `HasXxx()` method for presence detection.

```go
func (m *Player) GetName() string {
    if m != nil && m.Name != nil { return *m.Name }
    return ""
}

func (m *Player) HasName() bool {
    return m != nil && m.Name != nil
}
```

### Size / Marshal / Unmarshal

```go
// Compute serialized size without allocating.
func (m *Player) Size() int

// Serialize into a pre-allocated buffer (zero allocation).
func (m *Player) MarshalTo(buf []byte) (int, error)

// Allocate and return a new byte slice.
func (m *Player) Marshal() ([]byte, error)

// Deserialize into arena memory. All allocations go into ar.
func UnmarshalPlayer(data []byte, ar *arena.Arena) (*Player, error)
```

### Map Fields

Proto `map<K,V>` fields are generated as `*arena.Map[K,V]` — the arena-native hash map. Maps are lazily allocated on first insertion during unmarshal.

```go
// Read
v, ok := player.Attributes.Get("level")

// Write
player.Attributes.Put("level", 42)
```

### Oneof Fields

Oneof groups generate an interface and wrapper structs, following the same pattern as standard protoc-gen-go:

```protobuf
oneof payload {
    string text = 7;
    bytes binary = 8;
}
```

```go
// Interface
type isPlayer_Payload interface{ isPlayer_Payload() }

// Wrappers
type Player_Text struct { Value string }
type Player_Binary struct { Value []byte }

// Field on struct
Payload isPlayer_Payload

// Getter
func (m *Player) GetText() string
func (m *Player) GetBinary() []byte
```

### Enums

```go
type Role int32

const (
    Role_UNKNOWN Role = 0
    Role_ADMIN   Role = 1
    Role_USER    Role = 2
)
```

### Required Fields

Required fields are validated at the end of unmarshal. Missing required fields return an error:

```go
if m.ReqString == nil {
    return nil, fmt.Errorf("proto: required field 'req_string' not set")
}
```

### Default Values

Default values are embedded in getters:

```protobuf
optional int32 timeout = 1 [default = 30];
```

```go
func (m *Config) GetTimeout() int32 {
    if m != nil && m.Timeout != nil { return *m.Timeout }
    return 30
}
```

## Interoperability

Generated code is **binary-compatible** with `google.golang.org/protobuf`. You can:

- Marshal with standard `proto.Marshal()` and unmarshal with `UnmarshalXxx(ar)`
- Marshal with `.Marshal()` and unmarshal with standard `proto.Unmarshal()`
- Mix arena and standard types in the same pipeline

```go
// Standard → Arena
stdBytes, _ := proto.Marshal(stdMsg)
arenaMsg, _ := pb.UnmarshalMsg(stdBytes, ar)

// Arena → Standard
arenaBytes, _ := arenaMsg.Marshal()
stdMsg := &stdpb.Msg{}
_ = proto.Unmarshal(arenaBytes, stdMsg)
```

## Supported Features

| Feature | Status |
|---------|--------|
| Proto2 syntax | Supported |
| All scalar types (int32, int64, uint32, uint64, sint32, sint64, fixed32, fixed64, sfixed32, sfixed64, float, double, bool, string, bytes) | Supported |
| Enum | Supported |
| Optional fields | Supported |
| Required fields | Supported |
| Default values | Supported |
| Repeated fields (packed & unpacked) | Supported |
| Map fields | Supported |
| Nested messages | Supported |
| Oneof | Supported |
| Unknown field skipping | Supported |

## Limitations

| Feature | Status |
|---------|--------|
| Proto3 syntax | Not supported (use proto2) |
| Service definitions (gRPC) | Not supported |
| Extension fields | Not supported |
| Group fields (deprecated) | Not supported |

## Benchmarks

The test suite includes benchmarks comparing arena and standard protobuf for all message types. Run them with:

```bash
cd cmd/protoc-gen-arena/testdata/wiretest
go test -bench=. -benchmem
```

## Memory Safety

Arena-generated types follow the same safety rules as the base arena library. See [Arena Memory Safety](../../README.md#memory-safety-rules) for the full guide. Key points for protobuf usage:

- All unmarshalled data is arena-managed — strings, bytes, sub-messages, map entries.
- Do not hold arena pointers after `ar.Reset()`.
- Do not assign Go heap pointers into arena struct fields; use `arena.DeepCopy` or arena APIs.

## License

Released under version 2.0 of the Apache License.
