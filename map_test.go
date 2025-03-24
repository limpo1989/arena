package arena

import (
	"runtime"
	"strconv"
	"testing"
	"time"
)

func TestMapBasicOperations(t *testing.T) {
	a := NewArena()
	defer a.Reset()

	m := NewMap[string, int](a, 10)

	// Test Put and Get
	m.Put("one", 1)
	if v := m.Get("one"); v == nil || *v != 1 {
		t.Errorf("Expected 1, got %v", v)
	}

	// Test overwrite
	m.Put("one", 100)
	if v := m.Get("one"); *v != 100 {
		t.Errorf("Expected 100, got %v", *v)
	}

	// Test missing key
	if v := m.Get("missing"); v != nil {
		t.Errorf("Expected nil, got %v", v)
	}
}

func TestAddIfAbsent(t *testing.T) {
	a := NewArena()
	defer a.Reset()

	m := NewMap[int, string](a, 5)

	// First add should succeed
	if added := m.AddIfAbsent(1, "first"); !added {
		t.Error("Expected true for new key")
	}

	// Second add should fail
	if added := m.AddIfAbsent(1, "second"); added {
		t.Error("Expected false for existing key")
	}

	if v := m.Get(1); *v != "first" {
		t.Errorf("Expected 'first', got %v", *v)
	}
}

func TestRemoveAndFree(t *testing.T) {
	a := NewArena()
	defer a.Reset()

	m := NewMap[string, []byte](a, 5)
	key := "data"
	value := []byte{1, 2, 3}

	m.Put(key, value)

	m.Remove(key)
	if m.Len() != 0 {
		t.Error("Map should be empty after removal")
	}

}

func TestClear(t *testing.T) {
	a := NewArena()
	defer a.Reset()

	m := NewMap[int, float64](a, 10)
	for i := 0; i < 100; i++ {
		m.Put(i, float64(i))
	}

	if m.Len() != 100 {
		t.Fatal("Map initialization failed")
	}

	m.Clear()
	if m.Len() != 0 {
		t.Error("Map should be empty after Clear")
	}
}

func TestRange(t *testing.T) {
	a := NewArena()
	defer a.Reset()

	m := NewMap[int, string](a, 5)
	for i := 0; i < 10; i++ {
		m.Put(i, strconv.Itoa(i))
	}

	// Test full iteration
	count := 0
	m.Range(func(k int, v *string) bool {
		count++
		return true
	})
	if count != 10 {
		t.Errorf("Expected 10 iterations, got %d", count)
	}

	// Test early termination
	stoppedAt := 0
	m.Range(func(k int, v *string) bool {
		stoppedAt = k
		return k < 4
	})
	if stoppedAt < 4 {
		t.Errorf("Expected to stop value, stopped at %d", stoppedAt)
	}
}

func TestValueIsolation(t *testing.T) {
	a := NewArena()
	defer a.Reset()

	m := NewMap[string, []int](a, 5)

	// Original slice
	original := []int{1, 2, 3}
	m.Put("slice", original)

	// Modify original slice
	original[0] = 100

	// Verify stored slice remains unchanged
	stored := m.Get("slice")
	if (*stored)[0] != 1 {
		t.Error("DeepCopy failed to isolate arena-stored value")
	}
}

func TestStdMap(t *testing.T) {
	var m = make(map[int]*largeMessage, largeSize)
	for i := 0; i < largeSize; i++ {
		m[i] = prepareArgs()
	}
	start := time.Now()
	runtime.GC()
	var memStat runtime.MemStats
	runtime.ReadMemStats(&memStat)
	t.Logf("Heap GC took time: %v, living objects: %d", time.Since(start), memStat.HeapObjects)
	runtime.KeepAlive(m)
}

func TestArenaMap(t *testing.T) {
	var allocator = NewArena(WithChunkSize(1024 * 1024))
	defer allocator.Reset()

	var m = NewMap[int, largeMessage](allocator, largeSize)
	for i := 0; i < largeSize; i++ {
		m.Put(i, *prepareArgs())
	}
	start := time.Now()
	runtime.GC()
	var memStat runtime.MemStats
	runtime.ReadMemStats(&memStat)
	t.Logf("Arena GC took time: %v, living objects: %d", time.Since(start), memStat.HeapObjects)
	runtime.KeepAlive(m)
}
