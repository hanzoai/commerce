// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package acp

import "testing"

// The equality of rfc.agentic_checkout.md §6.2.
func TestFingerprintFollowsTheSpecsEquality(t *testing.T) {
	same := [][2]string{
		{`{"a":1,"b":2}`, `{"b":2,"a":1}`},
		{`{"a":{"x":1,"y":[1,2]}}`, `{"a":{"y":[1,2],"x":1}}`},
		{`{"n":1}`, `{"n":1.0}`},
		{`{"n":1}`, `{"n":1.000}`},
		{`{"n":0.1}`, `{"n":1e-1}`},
		{`{"n":10}`, `{"n":1e1}`},
		{`{"n":100}`, `{"n":1.0E2}`},
		{`{"n":-2.50}`, `{"n":-2.5}`},
		{`{"n":0}`, `{"n":0.0}`},
		{"{ \"a\" : [ 1 , 2 ] }", `{"a":[1,2]}`},
		{`{"s":"A"}`, `{"s":"A"}`},
	}
	for _, p := range same {
		a, err1 := fingerprint([]byte(p[0]))
		b, err2 := fingerprint([]byte(p[1]))
		if err1 != nil || err2 != nil || a != b {
			t.Errorf("%s and %s should be equal (%v %v)", p[0], p[1], err1, err2)
		}
	}
	different := [][2]string{
		{`{"a":[1,2]}`, `{"a":[2,1]}`},
		{`{"a":null}`, `{}`},
		{`{"a":null}`, `{"a":false}`},
		{`{"a":1}`, `{"a":2}`},
		{`{"a":1}`, `{"a":"1"}`},
		{`{"a":0.1}`, `{"a":0.10001}`},
		{`{"a":-1}`, `{"a":1}`},
		{`{"a":"x"}`, `{"a":"X"}`},
		{`{"a":1}`, `{"a":1,"b":null}`},
		{`{"a":[]}`, `{"a":{}}`},
		{``, `{}`},
		{`{"n":1e999999999}`, `{"n":1e999999998}`},
	}
	for _, p := range different {
		a, err1 := fingerprint([]byte(p[0]))
		b, err2 := fingerprint([]byte(p[1]))
		if err1 != nil || err2 != nil || a == b {
			t.Errorf("%s and %s should differ (%v %v)", p[0], p[1], err1, err2)
		}
	}
	for _, bad := range []string{`{`, `nope`, `{} {}`, `{"a":1}x`} {
		if _, err := fingerprint([]byte(bad)); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}

func TestNumberNormalisation(t *testing.T) {
	for in, want := range map[string]string{
		"1": "1e0", "1.0": "1e0", "10": "1e1", "0.1": "1e-1", "0.05": "5e-2", "100": "1e2", "1e2": "1e2",
		"-12.50": "-125e-1", "0": "0", "0.000": "0", "1E+3": "1e3", "1e999999999": "1e999999999", "-1e999999999": "-1e999999999",
	} {
		if got := number(in); got != want {
			t.Errorf("number(%q) = %q; want %q", in, got, want)
		}
	}
}
