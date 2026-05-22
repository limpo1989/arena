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

var validatedTypes sync.Map // map[reflect.Type]bool

func validateType[T any]() {
	t := reflect.TypeFor[T]()
	if _, ok := validatedTypes.Load(t); ok {
		return
	}
	if err := validateTypeRecursive(t, t.String(), make(map[reflect.Type]bool)); err != nil {
		panic(err.Error())
	}
	validatedTypes.Store(t, true)
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

	// arena.Map and arena.Vector are always valid — their type params are checked at construction
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
