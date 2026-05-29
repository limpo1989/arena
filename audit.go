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
	"reflect"
	"unsafe"
)

// ViolationKind describes the type of pointer violation found by AuditPointers.
type ViolationKind string

const (
	ViolationPointer ViolationKind = "pointer"
	ViolationSlice   ViolationKind = "slice"
	ViolationString  ViolationKind = "string"
	ViolationFunc    ViolationKind = "func"
)

// PointerViolation describes a single pointer safety violation found in an arena object.
type PointerViolation struct {
	Path    string        // Full field path, e.g. "GameState.Players.vec[3].Name"
	Kind    ViolationKind // Type of violation: pointer, slice, string, or func
	Address uintptr       // The offending pointer address
	Type    reflect.Type  // The reflect.Type of the field
	Hint    string        // Additional context to help diagnose the violation
}

// arenaAuditer is an internal interface for arena container types to control
// how their internal elements are scanned for pointer violations.
type arenaAuditer interface {
	auditPointers(ar *Arena, path string, violations *[]PointerViolation, visited map[uintptr]struct{})
}

// AuditPointers recursively scans an arena-allocated object for pointers that do not
// belong to this arena's memory. Returns a list of violations describing each
// non-arena pointer found, including the field path and type information.
//
// This is a debugging tool — it walks the object graph using reflect and checks
// every pointer, slice backing array, string data, and func closure pointer
// against the arena's managed address ranges.
func (ar *Arena) AuditPointers(obj any) []PointerViolation {
	ar.locker.Lock()
	defer ar.locker.Unlock()

	violations := make([]PointerViolation, 0)
	val := reflect.ValueOf(obj)
	visited := make(map[uintptr]struct{}, 32)
	auditValue(ar, val, "", &violations, visited)
	return violations
}

func auditValue(ar *Arena, val reflect.Value, path string, violations *[]PointerViolation, visited map[uintptr]struct{}) {
	if !val.IsValid() {
		return
	}

	switch val.Kind() {
	case reflect.Ptr:
		auditPtr(ar, val, path, violations, visited)
	case reflect.Slice:
		auditSlice(ar, val, path, violations, visited)
	case reflect.String:
		auditString(ar, val, path, violations)
	case reflect.Struct:
		auditStruct(ar, val, path, violations, visited)
	case reflect.Array:
		auditArray(ar, val, path, violations, visited)
	case reflect.Interface:
		auditInterface(ar, val, path, violations, visited)
	case reflect.Func:
		auditFunc(ar, val, path, violations)
	}
}

func auditPtr(ar *Arena, val reflect.Value, path string, violations *[]PointerViolation, visited map[uintptr]struct{}) {
	if val.IsNil() {
		return
	}

	ptr := val.Pointer()
	if ptr == 0 {
		return
	}

	if !ar.isManaged(ptr) {
		*violations = append(*violations, PointerViolation{
			Path:    path,
			Kind:    ViolationPointer,
			Address: ptr,
			Type:    val.Type(),
			Hint:    "pointer does not belong to this arena; may be from Go heap (dangerous) or another arena instance",
		})
		return
	}

	// Pointer is in arena — recurse into target
	if val.Elem().Kind() == reflect.Struct || val.Elem().Kind() == reflect.Ptr ||
		val.Elem().Kind() == reflect.Slice || val.Elem().Kind() == reflect.Array ||
		val.Elem().Kind() == reflect.Interface || val.Elem().Kind() == reflect.String {
		patchVal := patchValue(val.Elem())
		if _, seen := visited[ptr]; !seen {
			visited[ptr] = struct{}{}
			auditValue(ar, patchVal, path, violations, visited)
		}
	}
}

func auditSlice(ar *Arena, val reflect.Value, path string, violations *[]PointerViolation, visited map[uintptr]struct{}) {
	if val.IsNil() {
		return
	}

	// Check backing array
	dataPtr := uintptr(val.UnsafePointer())
	if dataPtr != 0 && !ar.isManaged(dataPtr) {
		*violations = append(*violations, PointerViolation{
			Path:    path,
			Kind:    ViolationSlice,
			Address: dataPtr,
			Type:    val.Type(),
			Hint:    "slice backing array does not belong to this arena",
		})
		return
	}

	// Recurse into elements
	elemType := val.Type().Elem()
	needsRecurse := elemType.Kind() == reflect.Ptr || elemType.Kind() == reflect.Struct ||
		elemType.Kind() == reflect.Slice || elemType.Kind() == reflect.Array ||
		elemType.Kind() == reflect.Interface || elemType.Kind() == reflect.String

	if !needsRecurse {
		return
	}

	for i := 0; i < val.Len(); i++ {
		elem := val.Index(i)
		patchVal := patchValue(elem)
		elemPath := path + "[" + itoa(i) + "]"
		auditValue(ar, patchVal, elemPath, violations, visited)
	}
}

func auditString(ar *Arena, val reflect.Value, path string, violations *[]PointerViolation) {
	if val.Len() == 0 {
		return
	}

	// Get string data pointer via unsafe
	var dataPtr uintptr
	s := val.String()
	sp := *(*reflect.StringHeader)(unsafe.Pointer(&s))
	dataPtr = sp.Data

	if dataPtr == 0 {
		return
	}

	if !ar.isManaged(dataPtr) {
		*violations = append(*violations, PointerViolation{
			Path:    path,
			Kind:    ViolationString,
			Address: dataPtr,
			Type:    val.Type(),
			Hint:    "string data not in arena; may be a Go string literal (static, safe) or heap-allocated (dangerous)",
		})
	}
}

func auditStruct(ar *Arena, val reflect.Value, path string, violations *[]PointerViolation, visited map[uintptr]struct{}) {
	// Check if this type implements arenaAuditer via pointer receiver
	if val.CanAddr() {
		if auditer, ok := val.Addr().Interface().(arenaAuditer); ok {
			auditer.auditPointers(ar, path, violations, visited)
			return
		}
	} else if val.CanInterface() {
		// For non-addressable values, try creating a pointer via reflect
		ptrVal := reflect.New(val.Type())
		patchField := patchValue(ptrVal.Elem())
		patchField.Set(val)
		if auditer, ok := ptrVal.Interface().(arenaAuditer); ok {
			auditer.auditPointers(ar, path, violations, visited)
			return
		}
	}

	for i := 0; i < val.NumField(); i++ {
		field := val.Type().Field(i)
		fieldVal := val.Field(i)
		patchVal := patchValue(fieldVal)

		// Skip *Arena fields (hardcoded whitelist)
		if field.Type == reflect.TypeOf((*Arena)(nil)) {
			continue
		}

		// Skip fields tagged arena:"safe"
		if tag, ok := field.Tag.Lookup("arena"); ok && tag == "safe" {
			continue
		}

		fieldPath := path
		if fieldPath != "" {
			fieldPath += "."
		}
		fieldPath += field.Name

		auditValue(ar, patchVal, fieldPath, violations, visited)
	}
}

func auditArray(ar *Arena, val reflect.Value, path string, violations *[]PointerViolation, visited map[uintptr]struct{}) {
	elemType := val.Type().Elem()
	needsRecurse := elemType.Kind() == reflect.Ptr || elemType.Kind() == reflect.Struct ||
		elemType.Kind() == reflect.Slice || elemType.Kind() == reflect.Array ||
		elemType.Kind() == reflect.Interface || elemType.Kind() == reflect.String

	if !needsRecurse {
		return
	}

	for i := 0; i < val.Len(); i++ {
		elem := val.Index(i)
		patchVal := patchValue(elem)
		elemPath := path + "[" + itoa(i) + "]"
		auditValue(ar, patchVal, elemPath, violations, visited)
	}
}

func auditInterface(ar *Arena, val reflect.Value, path string, violations *[]PointerViolation, visited map[uintptr]struct{}) {
	if val.IsNil() {
		return
	}
	concrete := val.Elem()
	patchVal := patchValue(concrete)
	auditValue(ar, patchVal, path, violations, visited)
}

func auditFunc(ar *Arena, val reflect.Value, path string, violations *[]PointerViolation) {
	if val.IsNil() {
		return
	}

	// A func value in a struct is stored as a single pointer (*funcval).
	// Read the funcval pointer from the struct memory.
	var funcvalPtr uintptr
	if val.CanAddr() {
		funcvalPtr = *(*uintptr)(unsafe.Pointer(val.UnsafeAddr()))
	} else {
		// Fallback: get func as interface, extract data pointer
		fn := val.Interface()
		type ifaceHeader struct {
			_    unsafe.Pointer
			data unsafe.Pointer
		}
		hdr := (*ifaceHeader)(unsafe.Pointer(&fn))
		funcvalPtr = uintptr(hdr.data)
	}

	if funcvalPtr == 0 {
		return
	}

	if !ar.isManaged(funcvalPtr) {
		*violations = append(*violations, PointerViolation{
			Path:    path,
			Kind:    ViolationFunc,
			Address: funcvalPtr,
			Type:    val.Type(),
			Hint:    "func closure not in arena; may be a pure function (safe) or closure with heap captures (dangerous); add arena:\"safe\" tag if safe",
		})
	}
}

func itoa(i int) string {
	if i < 10 {
		return digits[i : i+1]
	}
	return _itoa(i)
}

var digits = "0123456789"

func _itoa(i int) string {
	buf := make([]byte, 0, 8)
	for i > 0 {
		buf = append(buf, digits[i%10])
		i /= 10
	}
	for l, r := 0, len(buf)-1; l < r; l, r = l+1, r-1 {
		buf[l], buf[r] = buf[r], buf[l]
	}
	return string(buf)
}
