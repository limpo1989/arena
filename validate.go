package arena

import (
	"fmt"
	"reflect"
	"sync"
)

type validationError struct {
	typePath string
	kind     reflect.Kind
}

func (e *validationError) Error() string {
	switch e.kind {
	case reflect.Map:
		return fmt.Sprintf("arena: type %s contains map (use arena.Map[K,V] instead)", e.typePath)
	default:
		return fmt.Sprintf("arena: type %s contains unsupported kind %s", e.typePath, e.kind)
	}
}

// typeInfo holds validation results and type properties for a Go type.
type typeInfo struct {
	valid  bool
	flat   bool // true if all fields are value types (memcpy-able)
	cyclic bool // true if type contains circular pointer references
}

var validatedTypes sync.Map // map[reflect.Type]typeInfo

func validateType[T any]() {
	t := reflect.TypeFor[T]()
	if _, ok := validatedTypes.Load(t); ok {
		return
	}
	if err := validateTypeRecursive(t, t.String(), make(map[reflect.Type]bool)); err != nil {
		panic(err.Error())
	}
	info := computeTypeInfo(t, make(map[reflect.Type]bool))
	validatedTypes.Store(t, info)
}

// getTypeInfo returns the cached typeInfo for T, analyzing it on first call.
func getTypeInfo[T any]() typeInfo {
	t := reflect.TypeFor[T]()
	if v, ok := validatedTypes.Load(t); ok {
		return v.(typeInfo)
	}
	if err := validateTypeRecursive(t, t.String(), make(map[reflect.Type]bool)); err != nil {
		panic(err.Error())
	}
	info := computeTypeInfo(t, make(map[reflect.Type]bool))
	validatedTypes.Store(t, info)
	return info
}

func Validate[T any]() error {
	t := reflect.TypeFor[T]()
	return validateTypeRecursive(t, t.String(), make(map[reflect.Type]bool))
}

func MustValidate[T any]() {
	if err := Validate[T](); err != nil {
		panic(err.Error())
	}
}

func isArenaContainerType(t reflect.Type) bool {
	if t.PkgPath() != "github.com/limpo1989/arena" || t.Kind() != reflect.Struct {
		return false
	}
	name := t.Name()
	return (len(name) > 4 && name[:4] == "Map[") || (len(name) > 7 && name[:7] == "Vector[")
}

func isUnsupportedSyncType(t reflect.Type) bool {
	return t.PkgPath() == "sync" && (t.Name() == "Mutex" || t.Name() == "RWMutex" ||
		t.Name() == "WaitGroup" || t.Name() == "Once" || t.Name() == "Cond")
}

// validateTypeRecursive checks if a type is valid for arena allocation.
// Returns detailed error messages for invalid types.
func validateTypeRecursive(t reflect.Type, path string, visited map[reflect.Type]bool) error {
	if visited[t] {
		return nil
	}
	visited[t] = true

	switch t.Kind() {
	case reflect.Map:
		return &validationError{typePath: path, kind: reflect.Map}
	case reflect.Chan:
		return &validationError{typePath: path, kind: reflect.Chan}
	case reflect.Func:
		return &validationError{typePath: path, kind: reflect.Func}
	case reflect.UnsafePointer:
		return &validationError{typePath: path, kind: reflect.UnsafePointer}
	}

	if isUnsupportedSyncType(t) {
		return &validationError{typePath: path, kind: reflect.Struct}
	}

	if isArenaContainerType(t) {
		return nil
	}

	switch t.Kind() {
	case reflect.Ptr, reflect.Slice, reflect.Array:
		return validateTypeRecursive(t.Elem(), path+"."+t.Kind().String(), visited)
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if err := validateTypeRecursive(field.Type, path+"."+field.Name, visited); err != nil {
				return err
			}
		}
	}

	return nil
}

// computeTypeInfo determines flat and cyclic properties for a validated type.
func computeTypeInfo(t reflect.Type, ancestors map[reflect.Type]bool) typeInfo {
	if v, ok := validatedTypes.Load(t); ok {
		return v.(typeInfo)
	}

	if ancestors[t] {
		return typeInfo{valid: true, flat: false, cyclic: true}
	}

	if isArenaContainerType(t) {
		return typeInfo{valid: true, flat: false, cyclic: false}
	}

	switch t.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.Uintptr:
		return typeInfo{valid: true, flat: true, cyclic: false}

	case reflect.String:
		return typeInfo{valid: true, flat: false, cyclic: false}

	case reflect.Ptr:
		elemInfo := computeTypeInfo(t.Elem(), ancestors)
		cyclic := ancestors[t.Elem()] || elemInfo.cyclic
		return typeInfo{valid: true, flat: false, cyclic: cyclic}

	case reflect.Array:
		elemInfo := computeTypeInfo(t.Elem(), ancestors)
		return typeInfo{valid: true, flat: elemInfo.flat, cyclic: elemInfo.cyclic}

	case reflect.Slice:
		elemInfo := computeTypeInfo(t.Elem(), ancestors)
		return typeInfo{valid: true, flat: false, cyclic: elemInfo.cyclic}

	case reflect.Struct:
		flat := true
		cyclic := false
		ancestors[t] = true
		for i := 0; i < t.NumField(); i++ {
			fi := computeTypeInfo(t.Field(i).Type, ancestors)
			if !fi.flat {
				flat = false
			}
			if fi.cyclic {
				cyclic = true
			}
		}
		delete(ancestors, t)
		return typeInfo{valid: true, flat: flat, cyclic: cyclic}

	case reflect.Interface:
		return typeInfo{valid: true, flat: false, cyclic: false}

	default:
		return typeInfo{valid: true, flat: false, cyclic: false}
	}
}
