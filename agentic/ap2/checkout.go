// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ap2

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// AP2 leaves the content of the merchant-signed checkout to the commerce protocol: with UCP it is
// the Checkout object, as the AP2 schemas carry it (schemas/ucp/types), with `merchant` as AP2's
// extension. Only the members this package reads and writes are here.

// Checkout is the object the merchant signs into `checkout_jwt`.
type Checkout struct {
	ID        string     `json:"id"`
	Merchant  *Merchant  `json:"merchant,omitempty"`
	LineItems []LineItem `json:"line_items"`
	Status    string     `json:"status"`
	Currency  string     `json:"currency"`
	Totals    []Total    `json:"totals"`
	Links     []Link     `json:"links"`
	ExpiresAt string     `json:"expires_at,omitempty"`
}

// LineItem is one line of a checkout.
type LineItem struct {
	ID       string  `json:"id"`
	Item     Product `json:"item"`
	Quantity int64   `json:"quantity"`
	Totals   []Total `json:"totals"`
}

// Product is types/item.json: the product a line sells, priced per unit in minor units.
type Product struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Price int64  `json:"price"`
}

// Total is one entry of a cost breakdown, in signed minor units.
type Total struct {
	Type   string `json:"type"`
	Amount int64  `json:"amount"`
}

// Link is a typed URL reference.
type Link struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// ReadyForComplete is the status of a checkout that can be completed.
const ReadyForComplete = "ready_for_complete"

// Total is the entry of the given type, which a checkout holds exactly once.
func (c Checkout) Total(typ string) (int64, bool) {
	var n int64
	found := 0
	for _, t := range c.Totals {
		if t.Type == typ {
			n, found = t.Amount, found+1
		}
	}
	return n, found == 1
}

// decodeCheckout reads and validates a checkout payload against types/checkout.json.
func decodeCheckout(payload []byte) (*Checkout, error) {
	raw, err := object(payload)
	if err != nil {
		return nil, err
	}
	for _, need := range []string{"id", "line_items", "status", "currency", "totals", "links"} {
		if raw[need] == nil {
			return nil, fmt.Errorf("lacks %s", need)
		}
	}
	var c Checkout
	if err := json.Unmarshal(payload, &c); err != nil {
		return nil, err
	}
	switch c.Status {
	case "incomplete", "requires_escalation", ReadyForComplete, "complete_in_progress", "completed", "canceled":
	default:
		return nil, fmt.Errorf("status %q", c.Status)
	}
	if len(c.Currency) != 3 || c.Currency != strings.ToUpper(c.Currency) {
		return nil, fmt.Errorf("currency %q is not an ISO 4217 alphabetic code", c.Currency)
	}
	if _, ok := c.Total("subtotal"); !ok {
		return nil, fmt.Errorf("totals hold exactly one subtotal")
	}
	if _, ok := c.Total("total"); !ok {
		return nil, fmt.Errorf("totals hold exactly one total")
	}
	for _, l := range c.LineItems {
		if l.ID == "" || l.Item.ID == "" || l.Item.Title == "" || l.Item.Price < 0 || l.Quantity < 1 || l.Totals == nil {
			return nil, fmt.Errorf("line item %q is not a line item", l.ID)
		}
	}
	return &c, nil
}

// checkoutOf reads the checkout a closed Checkout Mandate carries: the JWT must verify under the
// merchant's key and hash to the mandate's `checkout_hash` (specification.md, "Merchant"), and
// the checkout must not be past its `expires_at`.
func checkoutOf(m *CheckoutMandate, key *ecdsa.PublicKey, now time.Time) (*Checkout, *Error) {
	if !same(m.CheckoutHash, digest(m.CheckoutJWT)) {
		return nil, mandate(ErrHash, "checkout_hash is not the hash of checkout_jwt")
	}
	j, err := parseJWS(m.CheckoutJWT)
	if err == nil {
		err = j.verify(key)
	}
	if err != nil {
		return nil, mandate(ErrCheckout, "checkout_jwt: %v", err)
	}
	c, err := decodeCheckout(j.payload)
	if err != nil {
		return nil, mandate(ErrCheckout, "checkout_jwt payload: %v", err)
	}
	if c.ExpiresAt != "" {
		exp, err := time.Parse(time.RFC3339, c.ExpiresAt)
		if err != nil {
			return nil, mandate(ErrCheckout, "expires_at: %v", err)
		}
		if now.After(exp) {
			return nil, mandate(ErrExpired, "the checkout expired at %s", c.ExpiresAt)
		}
	}
	return c, nil
}
