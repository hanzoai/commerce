// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ap2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// The `vct` of each mandate type. A verifier matches the whole string, version suffix
// included (specification.md "Mandate Versioning").
const (
	VCTCheckout     = "mandate.checkout.1"
	VCTCheckoutOpen = "mandate.checkout.open.1"
	VCTPayment      = "mandate.payment.1"
	VCTPaymentOpen  = "mandate.payment.open.1"
)

// Merchant is types/merchant.json.
type Merchant struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Website string `json:"website,omitempty"`
}

// Amount is types/amount.json: an amount in minor units of an ISO 4217 currency.
type Amount struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

// Instrument is types/payment_instrument.json.
type Instrument struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

// Pisp is types/pisp.json, a Payment Initiation Service Provider.
type Pisp struct {
	LegalName  string `json:"legal_name"`
	BrandName  string `json:"brand_name"`
	DomainName string `json:"domain_name"`
}

// CheckoutMandate is the closed Checkout Mandate (checkout_mandate.json).
type CheckoutMandate struct {
	VCT          string `json:"vct"`
	CheckoutJWT  string `json:"checkout_jwt"`
	CheckoutHash string `json:"checkout_hash"`
	IAT          *int64 `json:"iat,omitempty"`
	EXP          *int64 `json:"exp,omitempty"`
}

// PaymentMandate is the closed Payment Mandate (payment_mandate.json).
type PaymentMandate struct {
	VCT           string         `json:"vct"`
	TransactionID string         `json:"transaction_id"`
	Payee         Merchant       `json:"payee"`
	Pisp          *Pisp          `json:"pisp,omitempty"`
	PaymentAmount Amount         `json:"payment_amount"`
	Instrument    Instrument     `json:"payment_instrument"`
	ExecutionDate string         `json:"execution_date,omitempty"`
	RiskData      map[string]any `json:"risk_data,omitempty"`
	IAT           *int64         `json:"iat,omitempty"`
	EXP           *int64         `json:"exp,omitempty"`
}

// OpenCheckoutMandate is the open Checkout Mandate (open_checkout_mandate.json).
type OpenCheckoutMandate struct {
	VCT         string      `json:"vct"`
	Constraints Constraints `json:"constraints"`
	CNF         CNF         `json:"cnf"`
	IAT         *int64      `json:"iat,omitempty"`
	EXP         *int64      `json:"exp,omitempty"`
}

// OpenPaymentMandate is the open Payment Mandate (open_payment_mandate.json). It may carry
// any property of the closed mandate, pre-set.
type OpenPaymentMandate struct {
	VCT           string         `json:"vct"`
	Constraints   Constraints    `json:"constraints"`
	CNF           CNF            `json:"cnf"`
	Payee         *Merchant      `json:"payee,omitempty"`
	PaymentAmount *Amount        `json:"payment_amount,omitempty"`
	Instrument    *Instrument    `json:"payment_instrument,omitempty"`
	Pisp          *Pisp          `json:"pisp,omitempty"`
	ExecutionDate string         `json:"execution_date,omitempty"`
	RiskData      map[string]any `json:"risk_data,omitempty"`
	IAT           *int64         `json:"iat,omitempty"`
	EXP           *int64         `json:"exp,omitempty"`
}

// CNF is the confirmation claim of RFC 7800: the key the closed mandate is signed with.
type CNF struct {
	JWK JWK `json:"jwk"`
}

// Constraints is the `constraints` of an open mandate.
type Constraints []Constraint

// UnmarshalJSON reads each constraint as decodeConstraint does.
func (c *Constraints) UnmarshalJSON(b []byte) error {
	v, err := parse(b)
	if err != nil {
		return err
	}
	list, ok := v.([]any)
	if !ok {
		return fmt.Errorf("constraints is not an array")
	}
	*c = make(Constraints, len(list))
	for i, e := range list {
		(*c)[i] = decodeConstraint(e)
	}
	return nil
}

// A Constraint limits what the closed mandate may contain. The concrete types are the
// constraints AP2 v0.2 defines; Unknown is any other, which never passes.
type Constraint interface{ kind() string }

// AllowedMerchants is `checkout.allowed_merchants`.
type AllowedMerchants struct {
	Type    string     `json:"type"`
	Allowed []Merchant `json:"allowed"`
}

// LineItems is `checkout.line_items`.
type LineItems struct {
	Type  string            `json:"type"`
	Items []LineRequirement `json:"items"`
}

// LineRequirement is one entry of `checkout.line_items`: Quantity units of any of the
// acceptable items.
type LineRequirement struct {
	ID              string `json:"id"`
	AcceptableItems []Item `json:"acceptable_items"`
	Quantity        int64  `json:"quantity"`
}

// Item is an item a line requirement accepts (open_checkout_mandate.json, item).
type Item struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// AllowedPayees is `payment.allowed_payees`.
type AllowedPayees struct {
	Type    string     `json:"type"`
	Allowed []Merchant `json:"allowed"`
}

// AllowedInstruments is `payment.allowed_payment_instruments`.
type AllowedInstruments struct {
	Type    string       `json:"type"`
	Allowed []Instrument `json:"allowed"`
}

// AllowedPisps is `payment.allowed_pisps`.
type AllowedPisps struct {
	Type    string `json:"type"`
	Allowed []Pisp `json:"allowed"`
}

// AmountRange is `payment.amount_range`; Max and Min are minor units.
type AmountRange struct {
	Type     string `json:"type"`
	Currency string `json:"currency"`
	Max      int64  `json:"max"`
	Min      *int64 `json:"min,omitempty"`
}

// Budget is `payment.budget`.
type Budget struct {
	Type     string      `json:"type"`
	Max      json.Number `json:"max"`
	Currency string      `json:"currency"`
}

// Recurrence is `payment.agent_recurrence`.
type Recurrence struct {
	Type           string `json:"type"`
	Frequency      string `json:"frequency"`
	MaxOccurrences *int64 `json:"max_occurrences,omitempty"`
}

// Reference is `payment.reference`: the hash of the open Checkout Mandate this payment is
// for.
type Reference struct {
	Type                     string `json:"type"`
	ConditionalTransactionID string `json:"conditional_transaction_id"`
}

// ExecutionDate is `payment.execution_date`.
type ExecutionDate struct {
	Type      string `json:"type"`
	NotBefore string `json:"not_before,omitempty"`
	NotAfter  string `json:"not_after,omitempty"`
}

// Unknown is a constraint this verifier cannot evaluate: a type it does not know, or a known
// type with a member it does not know. It always fails (agent_authorization.md).
type Unknown struct {
	Type string
	Why  string
	Raw  any
}

// MarshalJSON writes the constraint as it was read.
func (u Unknown) MarshalJSON() ([]byte, error) { return json.Marshal(u.Raw) }

func (AllowedMerchants) kind() string   { return "checkout.allowed_merchants" }
func (LineItems) kind() string          { return "checkout.line_items" }
func (AllowedPayees) kind() string      { return "payment.allowed_payees" }
func (AllowedInstruments) kind() string { return "payment.allowed_payment_instruments" }
func (AllowedPisps) kind() string       { return "payment.allowed_pisps" }
func (AmountRange) kind() string        { return "payment.amount_range" }
func (Budget) kind() string             { return "payment.budget" }
func (Recurrence) kind() string         { return "payment.agent_recurrence" }
func (Reference) kind() string          { return "payment.reference" }
func (ExecutionDate) kind() string      { return "payment.execution_date" }
func (u Unknown) kind() string          { return u.Type }

// members lists the members of each constraint, the whole of its schema. A member beyond
// them would change what the constraint means, so the constraint is not evaluated.
var members = map[string][]string{
	"checkout.allowed_merchants":          {"type", "allowed"},
	"checkout.line_items":                 {"type", "items"},
	"payment.allowed_payees":              {"type", "allowed"},
	"payment.allowed_payment_instruments": {"type", "allowed"},
	"payment.allowed_pisps":               {"type", "allowed"},
	"payment.amount_range":                {"type", "currency", "max", "min"},
	"payment.budget":                      {"type", "max", "currency"},
	"payment.agent_recurrence":            {"type", "frequency", "max_occurrences"},
	"payment.reference":                   {"type", "conditional_transaction_id"},
	"payment.execution_date":              {"type", "not_before", "not_after"},
}

func newConstraint(typ string) Constraint {
	switch typ {
	case "checkout.allowed_merchants":
		return &AllowedMerchants{}
	case "checkout.line_items":
		return &LineItems{}
	case "payment.allowed_payees":
		return &AllowedPayees{}
	case "payment.allowed_payment_instruments":
		return &AllowedInstruments{}
	case "payment.allowed_pisps":
		return &AllowedPisps{}
	case "payment.amount_range":
		return &AmountRange{}
	case "payment.budget":
		return &Budget{}
	case "payment.agent_recurrence":
		return &Recurrence{}
	case "payment.reference":
		return &Reference{}
	case "payment.execution_date":
		return &ExecutionDate{}
	}
	return nil
}

// decodeConstraint reads one constraint of an open mandate. It never fails: what it cannot
// read is an Unknown, which fails when the constraints are evaluated.
func decodeConstraint(v any) Constraint {
	m, ok := v.(map[string]any)
	if !ok {
		return Unknown{Why: "not an object", Raw: v}
	}
	typ, _ := m["type"].(string)
	c := newConstraint(typ)
	if c == nil {
		return Unknown{Type: typ, Why: "a constraint this verifier does not know", Raw: m}
	}
	for k := range m {
		known := false
		for _, name := range members[typ] {
			known = known || name == k
		}
		if !known {
			return Unknown{Type: typ, Why: fmt.Sprintf("member %q is not in the constraint's schema", k), Raw: m}
		}
	}
	raw, err := json.Marshal(m)
	if err == nil {
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		err = dec.Decode(c)
	}
	if err != nil {
		return Unknown{Type: typ, Why: err.Error(), Raw: m}
	}
	return deref(c)
}

func deref(c Constraint) Constraint {
	switch t := c.(type) {
	case *AllowedMerchants:
		return *t
	case *LineItems:
		return *t
	case *AllowedPayees:
		return *t
	case *AllowedInstruments:
		return *t
	case *AllowedPisps:
		return *t
	case *AmountRange:
		return *t
	case *Budget:
		return *t
	case *Recurrence:
		return *t
	case *Reference:
		return *t
	case *ExecutionDate:
		return *t
	}
	return c
}

// claims decodes a resolved delegate payload into a mandate struct. The mandates allow
// claims beyond their schema (any SD-JWT VC claim may be used), so unknown members are
// ignored.
func claims(m map[string]any, into any) error {
	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, into)
}

// closedCheckout reads the closed Checkout Mandate of a delegate payload.
func closedCheckout(m map[string]any) (*CheckoutMandate, *Error) {
	var c CheckoutMandate
	if err := claims(m, &c); err != nil {
		return nil, mandate(ErrMandate, "checkout mandate: %v", err)
	}
	if c.VCT != VCTCheckout {
		return nil, mandate(ErrType, "vct %q is not %q", c.VCT, VCTCheckout)
	}
	if c.CheckoutJWT == "" || c.CheckoutHash == "" {
		return nil, mandate(ErrMandate, "checkout mandate lacks checkout_jwt or checkout_hash")
	}
	return &c, nil
}

// closedPayment reads the closed Payment Mandate of a delegate payload.
func closedPayment(m map[string]any) (*PaymentMandate, *Error) {
	var p PaymentMandate
	if err := claims(m, &p); err != nil {
		return nil, mandate(ErrMandate, "payment mandate: %v", err)
	}
	switch {
	case p.VCT != VCTPayment:
		return nil, mandate(ErrType, "vct %q is not %q", p.VCT, VCTPayment)
	case p.TransactionID == "":
		return nil, mandate(ErrMandate, "payment mandate lacks transaction_id")
	case p.Payee.ID == "" || p.Payee.Name == "":
		return nil, mandate(ErrMandate, "payment mandate payee needs id and name")
	case p.PaymentAmount.Currency == "":
		return nil, mandate(ErrMandate, "payment mandate payment_amount needs amount and currency")
	case p.Instrument.ID == "" || p.Instrument.Type == "":
		return nil, mandate(ErrMandate, "payment mandate payment_instrument needs id and type")
	}
	if pa, _ := m["payment_amount"].(map[string]any); pa["amount"] == nil {
		return nil, mandate(ErrMandate, "payment mandate payment_amount needs amount and currency")
	}
	return &p, nil
}

// openMandate checks the members both open mandates must carry: the type, and a key to bind
// the closed mandate to.
func openMandate(m map[string]any, vct string) *Error {
	if got, _ := m["vct"].(string); got != vct {
		return mandate(ErrType, "vct %q is not %q", got, vct)
	}
	cnf, ok := m["cnf"].(map[string]any)
	if !ok {
		return credential(ErrChain, "an open mandate needs cnf")
	}
	jwk, err := decodeJWK(cnf["jwk"])
	if err == nil {
		_, err = jwk.Key()
	}
	if err != nil {
		return credential(ErrKey, "cnf: %v", err)
	}
	if _, ok := m["constraints"].([]any); !ok {
		return mandate(ErrMandate, "an open mandate needs constraints")
	}
	return nil
}

// openCheckout reads an open Checkout Mandate. Its constraints must include
// `checkout.line_items`.
func openCheckout(m map[string]any) (*OpenCheckoutMandate, *Error) {
	if err := openMandate(m, VCTCheckoutOpen); err != nil {
		return nil, err
	}
	var o OpenCheckoutMandate
	if err := claims(m, &o); err != nil {
		return nil, mandate(ErrMandate, "open checkout mandate: %v", err)
	}
	for _, c := range o.Constraints {
		if c.kind() == "checkout.line_items" {
			return &o, nil
		}
	}
	return nil, mandate(ErrMandate, "an open checkout mandate holds a checkout.line_items constraint")
}

// openPayment reads an open Payment Mandate. Its constraints must include `payment.reference`.
func openPayment(m map[string]any) (*OpenPaymentMandate, *Error) {
	if err := openMandate(m, VCTPaymentOpen); err != nil {
		return nil, err
	}
	var o OpenPaymentMandate
	if err := claims(m, &o); err != nil {
		return nil, mandate(ErrMandate, "open payment mandate: %v", err)
	}
	for _, c := range o.Constraints {
		if c.kind() == "payment.reference" {
			return &o, nil
		}
	}
	return nil, mandate(ErrMandate, "an open payment mandate holds a payment.reference constraint")
}

// sameMerchant says two merchants are the same: by id when both have one, otherwise by name
// and website, both present. (The spec says a merchant must be "present" in a list without
// saying how two are compared.)
func sameMerchant(a, b Merchant) bool {
	if a.ID != "" && b.ID != "" {
		return a.ID == b.ID
	}
	return a.Name != "" && a.Website != "" && a.Name == b.Name && a.Website == b.Website
}

// presetClaims requires each member the open mandate fixes to reach the closed mandate
// unchanged (agent_authorization.md "Verification and Processing Rules", step 2).
func presetClaims(open, closed map[string]any, names ...string) *Error {
	for _, n := range names {
		if v, ok := open[n]; ok && !equal(v, closed[n]) {
			return unresolved("the open mandate fixes %s and the closed mandate changes it", n)
		}
	}
	return nil
}

func upper(s string) string { return strings.ToUpper(s) }
