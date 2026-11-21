// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package mpp

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// Appendix B.2 of the draft: each Accept-Payment request and the order of challenges the
// draft shows back. The server's own order of preference is not given by the draft; it is
// the order in which it lists what it "can offer", and ties keep it.
func TestAcceptPaymentExamples(t *testing.T) {
	offered := regexp.MustCompile(`method="(\w+)", intent="(\w+)"`)
	servers := [][][2]string{
		{{"tempo", "charge"}, {"tempo", "session"}, {"solana", "charge"}, {"stripe", "charge"}},
		{{"solana", "charge"}, {"tempo", "charge"}, {"tempo", "session"}, {"stripe", "charge"}},
		{{"tempo", "session"}, {"tempo", "charge"}, {"solana", "charge"}, {"stripe", "charge"}},
		{{"tempo", "charge"}, {"tempo", "session"}, {"solana", "charge"}, {"stripe", "charge"}},
	}
	blocks := fences(t, "core.md")
	n := 0
	for i, b := range blocks {
		if b.lang != "http" || !strings.HasPrefix(b.text, "GET ") || !strings.Contains(b.text, "Accept-Payment:") {
			continue
		}
		req := readRequest(t, b.text)
		ranges, err := parseAccept(req.Header.Values("Accept-Payment"))
		if err != nil {
			t.Fatalf("line %d: %v", b.line, err)
		}
		var want [][2]string
		for _, m := range offered.FindAllStringSubmatch(blocks[i+1].text, -1) {
			want = append(want, [2]string{m[1], m[2]})
		}
		offers := servers[n]
		order, err := rank(offers, ranges)
		if err != nil {
			t.Fatalf("line %d: %v", b.line, err)
		}
		var got [][2]string
		for _, o := range order {
			got = append(got, offers[o])
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("line %d: Accept-Payment %q\n got  %v\n want %v", b.line, req.Header.Get("Accept-Payment"), got, want)
		}
		n++
	}
	if n != len(servers) {
		t.Fatalf("found %d Accept-Payment examples, want %d", n, len(servers))
	}
}

func TestAcceptRanges(t *testing.T) {
	got, err := parseAccept([]string{"tempo/*, solana/*;q=0.6", "*/session;q=0.3, stripe/charge ; q=1.000"})
	want := []Range{{"tempo", "*", 1000}, {"solana", "*", 600}, {"*", "session", 300}, {"stripe", "charge", 1000}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("%v %v", got, err)
	}
	for _, bad := range []string{"tempo", "tempo/", "/charge", "Tempo/charge", "tempo/charge;q=1.5", "tempo/charge;q=0.5555", "tempo/charge;q=-1", "tempo/charge;x=1", "tempo/ch arge", "tempo/charge;q=a"} {
		if _, err := parseAccept([]string{bad}); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	for _, r := range want {
		back, err := parseAccept([]string{r.String()})
		if err != nil || len(back) != 1 || back[0] != r {
			t.Errorf("%v does not round-trip: %v %v", r, back, err)
		}
	}
	if (Range{"a", "b", 250}).String() != "a/b;q=0.25" || (Range{"a", "b", 0}).String() != "a/b;q=0" {
		t.Error("String")
	}
}

// The most specific range decides an offer, the first listed among equals.
func TestRankSpecificity(t *testing.T) {
	offers := [][2]string{{"tempo", "charge"}, {"tempo", "session"}, {"stripe", "charge"}}
	ranges := []Range{{"*", "*", 100}, {"tempo", "*", 500}, {"tempo", "charge", 900}, {"*", "charge", 300}}
	order, err := rank(offers, ranges)
	if err != nil || !reflect.DeepEqual(order, []int{0, 1, 2}) {
		t.Fatalf("%v %v", order, err)
	}
	// tempo/charge matches tempo/* and */charge alike, one wildcard each: the first listed decides.
	order, _ = rank(offers, []Range{{"tempo", "*", 200}, {"*", "charge", 800}})
	if !reflect.DeepEqual(order, []int{2, 0, 1}) {
		t.Fatalf("%v", order)
	}
	if _, err := rank(offers, []Range{{"bitcoin", "*", 1000}}); err != errNoMatch {
		t.Errorf("no match: %v", err)
	}
	if _, err := rank(offers, []Range{{"*", "*", 0}}); err != errNoMatch {
		t.Errorf("everything at q=0: %v", err)
	}
}
