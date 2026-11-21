// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package mpp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Offer is a payment alternative of an operation in x-payment-info
// (draft-payment-discovery-01 section 4.4.1). Amount is nil when the price depends on the
// request. The 402 challenge is authoritative (section 5).
type Offer struct {
	Intent      string  `json:"intent"`
	Method      string  `json:"method"`
	Amount      *string `json:"amount"`
	Currency    string  `json:"currency,omitempty"`
	Description string  `json:"description,omitempty"`
}

// Info is the value of x-payment-info: the ways to pay for one operation.
type Info struct {
	Offers []Offer
}

// Valid checks the JSON Schema of appendix D.
func (i Info) Valid() error {
	if len(i.Offers) == 0 {
		return errors.New("x-payment-info has no offers")
	}
	for n, o := range i.Offers {
		if o.Intent != Charge && o.Intent != "session" {
			return fmt.Errorf("offer %d: intent %q is not charge or session", n, o.Intent)
		}
		if !methodShape.MatchString(o.Method) {
			return fmt.Errorf("offer %d: method %q is not lowercase ASCII letters", n, o.Method)
		}
		if o.Amount != nil && !isAmount(*o.Amount) {
			return fmt.Errorf("offer %d: amount %q is not a non-negative integer in base units", n, *o.Amount)
		}
	}
	return nil
}

// MarshalJSON writes the multi-offer form (section 4.4).
func (i Info) MarshalJSON() ([]byte, error) {
	if err := i.Valid(); err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		Offers []Offer `json:"offers"`
	}{i.Offers})
}

// UnmarshalJSON reads the multi-offer form or the single-offer shorthand (section 4.4),
// refusing members the schema does not name and missing required ones.
func (i *Info) UnmarshalJSON(b []byte) error {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(b, &top); err != nil {
		return fmt.Errorf("x-payment-info is not an object: %w", err)
	}
	var out Info
	if list, ok := top["offers"]; ok {
		if len(top) != 1 {
			return errors.New("x-payment-info with offers has other members")
		}
		var items []json.RawMessage
		if err := json.Unmarshal(list, &items); err != nil {
			return fmt.Errorf("x-payment-info offers is not an array: %w", err)
		}
		for _, it := range items {
			o, err := offer(it)
			if err != nil {
				return err
			}
			out.Offers = append(out.Offers, o)
		}
	} else {
		o, err := offer(b)
		if err != nil {
			return err
		}
		out.Offers = []Offer{o}
	}
	if err := out.Valid(); err != nil {
		return err
	}
	*i = out
	return nil
}

// offer reads an offer object (appendix D).
func offer(b []byte) (Offer, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return Offer{}, fmt.Errorf("offer is not an object: %w", err)
	}
	for _, k := range []string{"intent", "method", "amount"} {
		if _, ok := m[k]; !ok {
			return Offer{}, fmt.Errorf("offer has no %s", k)
		}
	}
	var o Offer
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&o); err != nil {
		return Offer{}, fmt.Errorf("offer: %w", err)
	}
	return o, nil
}

// Info is the x-payment-info of a request: an offer per method that takes the host's
// price for it. It is nil when the request pays nothing or no method offers.
func (g *Gate) Info(r *http.Request) (*Info, error) {
	caller, err := g.c.Caller(r)
	if err != nil {
		return nil, err
	}
	q, err := g.c.Price(r)
	if err != nil || q == nil {
		return nil, err
	}
	if err := q.Valid(); err != nil {
		return nil, err
	}
	props, err := g.proposals(r.Context(), caller, *q)
	if err != nil || len(props) == 0 {
		return nil, err
	}
	var out Info
	for _, p := range props {
		amount := q.Amount
		out.Offers = append(out.Offers, Offer{Intent: Charge, Method: p.method.Name(), Amount: &amount, Currency: q.Currency, Description: q.Description})
	}
	return &out, nil
}
