package datastore

import (
	"encoding/json"
	"reflect"
	"strings"
)

// Property represents a datastore property
type Property struct {
	Name    string
	Value   interface{}
	NoIndex bool
	Multi   bool
}

// PropertyList is a list of properties
type PropertyList []Property

// PropertyLoadSaver is the interface for custom property loading/saving
type PropertyLoadSaver interface {
	Load([]Property) error
	Save() ([]Property, error)
}

type structFieldMeta struct {
	skip    bool
	noIndex bool
}

func parseDatastoreTag(dsTag string) (skip bool, noIndex bool) {
	if dsTag == "-" {
		return true, false
	}
	if dsTag == "" {
		return false, false
	}
	parts := strings.Split(dsTag, ",")
	if parts[0] == "-" {
		skip = true
	}
	for _, opt := range parts[1:] {
		if strings.TrimSpace(opt) == "noindex" {
			noIndex = true
		}
	}
	return skip, noIndex
}

func collectStructFieldMeta(t reflect.Type, out map[string]structFieldMeta, parentSkip bool, parentNoIndex bool) {
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
		fieldSkip, fieldNoIndex := parseDatastoreTag(dsTag)
		effectiveSkip := parentSkip || fieldSkip
		effectiveNoIndex := parentNoIndex || fieldNoIndex

		// Handle anonymous embedded structs flattened by encoding/json
		if sf.Anonymous {
			ft := sf.Type
			for ft.Kind() == reflect.Ptr {
				ft = ft.Elem()
			}
			jsonName := strings.Split(jsonTag, ",")[0]
			if ft.Kind() == reflect.Struct && jsonName == "" && jsonTag != "-" {
				collectStructFieldMeta(ft, out, effectiveSkip, effectiveNoIndex)
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

		out[propName] = structFieldMeta{
			skip:    effectiveSkip,
			noIndex: effectiveNoIndex,
		}
	}
}

// LoadStruct loads properties into a struct (dst should be a pointer)
func LoadStruct(dst interface{}, ps []Property) error {
	// Convert properties to map
	m := make(map[string]interface{})
	for _, p := range ps {
		m[p.Name] = p.Value
	}

	// Marshal to JSON and unmarshal to struct
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, dst)
}

// SaveStruct saves a struct to properties (src should be a pointer)
func SaveStruct(src interface{}) ([]Property, error) {
	// Marshal struct to JSON
	data, err := json.Marshal(src)
	if err != nil {
		return nil, err
	}

	// Unmarshal to map
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	fieldMeta := make(map[string]structFieldMeta)
	if src != nil {
		collectStructFieldMeta(reflect.TypeOf(src), fieldMeta, false, false)
	}

	// Convert map to properties honoring datastore:"-" and datastore:",noindex"
	ps := make([]Property, 0, len(m))
	for k, v := range m {
		meta := fieldMeta[k]
		if meta.skip {
			continue
		}
		ps = append(ps, Property{
			Name:    k,
			Value:   v,
			NoIndex: meta.noIndex,
		})
	}

	return ps, nil
}
