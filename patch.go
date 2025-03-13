package arena

import (
	"reflect"
	"unsafe"
)

const (
	flagStickyRO = 1 << 5
	flagEmbedRO  = 1 << 6
	flagRO       = flagStickyRO | flagEmbedRO
)

type reflectValue struct {
	ptr   unsafe.Pointer
	typ   unsafe.Pointer
	flags uintptr
}

// patchValue modifies a reflect.Value to make unexported struct fields assignable.
// WARNING: This bypasses Go's type safety and should be used with caution.
func patchValue(v reflect.Value) reflect.Value {
	//rv := reflect.ValueOf(&v)
	//flag := rv.Elem().FieldByName("flag")
	//ptrFlag := (*uintptr)(unsafe.Pointer(flag.UnsafeAddr()))
	//*ptrFlag = *ptrFlag &^ flagRO
	rv := (*reflectValue)(unsafe.Pointer(&v))
	rv.flags = rv.flags &^ flagRO
	return v
}
