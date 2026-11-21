// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package mpp

import (
	"encoding/json"
	"math"
	"testing"
)

// The RFC 8785 section 3.2.2 example: member order, string escapes and number forms.
func TestCanonicalRFC8785Example(t *testing.T) {
	in := `{"numbers":[333333333.33333329,1E30,4.50,2e-3,0.000000000000000000000000001],
	"string":"\u20ac$\u000F\u000aA'\u0042\u0022\u005c\\\"\/",
	"literals":[null,true,false]}`
	var v any
	d := json.NewDecoder(stringsReader(in))
	d.UseNumber()
	if err := d.Decode(&v); err != nil {
		t.Fatal(err)
	}
	got, err := Canonical(v)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"literals":[null,true,false],"numbers":[333333333.3333333,1e+30,4.5,0.002,1e-27],"string":"€$\u000f\nA'B\"\\\\\"/"}`
	if string(got) != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

// RFC 8785 section 3.2.3: members sort by UTF-16 code unit, so a character outside the
// basic plane sorts before U+FB33 although its code point is larger.
func TestCanonicalSortsByUTF16(t *testing.T) {
	m := map[string]int{"\u20ac": 1, "\r": 2, "\ufb33": 3, "1": 4, "\U0001F600": 5, "\u0080": 6, "\u00f6": 7}
	got, err := Canonical(m)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"\\r\":2,\"1\":4,\"\u0080\":6,\"\u00f6\":7,\"\u20ac\":1,\"\U0001F600\":5,\"\ufb33\":3}"
	if string(got) != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

// The number forms ECMAScript produces, which RFC 8785 appendix B tabulates. The expected
// strings are what JSON.stringify prints for the same doubles.
func TestCanonicalNumbers(t *testing.T) {
	for _, c := range []struct {
		f    float64
		want string
	}{
		{0, "0"}, {math.Copysign(0, -1), "0"}, {5e-324, "5e-324"}, {1.7976931348623157e308, "1.7976931348623157e+308"},
		{9007199254740992, "9007199254740992"}, {-9007199254740992, "-9007199254740992"},
		{295147905179352830000, "295147905179352830000"}, {1e21, "1e+21"}, {1e23, "1e+23"},
		{0.000001, "0.000001"}, {9.999999999999997e-7, "9.999999999999997e-7"}, {1e-7, "1e-7"},
		{333333333.33333329, "333333333.3333333"}, {4.5, "4.5"}, {-1.5e-9, "-1.5e-9"}, {123456789012345680000, "123456789012345680000"},
	} {
		got, err := number(c.f)
		if err != nil || got != c.want {
			t.Errorf("number(%v) = %q, %v; want %q", c.f, got, err, c.want)
		}
	}
	if _, err := number(math.Inf(1)); err == nil {
		t.Error("infinity must be refused")
	}
}

// A request is hashed as the text of its base64url, so html-escaping must not leak in.
func TestCanonicalDoesNotEscapeMarkup(t *testing.T) {
	got, err := Canonical(map[string]string{"description": "a<b>&c"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"description":"a<b>&c"}` {
		t.Fatalf("got %s", got)
	}
}
