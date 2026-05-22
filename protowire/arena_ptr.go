package protowire

import (
	"reflect"
	"unsafe"

	"github.com/limpo1989/arena"
)

// StringPtr allocates a string header and string data in arena memory.
// Returns a *string suitable for proto2 optional string fields.
func StringPtr(ar *arena.Arena, data []byte) *string {
	if len(data) == 0 {
		p := (*string)(unsafe.Pointer(ar.Malloc(uintptr(unsafe.Sizeof("")))))
		*p = ""
		return p
	}
	// Allocate string header (16 bytes on 64-bit)
	hdr := (*reflect.StringHeader)(unsafe.Pointer(
		ar.Malloc(unsafe.Sizeof(reflect.StringHeader{})),
	))
	// Allocate and copy string data into arena
	buf := ar.Bytes(data)
	hdr.Data = uintptr(unsafe.Pointer(&buf[0]))
	hdr.Len = len(data)
	return (*string)(unsafe.Pointer(hdr))
}

// String allocates string data in arena memory and returns a string
// whose data pointer is arena-managed.
func String(ar *arena.Arena, data []byte) string {
	if len(data) == 0 {
		return ""
	}
	buf := ar.Bytes(data)
	return unsafe.String(&buf[0], len(buf))
}

// Ptr allocates a T-sized slot in arena, writes v, and returns a *T.
// Used for proto2 optional scalar fields.
func Ptr[T any](ar *arena.Arena, v T) *T {
	p := (*T)(unsafe.Pointer(ar.Malloc(unsafe.Sizeof(v))))
	*p = v
	return p
}
