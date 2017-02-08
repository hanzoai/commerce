package mixin

import "reflect"

// Get value from *[]*Model slice (returned by Slice()
func getSlice(iface interface{}) reflect.Value {
	// Get value of slice
	slice := reflect.ValueOf(iface)

	// De-pointer
	for slice.Kind() == reflect.Ptr {
		slice = reflect.Indirect(slice)
	}

	return slice
}

var types = make(map[string]reflect.Type)

// Get underlying type for entity of this kind
func Type(k Kind) reflect.Type {
	if typ, ok := types[k.Kind()]; ok {
		return typ
	}

	value := reflect.ValueOf(k)

	// De-pointer if necessary
	for value.Kind() == reflect.Ptr {
		value = reflect.Indirect(value)
	}

	typ := value.Type()
	types[k.Kind()] = typ
	return typ
}

func EntityFromKind(kind Kind) Entity {
	typ := Type(kind)
	entity := reflect.New(typ).Interface().(Entity)
	entity.Init(nil)
	return entity
}
