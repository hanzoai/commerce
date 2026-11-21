// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ap2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
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

// A Constraint is one of the constraint types below, or Unknown.
type Constraint any

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

// Unknown is a constraint this verifier cannot evaluate: a type it does not know, or a known type
// with a member it does not know. It always fails (agent_authorization.md).
type Unknown struct {
	Type string
	Why  string
}

// constraintTypes maps each constraint AP2 v0.2 defines to its struct.
var constraintTypes = map[string]reflect.Type{
	"checkout.allowed_merchants":          reflect.TypeOf(AllowedMerchants{}),
	"checkout.line_items":                 reflect.TypeOf(LineItems{}),
	"payment.allowed_payees":              reflect.TypeOf(AllowedPayees{}),
	"payment.allowed_payment_instruments": reflect.TypeOf(AllowedInstruments{}),
	"payment.allowed_pisps":               reflect.TypeOf(AllowedPisps{}),
	"payment.amount_range":                reflect.TypeOf(AmountRange{}),
	"payment.budget":                      reflect.TypeOf(Budget{}),
	"payment.agent_recurrence":            reflect.TypeOf(Recurrence{}),
	"payment.reference":                   reflect.TypeOf(Reference{}),
	"payment.execution_date":              reflect.TypeOf(ExecutionDate{}),
}

// typeOf is the `type` of a constraint.
func typeOf(c Constraint) string { return reflect.ValueOf(c).FieldByName("Type").String() }

// has says a mandate's constraints include one of type typ, evaluable or not.
func has(cs Constraints, typ string) bool {
	for _, c := range cs {
		if typeOf(c) == typ {
			return true
		}
	}
	return false
}

// decodeConstraint reads one constraint, whose members are the whole of its schema: one beyond
// them would change what it means. What cannot be read is an Unknown, which fails when the
// constraints are evaluated.
func decodeConstraint(v any) Constraint {
	m, ok := v.(map[string]any)
	if !ok {
		return Unknown{Why: "not an object"}
	}
	typ, _ := m["type"].(string)
	t, ok := constraintTypes[typ]
	if !ok {
		return Unknown{Type: typ, Why: "a constraint this verifier does not know"}
	}
	fields := jsonFields(t)
	for k := range m {
		if _, ok := fields[k]; !ok {
			return Unknown{Type: typ, Why: fmt.Sprintf("member %q is not in the constraint's schema", k)}
		}
	}
	raw, _ := json.Marshal(m)
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	c := reflect.New(t)
	if err := dec.Decode(c.Interface()); err != nil {
		return Unknown{Type: typ, Why: err.Error()}
	}
	return c.Elem().Interface()
}

// claims decodes a resolved delegate payload into a mandate struct. The mandates allow
// claims beyond their schema (any SD-JWT VC claim may be used), so unknown members are
// ignored; but not one that is a twin of a member of the schema, see exactKeys.
func claims(m map[string]any, into any) error {
	if err := exactKeys(m, reflect.TypeOf(into)); err != nil {
		return err
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, into)
}

// jsonFields is a struct's members as encoding/json names them, with their types.
func jsonFields(t reflect.Type) map[string]reflect.Type {
	fields := map[string]reflect.Type{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		switch {
		case !f.IsExported() || name == "-":
		case name == "":
			fields[f.Name] = f.Type
		default:
			fields[name] = f.Type
		}
	}
	return fields
}

// exactKeys refuses a member whose name is not one of the struct's but is a case-folded twin of
// one. encoding/json matches names case-insensitively, folding Unicode (the long s folds to s),
// so a mandate carrying both payment_instrument and a twin of it is read by the struct as one
// thing and by an exact-key comparison, presetClaims, as another.
func exactKeys(v any, t reflect.Type) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Slice:
		if list, ok := v.([]any); ok {
			for _, e := range list {
				if err := exactKeys(e, t.Elem()); err != nil {
					return err
				}
			}
		}
	case reflect.Struct:
		m, _ := v.(map[string]any)
		fields := jsonFields(t)
		for k, e := range m {
			if ft, ok := fields[k]; ok {
				if err := exactKeys(e, ft); err != nil {
					return err
				}
				continue
			}
			for name := range fields {
				if strings.EqualFold(k, name) {
					return fmt.Errorf("member %q is a case-folded twin of %q", k, name)
				}
			}
		}
	}
	return nil
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
	case p.PaymentAmount.Amount <= 0:
		return nil, mandate(ErrMandate, "payment mandate payment_amount is %d: an amount is above zero", p.PaymentAmount.Amount)
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
	if !has(o.Constraints, "checkout.line_items") {
		return nil, mandate(ErrMandate, "an open checkout mandate holds a checkout.line_items constraint")
	}
	return &o, nil
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
	if !has(o.Constraints, "payment.reference") {
		return nil, mandate(ErrMandate, "an open payment mandate holds a payment.reference constraint")
	}
	return &o, nil
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
