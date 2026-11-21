// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package mpp

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Range is a payment-range of Accept-Payment (section 7.4): a method and an intent, either
// "*", and a weight in thousandths (1000 is q=1, 0 is "do not use").
type Range struct {
	Method string
	Intent string
	Weight int
}

// String is the range as written in the header.
func (r Range) String() string {
	s := r.Method + "/" + r.Intent
	if r.Weight == 1000 {
		return s
	}
	q := strconv.FormatFloat(float64(r.Weight)/1000, 'f', 3, 64)
	q = strings.TrimRight(strings.TrimRight(q, "0"), ".")
	return s + ";q=" + q
}

// specific says how many of method and intent the range names exactly; ok is false when it
// does not match.
func (r Range) specific(method, intent string) (n int, ok bool) {
	if r.Method != "*" {
		if r.Method != method {
			return 0, false
		}
		n++
	}
	if r.Intent != "*" {
		if r.Intent != intent {
			return 0, false
		}
		n++
	}
	return n, true
}

// parseAccept reads Accept-Payment values; a malformed one is an error, which a server may
// ignore (section 7.4).
func parseAccept(values []string) ([]Range, error) {
	var out []Range
	for _, v := range values {
		for _, item := range strings.Split(v, ",") {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			r, err := parseRange(item)
			if err != nil {
				return nil, err
			}
			out = append(out, r)
		}
	}
	return out, nil
}

func parseRange(item string) (Range, error) {
	tok, param, hasWeight := strings.Cut(item, ";")
	method, intent, ok := strings.Cut(strings.TrimSpace(tok), "/")
	if !ok || (method != "*" && !methodShape.MatchString(method)) || (intent != "*" && !intentShape.MatchString(intent)) {
		return Range{}, fmt.Errorf("accept-payment: %q is not method/intent", tok)
	}
	r := Range{Method: method, Intent: intent, Weight: 1000}
	if !hasWeight {
		return r, nil
	}
	name, q, ok := strings.Cut(strings.TrimSpace(param), "=")
	if !ok || !strings.EqualFold(strings.TrimSpace(name), "q") {
		return Range{}, fmt.Errorf("accept-payment: %q is not a weight", param)
	}
	w, err := weight(strings.TrimSpace(q))
	if err != nil {
		return Range{}, err
	}
	r.Weight = w
	return r, nil
}

// weight reads an RFC 9110 qvalue in thousandths.
func weight(q string) (int, error) {
	whole, frac, _ := strings.Cut(q, ".")
	if (whole != "0" && whole != "1") || len(frac) > 3 {
		return 0, fmt.Errorf("accept-payment: %q is not a qvalue", q)
	}
	n := 0
	for _, c := range frac {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("accept-payment: %q is not a qvalue", q)
		}
	}
	for i := 0; i < 3; i++ {
		n *= 10
		if i < len(frac) {
			n += int(frac[i] - '0')
		}
	}
	if whole == "1" {
		if n != 0 {
			return 0, fmt.Errorf("accept-payment: %q is not a qvalue", q)
		}
		return 1000, nil
	}
	return n, nil
}

// errNoMatch says no offer matched a range with a weight above zero.
var errNoMatch = errors.New("accept-payment: no offer matches")

// rank orders offers (method, intent) by ranges (section 7.4): each is judged by the most
// specific range matching it, the first listed among equals, and dropped at weight zero;
// the rest keep their order within equal weights. It returns indexes into offers, or
// errNoMatch when none survives.
func rank(offers [][2]string, ranges []Range) ([]int, error) {
	type scored struct{ index, weight int }
	var kept []scored
	for i, o := range offers {
		best, found := -1, Range{}
		for _, r := range ranges {
			if n, ok := r.specific(o[0], o[1]); ok && n > best {
				best, found = n, r
			}
		}
		if best >= 0 && found.Weight > 0 {
			kept = append(kept, scored{i, found.Weight})
		}
	}
	if len(kept) == 0 {
		return nil, errNoMatch
	}
	sort.SliceStable(kept, func(a, b int) bool { return kept[a].weight > kept[b].weight })
	out := make([]int, len(kept))
	for i, k := range kept {
		out[i] = k.index
	}
	return out, nil
}
