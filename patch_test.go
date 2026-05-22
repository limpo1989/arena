package arena

import (
	"math/rand"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type patchTest struct {
	id int32
}

func TestPatch(t *testing.T) {

	p := patchTest{id: 111}
	pv := reflect.ValueOf(&p).Elem()
	for i := 0; i < pv.NumField(); i++ {
		ft := pv.Type().Field(i)
		f := pv.Field(i)
		if !f.CanSet() {
			pf := patchValue(f)
			if !pf.CanSet() {
				t.Fatalf("PatchValue[%d].CanSet()=false", i)
			}

			if "id" == ft.Name {
				var randInt = int64(rand.Int31())
				pf.SetInt(randInt)
				if getInt := pf.Int(); getInt != randInt {
					t.Fatalf("PatchValue[%d].Int()=%d", i, randInt)
				}
			}
		}
	}

}

// ---------------------------------------------------------------------------
// Additional patch tests
// ---------------------------------------------------------------------------

// patchMultiField has multiple unexported fields of different kinds.
type patchMultiField struct {
	code  int32
	label string
	flag  bool
}

func TestPatchMultiField(t *testing.T) {
	obj := patchMultiField{code: 7, label: "hello", flag: true}
	pv := reflect.ValueOf(&obj).Elem()

	for i := 0; i < pv.NumField(); i++ {
		f := pv.Field(i)
		assert.False(t, f.CanSet(), "unexported field should not be settable before patch")

		pf := patchValue(f)
		assert.True(t, pf.CanSet(), "field %d should be settable after patchValue", i)
	}

	// Set individual fields via patched values
	nameField := patchValue(pv.FieldByName("label"))
	nameField.SetString("world")
	assert.Equal(t, "world", obj.label)

	codeField := patchValue(pv.FieldByName("code"))
	codeField.SetInt(42)
	assert.Equal(t, int32(42), obj.code)

	flagField := patchValue(pv.FieldByName("flag"))
	flagField.SetBool(false)
	assert.Equal(t, false, obj.flag)
}

// patchInner is the inner struct for TestPatchNestedStruct.
type patchInner struct {
	x int
	y int
}

// patchOuter contains a nested unexported struct.
type patchOuter struct {
	name  string
	inner patchInner
}

func TestPatchNestedStruct(t *testing.T) {
	obj := patchOuter{
		name:  "outer",
		inner: patchInner{x: 1, y: 2},
	}
	pv := reflect.ValueOf(&obj).Elem()

	// Patch the top-level unexported fields
	nameF := patchValue(pv.FieldByName("name"))
	assert.True(t, nameF.CanSet())
	nameF.SetString("patched")
	assert.Equal(t, "patched", obj.name)

	// Patch the nested struct field
	innerF := patchValue(pv.FieldByName("inner"))
	assert.True(t, innerF.CanSet())

	// Access fields of the nested struct — inner fields are also unexported
	// and need patchValue applied individually.
	xF := patchValue(innerF.FieldByName("x"))
	assert.True(t, xF.CanSet())
	xF.SetInt(99)
	assert.Equal(t, 99, obj.inner.x)

	yF := patchValue(innerF.FieldByName("y"))
	assert.True(t, yF.CanSet())
	yF.SetInt(88)
	assert.Equal(t, 88, obj.inner.y)
}
