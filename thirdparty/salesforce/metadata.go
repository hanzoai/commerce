package salesforce

import (
	"reflect"
	"strings"
)

var mapping map[reflect.Type]string

func init() {
	mapping = make(map[reflect.Type]string)
	mapping[reflect.TypeOf("")] = "TEXT(255)"
	mapping[reflect.TypeOf(Currency(0.0))] = "CURRENCY(16,2)"
	mapping[reflect.TypeOf(true)] = "CHECKBOX"
}

type Metadata struct {
	Name string
	Type string
}

func GetCustomFieldMetadata(sobject interface{}) []Metadata {
	v := reflect.ValueOf(sobject)
	k := v.Kind()
	for k == reflect.Ptr {
		v = reflect.Indirect(v)
		k = v.Kind()
	}

	t := v.Type()
	nFields := t.NumField()
	metadata := make([]Metadata, 0)
	for i := 0; i < nFields; i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		name := strings.Split(jsonTag, ",")[0]
		if strings.Contains(name, "__C") {
			metadata = append(metadata, Metadata{Name: strings.Replace(name, "__C", "", -1), Type: mapping[field.Type]})
		}
	}

	return metadata
}
