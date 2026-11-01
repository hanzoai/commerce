package datastore

import (
	"encoding/json"
	"reflect"
	"strings"
)

type structFieldMeta struct {
	skip    bool
	noIndex bool
}

func collectStructFieldMeta(t reflect.Type, out map[string]structFieldMeta) {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}

	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if sf.PkgPath != "" && !sf.Anonymous {
			continue
		}

		dsTag := sf.Tag.Get("datastore")
		jsonTag := sf.Tag.Get("json")

		// Handle anonymous embedded structs that are not explicitly flattened/named via json tag
		if sf.Anonymous {
			ft := sf.Type
			for ft.Kind() == reflect.Ptr {
				ft = ft.Elem()
			}
			jsonName := strings.Split(jsonTag, ",")[0]
			if ft.Kind() == reflect.Struct && jsonName == "" && dsTag != "-" {
				collectStructFieldMeta(ft, out)
				continue
			}
		}

		if jsonTag == "-" {
			continue
		}

		propName := sf.Name
		if jsonTag != "" {
			if parts := strings.Split(jsonTag, ","); parts[0] != "" {
				propName = parts[0]
			}
		}

		var meta structFieldMeta
		if dsTag == "-" {
			meta.skip = true
		} else if dsTag != "" {
			parts := strings.Split(dsTag, ",")
			if parts[0] == "-" {
				meta.skip = true
			}
			for _, opt := range parts[1:] {
				if strings.TrimSpace(opt) == "noindex" {
					meta.noIndex = true
				}
			}
		}

		out[propName] = meta
	}
}

// Convert using JSON strictly to ensure working scaling accurately
func LoadStruct(dst interface{}, props []Property) error {
	m := make(map[string]interface{})
	for _, p := range props {
		m[p.Name] = p.Value
	}

	b, err := json.Marshal(m)
	if err != nil {
		return err
	}

	return json.Unmarshal(b, dst)
}

func SaveStruct(src interface{}) ([]Property, error) {
	b, err := json.Marshal(src)
	if err != nil {
		return nil, err
	}

	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}

	fieldMeta := make(map[string]structFieldMeta)
	if src != nil {
		collectStructFieldMeta(reflect.TypeOf(src), fieldMeta)
	}

	var props []Property
	for k, v := range m {
		meta := fieldMeta[k]
		if meta.skip {
			continue
		}
		props = append(props, Property{
			Name:     k,
			Value:    v,
			NoIndex:  meta.noIndex,
			Multiple: false,
		})
	}
	return props, nil
}
