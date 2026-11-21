// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
)

// UCP schemas are open: members a type does not declare, such as those an
// extension composes onto a capability (overview, "Schema Composition"), are
// kept in Extra so a document survives a decode and an encode unchanged.

// members names the JSON members a struct type declares.
func members(t reflect.Type) map[string]bool {
	out := map[string]bool{}
	var walk func(t reflect.Type)
	walk = func(t reflect.Type) {
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag := f.Tag.Get("json")
			name, _, _ := strings.Cut(tag, ",")
			if name == "-" {
				continue
			}
			if f.Anonymous && name == "" {
				walk(f.Type)
				continue
			}
			if name == "" {
				name = f.Name
			}
			out[name] = true
		}
	}
	walk(t)
	return out
}

// extras returns the members of the JSON object data that the struct type t
// does not declare.
func extras(data []byte, t reflect.Type) (map[string]json.RawMessage, error) {
	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, err
	}
	known := members(t)
	var out map[string]json.RawMessage
	for k, v := range all {
		if !known[k] {
			if out == nil {
				out = map[string]json.RawMessage{}
			}
			out[k] = v
		}
	}
	return out, nil
}

// merge encodes v and adds the extra members to the object, never replacing a
// declared member. Members are written in sorted order.
func merge(v any, extra map[string]json.RawMessage) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil || len(extra) == 0 {
		return b, err
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal(b, &all); err != nil {
		return nil, err
	}
	for k, x := range extra {
		if _, dup := all[k]; !dup {
			all[k] = x
		}
	}
	keys := make([]string, 0, len(all))
	for k := range all {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		buf.Write(kb)
		buf.WriteByte(':')
		buf.Write(all[k])
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// decodeOpen decodes b into plain, a pointer to the declared members of an open
// type, and stores the members it does not declare in extra.
func decodeOpen(b []byte, plain any, extra *map[string]json.RawMessage) error {
	if err := json.Unmarshal(b, plain); err != nil {
		return err
	}
	ex, err := extras(b, reflect.TypeOf(plain).Elem())
	if err != nil {
		return err
	}
	*extra = ex
	return nil
}
