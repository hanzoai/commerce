package structs

import "reflect"

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
