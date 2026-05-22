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
	"hash/maphash"
	"iter"
	"reflect"
	"unsafe"
)

const (
	ctrlEmpty      int8 = -128
	ctrlDeleted    int8 = -1
	loadFactor          = 0.7
	tombstoneRatio      = 0.85
	minCapacity         = 8

	flagKeyDeep   int8 = 1 << 0
	flagValueDeep int8 = 1 << 1
)

// Map is an arena-native hash map using linear probing.
// All data (ctrl, keys, values) is stored in arena-allocated slices.
type Map[K comparable, V any] struct {
	allocator *Arena
	seed      maphash.Seed
	length    int
	deleted   int
	capacity  int
	flags     int8
	ctrl      []int8
	keys      []K
	values    []V
}

// NewMap creates a new arena-native hash map with specified initial capacity.
func NewMap[K comparable, V any](allocator *Arena, capacity int) *Map[K, V] {
	validateType[K]()
	validateType[V]()

	capacity = nextPowerOf2(max(capacity, minCapacity))
	m := New[Map[K, V]](allocator)
	m.allocator = allocator
	m.seed = maphash.MakeSeed()
	m.capacity = capacity
	if needsDeepCopy(reflect.TypeFor[K]().Kind()) {
		m.flags |= flagKeyDeep
	}
	if needsDeepCopy(reflect.TypeFor[V]().Kind()) {
		m.flags |= flagValueDeep
	}
	m.allocArrays(capacity)
	return m
}

func (m *Map[K, V]) allocArrays(capacity int) {
	m.ctrl = NewSlice[int8](m.allocator, capacity, capacity)
	for i := range m.ctrl {
		m.ctrl[i] = ctrlEmpty
	}
	m.keys = NewSlice[K](m.allocator, capacity, capacity)
	m.values = NewSlice[V](m.allocator, capacity, capacity)
}

func (m *Map[K, V]) hash(key K) uint64 {
	return maphash.Comparable(m.seed, key)
}

func (m *Map[K, V]) probeIndex(h uint64) int {
	return int(h & uint64(m.capacity-1))
}

func (m *Map[K, V]) ctrlHint(h uint64) int8 {
	return int8(h >> 57)
}

// Put stores a key-value pair. Existing values are freed and overwritten.
func (m *Map[K, V]) Put(key K, value V) {
	if m.shouldResize() {
		m.resize()
	}

	h := m.hash(key)
	hint := m.ctrlHint(h)
	idx := m.probeIndex(h)

	for i := 0; i < m.capacity; i++ {
		if m.ctrl[idx] == ctrlEmpty {
			m.ctrl[idx] = hint
			m.keys[idx] = key
			m.values[idx] = value
			m.deepCopyKey(idx)
			m.deepCopyValue(idx)
			m.length++
			return
		}
		if m.ctrl[idx] == hint && m.keys[idx] == key {
			m.deepFreeValue(idx)
			m.values[idx] = value
			m.deepCopyValue(idx)
			return
		}
		idx = (idx + 1) & (m.capacity - 1)
	}

	// Should not reach here if resize works correctly
	panic("map is full")
}

// Get retrieves the value for the given key.
// Returns the value and true if found, or the zero value and false otherwise.
func (m *Map[K, V]) Get(key K) (V, bool) {
	h := m.hash(key)
	hint := m.ctrlHint(h)
	idx := m.probeIndex(h)

	for i := 0; i < m.capacity; i++ {
		if m.ctrl[idx] == ctrlEmpty {
			var zero V
			return zero, false
		}
		if m.ctrl[idx] == hint && m.keys[idx] == key {
			return m.values[idx], true
		}
		idx = (idx + 1) & (m.capacity - 1)
	}
	var zero V
	return zero, false
}

// Remove deletes a key-value pair and frees the value's arena memory.
func (m *Map[K, V]) Remove(key K) {
	h := m.hash(key)
	hint := m.ctrlHint(h)
	idx := m.probeIndex(h)

	for i := 0; i < m.capacity; i++ {
		if m.ctrl[idx] == ctrlEmpty {
			return
		}
		if m.ctrl[idx] == hint && m.keys[idx] == key {
			m.deepFreeKey(idx)
			m.deepFreeValue(idx)
			var zeroV V
			m.values[idx] = zeroV
			var zeroK K
			m.keys[idx] = zeroK
			m.ctrl[idx] = ctrlDeleted
			m.length--
			m.deleted++
			return
		}
		idx = (idx + 1) & (m.capacity - 1)
	}
}

// AddIfAbsent stores a key-value pair only if the key doesn't exist.
// Returns true if added, false if the key already existed.
func (m *Map[K, V]) AddIfAbsent(key K, value V) bool {
	if _, ok := m.Get(key); ok {
		return false
	}
	m.Put(key, value)
	return true
}

// Clear removes all entries and frees all value arena memory.
func (m *Map[K, V]) Clear() {
	for i := 0; i < m.capacity; i++ {
		if m.ctrl[i] != ctrlEmpty && m.ctrl[i] != ctrlDeleted {
			m.deepFreeKey(i)
			m.deepFreeValue(i)
			var zeroV V
			m.values[i] = zeroV
			var zeroK K
			m.keys[i] = zeroK
		}
		m.ctrl[i] = ctrlEmpty
	}
	m.length = 0
	m.deleted = 0
}

// Len returns the number of entries in the map.
func (m *Map[K, V]) Len() int {
	return m.length
}

// All provides an iterator compatible with range loops.
func (m *Map[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for i := 0; i < m.capacity; i++ {
			if m.ctrl[i] != ctrlEmpty && m.ctrl[i] != ctrlDeleted {
				if !yield(m.keys[i], m.values[i]) {
					return
				}
			}
		}
	}
}

func (m *Map[K, V]) shouldResize() bool {
	if m.capacity == 0 {
		return true
	}
	load := float64(m.length+1) / float64(m.capacity)
	if load > loadFactor {
		return true
	}
	tombstone := float64(m.length+m.deleted+1) / float64(m.capacity)
	return tombstone > tombstoneRatio
}

func (m *Map[K, V]) resize() {
	oldCtrl := m.ctrl
	oldKeys := m.keys
	oldValues := m.values
	oldCapacity := m.capacity

	newCapacity := m.capacity * 2
	if newCapacity < minCapacity {
		newCapacity = minCapacity
	}

	m.allocArrays(newCapacity)
	m.capacity = newCapacity
	m.length = 0
	m.deleted = 0

	for i := 0; i < oldCapacity; i++ {
		if oldCtrl[i] != ctrlEmpty && oldCtrl[i] != ctrlDeleted {
			idx := m.probeIndex(m.hash(oldKeys[i]))
			for j := 0; j < m.capacity; j++ {
				if m.ctrl[idx] == ctrlEmpty {
					m.ctrl[idx] = m.ctrlHint(m.hash(oldKeys[i]))
					m.keys[idx] = oldKeys[i]
					m.values[idx] = oldValues[i]
					m.deepCopyKey(idx)
					m.deepCopyValue(idx)
					m.length++
					break
				}
				idx = (idx + 1) & (m.capacity - 1)
			}
		}
	}

	// Free old arrays
	m.allocator.Free(oldCtrl)
	m.allocator.Free(oldKeys)
	m.allocator.Free(oldValues)
}

// needsDeepCopy reports whether a value of the given kind requires recursive deep copy.
func needsDeepCopy(k reflect.Kind) bool {
	return k == reflect.Ptr || k == reflect.Struct ||
		k == reflect.Slice || k == reflect.Array ||
		k == reflect.String || k == reflect.Interface
}

// deepCopyValue recursively arena-ifies pointer fields within values[idx].
func (m *Map[K, V]) deepCopyValue(idx int) {
	if m.flags&flagValueDeep == 0 {
		return
	}
	val := reflect.ValueOf(&m.values[idx]).Elem()
	visited := make(map[uintptr]reflect.Value, 64)
	src := reflect.ValueOf(m.values[idx])
	deepCopy(m.allocator, src, val, visited)
}

// deepCopyKey recursively arena-ifies pointer fields within keys[idx].
func (m *Map[K, V]) deepCopyKey(idx int) {
	if m.flags&flagKeyDeep == 0 {
		return
	}
	key := reflect.ValueOf(&m.keys[idx]).Elem()
	visited := make(map[uintptr]reflect.Value, 64)
	src := reflect.ValueOf(m.keys[idx])
	deepCopy(m.allocator, src, key, visited)
}

// deepFreeKey recursively frees arena sub-allocations of keys[idx].
func (m *Map[K, V]) deepFreeKey(idx int) {
	if m.flags&flagKeyDeep == 0 {
		return
	}
	m.allocator.Free(m.keys[idx])
}

// deepFreeValue recursively frees arena sub-allocations of values[idx].
func (m *Map[K, V]) deepFreeValue(idx int) {
	if m.flags&flagValueDeep == 0 {
		return
	}
	m.allocator.Free(m.values[idx])
}

// arenaDeepCopy implements the deepCopier interface.
func (m *Map[K, V]) arenaDeepCopy(allocator *Arena) reflect.Value {
	newMap := NewMap[K, V](allocator, m.length)
	for key, val := range m.All() {
		newMap.Put(key, val)
	}
	return reflect.ValueOf(newMap)
}

// auditPointers implements the arenaAuditer interface.
func (m *Map[K, V]) auditPointers(ar *Arena, path string, violations *[]PointerViolation, visited map[uintptr]struct{}) {
	for i := 0; i < m.capacity; i++ {
		if m.ctrl[i] <= ctrlDeleted {
			continue
		}

		keyPath := path + ".keys[" + itoa(i) + "]"
		keyVal := reflect.ValueOf(m.keys[i])
		auditValue(ar, keyVal, keyPath, violations, visited)

		valPath := path + ".values[" + itoa(i) + "]"
		valField := reflect.ValueOf(&m.values[i]).Elem()
		patchVal := patchValue(valField)
		auditValue(ar, patchVal, valPath, violations, visited)
	}
}

func nextPowerOf2(n int) int {
	if n <= 0 {
		return minCapacity
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n++
	return n
}

// makeSliceFromPtr creates a reflect.Value of a slice type backed by arena memory.
func makeSliceFromPtr(elemType reflect.Type, ptr unsafe.Pointer, length, capacity int) reflect.Value {
	sliceType := reflect.SliceOf(elemType)
	sliceHeader := reflect.NewAt(sliceType, unsafe.Pointer(&struct {
		Data uintptr
		Len  int
		Cap  int
	}{uintptr(ptr), length, capacity})).Elem()
	return sliceHeader
}
