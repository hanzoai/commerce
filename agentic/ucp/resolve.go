// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"encoding/json"
	"slices"
	"sort"
)

// handlers returns the payment handlers a checkout response advertises: the
// business's, with each handler's available_instruments resolved against what
// the platform declares for the same handler (payment guide, "Resolving
// available_instruments"). A handler whose instruments the platform cannot use
// at all is left out. The response is authoritative for the checkout.
func (s *Server) handlers(platform *Profile) map[string][]Handler {
	biz := s.cfg.Profile.UCP.PaymentHandlers
	out := make(map[string][]Handler, len(biz))
	for name, hs := range biz {
		var theirs []AvailableInstrument
		declared := false
		for _, ph := range platform.UCP.PaymentHandlers[name] {
			if ph.AvailableInstruments != nil {
				declared = true
				theirs = append(theirs, ph.AvailableInstruments...)
			}
		}
		for _, h := range hs {
			if declared && h.AvailableInstruments != nil {
				h.AvailableInstruments = resolve(h.AvailableInstruments, theirs)
				if len(h.AvailableInstruments) == 0 {
					continue
				}
			}
			out[name] = append(out[name], h)
		}
	}
	return out
}

// resolve intersects the instruments a business accepts with the ones a
// platform supports: an instrument type must be on both sides, and a property
// whose constraint is an enum or a const on both sides is narrowed to the
// values they share, dropping the instrument when none is shared. Any other
// constraint is the business's own and is kept as it declared it. The order of
// the business's array, its preferred presentation, is kept.
func resolve(biz, plat []AvailableInstrument) []AvailableInstrument {
	var out []AvailableInstrument
	for _, b := range biz {
		var candidates []AvailableInstrument
		for _, p := range plat {
			if p.Type == b.Type {
				candidates = append(candidates, p)
			}
		}
		if len(candidates) == 0 {
			continue
		}
		merged := b
		kept := false
		for _, p := range candidates {
			c, ok := narrow(b.Constraints, p.Constraints)
			if ok {
				merged.Constraints = c
				kept = true
				break
			}
		}
		if kept {
			out = append(out, merged)
		}
	}
	return out
}

// valueSet reads an enum or const constraint of one property.
func valueSet(raw json.RawMessage) ([]json.RawMessage, bool) {
	var v struct {
		Enum  []json.RawMessage `json:"enum"`
		Const json.RawMessage   `json:"const"`
	}
	if json.Unmarshal(raw, &v) != nil {
		return nil, false
	}
	switch {
	case len(v.Enum) > 0:
		return v.Enum, true
	case len(v.Const) > 0:
		return []json.RawMessage{v.Const}, true
	}
	return nil, false
}

// narrow intersects two constraint expressions of the form
// {"properties": {name: {"enum": [...]}}}; it reports false when a property has
// no value in common.
func narrow(biz, plat json.RawMessage) (json.RawMessage, bool) {
	if len(plat) == 0 {
		return biz, true
	}
	if len(biz) == 0 {
		return plat, true
	}
	var b, p struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if json.Unmarshal(biz, &b) != nil || json.Unmarshal(plat, &p) != nil || len(b.Properties) == 0 || len(p.Properties) == 0 {
		return biz, true
	}
	var all map[string]json.RawMessage
	if json.Unmarshal(biz, &all) != nil {
		return biz, true
	}
	props := make(map[string]json.RawMessage, len(b.Properties))
	for k, v := range b.Properties {
		props[k] = v
	}
	names := make([]string, 0, len(p.Properties))
	for k := range p.Properties {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		pv := p.Properties[k]
		bv, shared := b.Properties[k]
		if !shared {
			props[k] = pv
			continue
		}
		bs, bok := valueSet(bv)
		ps, pok := valueSet(pv)
		if !bok || !pok {
			continue
		}
		var both []json.RawMessage
		for _, x := range bs {
			if slices.ContainsFunc(ps, func(y json.RawMessage) bool { return sameJSON(x, y) }) {
				both = append(both, x)
			}
		}
		if len(both) == 0 {
			return nil, false
		}
		enum, _ := json.Marshal(map[string]any{"enum": both})
		props[k] = enum
	}
	all["properties"], _ = json.Marshal(props)
	out, err := json.Marshal(all)
	if err != nil {
		return biz, true
	}
	return out, true
}

func sameJSON(a, b json.RawMessage) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	ax, _ := json.Marshal(x)
	by, _ := json.Marshal(y)
	return string(ax) == string(by)
}
