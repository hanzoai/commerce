// Copyright © 2026 Hanzo AI. MIT License.

package rate

import (
	"testing"

	"github.com/hanzoai/commerce/models/mixin"
)

// A FIELD NAMED LIKE AN ENTITY METHOD SHADOWS IT, AND NOTHING SAYS SO.
//
// mixin.Model provides Key() among ~40 methods that make a struct an Entity.
// This model first carried a field called `Key` — the metered thing's name —
// which shadowed that method, so *Rate quietly stopped satisfying mixin.Entity.
//
// The failure lands nowhere near the cause. Model.Query() type-asserts to Entity
// and, on failure, simply leaves its `entity` nil; the query still builds, the
// filter still applies, and the write path works fine. The panic arrives later
// inside ModelQuery.Get() dereferencing that nil — a stack that names the mixin
// and the caller and never mentions the field. I chased it through the query
// builder, the decoder and two type simplifications before asking the compiler.
//
// This is that question, asked at build time. `var _ mixin.Entity = (*Rate)(nil)`
// would do it in one line; the test exists so the failure carries the
// explanation rather than a bare type error.
func TestMeterIsAnEntity(t *testing.T) {
	if _, ok := any(&Rate{}).(mixin.Entity); !ok {
		t.Fatal("*Rate no longer satisfies mixin.Entity — a field is shadowing one of its methods " +
			"(Key, Kind, Namespace, Context, Datastore, Query…). Query() will leave its entity nil " +
			"and Get() will nil-panic inside the mixin, naming neither this type nor the field. " +
			"Build with `var _ mixin.Entity = (*Rate)(nil)` to have the compiler name it.")
	}
}
