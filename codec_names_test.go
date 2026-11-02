// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package commerce

import (
	"reflect"
	"sort"
	"strings"
	"testing"
	"unicode"

	"github.com/hanzoai/orm"
)

// A Postgres row is stored under two sets of names (db.encodeRow): the codec's —
// each field's Go name, first letter lowered — and, beside them, each field's json
// tag, which an image from before the codec reads. Where one field's tag is
// another field's name, the two sets name one key for two values, and the row
// cannot carry both. No registered model may be so, at any depth.
func TestNoModelNamesOneKeyForTwoFields(t *testing.T) {
	kinds := orm.Kinds()
	if len(kinds) < 50 {
		t.Fatalf("only %d models are registered: this package no longer links them all", len(kinds))
	}
	var clashes []string
	seen := map[reflect.Type]bool{}
	var walk func(path string, typ reflect.Type)
	walk = func(path string, typ reflect.Type) {
		for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array || typ.Kind() == reflect.Map {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct || seen[typ] || typ.PkgPath() == "time" {
			return
		}
		seen[typ] = true
		byName, byTag := map[string]string{}, map[string]string{}
		var fields func(typ reflect.Type)
		fields = func(typ reflect.Type) {
			for i := 0; i < typ.NumField(); i++ {
				f := typ.Field(i)
				if !f.IsExported() {
					continue
				}
				if f.Anonymous && f.Tag.Get("json") == "" {
					ft := f.Type
					if ft.Kind() == reflect.Pointer {
						ft = ft.Elem()
					}
					if ft.Kind() == reflect.Struct {
						fields(ft)
						continue
					}
				}
				if f.Tag.Get("datastore") != "-" {
					byName[lowered(f.Name)] = f.Name
				}
				if tag, _, _ := strings.Cut(f.Tag.Get("json"), ","); tag != "-" {
					if tag == "" {
						tag = f.Name
					}
					byTag[tag] = f.Name
				}
				walk(path+"."+f.Name, f.Type)
			}
		}
		fields(typ)
		for key, tagged := range byTag {
			if named, ok := byName[key]; ok && named != tagged && !oneFact[typ.Name()+"."+named+"="+tagged] {
				clashes = append(clashes, path+": key "+key+" is "+named+"'s name and "+tagged+"'s tag")
			}
		}
	}
	for _, k := range kinds {
		if m, ok := orm.Lookup(k); ok && m.Type != nil {
			walk(k, m.Type)
		}
	}
	sort.Strings(clashes)
	for _, c := range clashes {
		t.Error(c)
	}
}

// oneFact are the pairs that share a key and are one fact stated twice, so the key
// holds the right value for both: an analytics integration's Disabled is stored,
// and Disabled_ is its display copy (UpdateShownDisabledStatus), tagged "disabled".
var oneFact = map[string]bool{
	"Integration.Disabled=Disabled_": true,
}

func lowered(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	r[0] = unicode.ToLower(r[0])
	return string(r)
}
