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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// TestValidateAccepts — types that should pass validation
// ---------------------------------------------------------------------------

func TestValidateAccepts(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		assert.Nil(t, Validate[int]())
	})
	t.Run("string", func(t *testing.T) {
		assert.Nil(t, Validate[string]())
	})
	t.Run("*int", func(t *testing.T) {
		assert.Nil(t, Validate[*int]())
	})
	t.Run("[]int", func(t *testing.T) {
		assert.Nil(t, Validate[[]int]())
	})
	t.Run("struct{X int}", func(t *testing.T) {
		type simple struct{ X int }
		assert.Nil(t, Validate[simple]())
	})
	t.Run("[4]int", func(t *testing.T) {
		assert.Nil(t, Validate[[4]int]())
	})
	t.Run("*arena.Map[string,int]", func(t *testing.T) {
		assert.Nil(t, Validate[*Map[string, int]]())
	})
	t.Run("*arena.Vector[int]", func(t *testing.T) {
		assert.Nil(t, Validate[*Vector[int]]())
	})
}

// ---------------------------------------------------------------------------
// TestValidateRejectsMap — Go map type is rejected
// ---------------------------------------------------------------------------

func TestValidateRejectsMap(t *testing.T) {
	// Note: struct types defined inside this package (internal test) are
	// treated as arena types by isArenaType, so their fields are not
	// recursively checked. We test map rejection directly and through
	// pointer/slice/array wrappers instead.
	assert.Panics(t, func() {
		MustValidate[map[int]string]()
	})
}

// ---------------------------------------------------------------------------
// TestValidateRejectsChan — chan type is rejected
// ---------------------------------------------------------------------------

func TestValidateRejectsChan(t *testing.T) {
	assert.Panics(t, func() {
		MustValidate[chan int]()
	})
}

// ---------------------------------------------------------------------------
// TestValidateRejectsFunc — func type is rejected
// ---------------------------------------------------------------------------

func TestValidateRejectsFunc(t *testing.T) {
	assert.Panics(t, func() {
		MustValidate[func()]()
	})
}

// ---------------------------------------------------------------------------
// TestValidateRejectsNested — pointer to map is rejected (nested rejection)
// ---------------------------------------------------------------------------

func TestValidateRejectsNested(t *testing.T) {
	// *map[int]int — the validator recurses through pointers and rejects the map.
	assert.Panics(t, func() {
		MustValidate[*map[int]int]()
	})
}

// ---------------------------------------------------------------------------
// TestValidateRejectsSlice — slice of maps is rejected
// ---------------------------------------------------------------------------

func TestValidateRejectsSlice(t *testing.T) {
	assert.Panics(t, func() {
		MustValidate[[]map[int]string]()
	})
}

// ---------------------------------------------------------------------------
// TestValidateReturnsError — Validate returns error for bad, nil for good
// ---------------------------------------------------------------------------

func TestValidateReturnsError(t *testing.T) {
	t.Run("bad type returns error", func(t *testing.T) {
		err := Validate[map[int]string]()
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "map")
	})

	t.Run("good type returns nil", func(t *testing.T) {
		assert.Nil(t, Validate[int]())
	})
}

// ---------------------------------------------------------------------------
// TestValidateClearMessages — error messages mention "map" and "arena.Map"
// ---------------------------------------------------------------------------

func TestValidateClearMessages(t *testing.T) {
	t.Run("map type message", func(t *testing.T) {
		err := Validate[map[int]string]()
		assert.NotNil(t, err)
		msg := err.Error()
		assert.True(t, strings.Contains(msg, "map"), "error should mention 'map': %s", msg)
		assert.True(t, strings.Contains(msg, "arena.Map"), "error should suggest 'arena.Map': %s", msg)
	})

	t.Run("chan type message", func(t *testing.T) {
		err := Validate[chan int]()
		assert.NotNil(t, err)
		msg := err.Error()
		assert.True(t, strings.Contains(msg, "chan"), "error should mention 'chan': %s", msg)
	})
}

// ---------------------------------------------------------------------------
// TestValidateAutoInNew — New panics with validation error for bad types
// ---------------------------------------------------------------------------

func TestValidateAutoInNew(t *testing.T) {
	a := NewArena()
	defer a.Reset()

	assert.Panics(t, func() {
		New[map[int]int](a)
	})
}

// ---------------------------------------------------------------------------
// TestValidateAutoInDeepCopy — DeepCopy panics for map type
// ---------------------------------------------------------------------------

func TestValidateAutoInDeepCopy(t *testing.T) {
	a := NewArena()
	defer a.Reset()

	assert.Panics(t, func() {
		DeepCopy[map[int]int](a, map[int]int{1: 2})
	})
}
