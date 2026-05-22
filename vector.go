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
	"iter"
	"reflect"
)

// Vector is an Arena-backed dynamic array providing type-safe operations.
// It reduces GC pressure by storing elements in contiguous Arena memory.
type Vector[T any] struct {
	allocator *Arena
	equatable func(a, b T) bool `arena:"safe"`
	vec       []T
}

// NewVector creates a new Vector with specified initial capacity.
// The vector's memory is managed by the provided Arena allocator.
func NewVector[T any](allocator *Arena, capacity int) *Vector[T] {
	validateType[T]()
	v := New[Vector[T]](allocator)
	v.allocator = allocator
	v.equatable = deepEqual[T]
	v.vec = NewSlice[T](allocator, 0, capacity)
	return v
}

// Equatable sets a custom equality comparison function for element comparison.
func (v *Vector[T]) Equatable(equatable func(a, b T) bool) *Vector[T] {
	v.equatable = equatable
	return v
}

// Len returns the current number of elements in the vector.
func (v *Vector[T]) Len() int {
	return len(v.vec)
}

// Cap returns the current capacity of the vector.
func (v *Vector[T]) Cap() int {
	return cap(v.vec)
}

// At retrieves the element at the specified index.
func (v *Vector[T]) At(index int) T {
	return v.vec[index]
}

// All provides an iterator function compatible with range loops.
func (v *Vector[T]) All() iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		for i, val := range v.vec {
			if !yield(i, val) {
				return
			}
		}
	}
}

// Append adds elements to the end of the vector.
func (v *Vector[T]) Append(values ...T) *Vector[T] {
	v.vec = Append(v.allocator, v.vec, values...)
	return v
}

// AddIfAbsent adds an element only if it doesn't already exist in the vector.
func (v *Vector[T]) AddIfAbsent(value T) bool {
	// 已经存在元素不添加
	if idx := v.Index(value); -1 != idx {
		return false
	}
	// 追加元素
	v.Append(value)
	return true
}

// RemoveIdx removes the element at the specified index.
func (v *Vector[T]) RemoveIdx(idx int) {
	v.vec = Append(v.allocator, v.vec[:idx], v.vec[idx+1:]...)
}

// Remove deletes the first occurrence of the specified element.
func (v *Vector[T]) Remove(value T) bool {
	// 找到元素位置，并进行移除
	if idx := v.Index(value); -1 != idx {
		v.RemoveIdx(idx)
		return true
	}
	return false
}

// RemoveBy removes elements matching a condition with quantity control.
// use limit param to control maximum number of elements to remove (0 = unlimited)
func (v *Vector[T]) RemoveBy(limit int, fn func(index int, v T) bool) int {
	var removed int
	for i := len(v.vec) - 1; i >= 0; i-- {
		if fn(i, v.vec[i]) {
			v.RemoveIdx(i)
			if removed++; removed >= limit && limit > 0 {
				return removed
			}
		}
	}
	return removed
}

// Index finds the first occurrence of an element.
// Index of first match, or -1 if not found
func (v *Vector[T]) Index(value T) int {
	for i := 0; i < len(v.vec); i++ {
		if v.equatable(v.vec[i], value) {
			return i
		}
	}
	return -1
}

// LastIndex finds the last occurrence of an element.
// Index of last match, or -1 if not found
func (v *Vector[T]) LastIndex(value T) int {
	for i := len(v.vec) - 1; i >= 0; i-- {
		if v.equatable(v.vec[i], value) {
			return i
		}
	}
	return -1
}

// Clear remove all elements.
func (v *Vector[T]) Clear() {
	v.allocator.Free(v.vec)
	v.vec = nil
}

// arenaDeepCopy implements the deepCopier interface.
func (v *Vector[T]) arenaDeepCopy(allocator *Arena) reflect.Value {
	newVec := NewVector[T](allocator, v.Len())
	for i := 0; i < v.Len(); i++ {
		newVec.Append(v.At(i))
	}
	return reflect.ValueOf(newVec)
}

// auditPointers implements the arenaAuditer interface.
func (v *Vector[T]) auditPointers(ar *Arena, path string, violations *[]PointerViolation, visited map[uintptr]struct{}) {
	for i := 0; i < len(v.vec); i++ {
		elemPath := path + ".vec[" + itoa(i) + "]"
		elemField := reflect.ValueOf(&v.vec[i]).Elem()
		elemVal := patchValue(elemField)
		auditValue(ar, elemVal, elemPath, violations, visited)
	}
}
