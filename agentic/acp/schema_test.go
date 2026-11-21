// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package acp

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

// A validator for the subset of JSON Schema (draft 2020-12) that the spec's own schema uses:
// $ref into $defs, type, properties, required, additionalProperties, items, enum, const,
// oneOf, anyOf, allOf, if/then, pattern, minimum, maximum, minLength, maxLength, minItems,
// uniqueItems and the formats date-time, uri and email. It reads the schema published with
// the spec (testdata/schema.agentic_checkout.json), so the tests judge the package by the
// spec's own contract.

type schema struct{ defs map[string]any }

func loadSchema(t testing.TB) *schema {
	t.Helper()
	b, err := os.ReadFile("testdata/schema.agentic_checkout.json")
	if err != nil {
		t.Fatal(err)
	}
	var root struct {
		Defs map[string]any `json:"$defs"`
	}
	if err := json.Unmarshal(b, &root); err != nil {
		t.Fatal(err)
	}
	return &schema{defs: root.Defs}
}

// check returns what is wrong with a decoded JSON document against a definition.
func (s *schema) check(def string, doc any) []string {
	d, ok := s.defs[def]
	if !ok {
		return []string{"no definition " + def}
	}
	return s.walk(d.(map[string]any), doc, "$")
}

// valid validates raw JSON against a definition and fails the test with every problem.
func (s *schema) valid(t testing.TB, def string, raw []byte) {
	t.Helper()
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("%s: not JSON: %v", def, err)
	}
	if errs := s.check(def, doc); len(errs) > 0 {
		t.Fatalf("%s: %s\n%s", def, strings.Join(errs, "\n"), raw)
	}
}

func (s *schema) walk(sch map[string]any, v any, at string) []string {
	var errs []string
	add := func(f string, a ...any) { errs = append(errs, at+": "+fmt.Sprintf(f, a...)) }
	if ref, ok := sch["$ref"].(string); ok {
		name := strings.TrimPrefix(ref, "#/$defs/")
		d, ok := s.defs[name]
		if !ok {
			return []string{at + ": unresolved " + ref}
		}
		errs = append(errs, s.walk(d.(map[string]any), v, at)...)
	}
	if t, ok := sch["type"]; ok && !typed(t, v) {
		add("type %v, got %T", t, v)
		return errs
	}
	if e, ok := sch["enum"].([]any); ok && !member(e, v) {
		add("%v is not one of %v", v, e)
	}
	if c, ok := sch["const"]; ok && !equal(c, v) {
		add("%v is not %v", v, c)
	}
	switch x := v.(type) {
	case map[string]any:
		props, _ := sch["properties"].(map[string]any)
		if req, ok := sch["required"].([]any); ok {
			for _, r := range req {
				if _, has := x[r.(string)]; !has {
					add("missing required %s", r)
				}
			}
		}
		for k, val := range x {
			if p, ok := props[k]; ok {
				errs = append(errs, s.walk(p.(map[string]any), val, at+"."+k)...)
				continue
			}
			switch ap := sch["additionalProperties"].(type) {
			case bool:
				if !ap {
					add("unknown property %s", k)
				}
			case map[string]any:
				errs = append(errs, s.walk(ap, val, at+"."+k)...)
			}
		}
	case []any:
		if it, ok := sch["items"].(map[string]any); ok {
			for i, e := range x {
				errs = append(errs, s.walk(it, e, fmt.Sprintf("%s[%d]", at, i))...)
			}
		}
		if n, ok := sch["minItems"].(float64); ok && float64(len(x)) < n {
			add("fewer than %v items", n)
		}
		if u, _ := sch["uniqueItems"].(bool); u {
			for i := range x {
				for j := i + 1; j < len(x); j++ {
					if equal(x[i], x[j]) {
						add("items %d and %d are equal", i, j)
					}
				}
			}
		}
	case string:
		if n, ok := sch["minLength"].(float64); ok && float64(len([]rune(x))) < n {
			add("shorter than %v", n)
		}
		if n, ok := sch["maxLength"].(float64); ok && float64(len([]rune(x))) > n {
			add("longer than %v", n)
		}
		if p, ok := sch["pattern"].(string); ok && !regexp.MustCompile(p).MatchString(x) {
			add("%q does not match %s", x, p)
		}
		switch sch["format"] {
		case "date-time":
			if _, err := time.Parse(time.RFC3339, x); err != nil {
				add("%q is not a date-time", x)
			}
		case "uri":
			if u, err := url.Parse(x); err != nil || u.Scheme == "" {
				add("%q is not a URI", x)
			}
		case "email":
			if !strings.Contains(x, "@") {
				add("%q is not an email", x)
			}
		}
	case float64:
		if n, ok := sch["minimum"].(float64); ok && x < n {
			add("%v is below %v", x, n)
		}
		if n, ok := sch["maximum"].(float64); ok && x > n {
			add("%v is above %v", x, n)
		}
	}
	if all, ok := sch["allOf"].([]any); ok {
		for _, a := range all {
			errs = append(errs, s.walk(a.(map[string]any), v, at)...)
		}
	}
	if one, ok := sch["oneOf"].([]any); ok {
		n := 0
		for _, a := range one {
			if len(s.walk(a.(map[string]any), v, at)) == 0 {
				n++
			}
		}
		if n != 1 {
			add("matches %d of the oneOf alternatives", n)
		}
	}
	if any_, ok := sch["anyOf"].([]any); ok {
		n := 0
		for _, a := range any_ {
			if len(s.walk(a.(map[string]any), v, at)) == 0 {
				n++
			}
		}
		if n == 0 {
			add("matches none of the anyOf alternatives")
		}
	}
	if cond, ok := sch["if"].(map[string]any); ok && len(s.walk(cond, v, at)) == 0 {
		if then, ok := sch["then"].(map[string]any); ok {
			errs = append(errs, s.walk(then, v, at)...)
		}
	}
	return errs
}

func typed(t any, v any) bool {
	if list, ok := t.([]any); ok {
		for _, e := range list {
			if typed(e, v) {
				return true
			}
		}
		return false
	}
	switch t {
	case "object":
		_, ok := v.(map[string]any)
		return ok
	case "array":
		_, ok := v.([]any)
		return ok
	case "string":
		_, ok := v.(string)
		return ok
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "number":
		_, ok := v.(float64)
		return ok
	case "integer":
		f, ok := v.(float64)
		return ok && f == math.Trunc(f)
	case "null":
		return v == nil
	}
	return false
}

func member(list []any, v any) bool {
	for _, e := range list {
		if equal(e, v) {
			return true
		}
	}
	return false
}

func equal(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}
