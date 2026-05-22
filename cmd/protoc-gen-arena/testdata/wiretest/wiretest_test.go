package wiretest

import (
	"math"
	"testing"

	"github.com/limpo1989/arena"
	"github.com/limpo1989/arena/cmd/protoc-gen-arena/testdata/arenatest"
	stdproto "github.com/limpo1989/arena/cmd/protoc-gen-arena/testdata/stdproto"
	"google.golang.org/protobuf/proto"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func boolPtr(v bool) *bool          { return &v }
func int32Ptr(v int32) *int32       { return &v }
func int64Ptr(v int64) *int64       { return &v }
func uint32Ptr(v uint32) *uint32    { return &v }
func uint64Ptr(v uint64) *uint64    { return &v }
func float32Ptr(v float32) *float32 { return &v }
func float64Ptr(v float64) *float64 { return &v }
func stringPtr(v string) *string    { return &v }

func arenaPtr[T any](ar *arena.Arena, v T) *T {
	p := arena.New[T](ar)
	*p = v
	return p
}

func arenaStringPtr(ar *arena.Arena, s string) *string {
	p := arena.New[string](ar)
	*p = s
	return p
}

// ---------------------------------------------------------------------------
// 10.1 Test proto file covers all types (verified by generation)
// ---------------------------------------------------------------------------

// The test.proto file covers: optional/required scalars, repeated (packed+unpacked),
// map (string→int32, string→string, int32→message), oneof, nested (3 levels), enum.
// Compilation of arenatest and stdproto packages validates this.

// ---------------------------------------------------------------------------
// 10.2 Standard marshal → arena unmarshal → arena marshal byte equality
// ---------------------------------------------------------------------------

func TestStdMarshal_ArenaUnmarshal_ArenaMarshal(t *testing.T) {
	ar := arena.NewArena()

	std := &stdproto.Scalars{
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
		OptString:   stringPtr("hello arena"),
		OptEnum:     stdproto.Role_ADMIN.Enum(),
		ReqString:   stringPtr("required"),
		ReqInt32:    int32Ptr(100),
	}

	stdBytes, err := proto.Marshal(std)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}

	arenaMsg, err := arenatest.UnmarshalScalars(stdBytes, ar)
	if err != nil {
		t.Fatalf("UnmarshalScalars: %v", err)
	}

	arenaBytes, err := arenaMsg.Marshal()
	if err != nil {
		t.Fatalf("arenaMsg.Marshal: %v", err)
	}

	if string(stdBytes) != string(arenaBytes) {
		t.Errorf("byte mismatch:\nstd:   %x\narena: %x", stdBytes, arenaBytes)
	}
}

// ---------------------------------------------------------------------------
// 10.3 Arena marshal → standard unmarshal field value equality
// ---------------------------------------------------------------------------

func TestArenaMarshal_StdUnmarshal(t *testing.T) {
	ar := arena.NewArena()

	arenaMsg := arena.New[arenatest.Scalars](ar)
	// Set fields manually
	arenaMsg.OptInt32 = int32Ptr(42)
	arenaMsg.OptString = stringPtr("test")
	arenaMsg.ReqString = stringPtr("required")
	arenaMsg.ReqInt32 = int32Ptr(100)

	arenaBytes, err := arenaMsg.Marshal()
	if err != nil {
		t.Fatalf("arenaMsg.Marshal: %v", err)
	}

	stdMsg := &stdproto.Scalars{}
	if err := proto.Unmarshal(arenaBytes, stdMsg); err != nil {
		t.Fatalf("proto.Unmarshal: %v", err)
	}

	if stdMsg.GetOptInt32() != 42 {
		t.Errorf("OptInt32: got %d, want 42", stdMsg.GetOptInt32())
	}
	if stdMsg.GetOptString() != "test" {
		t.Errorf("OptString: got %q, want %q", stdMsg.GetOptString(), "test")
	}
	if stdMsg.GetReqString() != "required" {
		t.Errorf("ReqString: got %q, want %q", stdMsg.GetReqString(), "required")
	}
}

// ---------------------------------------------------------------------------
// 10.4 Packed repeated: marshal produces single block; unmarshal accepts both
// ---------------------------------------------------------------------------

func TestPackedRepeated(t *testing.T) {
	ar := arena.NewArena()

	std := &stdproto.Repeated{
		PackedInt32: []int32{1, 2, 3, 100, -1},
		PackedEnum:  []stdproto.Role{stdproto.Role_ADMIN, stdproto.Role_USER},
		Doubles:     []float64{1.1, 2.2, 3.3},
	}

	stdBytes, err := proto.Marshal(std)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}

	arenaMsg, err := arenatest.UnmarshalRepeated(stdBytes, ar)
	if err != nil {
		t.Fatalf("UnmarshalRepeated: %v", err)
	}

	if len(arenaMsg.PackedInt32) != 5 {
		t.Fatalf("PackedInt32 length: got %d, want 5", len(arenaMsg.PackedInt32))
	}
	expected := []int32{1, 2, 3, 100, -1}
	for i, v := range expected {
		if arenaMsg.PackedInt32[i] != v {
			t.Errorf("PackedInt32[%d]: got %d, want %d", i, arenaMsg.PackedInt32[i], v)
		}
	}

	if len(arenaMsg.PackedEnum) != 2 {
		t.Errorf("PackedEnum length: got %d, want 2", len(arenaMsg.PackedEnum))
	}
	if arenaMsg.PackedEnum[0] != arenatest.Role_ADMIN {
		t.Errorf("PackedEnum[0]: got %d, want ADMIN", arenaMsg.PackedEnum[0])
	}

	// Round-trip back
	arenaBytes, err := arenaMsg.Marshal()
	if err != nil {
		t.Fatalf("arenaMsg.Marshal: %v", err)
	}
	if string(stdBytes) != string(arenaBytes) {
		t.Errorf("packed byte mismatch:\nstd:   %x\narena: %x", stdBytes, arenaBytes)
	}
}

// ---------------------------------------------------------------------------
// 10.5 Map field round-trip
// ---------------------------------------------------------------------------

func TestMapStringInt32(t *testing.T) {
	ar := arena.NewArena()

	std := &stdproto.Maps{
		StringInt32: map[string]int32{"a": 1, "b": 2},
	}

	stdBytes, err := proto.Marshal(std)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}

	arenaMsg, err := arenatest.UnmarshalMaps(stdBytes, ar)
	if err != nil {
		t.Fatalf("UnmarshalMaps: %v", err)
	}

	if arenaMsg.StringInt32 == nil {
		t.Fatal("StringInt32 is nil")
	}
	if arenaMsg.StringInt32.Len() != 2 {
		t.Errorf("StringInt32.Len(): got %d, want 2", arenaMsg.StringInt32.Len())
	}
	if v, ok := arenaMsg.StringInt32.Get("a"); !ok || v != 1 {
		t.Errorf("StringInt32['a']: got %v, ok %v, want 1", v, ok)
	}
	if v, ok := arenaMsg.StringInt32.Get("b"); !ok || v != 2 {
		t.Errorf("StringInt32['b']: got %v, ok %v, want 2", v, ok)
	}
}

func TestMapStringString(t *testing.T) {
	ar := arena.NewArena()

	std := &stdproto.Maps{
		StringString: map[string]string{"hello": "world", "foo": "bar"},
	}

	stdBytes, err := proto.Marshal(std)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}

	arenaMsg, err := arenatest.UnmarshalMaps(stdBytes, ar)
	if err != nil {
		t.Fatalf("UnmarshalMaps: %v", err)
	}

	if arenaMsg.StringString.Len() != 2 {
		t.Errorf("StringString.Len(): got %d, want 2", arenaMsg.StringString.Len())
	}
	if v, ok := arenaMsg.StringString.Get("hello"); !ok || v != "world" {
		t.Errorf("StringString['hello']: got %v, ok %v, want 'world'", v, ok)
	}
}

// ---------------------------------------------------------------------------
// 10.7 Nested message round-trip (3 levels)
// ---------------------------------------------------------------------------

func TestNestedMessage(t *testing.T) {
	ar := arena.NewArena()

	std := &stdproto.Outer{
		Middle: &stdproto.Middle{
			Inner: &stdproto.Inner{
				Name:  stringPtr("deep"),
				Value: int32Ptr(42),
			},
			Label: stringPtr("mid"),
		},
		Tag: stringPtr("outer"),
	}

	stdBytes, err := proto.Marshal(std)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}

	arenaMsg, err := arenatest.UnmarshalOuter(stdBytes, ar)
	if err != nil {
		t.Fatalf("UnmarshalOuter: %v", err)
	}

	if arenaMsg.Tag == nil || *arenaMsg.Tag != "outer" {
		t.Errorf("Tag: got %v, want 'outer'", arenaMsg.Tag)
	}
	if arenaMsg.Middle == nil {
		t.Fatal("Middle is nil")
	}
	if arenaMsg.Middle.Label == nil || *arenaMsg.Middle.Label != "mid" {
		t.Errorf("Middle.Label: got %v, want 'mid'", arenaMsg.Middle.Label)
	}
	if arenaMsg.Middle.Inner == nil {
		t.Fatal("Middle.Inner is nil")
	}
	if arenaMsg.Middle.Inner.Name == nil || *arenaMsg.Middle.Inner.Name != "deep" {
		t.Errorf("Middle.Inner.Name: got %v, want 'deep'", arenaMsg.Middle.Inner.Name)
	}
	if arenaMsg.Middle.Inner.Value == nil || *arenaMsg.Middle.Inner.Value != 42 {
		t.Errorf("Middle.Inner.Value: got %v, want 42", arenaMsg.Middle.Inner.Value)
	}

	// Round-trip bytes
	arenaBytes, err := arenaMsg.Marshal()
	if err != nil {
		t.Fatalf("arenaMsg.Marshal: %v", err)
	}
	if string(stdBytes) != string(arenaBytes) {
		t.Errorf("nested byte mismatch:\nstd:   %x\narena: %x", stdBytes, arenaBytes)
	}
}

// ---------------------------------------------------------------------------
// 10.8 Required field validation
// ---------------------------------------------------------------------------

func TestRequiredFieldValidation(t *testing.T) {
	ar := arena.NewArena()

	// Create scalars with required fields missing
	data := []byte{} // empty data

	_, err := arenatest.UnmarshalScalars(data, ar)
	if err == nil {
		t.Fatal("expected error for missing required fields")
	}
}

// ---------------------------------------------------------------------------
// 10.9 Default values
// ---------------------------------------------------------------------------

func TestDefaultValues(t *testing.T) {
	ar := arena.NewArena()

	// Empty Defaults message
	data := []byte{}
	msg, err := arenatest.UnmarshalDefaults(data, ar)
	if err != nil {
		t.Fatalf("UnmarshalDefaults: %v", err)
	}

	// Explicit defaults
	if msg.GetWithDefault() != 42 {
		t.Errorf("WithDefault: got %d, want 42", msg.GetWithDefault())
	}
	if msg.GetStrDefault() != "hello" {
		t.Errorf("StrDefault: got %q, want 'hello'", msg.GetStrDefault())
	}
	if msg.GetBoolDefault() != true {
		t.Errorf("BoolDefault: got %v, want true", msg.GetBoolDefault())
	}
	if msg.GetEnumDefault() != arenatest.Role_ADMIN {
		t.Errorf("EnumDefault: got %d, want ADMIN", msg.GetEnumDefault())
	}

	// Has methods should return false for unset fields
	if msg.HasWithDefault() {
		t.Error("HasWithDefault should be false")
	}
}

// ---------------------------------------------------------------------------
// 10.10 Edge cases
// ---------------------------------------------------------------------------

func TestEmptyMessage(t *testing.T) {
	ar := arena.NewArena()

	data := []byte{}
	msg, err := arenatest.UnmarshalDefaults(data, ar)
	if err != nil {
		t.Fatalf("UnmarshalDefaults empty: %v", err)
	}
	if msg == nil {
		t.Fatal("msg is nil")
	}
}

func TestAllFieldsUnset(t *testing.T) {
	ar := arena.NewArena()

	std := &stdproto.Inner{}
	stdBytes, err := proto.Marshal(std)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}

	msg, err := arenatest.UnmarshalInner(stdBytes, ar)
	if err != nil {
		t.Fatalf("UnmarshalInner: %v", err)
	}
	if msg.Name != nil {
		t.Errorf("Name should be nil")
	}
	if msg.Value != nil {
		t.Errorf("Value should be nil")
	}
	if msg.Size() != 0 {
		t.Errorf("Size() should be 0, got %d", msg.Size())
	}
}

func TestLargeRepeatedField(t *testing.T) {
	ar := arena.NewArena()

	std := &stdproto.Repeated{}
	for i := 0; i < 1000; i++ {
		std.Strings = append(std.Strings, "item")
	}

	stdBytes, err := proto.Marshal(std)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}

	msg, err := arenatest.UnmarshalRepeated(stdBytes, ar)
	if err != nil {
		t.Fatalf("UnmarshalRepeated: %v", err)
	}

	if len(msg.Strings) != 1000 {
		t.Errorf("Strings length: got %d, want 1000", len(msg.Strings))
	}
	for i, s := range msg.Strings {
		if s != "item" {
			t.Errorf("Strings[%d]: got %q, want 'item'", i, s)
			break
		}
	}
}

func TestNegativeEnumValues(t *testing.T) {
	ar := arena.NewArena()

	std := &stdproto.Scalars{
		OptEnum:   stdproto.Role_GUEST.Enum(),
		ReqString: stringPtr("req"),
		ReqInt32:  int32Ptr(1),
	}

	stdBytes, err := proto.Marshal(std)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}

	msg, err := arenatest.UnmarshalScalars(stdBytes, ar)
	if err != nil {
		t.Fatalf("UnmarshalScalars: %v", err)
	}

	if msg.OptEnum == nil || *msg.OptEnum != arenatest.Role_GUEST {
		t.Errorf("OptEnum: got %v, want GUEST (-1)", msg.OptEnum)
	}
}

// ---------------------------------------------------------------------------
// 10.11 All scalar types with boundary values
// ---------------------------------------------------------------------------

func TestAllScalarBoundaryValues(t *testing.T) {
	ar := arena.NewArena()

	std := &stdproto.Scalars{
		OptInt32:    int32Ptr(0),
		OptInt64:    int64Ptr(math.MaxInt64),
		OptUint32:   uint32Ptr(math.MaxUint32),
		OptUint64:   uint64Ptr(math.MaxUint64),
		OptSint32:   int32Ptr(math.MinInt32),
		OptSint64:   int64Ptr(math.MinInt64),
		OptFixed32:  uint32Ptr(0),
		OptFixed64:  uint64Ptr(0),
		OptSfixed32: int32Ptr(math.MinInt32),
		OptSfixed64: int64Ptr(math.MinInt64),
		OptFloat:    float32Ptr(float32(math.Inf(1))),
		OptDouble:   float64Ptr(math.Inf(-1)),
		OptBool:     boolPtr(false),
		OptString:   stringPtr(""),
		OptBytes:    []byte{},
		ReqString:   stringPtr("r"),
		ReqInt32:    int32Ptr(0),
	}

	stdBytes, err := proto.Marshal(std)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}

	msg, err := arenatest.UnmarshalScalars(stdBytes, ar)
	if err != nil {
		t.Fatalf("UnmarshalScalars: %v", err)
	}

	// Verify key boundary values
	if *msg.OptInt32 != 0 {
		t.Errorf("OptInt32: got %d, want 0", *msg.OptInt32)
	}
	if *msg.OptInt64 != math.MaxInt64 {
		t.Errorf("OptInt64: got %d, want MaxInt64", *msg.OptInt64)
	}
	if *msg.OptUint32 != math.MaxUint32 {
		t.Errorf("OptUint32: got %d, want MaxUint32", *msg.OptUint32)
	}
	if *msg.OptUint64 != math.MaxUint64 {
		t.Errorf("OptUint64: got %d, want MaxUint64", *msg.OptUint64)
	}
	if *msg.OptSint32 != math.MinInt32 {
		t.Errorf("OptSint32: got %d, want MinInt32", *msg.OptSint32)
	}
	if *msg.OptSint64 != math.MinInt64 {
		t.Errorf("OptSint64: got %d, want MinInt64", *msg.OptSint64)
	}

	// Round-trip
	arenaBytes, err := msg.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(stdBytes) != string(arenaBytes) {
		t.Errorf("scalar boundary byte mismatch:\nstd:   %x\narena: %x", stdBytes, arenaBytes)
	}
}

func TestSizeMatchesMarshalTo(t *testing.T) {
	ar := arena.NewArena()

	std := &stdproto.Scalars{
		OptInt32:  int32Ptr(42),
		OptString: stringPtr("hello"),
		OptFloat:  float32Ptr(3.14),
		OptBool:   boolPtr(true),
		ReqString: stringPtr("req"),
		ReqInt32:  int32Ptr(7),
	}

	stdBytes, err := proto.Marshal(std)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}

	msg, err := arenatest.UnmarshalScalars(stdBytes, ar)
	if err != nil {
		t.Fatalf("UnmarshalScalars: %v", err)
	}

	if msg.Size() != len(stdBytes) {
		t.Errorf("Size()=%d != len(Marshal)=%d", msg.Size(), len(stdBytes))
	}
}

func TestRepeatedUnpackedRoundTrip(t *testing.T) {
	ar := arena.NewArena()

	std := &stdproto.Repeated{
		UnpackedInt32: []int32{10, 20, 30},
		Strings:       []string{"a", "b", "c"},
		ByteSlices:    [][]byte{[]byte("x"), []byte("yy")},
	}

	stdBytes, err := proto.Marshal(std)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}

	msg, err := arenatest.UnmarshalRepeated(stdBytes, ar)
	if err != nil {
		t.Fatalf("UnmarshalRepeated: %v", err)
	}

	if len(msg.UnpackedInt32) != 3 || msg.UnpackedInt32[0] != 10 || msg.UnpackedInt32[2] != 30 {
		t.Errorf("UnpackedInt32: got %v", msg.UnpackedInt32)
	}
	if len(msg.Strings) != 3 || msg.Strings[1] != "b" {
		t.Errorf("Strings: got %v", msg.Strings)
	}
	if len(msg.ByteSlices) != 2 || string(msg.ByteSlices[1]) != "yy" {
		t.Errorf("ByteSlices: got %v", msg.ByteSlices)
	}

	// Round-trip
	arenaBytes, err := msg.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(stdBytes) != string(arenaBytes) {
		t.Errorf("repeated byte mismatch:\nstd:   %x\narena: %x", stdBytes, arenaBytes)
	}
}

// ---------------------------------------------------------------------------
// 10.6 Oneof round-trip
// ---------------------------------------------------------------------------

func TestOneofStringVariant(t *testing.T) {
	ar := arena.NewArena()

	// Standard marshal
	std := &stdproto.OneofMsg{
		Name: stringPtr("hello"),
		Choice: &stdproto.OneofMsg_StrVal{
			StrVal: "oneof_string",
		},
	}
	stdBytes, err := proto.Marshal(std)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}

	// Arena unmarshal
	arenaMsg, err := arenatest.UnmarshalOneofMsg(stdBytes, ar)
	if err != nil {
		t.Fatalf("UnmarshalOneofMsg: %v", err)
	}

	// Cross-validate: arena field values match std
	if arenaMsg.Name == nil || *arenaMsg.Name != "hello" {
		t.Errorf("Name: got %v, want 'hello'", arenaMsg.Name)
	}
	if arenaMsg.GetStrVal() != "oneof_string" {
		t.Errorf("StrVal: got %q, want 'oneof_string'", arenaMsg.GetStrVal())
	}

	// Binary comparison: arena marshal == std marshal
	arenaBytes, err := arenaMsg.Marshal()
	if err != nil {
		t.Fatalf("arenaMsg.Marshal: %v", err)
	}
	if string(stdBytes) != string(arenaBytes) {
		t.Errorf("oneof string byte mismatch:\nstd:   %x\narena: %x", stdBytes, arenaBytes)
	}

	// Reverse cross-validate: arena marshal -> std unmarshal
	stdMsg2 := &stdproto.OneofMsg{}
	if err := proto.Unmarshal(arenaBytes, stdMsg2); err != nil {
		t.Fatalf("proto.Unmarshal(arena bytes): %v", err)
	}
	if stdMsg2.GetStrVal() != "oneof_string" {
		t.Errorf("reverse cross: StrVal got %q, want 'oneof_string'", stdMsg2.GetStrVal())
	}
}

func TestOneofBytesVariant(t *testing.T) {
	ar := arena.NewArena()

	std := &stdproto.OneofMsg{
		Choice: &stdproto.OneofMsg_BytesVal{
			BytesVal: []byte{0xDE, 0xAD, 0xBE, 0xEF},
		},
	}
	stdBytes, err := proto.Marshal(std)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}

	arenaMsg, err := arenatest.UnmarshalOneofMsg(stdBytes, ar)
	if err != nil {
		t.Fatalf("UnmarshalOneofMsg: %v", err)
	}

	if v := arenaMsg.GetBytesVal(); string(v) != string([]byte{0xDE, 0xAD, 0xBE, 0xEF}) {
		t.Errorf("BytesVal: got %x, want deadbeef", v)
	}

	arenaBytes, err := arenaMsg.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(stdBytes) != string(arenaBytes) {
		t.Errorf("oneof bytes byte mismatch:\nstd:   %x\narena: %x", stdBytes, arenaBytes)
	}

	// Reverse cross-validate
	stdMsg2 := &stdproto.OneofMsg{}
	if err := proto.Unmarshal(arenaBytes, stdMsg2); err != nil {
		t.Fatalf("proto.Unmarshal(arena bytes): %v", err)
	}
	if string(stdMsg2.GetBytesVal()) != "\xde\xad\xbe\xef" {
		t.Errorf("reverse cross: BytesVal got %x", stdMsg2.GetBytesVal())
	}
}

func TestOneofMessageVariant(t *testing.T) {
	ar := arena.NewArena()

	std := &stdproto.OneofMsg{
		Choice: &stdproto.OneofMsg_MsgVal{
			MsgVal: &stdproto.Inner{
				Name:  stringPtr("inner_oneof"),
				Value: int32Ptr(99),
			},
		},
	}
	stdBytes, err := proto.Marshal(std)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}

	arenaMsg, err := arenatest.UnmarshalOneofMsg(stdBytes, ar)
	if err != nil {
		t.Fatalf("UnmarshalOneofMsg: %v", err)
	}

	// Cross-validate nested message fields
	msgVal := arenaMsg.GetMsgVal()
	if msgVal == nil {
		t.Fatal("MsgVal is nil")
	}
	if msgVal.Name == nil || *msgVal.Name != "inner_oneof" {
		t.Errorf("MsgVal.Name: got %v, want 'inner_oneof'", msgVal.Name)
	}
	if msgVal.Value == nil || *msgVal.Value != 99 {
		t.Errorf("MsgVal.Value: got %v, want 99", msgVal.Value)
	}

	// Binary comparison
	arenaBytes, err := arenaMsg.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(stdBytes) != string(arenaBytes) {
		t.Errorf("oneof message byte mismatch:\nstd:   %x\narena: %x", stdBytes, arenaBytes)
	}

	// Reverse cross-validate
	stdMsg2 := &stdproto.OneofMsg{}
	if err := proto.Unmarshal(arenaBytes, stdMsg2); err != nil {
		t.Fatalf("proto.Unmarshal(arena bytes): %v", err)
	}
	if stdMsg2.GetMsgVal().GetName() != "inner_oneof" {
		t.Errorf("reverse cross: MsgVal.Name got %q", stdMsg2.GetMsgVal().GetName())
	}
}

func TestOneofIntVariant(t *testing.T) {
	ar := arena.NewArena()

	std := &stdproto.OneofMsg{
		Choice: &stdproto.OneofMsg_IntVal{
			IntVal: -42,
		},
	}
	stdBytes, err := proto.Marshal(std)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}

	arenaMsg, err := arenatest.UnmarshalOneofMsg(stdBytes, ar)
	if err != nil {
		t.Fatalf("UnmarshalOneofMsg: %v", err)
	}

	if arenaMsg.GetIntVal() != -42 {
		t.Errorf("IntVal: got %d, want -42", arenaMsg.GetIntVal())
	}

	arenaBytes, err := arenaMsg.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(stdBytes) != string(arenaBytes) {
		t.Errorf("oneof int byte mismatch:\nstd:   %x\narena: %x", stdBytes, arenaBytes)
	}

	// Reverse cross-validate
	stdMsg2 := &stdproto.OneofMsg{}
	if err := proto.Unmarshal(arenaBytes, stdMsg2); err != nil {
		t.Fatalf("proto.Unmarshal(arena bytes): %v", err)
	}
	if stdMsg2.GetIntVal() != -42 {
		t.Errorf("reverse cross: IntVal got %d, want -42", stdMsg2.GetIntVal())
	}
}

func TestOneofNoVariant(t *testing.T) {
	ar := arena.NewArena()

	std := &stdproto.OneofMsg{
		Name: stringPtr("only_name"),
	}
	stdBytes, err := proto.Marshal(std)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}

	arenaMsg, err := arenatest.UnmarshalOneofMsg(stdBytes, ar)
	if err != nil {
		t.Fatalf("UnmarshalOneofMsg: %v", err)
	}

	if arenaMsg.Name == nil || *arenaMsg.Name != "only_name" {
		t.Errorf("Name: got %v, want 'only_name'", arenaMsg.Name)
	}
	if arenaMsg.Choice != nil {
		t.Errorf("Choice should be nil when no oneof variant set")
	}

	arenaBytes, err := arenaMsg.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(stdBytes) != string(arenaBytes) {
		t.Errorf("oneof empty byte mismatch:\nstd:   %x\narena: %x", stdBytes, arenaBytes)
	}
}

func TestFullMessageOneof(t *testing.T) {
	ar := arena.NewArena()

	std := &stdproto.FullMessage{
		Id:     int32Ptr(1),
		Label:  stringPtr("full"),
		Tags:   []string{"a", "b"},
		Scores: map[string]int32{"x": 10},
		Nested: &stdproto.Inner{
			Name:  stringPtr("deep"),
			Value: int32Ptr(5),
		},
		Role: stdproto.Role_ADMIN.Enum(),
		Payload: &stdproto.FullMessage_TextPayload{
			TextPayload: "hello payload",
		},
	}

	stdBytes, err := proto.Marshal(std)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}

	arenaMsg, err := arenatest.UnmarshalFullMessage(stdBytes, ar)
	if err != nil {
		t.Fatalf("UnmarshalFullMessage: %v", err)
	}

	// Cross-validate all fields
	if arenaMsg.Id == nil || *arenaMsg.Id != 1 {
		t.Errorf("Id: got %v, want 1", arenaMsg.Id)
	}
	if arenaMsg.Label == nil || *arenaMsg.Label != "full" {
		t.Errorf("Label: got %v, want 'full'", arenaMsg.Label)
	}
	if len(arenaMsg.Tags) != 2 || arenaMsg.Tags[0] != "a" || arenaMsg.Tags[1] != "b" {
		t.Errorf("Tags: got %v", arenaMsg.Tags)
	}
	if arenaMsg.Scores == nil || arenaMsg.Scores.Len() != 1 {
		t.Errorf("Scores: got %v", arenaMsg.Scores)
	} else {
		v, ok := arenaMsg.Scores.Get("x")
		if !ok || v != 10 {
			t.Errorf("Scores['x']: got %v, want 10", v)
		}
	}
	if arenaMsg.Nested == nil || arenaMsg.Nested.Name == nil || *arenaMsg.Nested.Name != "deep" {
		t.Errorf("Nested.Name: got %v", arenaMsg.Nested)
	}
	if arenaMsg.Role == nil || *arenaMsg.Role != arenatest.Role_ADMIN {
		t.Errorf("Role: got %v, want ADMIN", arenaMsg.Role)
	}
	if arenaMsg.GetTextPayload() != "hello payload" {
		t.Errorf("TextPayload: got %q, want 'hello payload'", arenaMsg.GetTextPayload())
	}

	// Binary comparison
	arenaBytes, err := arenaMsg.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(stdBytes) != string(arenaBytes) {
		t.Errorf("full message byte mismatch:\nstd:   %x\narena: %x", stdBytes, arenaBytes)
	}

	// Reverse cross-validate
	stdMsg2 := &stdproto.FullMessage{}
	if err := proto.Unmarshal(arenaBytes, stdMsg2); err != nil {
		t.Fatalf("proto.Unmarshal(arena bytes): %v", err)
	}
	if stdMsg2.GetTextPayload() != "hello payload" {
		t.Errorf("reverse cross: TextPayload got %q", stdMsg2.GetTextPayload())
	}
	if stdMsg2.GetId() != 1 {
		t.Errorf("reverse cross: Id got %d", stdMsg2.GetId())
	}
	if len(stdMsg2.GetTags()) != 2 {
		t.Errorf("reverse cross: Tags got %v", stdMsg2.GetTags())
	}
}
