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

// Range is one payment-range of an Accept-Payment header (section 7.4): a method and an
// intent, either of which may be "*", and a weight in thousandths, 1000 being q=1 and 0
// meaning "do not use".
type Range struct {
	Method string
	Intent string
	Weight int
}

// String is the range as it is written in the header.
func (r Range) String() string {
	s := r.Method + "/" + r.Intent
	if r.Weight == 1000 {
		return s
	}
	q := strconv.FormatFloat(float64(r.Weight)/1000, 'f', 3, 64)
	q = strings.TrimRight(strings.TrimRight(q, "0"), ".")
	return s + ";q=" + q
}

// specific says how exactly the range names a method and intent: 2 for both, 1 for one
// and a wildcard, 0 for two wildcards; ok is false when it does not match at all.
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

// ParseAccept reads Accept-Payment field values. A malformed header is an error, which
// a server may ignore (section 7.4).
func ParseAccept(values []string) ([]Range, error) {
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

// weight reads an RFC 9110 qvalue: 0 to 1 with at most three decimals.
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

// rank orders offers, given as method and intent, by the client's ranges (section 7.4): an
// offer is judged by the most specific range that matches it (the first listed among
// equals), dropped when that range's weight is zero, and the rest are stably ordered by
// descending weight, so equal weights keep the server's order. It returns the indexes
// into the offers, or errNoMatch when none survives, in which case the server may ignore
// the header.
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
