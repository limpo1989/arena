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
	"encoding/json"
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
	flagKeyPlan   int8 = 1 << 2 // copy plan fast path available for K
	flagValuePlan int8 = 1 << 3 // copy plan fast path available for V
)

// Map is an arena-native hash map using linear probing.
// All data (ctrl, keys, values) is stored in arena-allocated slices.
//
// Map is NOT thread-safe. Concurrent calls to Put/Get/Remove on the same Map
// instance require external synchronization (e.g., sync.Mutex). The Arena's
// internal lock (if enabled) only protects the allocator, not the Map's
// internal state.
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
	// Cached copy plans for deep copy fast path (avoids reflect overhead)
	copyPlanK *copyPlan
	copyPlanV *copyPlan
	// Precomputed integer thresholds for resize checks (avoid float64 division)
	loadThreshold      int
	tombstoneThreshold int
}

// NewMap creates a new arena-native hash map with specified initial capacity.
// The returned Map is NOT thread-safe — use external synchronization
// (e.g., sync.Mutex) for concurrent access.
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
	m.copyPlanK = getOrBuildCopyPlan(reflect.TypeFor[K]())
	m.copyPlanV = getOrBuildCopyPlan(reflect.TypeFor[V]())
	// Set plan flags if copy plan fast path is usable:
	// must be non-cyclic, have ops, and not wrap an arena container type.
	if m.copyPlanV != nil && !m.copyPlanV.cyclic && len(m.copyPlanV.ops) > 0 {
		vt := reflect.TypeFor[V]()
		if vt.Kind() == reflect.Ptr {
			vt = vt.Elem()
		}
		if !isArenaContainerType(vt) {
			m.flags |= flagValuePlan
		}
	}
	if m.copyPlanK != nil && !m.copyPlanK.cyclic && len(m.copyPlanK.ops) > 0 {
		kt := reflect.TypeFor[K]()
		if kt.Kind() == reflect.Ptr {
			kt = kt.Elem()
		}
		if !isArenaContainerType(kt) {
			m.flags |= flagKeyPlan
		}
	}
	m.allocArrays(capacity)
	m.computeThresholds()
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
	firstTombstone := -1

	for i := 0; i < m.capacity; i++ {
		c := m.ctrl[idx]
		if c == ctrlEmpty {
			// Prefer reusing a tombstone slot over an empty slot.
			insertIdx := idx
			if firstTombstone >= 0 {
				insertIdx = firstTombstone
			}
			m.ctrl[insertIdx] = hint
			m.keys[insertIdx] = key
			m.values[insertIdx] = value
			if m.flags&(flagKeyDeep|flagValueDeep) != 0 {
				if m.flags&flagKeyDeep != 0 {
					m.deepCopyKey(insertIdx)
				}
				if m.flags&flagValueDeep != 0 {
					m.deepCopyValue(insertIdx)
				}
			}
			if firstTombstone >= 0 {
				m.deleted--
			}
			m.length++
			return
		}
		if c == ctrlDeleted {
			if firstTombstone < 0 {
				firstTombstone = idx
			}
		} else if c == hint && m.keys[idx] == key {
			if m.flags&flagValueDeep != 0 {
				m.deepFreeValue(idx)
			}
			m.values[idx] = value
			if m.flags&flagValueDeep != 0 {
				m.deepCopyValue(idx)
			}
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
			m.ctrl[idx] = ctrlDeleted
			var zeroK K
			var zeroV V
			m.keys[idx] = zeroK
			m.values[idx] = zeroV
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
		// Word-at-a-time empty-group skip: check 8 ctrl bytes at once.
		// ctrlEmpty is -128 (0x80), so 8 empties = 0x8080808080808080.
		const emptyWord = uint64(0x8080808080808080)
		n := m.capacity &^ 7 // round down to multiple of 8
		for i := 0; i < n; i += 8 {
			if *(*uint64)(unsafe.Pointer(&m.ctrl[i])) == emptyWord {
				continue
			}
			for j := i; j < i+8; j++ {
				if m.ctrl[j] != ctrlEmpty && m.ctrl[j] != ctrlDeleted {
					if !yield(m.keys[j], m.values[j]) {
						return
					}
				}
			}
		}
		// Tail: remaining slots beyond the last full 8-group
		for i := n; i < m.capacity; i++ {
			if m.ctrl[i] != ctrlEmpty && m.ctrl[i] != ctrlDeleted {
				if !yield(m.keys[i], m.values[i]) {
					return
				}
			}
		}
	}
}

func (m *Map[K, V]) computeThresholds() {
	m.loadThreshold = int(float64(m.capacity) * loadFactor)
	m.tombstoneThreshold = int(float64(m.capacity) * tombstoneRatio)
}

func (m *Map[K, V]) shouldResize() bool {
	if m.capacity == 0 {
		return true
	}
	if m.length+1 > m.loadThreshold {
		return true
	}
	return m.length+m.deleted+1 > m.tombstoneThreshold
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
	m.computeThresholds()
	m.length = 0
	m.deleted = 0

	for i := 0; i < oldCapacity; i++ {
		if oldCtrl[i] != ctrlEmpty && oldCtrl[i] != ctrlDeleted {
			h := m.hash(oldKeys[i])
			idx := m.probeIndex(h)
			for j := 0; j < m.capacity; j++ {
				if m.ctrl[idx] == ctrlEmpty {
					m.ctrl[idx] = m.ctrlHint(h)
					m.keys[idx] = oldKeys[i]
					m.values[idx] = oldValues[i]
					// Zero old entries so subsequent Free() won't deep-free
					// the shared string/pointer data now owned by the new array.
					var zeroK K
					var zeroV V
					oldKeys[i] = zeroK
					oldValues[i] = zeroV
					m.length++
					break
				}
				idx = (idx + 1) & (m.capacity - 1)
			}
		}
	}

	// Free old arrays. Element data is already zeroed above for active entries,
	// so deepFree will skip them (IsZero early return) and only free the backing arrays.
	m.allocator.Free(oldCtrl)
	oldCtrl = nil
	m.allocator.Free(oldKeys)
	oldKeys = nil
	m.allocator.Free(oldValues)
	oldValues = nil
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
	// Fast path: use copy plan when available and type is not cyclic
	// and not an arena container type.
	if m.flags&flagValuePlan != 0 {
		ptr := unsafe.Pointer(&m.values[idx])
		executeCopyPlan(m.allocator, m.copyPlanV, ptr, ptr, nil)
		return
	}
	// Fallback to reflect for cyclic types or arena containers
	val := reflect.ValueOf(&m.values[idx]).Elem()
	visited := make(map[uintptr]unsafe.Pointer, 32)
	src := reflect.ValueOf(m.values[idx])
	deepCopy(m.allocator, src, val, visited)
}

// deepCopyKey recursively arena-ifies pointer fields within keys[idx].
func (m *Map[K, V]) deepCopyKey(idx int) {
	if m.flags&flagKeyDeep == 0 {
		return
	}
	// Fast path: use copy plan when available and type is not cyclic
	// and not an arena container type.
	if m.flags&flagKeyPlan != 0 {
		ptr := unsafe.Pointer(&m.keys[idx])
		executeCopyPlan(m.allocator, m.copyPlanK, ptr, ptr, nil)
		return
	}
	// Fallback to reflect for cyclic types or arena containers
	key := reflect.ValueOf(&m.keys[idx]).Elem()
	visited := make(map[uintptr]unsafe.Pointer, 32)
	src := reflect.ValueOf(m.keys[idx])
	deepCopy(m.allocator, src, key, visited)
}

// deepFreeKey recursively frees arena sub-allocations of keys[idx].
func (m *Map[K, V]) deepFreeKey(idx int) {
	if m.flags&flagKeyDeep == 0 {
		return
	}
	m.allocator.Free(m.keys[idx])
	var zeroK K
	m.keys[idx] = zeroK
}

// deepFreeValue recursively frees arena sub-allocations of values[idx].
func (m *Map[K, V]) deepFreeValue(idx int) {
	if m.flags&flagValueDeep == 0 {
		return
	}
	m.allocator.Free(m.values[idx])
	var zeroV V
	m.values[idx] = zeroV
}

// MarshalJSON implements the json.Marshaler interface.
// It serializes the Map as a JSON object ({"key": value, ...}).
func (m *Map[K, V]) MarshalJSON() ([]byte, error) {
	tmp := make(map[K]V, m.Len())
	for k, v := range m.All() {
		tmp[k] = v
	}
	return json.Marshal(tmp)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// It clears the Map and populates it from a JSON object.
// All data is deep-copied into arena memory.
func (m *Map[K, V]) UnmarshalJSON(data []byte) error {
	m.Clear()
	var tmp map[K]V
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	for k, v := range tmp {
		m.Put(k, v)
	}
	return nil
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
