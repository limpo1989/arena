package wiretest

import (
	"fmt"
	"testing"

	"github.com/limpo1989/arena"
	"github.com/limpo1989/arena/cmd/protoc-gen-arena/testdata/arenatest"
	stdproto "github.com/limpo1989/arena/cmd/protoc-gen-arena/testdata/stdproto"
	"google.golang.org/protobuf/proto"
)

// ============================================================================
// Pre-built test data (serialized once, reused across iterations)
// ============================================================================

func makeStdScalars() *stdproto.Scalars {
	return &stdproto.Scalars{
		OptInt32:    int32Ptr(42),
		OptInt64:    int64Ptr(-100),
		OptUint32:   uint32Ptr(300),
		OptUint64:   uint64Ptr(999),
		OptSint32:   int32Ptr(-1),
		OptSint64:   int64Ptr(-2),
		OptFixed32:  uint32Ptr(0xDEADBEEF),
		OptFixed64:  uint64Ptr(0xCAFEBABEDEADBEEF),
		OptSfixed32: int32Ptr(-12345),
		OptSfixed64: int64Ptr(-9876543210),
		OptFloat:    float32Ptr(3.14),
		OptDouble:   float64Ptr(2.718281828),
		OptBool:     boolPtr(true),
		OptString:   stringPtr("hello arena benchmark"),
		OptBytes:    []byte("some bytes for benchmark"),
		OptEnum:     stdproto.Role_ADMIN.Enum(),
		ReqString:   stringPtr("required"),
		ReqInt32:    int32Ptr(100),
	}
}

func makeStdRepeated() *stdproto.Repeated {
	return &stdproto.Repeated{
		UnpackedInt32: []int32{10, 20, 30, 40, 50},
		Strings:       []string{"alpha", "beta", "gamma", "delta"},
		PackedInt32:   []int32{1, 2, 3, 100, -1},
		PackedEnum:    []stdproto.Role{stdproto.Role_ADMIN, stdproto.Role_USER},
		Doubles:       []float64{1.1, 2.2, 3.3, 4.4, 5.5},
		ByteSlices:    [][]byte{[]byte("x"), []byte("yy"), []byte("zzz")},
	}
}

func makeStdMaps() *stdproto.Maps {
	return &stdproto.Maps{
		StringInt32:  map[string]int32{"a": 1, "b": 2, "c": 3},
		StringString: map[string]string{"hello": "world", "foo": "bar"},
	}
}

func makeStdOuter() *stdproto.Outer {
	return &stdproto.Outer{
		Middle: &stdproto.Middle{
			Inner: &stdproto.Inner{
				Name:  stringPtr("deep"),
				Value: int32Ptr(42),
			},
			Label: stringPtr("mid"),
		},
		Tag: stringPtr("outer"),
	}
}

func makeStdOneofMsg() *stdproto.OneofMsg {
	return &stdproto.OneofMsg{
		Name: stringPtr("hello"),
		Choice: &stdproto.OneofMsg_MsgVal{
			MsgVal: &stdproto.Inner{
				Name:  stringPtr("inner_oneof"),
				Value: int32Ptr(99),
			},
		},
	}
}

func makeStdFullMessage() *stdproto.FullMessage {
	return &stdproto.FullMessage{
		Id:     int32Ptr(1),
		Label:  stringPtr("full"),
		Tags:   []string{"a", "b", "c"},
		Scores: map[string]int32{"x": 10, "y": 20, "z": 30},
		Nested: &stdproto.Inner{
			Name:  stringPtr("deep"),
			Value: int32Ptr(5),
		},
		Role: stdproto.Role_ADMIN.Enum(),
		Payload: &stdproto.FullMessage_TextPayload{
			TextPayload: "hello payload",
		},
	}
}

// benchData holds pre-serialized bytes for benchmarking.
type benchData struct {
	scalarsBytes  []byte
	repeatedBytes []byte
	mapsBytes     []byte
	outerBytes    []byte
	oneofBytes    []byte
	fullMsgBytes  []byte
	scalarsSize   int
	repeatedSize  int
	mapsSize      int
	outerSize     int
	oneofSize     int
	fullMsgSize   int
}

func newBenchData() *benchData {
	d := &benchData{}
	scalars := makeStdScalars()
	d.scalarsBytes, _ = proto.Marshal(scalars)
	d.scalarsSize = len(d.scalarsBytes)

	repeated := makeStdRepeated()
	d.repeatedBytes, _ = proto.Marshal(repeated)
	d.repeatedSize = len(d.repeatedBytes)

	maps := makeStdMaps()
	d.mapsBytes, _ = proto.Marshal(maps)
	d.mapsSize = len(d.mapsBytes)

	outer := makeStdOuter()
	d.outerBytes, _ = proto.Marshal(outer)
	d.outerSize = len(d.outerBytes)

	oneof := makeStdOneofMsg()
	d.oneofBytes, _ = proto.Marshal(oneof)
	d.oneofSize = len(d.oneofBytes)

	full := makeStdFullMessage()
	d.fullMsgBytes, _ = proto.Marshal(full)
	d.fullMsgSize = len(d.fullMsgBytes)

	return d
}

// ============================================================================
// Unmarshal benchmarks
// ============================================================================

func BenchmarkUnmarshalScalars(b *testing.B) {
	bd := newBenchData()

	b.Run("Arena", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		ar := arena.NewArena()
		for i := 0; i < b.N; i++ {
			scalars, err := arenatest.UnmarshalScalars(bd.scalarsBytes, ar)
			if nil != err {
				b.Fatal(err)
			}
			ar.Free(scalars)
		}
	})

	b.Run("Standard", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			msg := &stdproto.Scalars{}
			_ = proto.Unmarshal(bd.scalarsBytes, msg)
		}
	})
}

func BenchmarkUnmarshalRepeated(b *testing.B) {
	bd := newBenchData()

	b.Run("Arena", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ar := arena.NewArena()
			_, _ = arenatest.UnmarshalRepeated(bd.repeatedBytes, ar)
			ar.Reset()
		}
	})

	b.Run("Standard", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			msg := &stdproto.Repeated{}
			_ = proto.Unmarshal(bd.repeatedBytes, msg)
		}
	})
}

func BenchmarkUnmarshalMaps(b *testing.B) {
	bd := newBenchData()

	b.Run("Arena", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ar := arena.NewArena()
			_, _ = arenatest.UnmarshalMaps(bd.mapsBytes, ar)
			ar.Reset()
		}
	})

	b.Run("Standard", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			msg := &stdproto.Maps{}
			_ = proto.Unmarshal(bd.mapsBytes, msg)
		}
	})
}

func BenchmarkUnmarshalOuter(b *testing.B) {
	bd := newBenchData()

	b.Run("Arena", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ar := arena.NewArena()
			_, _ = arenatest.UnmarshalOuter(bd.outerBytes, ar)
			ar.Reset()
		}
	})

	b.Run("Standard", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			msg := &stdproto.Outer{}
			_ = proto.Unmarshal(bd.outerBytes, msg)
		}
	})
}

func BenchmarkUnmarshalOneofMsg(b *testing.B) {
	bd := newBenchData()

	b.Run("Arena", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ar := arena.NewArena()
			_, _ = arenatest.UnmarshalOneofMsg(bd.oneofBytes, ar)
			ar.Reset()
		}
	})

	b.Run("Standard", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			msg := &stdproto.OneofMsg{}
			_ = proto.Unmarshal(bd.oneofBytes, msg)
		}
	})
}

func BenchmarkUnmarshalFullMessage(b *testing.B) {
	bd := newBenchData()

	b.Run("Arena", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ar := arena.NewArena()
			_, _ = arenatest.UnmarshalFullMessage(bd.fullMsgBytes, ar)
			ar.Reset()
		}
	})

	b.Run("Standard", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			msg := &stdproto.FullMessage{}
			_ = proto.Unmarshal(bd.fullMsgBytes, msg)
		}
	})
}

// ============================================================================
// Marshal benchmarks
// ============================================================================

func BenchmarkMarshalScalars(b *testing.B) {
	bd := newBenchData()
	ar := arena.NewArena()
	arenaMsg, _ := arenatest.UnmarshalScalars(bd.scalarsBytes, ar)
	stdMsg := &stdproto.Scalars{}
	_ = proto.Unmarshal(bd.scalarsBytes, stdMsg)

	b.Run("Arena", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = arenaMsg.Marshal()
		}
	})

	b.Run("Standard", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = proto.Marshal(stdMsg)
		}
	})
}

func BenchmarkMarshalRepeated(b *testing.B) {
	bd := newBenchData()
	ar := arena.NewArena()
	arenaMsg, _ := arenatest.UnmarshalRepeated(bd.repeatedBytes, ar)
	stdMsg := &stdproto.Repeated{}
	_ = proto.Unmarshal(bd.repeatedBytes, stdMsg)

	b.Run("Arena", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = arenaMsg.Marshal()
		}
	})

	b.Run("Standard", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = proto.Marshal(stdMsg)
		}
	})
}

func BenchmarkMarshalMaps(b *testing.B) {
	bd := newBenchData()
	ar := arena.NewArena()
	arenaMsg, _ := arenatest.UnmarshalMaps(bd.mapsBytes, ar)
	stdMsg := &stdproto.Maps{}
	_ = proto.Unmarshal(bd.mapsBytes, stdMsg)

	b.Run("Arena", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = arenaMsg.Marshal()
		}
	})

	b.Run("Standard", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = proto.Marshal(stdMsg)
		}
	})
}

func BenchmarkMarshalOuter(b *testing.B) {
	bd := newBenchData()
	ar := arena.NewArena()
	arenaMsg, _ := arenatest.UnmarshalOuter(bd.outerBytes, ar)
	stdMsg := &stdproto.Outer{}
	_ = proto.Unmarshal(bd.outerBytes, stdMsg)

	b.Run("Arena", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = arenaMsg.Marshal()
		}
	})

	b.Run("Standard", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = proto.Marshal(stdMsg)
		}
	})
}

func BenchmarkMarshalOneofMsg(b *testing.B) {
	bd := newBenchData()
	ar := arena.NewArena()
	arenaMsg, _ := arenatest.UnmarshalOneofMsg(bd.oneofBytes, ar)
	stdMsg := &stdproto.OneofMsg{}
	_ = proto.Unmarshal(bd.oneofBytes, stdMsg)

	b.Run("Arena", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = arenaMsg.Marshal()
		}
	})

	b.Run("Standard", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = proto.Marshal(stdMsg)
		}
	})
}

func BenchmarkMarshalFullMessage(b *testing.B) {
	bd := newBenchData()
	ar := arena.NewArena()
	arenaMsg, _ := arenatest.UnmarshalFullMessage(bd.fullMsgBytes, ar)
	stdMsg := &stdproto.FullMessage{}
	_ = proto.Unmarshal(bd.fullMsgBytes, stdMsg)

	b.Run("Arena", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = arenaMsg.Marshal()
		}
	})

	b.Run("Standard", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = proto.Marshal(stdMsg)
		}
	})
}

// ============================================================================
// MarshalTo benchmarks (pre-allocated buffer, no allocation)
// ============================================================================

func BenchmarkMarshalToScalars(b *testing.B) {
	bd := newBenchData()
	ar := arena.NewArena()
	arenaMsg, _ := arenatest.UnmarshalScalars(bd.scalarsBytes, ar)
	stdMsg := &stdproto.Scalars{}
	_ = proto.Unmarshal(bd.scalarsBytes, stdMsg)
	arenaBuf := make([]byte, bd.scalarsSize)

	b.Run("Arena", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = arenaMsg.MarshalTo(arenaBuf)
		}
	})

	b.Run("Standard", func(b *testing.B) {
		stdBuf := make([]byte, bd.scalarsSize)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = proto.MarshalOptions{}.MarshalAppend(stdBuf[:0], stdMsg)
		}
	})
}

func BenchmarkMarshalToFullMessage(b *testing.B) {
	bd := newBenchData()
	ar := arena.NewArena()
	arenaMsg, _ := arenatest.UnmarshalFullMessage(bd.fullMsgBytes, ar)
	stdMsg := &stdproto.FullMessage{}
	_ = proto.Unmarshal(bd.fullMsgBytes, stdMsg)
	arenaBuf := make([]byte, bd.fullMsgSize)

	b.Run("Arena", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = arenaMsg.MarshalTo(arenaBuf)
		}
	})

	b.Run("Standard", func(b *testing.B) {
		stdBuf := make([]byte, bd.fullMsgSize)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = proto.MarshalOptions{}.MarshalAppend(stdBuf[:0], stdMsg)
		}
	})
}

// ============================================================================
// Size benchmarks
// ============================================================================

func BenchmarkSize(b *testing.B) {
	bd := newBenchData()
	ar := arena.NewArena()

	arenaScalars, _ := arenatest.UnmarshalScalars(bd.scalarsBytes, ar)
	arenaRepeated, _ := arenatest.UnmarshalRepeated(bd.repeatedBytes, ar)
	arenaMaps, _ := arenatest.UnmarshalMaps(bd.mapsBytes, ar)
	arenaOuter, _ := arenatest.UnmarshalOuter(bd.outerBytes, ar)
	arenaOneof, _ := arenatest.UnmarshalOneofMsg(bd.oneofBytes, ar)
	arenaFull, _ := arenatest.UnmarshalFullMessage(bd.fullMsgBytes, ar)

	stdScalars := &stdproto.Scalars{}
	_ = proto.Unmarshal(bd.scalarsBytes, stdScalars)
	stdRepeated := &stdproto.Repeated{}
	_ = proto.Unmarshal(bd.repeatedBytes, stdRepeated)
	stdMaps := &stdproto.Maps{}
	_ = proto.Unmarshal(bd.mapsBytes, stdMaps)
	stdOuter := &stdproto.Outer{}
	_ = proto.Unmarshal(bd.outerBytes, stdOuter)
	stdOneof := &stdproto.OneofMsg{}
	_ = proto.Unmarshal(bd.oneofBytes, stdOneof)
	stdFull := &stdproto.FullMessage{}
	_ = proto.Unmarshal(bd.fullMsgBytes, stdFull)

	for _, tc := range []struct {
		name      string
		arenaSize func() int
		stdSize   func() int
	}{
		{"Scalars", arenaScalars.Size, func() int { return proto.Size(stdScalars) }},
		{"Repeated", arenaRepeated.Size, func() int { return proto.Size(stdRepeated) }},
		{"Maps", arenaMaps.Size, func() int { return proto.Size(stdMaps) }},
		{"Outer", arenaOuter.Size, func() int { return proto.Size(stdOuter) }},
		{"OneofMsg", arenaOneof.Size, func() int { return proto.Size(stdOneof) }},
		{"FullMessage", arenaFull.Size, func() int { return proto.Size(stdFull) }},
	} {
		b.Run(fmt.Sprintf("%s/Arena", tc.name), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = tc.arenaSize()
			}
		})
		b.Run(fmt.Sprintf("%s/Standard", tc.name), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = tc.stdSize()
			}
		})
	}
}

// ============================================================================
// Summary: message sizes
// ============================================================================

func BenchmarkMessageSizes(b *testing.B) {
	bd := newBenchData()
	b.Logf("%-15s %5d bytes", "Scalars", bd.scalarsSize)
	b.Logf("%-15s %5d bytes", "Repeated", bd.repeatedSize)
	b.Logf("%-15s %5d bytes", "Maps", bd.mapsSize)
	b.Logf("%-15s %5d bytes", "Outer", bd.outerSize)
	b.Logf("%-15s %5d bytes", "OneofMsg", bd.oneofSize)
	b.Logf("%-15s %5d bytes", "FullMessage", bd.fullMsgSize)
}
