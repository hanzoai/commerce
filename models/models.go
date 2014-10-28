package models

import (
	"encoding/json"
	"fmt"
	"github.com/mholt/binding"
)

// This is just a convenience struct for objects which are JSON
// serializable/deserialized only.
type FieldMapMixin struct{}

// Noop, binding delegates to encoding/json
func (f *FieldMapMixin) FieldMap() binding.FieldMap {
	return binding.FieldMap{}
}

func asJSON(value interface{}) string {
	b, err := json.Marshal(value)
	if err != nil {
		fmt.Println("error:", err)
	}
	return string(b)
}
