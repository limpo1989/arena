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

import "iter"

// Map is an arena-allocated generic hash map implementation.
// Keys are of any comparable type K, values are pointers to arena-allocated memory.
// The map manages both key-value storage and arena memory lifecycle.
type Map[K comparable, V any] struct {
	allocator *Arena
	data      map[K]*V
}

// NewMap creates a new arena-allocated map with specified initial capacity.
func NewMap[K comparable, V any](allocator *Arena, capacity int) *Map[K, V] {
	return &Map[K, V]{
		allocator: allocator,
		data:      make(map[K]*V, capacity),
	}
}

// Put stores a key-value pair in the map. The value is deep copied into arena-allocated memory.
// Existing values for the key are overwritten.
func (m *Map[K, V]) Put(key K, value V) {
	m.data[key] = DeepCopy(m.allocator, value)
}

// AddIfAbsent stores a key-value pair only if the key doesn't already exist.
// Returns true if the value was added, false if the key already existed.
func (m *Map[K, V]) AddIfAbsent(key K, value V) bool {
	if _, ok := m.data[key]; ok {
		return false
	}
	m.data[key] = DeepCopy(m.allocator, value)
	return true
}

// Get retrieves the value associated with the key.
// Returns nil pointer if the key is not found.
func (m *Map[K, V]) Get(key K) *V {
	return m.data[key]
}

// Remove deletes a key-value pair from the map and frees the associated value memory.
func (m *Map[K, V]) Remove(key K) {
	if value, ok := m.data[key]; ok {
		delete(m.data, key)
		m.allocator.Free(value)
	}
}

// Len returns the number of elements in the map.
func (m *Map[K, V]) Len() int {
	return len(m.data)
}

// Clear removes all elements from the map and frees all associated value memory.
func (m *Map[K, V]) Clear() {
	for key, value := range m.data {
		m.allocator.Free(value)
		delete(m.data, key)
	}
}

// Range iterates through the map elements, calling f for each key-value pair.
// Iteration stops if f returns false.
func (m *Map[K, V]) Range(f func(K, *V) bool) {
	for k, v := range m.data {
		if !f(k, v) {
			break
		}
	}
}

// Iter provides an iterator function compatible with range loops.
//
// Example:
//
//	for k, v := range v.Iter() {
//		// do something
//	}
func (m *Map[K, V]) Iter() iter.Seq2[K, *V] {
	return m.Range
}
