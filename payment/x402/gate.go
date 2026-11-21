// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package x402

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Gate is the resource server's half of the protocol, in the order the
// specification gives for the default flow (section 6.1, authorization): the
// payment is verified before the resource runs, and settled after it succeeds.
//
//	adm, ref := g.Admit(ctx, name, url, r.Header.Get(HeaderPaymentSignature))
//	if ref != nil { /* write ref and stop */ }
//	/* serve the resource */
//	ref = adm.Settle(ctx) // on success
//
// Middleware does exactly this around an http.Handler.
type Gate struct {
	Table       *Table
	Facilitator Facilitator
	// Now is the clock; nil means time.Now.
	Now func() time.Time
}

func (g Gate) now() time.Time {
	if g.Now != nil {
		return g.Now()
	}
	return time.Now()
}

// Required builds the 402 challenge for a priced resource. url is the resource's
// public URL as the client sees it; message says why payment is asked for. It
// reports false for a resource that is not for sale.
func (g Gate) Required(resource, url, message string) (PaymentRequired, bool) {
	accepts, info, ok := g.Table.Quote(resource)
	if !ok {
		return PaymentRequired{}, false
	}
	info.URL = url
	return PaymentRequired{X402Version: Version, Error: message, Resource: info, Accepts: accepts}, true
}

// Admission is a payment that checked out: the resource may be served, and the
// payment is settled once it has been.
type Admission struct {
	gate     Gate
	payload  PaymentPayload
	offered  PaymentRequirements
	payer    string
	resource ResourceInfo
}

// Payer is the address that signed the payment.
func (a *Admission) Payer() string { return a.payer }

// Requirements is the offered requirement the payment satisfied.
func (a *Admission) Requirements() PaymentRequirements { return a.offered }

// Payload is the payment as the client sent it.
func (a *Admission) Payload() PaymentPayload { return a.payload }

// Refusal is a request the gate will not serve: the status to answer with, and
// what rides the response. Status 402 carries the challenge; 400 is a payment
// that could not be read; a settlement that failed carries PAYMENT-RESPONSE.
type Refusal struct {
	Status     int
	Reason     string
	Required   *PaymentRequired
	Settlement *SettlementResponse
}

func (r *Refusal) Error() string { return fmt.Sprintf("x402: refused %d: %s", r.Status, r.Reason) }

// Headers is what a refusal puts on the response, as header name and value.
func (r *Refusal) Headers() (map[string]string, error) {
	h := map[string]string{}
	if r.Required != nil {
		v, err := EncodeHeader(r.Required)
		if err != nil {
			return nil, err
		}
		h[HeaderPaymentRequired] = v
	}
	if r.Settlement != nil {
		v, err := EncodeHeader(r.Settlement)
		if err != nil {
			return nil, err
		}
		h[HeaderPaymentResponse] = v
	}
	return h, nil
}

// Admit checks the payment a request carries for a priced resource. header is
// the PAYMENT-SIGNATURE value, empty when there is none. Exactly one of the
// results is non-nil. The resource must be for sale (Table.Priced); asking about
// one that is not is a Refusal of status 500, because serving it free by
// mistake is worse than refusing it.
//
// The checks, in order: the header decodes (400 if not); the payload echoes one
// of the offered requirements; the signature, recipient and amount verify
// (Verify); the window holds (InWindow); the facilitator accepts it (Verify,
// spec 7.1). Each failure is a 402 with a fresh challenge whose `error` is the
// spec reason.
func (g Gate) Admit(ctx context.Context, resource, url, header string) (*Admission, *Refusal) {
	required, ok := g.Required(resource, url, "")
	if !ok {
		return nil, &Refusal{Status: http.StatusInternalServerError, Reason: "resource is not for sale: " + resource}
	}
	deny := func(reason string) *Refusal {
		required.Error = reason
		return &Refusal{Status: http.StatusPaymentRequired, Reason: reason, Required: &required}
	}
	if header == "" {
		return nil, deny(HeaderPaymentSignature + " header is required")
	}
	pay, err := ParsePayment(header)
	if err != nil {
		return nil, &Refusal{Status: http.StatusBadRequest, Reason: ReasonOf(err, ReasonInvalidPayload)}
	}
	var offered *PaymentRequirements
	for i := range required.Accepts {
		if Same(required.Accepts[i], pay.Accepted) == nil {
			offered = &required.Accepts[i]
			break
		}
	}
	if offered == nil {
		// Report the mismatch against the first offer: that is the diagnosis
		// a client can act on.
		return nil, deny(ReasonOf(Same(required.Accepts[0], pay.Accepted), ReasonInvalidRequirements))
	}
	if err := Verify(*offered, pay); err != nil {
		return nil, deny(ReasonOf(err, ReasonVerifyError))
	}
	if err := InWindow(pay.Payload.Authorization, g.now()); err != nil {
		return nil, deny(ReasonOf(err, ReasonVerifyError))
	}
	v, err := g.Facilitator.Verify(ctx, pay, *offered)
	switch {
	case err != nil:
		return nil, &Refusal{Status: http.StatusInternalServerError, Reason: ReasonVerifyError}
	case !v.IsValid:
		reason := v.InvalidReason
		if reason == "" {
			reason = ReasonVerifyError
		}
		return nil, deny(reason)
	}
	return &Admission{gate: g, payload: pay, offered: *offered, payer: pay.Payload.Authorization.From, resource: required.Resource}, nil
}

// Settle commits the payment once the resource has been served, and answers the
// SettlementResponse for PAYMENT-RESPONSE. A nil Refusal means the payment
// settled; otherwise the resource was served and not paid for, and the Refusal
// carries the failed settlement for the response (spec 5.3: the response rides
// the failure leg too). Call it only after a successful serve: a failed serve
// settles nothing.
func (a *Admission) Settle(ctx context.Context) (SettlementResponse, *Refusal) {
	status := http.StatusPaymentRequired
	s, err := a.gate.Facilitator.Settle(ctx, a.payload, a.offered)
	if err != nil {
		// The facilitator could not answer: the payment's fate is unknown, which
		// is a server error and never a success.
		s = SettlementResponse{ErrorReason: ReasonSettleError, Payer: a.payer, Network: a.offered.Network}
		status = http.StatusInternalServerError
	}
	if s.Success {
		return s, nil
	}
	if s.ErrorReason == "" {
		s.ErrorReason = ReasonSettleError
	}
	required := PaymentRequired{X402Version: Version, Error: s.ErrorReason, Resource: a.resource, Accepts: []PaymentRequirements{a.offered}}
	return s, &Refusal{Status: status, Reason: s.ErrorReason, Required: &required, Settlement: &s}
}
