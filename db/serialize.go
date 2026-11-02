package db

// PropertyLoadSaver-compatible serialization for SQLite.
//
// The old Cloud Datastore used PropertyLoadSaver which serialized ALL exported
// struct fields using Go field names as property keys, regardless of json tags.
// Filter("Claims.OrganizationName=", v) matched property "Claims.OrganizationName".
//
// Our SQLite backend stores entities as JSON. To maintain compatibility with
// the query builder's toJSONFieldName() (which converts PascalCase → camelCase),
// we serialize using camelCase Go field names as JSON keys instead of json tags.
//
// This means:
//   - Filter("Email=", v) → json_extract(data, '$.email') — matches stored key "email" ✓
//   - Filter("Claims.OrganizationName=", v) → '$.claims.organizationName' — not '$.claims.org' ✓
//   - Fields with json:"-" ARE stored (unless datastore:"-") ✓
//   - Fields with datastore:"-" are NOT stored ✓
//   - Types implementing json.Marshaler preserve their serialization (time.Time, etc.) ✓

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"unicode"
)

var jsonMarshalerType = reflect.TypeOf((*json.Marshaler)(nil)).Elem()

// marshalForDB serializes a struct using camelCase Go field names as JSON keys.
func marshalForDB(src any) ([]byte, error) {
	v := reflect.ValueOf(src)
	m := dbStructToMap(v)
	if m == nil {
		// Fallback for non-structs (shouldn't happen for entities)
		return json.Marshal(src)
	}
	return json.Marshal(m)
}

// unmarshalForDB deserializes JSON into a struct, matching camelCase Go field names.
func unmarshalForDB(data []byte, dst any) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	return dbMapToStruct(m, reflect.ValueOf(dst))
}

// dbStructToMap converts a struct value to a map using camelCase Go field names.
// Embedded structs are flattened. Fields with datastore:"-" are skipped.
func dbStructToMap(v reflect.Value) map[string]any {
	v = dbDeref(v)
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return nil
	}

	result := make(map[string]any)
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fv := v.Field(i)

		if !field.IsExported() {
			continue
		}

		// Skip datastore:"-" (runtime-only fields)
		if field.Tag.Get("datastore") == "-" {
			continue
		}

		// Flatten embedded structs into parent
		if field.Anonymous {
			sub := dbStructToMap(fv)
			for k, val := range sub {
				result[k] = val
			}
			continue
		}

		name := dbLcFirst(field.Name)
		result[name] = dbToVal(fv)
	}

	return result
}

// dbToVal converts a reflect.Value to a JSON-serializable value.
// Structs are recursively converted to maps (using Go field names).
// Types implementing json.Marshaler are left as-is for json.Marshal.
func dbToVal(v reflect.Value) any {
	// Handle pointers / interfaces
	if v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	// Types with custom JSON serialization (time.Time, etc.) — use as-is
	if dbImplementsMarshaler(v) {
		return v.Interface()
	}

	switch v.Kind() {
	case reflect.Struct:
		return dbStructToMap(v)

	case reflect.Slice:
		if v.IsNil() {
			return nil
		}
		if dbIsRecursiveElem(v.Type().Elem()) {
			out := make([]any, v.Len())
			for i := 0; i < v.Len(); i++ {
				out[i] = dbToVal(v.Index(i))
			}
			return out
		}
		return v.Interface()

	case reflect.Map:
		if v.IsNil() {
			return nil
		}
		if dbIsRecursiveElem(v.Type().Elem()) {
			out := make(map[string]any)
			for _, k := range v.MapKeys() {
				out[k.String()] = dbToVal(v.MapIndex(k))
			}
			return out
		}
		return v.Interface()

	default:
		if !v.IsValid() {
			return nil
		}
		return v.Interface()
	}
}

// dbMapToStruct populates a struct from a JSON map, matching camelCase Go field names.
func dbMapToStruct(m map[string]json.RawMessage, v reflect.Value) error {
	v = dbDeref(v)
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return nil
	}

	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fv := v.Field(i)

		if !field.IsExported() || !fv.CanSet() {
			continue
		}

		if field.Tag.Get("datastore") == "-" {
			// A runtime field is never stored by this codec. A row stored by its json
			// tags (encodeRow's legacy keys, or a Postgres row written before it) holds
			// it there, and it is read from there; the field's own Load still runs
			// after, from the raw storage field, where there is one.
			if tag := jsonName(field); tag != "" {
				if raw, ok := m[tag]; ok && len(raw) > 0 && string(raw) != "null" {
					dbFromRaw(raw, field.Type, fv)
				}
			}
			continue
		}

		// Embedded structs — process from same level
		if field.Anonymous {
			target := fv
			if target.Kind() == reflect.Ptr {
				if target.IsNil() {
					target.Set(reflect.New(target.Type().Elem()))
				}
				target = target.Elem()
			}
			if target.Kind() == reflect.Struct {
				dbMapToStruct(m, target)
			}
			continue
		}

		name := dbLcFirst(field.Name)
		raw, ok := m[name]
		if !ok {
			// A row stored by json tags (a Postgres row written before this codec)
			// holds the field under its tag.
			if tag := jsonName(field); tag != "" && tag != name {
				raw, ok = m[tag]
			}
		}
		if !ok || len(raw) == 0 || string(raw) == "null" {
			continue
		}

		dbFromRaw(raw, field.Type, fv)
	}

	return nil
}

// jsonName is the key encoding/json names a field by: its json tag's name, the
// field's own name when the tag names none, "" for a field json leaves out.
func jsonName(f reflect.StructField) string {
	tag := f.Tag.Get("json")
	if tag == "-" {
		return ""
	}
	if name, _, _ := strings.Cut(tag, ","); name != "" {
		return name
	}
	return f.Name
}

// encodeRow is a row as Postgres stores it: this codec's keys — the camelCase Go
// field names every filter names and this codec reads — and, beside them, every
// key encoding/json gives the same value that those do not already hold. The
// second set is what an image before this codec read a Postgres row by (its json
// tags), so a row written now still reads there: a field whose tag is not its
// name (`Code_` tagged "code") is stored under both, a runtime field
// (datastore:"-") under its tag. Where the two name one key, this codec's value
// stands.
func encodeRow(src any) ([]byte, error) {
	m := dbStructToMap(reflect.ValueOf(src))
	if m == nil {
		return json.Marshal(src)
	}
	if legacy, err := json.Marshal(src); err == nil {
		var old map[string]any
		if exact(legacy, &old) == nil {
			union(m, old)
		}
	}
	return json.Marshal(m)
}

// exact decodes JSON keeping every number as written (json.Number), so a value
// re-read here is written back digit for digit, never through a float.
func exact(b []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	return d.Decode(v)
}

// union adds to m every key of old it does not hold, and does the same inside the
// objects and the lists of objects both hold under one key, reporting whether it
// added any. m's values are dbToVal's (a struct is a map[string]any, a list of
// them []any) and old's are decoded JSON; a value of m is re-read through JSON
// only when something is added inside it, so the rest is written as it was.
func union(m map[string]any, old map[string]any) bool {
	added := false
	for k, ov := range old {
		mv, held := m[k]
		if !held {
			m[k], added = ov, true
			continue
		}
		switch o := ov.(type) {
		case map[string]any:
			if mm, ok := asObject(mv); ok && union(mm, o) {
				m[k], added = mm, true
			}
		case []any:
			ml, ok := asList(mv)
			if !ok || len(ml) != len(o) {
				continue
			}
			grew := false
			for i := range ml {
				om, ook := o[i].(map[string]any)
				mm, mok := asObject(ml[i])
				if ook && mok && union(mm, om) {
					ml[i], grew = mm, true
				}
			}
			if grew {
				m[k], added = ml, true
			}
		}
	}
	return added
}

// asObject is v as a JSON object, when it is one.
func asObject(v any) (map[string]any, bool) {
	if m, ok := v.(map[string]any); ok {
		return m, true
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	var m map[string]any
	if exact(b, &m) != nil || m == nil {
		return nil, false
	}
	return m, true
}

// asList is v as a JSON list, when it is one.
func asList(v any) ([]any, bool) {
	if l, ok := v.([]any); ok {
		return l, true
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	var l []any
	if exact(b, &l) != nil || l == nil {
		return nil, false
	}
	return l, true
}

// dbFromRaw unmarshals a json.RawMessage into a struct field.
func dbFromRaw(raw json.RawMessage, ft reflect.Type, fv reflect.Value) {
	isPtr := ft.Kind() == reflect.Ptr
	baseType := ft
	if isPtr {
		baseType = ft.Elem()
	}

	// For struct types that need recursion, parse as nested map
	if dbIsRecursiveElem(baseType) {
		var nested map[string]json.RawMessage
		if json.Unmarshal(raw, &nested) == nil {
			if isPtr {
				if fv.IsNil() {
					fv.Set(reflect.New(baseType))
				}
				dbMapToStruct(nested, fv.Elem())
			} else {
				dbMapToStruct(nested, fv)
			}
			return
		}
	}

	// For slices of recursive structs
	if baseType.Kind() == reflect.Slice && dbIsRecursiveElem(baseType.Elem()) {
		var arr []json.RawMessage
		if json.Unmarshal(raw, &arr) == nil {
			elemType := baseType.Elem()
			elemIsPtr := elemType.Kind() == reflect.Ptr
			if elemIsPtr {
				elemType = elemType.Elem()
			}
			slice := reflect.MakeSlice(baseType, len(arr), len(arr))
			for j, item := range arr {
				var nested map[string]json.RawMessage
				if json.Unmarshal(item, &nested) == nil {
					elem := reflect.New(elemType).Elem()
					dbMapToStruct(nested, elem)
					if elemIsPtr {
						ptr := reflect.New(elemType)
						ptr.Elem().Set(elem)
						slice.Index(j).Set(ptr)
					} else {
						slice.Index(j).Set(elem)
					}
				}
			}
			if isPtr {
				if fv.IsNil() {
					fv.Set(reflect.New(baseType))
				}
				fv.Elem().Set(slice)
			} else {
				fv.Set(slice)
			}
			return
		}
	}

	// Default: json.Unmarshal (handles json.Unmarshaler, primitives, etc.)
	if fv.CanAddr() {
		json.Unmarshal(raw, fv.Addr().Interface())
	}
}

// dbImplementsMarshaler checks if a value's type implements json.Marshaler.
func dbImplementsMarshaler(v reflect.Value) bool {
	if !v.IsValid() {
		return false
	}
	t := v.Type()
	if t.Implements(jsonMarshalerType) {
		return true
	}
	if v.CanAddr() && reflect.PtrTo(t).Implements(jsonMarshalerType) {
		return true
	}
	return false
}

// dbIsRecursiveElem returns true if the element type is a struct that
// doesn't implement json.Marshaler (needs recursive Go-name serialization).
func dbIsRecursiveElem(t reflect.Type) bool {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return false
	}
	if t.Implements(jsonMarshalerType) || reflect.PtrTo(t).Implements(jsonMarshalerType) {
		return false
	}
	return true
}

// dbLcFirst lowercases the first rune of a string.
func dbLcFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

// dbDeref dereferences pointer values.
func dbDeref(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return v
		}
		v = v.Elem()
	}
	return v
}
