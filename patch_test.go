package arena

import (
	"math/rand"
	"reflect"
	"testing"
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
