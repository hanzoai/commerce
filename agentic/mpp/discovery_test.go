// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package mpp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// Every x-payment-info the discovery draft shows, in its multi-offer and shorthand forms,
// decodes, satisfies the draft's schema as the package states it, and encodes to the
// multi-offer form with the same offers.
func TestSpecDiscovery(t *testing.T) {
	infos := 0
	check := func(where string, raw json.RawMessage) {
		var info Info
		if err := json.Unmarshal(raw, &info); err != nil {
			t.Fatalf("%s: %v\n%s", where, err, raw)
		}
		infos++
		out, err := json.Marshal(info)
		if err != nil {
			t.Fatal(err)
		}
		var again Info
		if err := json.Unmarshal(out, &again); err != nil || len(again.Offers) != len(info.Offers) {
			t.Fatalf("%s: re-encoded: %v", where, err)
		}
		for i := range again.Offers {
			a, b := again.Offers[i], info.Offers[i]
			if a.Intent != b.Intent || a.Method != b.Method || a.Currency != b.Currency || a.Description != b.Description || (a.Amount == nil) != (b.Amount == nil) {
				t.Fatalf("%s: offer %d changed: %+v %+v", where, i, a, b)
			}
		}
	}
	for _, b := range fences(t, "discovery.md") {
		if b.lang != "json" {
			continue
		}
		var top map[string]json.RawMessage
		if json.Unmarshal([]byte(b.text), &top) != nil {
			continue
		}
		if v, ok := top["x-payment-info"]; ok {
			check("line "+strconv.Itoa(b.line), v)
		}
		if _, ok := top["openapi"]; ok {
			var doc struct {
				Paths map[string]map[string]struct {
					Info json.RawMessage `json:"x-payment-info"`
				} `json:"paths"`
			}
			if err := json.Unmarshal([]byte(b.text), &doc); err != nil {
				t.Fatal(err)
			}
			for path, ops := range doc.Paths {
				for method, op := range ops {
					if op.Info != nil {
						check(method+" "+path, op.Info)
					}
				}
			}
		}
	}
	if infos < 6 {
		t.Fatalf("only %d x-payment-info objects found", infos)
	}
}

func TestInfoRefuses(t *testing.T) {
	for name, in := range map[string]string{
		"not an object":        `[]`,
		"empty offers":         `{"offers":[]}`,
		"offers and more":      `{"offers":[{"intent":"charge","method":"tempo","amount":"1"}],"x":1}`,
		"offers not array":     `{"offers":{}}`,
		"unknown member":       `{"intent":"charge","method":"tempo","amount":"1","price":"1"}`,
		"no amount":            `{"intent":"charge","method":"tempo"}`,
		"no intent":            `{"method":"tempo","amount":"1"}`,
		"no method":            `{"intent":"charge","amount":"1"}`,
		"unknown intent":       `{"intent":"barter","method":"tempo","amount":"1"}`,
		"upper method":         `{"intent":"charge","method":"Tempo","amount":"1"}`,
		"leading zero":         `{"intent":"charge","method":"tempo","amount":"01"}`,
		"negative":             `{"intent":"charge","method":"tempo","amount":"-1"}`,
		"fraction":             `{"intent":"charge","method":"tempo","amount":"1.5"}`,
		"number amount":        `{"intent":"charge","method":"tempo","amount":1}`,
		"offer unknown member": `{"offers":[{"intent":"charge","method":"tempo","amount":"1","x":1}]}`,
		"offer not object":     `{"offers":[1]}`,
	} {
		var i Info
		if err := json.Unmarshal([]byte(in), &i); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	for _, in := range []string{`{"intent":"charge","method":"tempo","amount":"0"}`, `{"intent":"session","method":"tempo","amount":null}`} {
		var i Info
		if err := json.Unmarshal([]byte(in), &i); err != nil {
			t.Errorf("%s: %v", in, err)
		}
	}
	if _, err := json.Marshal(Info{}); err == nil {
		t.Error("an empty info was encoded")
	}
}

// A Gate describes what it will challenge for, at the price the host names, for each
// method that takes it.
func TestGateInfo(t *testing.T) {
	r := newRig(t, func(c *Config) {
		c.Methods = []Method{&invoice{name: "tempo"}, skipper{&invoice{}}, &invoice{name: "stripe"}}
		c.Price = func(req *http.Request) (*Quote, error) {
			if req.URL.Path == "/free" {
				return nil, nil
			}
			return &Quote{Amount: "500", Currency: "usd", Description: "Search"}, nil
		}
	})
	g, _ := New(r.cfg)
	info, err := g.Info(httptest.NewRequest("GET", "https://api.example.com/search", nil))
	if err != nil || info == nil || len(info.Offers) != 2 {
		t.Fatalf("%+v %v", info, err)
	}
	b, _ := json.Marshal(info)
	sameJSON(t, b, []byte(`{"offers":[
		{"intent":"charge","method":"tempo","amount":"500","currency":"usd","description":"Search"},
		{"intent":"charge","method":"stripe","amount":"500","currency":"usd","description":"Search"}]}`))
	free, err := g.Info(httptest.NewRequest("GET", "https://api.example.com/free", nil))
	if err != nil || free != nil {
		t.Fatalf("a free operation: %+v %v", free, err)
	}
}
