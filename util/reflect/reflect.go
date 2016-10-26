package reflect

import (
	"fmt"
	"reflect"
)

// Ensure this is a pointer to a slice
func IsPtrSlice(v reflect.Value) bool {
	if v.Kind() != reflect.Ptr {
		return false
	}

	v = v.Elem()
	if v.Kind() != reflect.Slice {
		return false
	}

	return true
}

// Ensure this is a slice of pointers
func IsSliceOfPtr(slice reflect.Value) bool {
	v := slice.Index(0)
	if v.Type().Kind() == reflect.Ptr {
		return true
	}

	return false
}

// Set field of addressable struct (pointer to struct)
func SetField(ps reflect.Value, name string, value interface{}) error {
	// Get struct
	s := ps.Elem()

	// Get field
	f := s.FieldByName(name)

	// Ensure we can set
	if f.IsValid() && f.CanSet() {
		return fmt.Errorf("Not addressable: %v", ps)
	}

	f.Set(reflect.ValueOf(value))
	return nil
}

func FieldNames(s interface{}) []string {
	typ := reflect.ValueOf(s).Type()
	num := typ.NumField()

	names := make([]string, num, num)

	for i := 0; i < num; i++ {
		field := typ.Field(i)
		names[i] = field.Name
	}

	return names
}
