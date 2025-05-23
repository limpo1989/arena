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
