package arena

// Vector is an Arena-backed dynamic array providing type-safe operations.
// It reduces GC pressure by storing elements in contiguous Arena memory.
type Vector[T any] struct {
	allocator *Arena
	equatable func(a, b T) bool
	vec       []T
}

// NewVector creates a new Vector with specified initial capacity.
// The vector's memory is managed by the provided Arena allocator.
func NewVector[T any](allocator *Arena, capacity int) *Vector[T] {
	return &Vector[T]{
		allocator: allocator,
		equatable: deepEqual[T],
		vec:       NewSlice[T](allocator, 0, capacity),
	}
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

// Range iterates over elements using a callback function.
func (v *Vector[T]) Range(fn func(index int, v T) bool) {
	for i := 0; i < len(v.vec); i++ {
		if !fn(i, v.vec[i]) {
			return
		}
	}
}

// Iter provides an iterator function compatible with range loops.
//
// Example:
//
//	for index, v := range v.Iter() {
//		// do something
//	}
func (v *Vector[T]) Iter() func(func(index int, v T) bool) {
	return v.Range
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
