// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package x402

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Gate is the resource server's half of the protocol, in the order the
// specification gives for the default flow (section 6.1, authorization): the
// payment is verified before the resource runs, and settled after it succeeds.
//
//	adm, ref := g.Admit(ctx, name, url, r.Header.Get(HeaderPaymentSignature))
//	if ref != nil { /* write ref and stop */ }
//	defer adm.Release()
//	/* serve the resource */
//	_, ref = adm.Settle(ctx) // only when it succeeded
//
// Middleware does exactly this around an http.Handler.
//
// A Gate holds the payments it has admitted and not yet settled, and refuses a
// second request carrying the same authorization while the first is in flight:
// without that, two concurrent requests with one payment both verify and both run,
// and only one settles. It does not span processes; across replicas the
// facilitator's own verify is what must refuse a spent or claimed nonce. A Gate
// must not be copied after first use.
type Gate struct {
	Table       *Table
	Facilitator Facilitator
	// Now is the clock; nil means time.Now.
	Now func() time.Time

	mu       sync.Mutex
	inflight map[string]struct{}
}

// within bounds a facilitator call by the time the payer was given to pay: the
// requirement's own maxTimeoutSeconds, which the operator configured.
func within(ctx context.Context, offered PaymentRequirements) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, time.Duration(offered.MaxTimeoutSeconds)*time.Second)
}

func (g *Gate) now() time.Time {
	if g.Now != nil {
		return g.Now()
	}
	return time.Now()
}

// Required builds the 402 challenge for a priced resource. url is the resource's
// public URL as the client sees it; message says why payment is asked for. It
// reports false for a resource that is not for sale.
func (g *Gate) Required(resource, url, message string) (PaymentRequired, bool) {
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
	gate     *Gate
	payload  PaymentPayload
	offered  PaymentRequirements
	payer    string
	resource ResourceInfo
	claim    string
	released atomic.Bool
}

// Release lets the gate admit this authorization again. Settle releases it; a
// caller that admits a payment and does not settle it (the resource failed) must
// release it, so a payment that moved no money can be used for a retry. It is safe
// to call more than once.
func (a *Admission) Release() {
	if a.released.CompareAndSwap(false, true) {
		a.gate.mu.Lock()
		delete(a.gate.inflight, a.claim)
		a.gate.mu.Unlock()
	}
}

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
func (g *Gate) Admit(ctx context.Context, resource, url, header string) (*Admission, *Refusal) {
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
	a := &Admission{gate: g, payload: pay, offered: *offered, payer: pay.Payload.Authorization.From, resource: required.Resource,
		claim: strings.ToLower(strings.Join([]string{offered.Network, offered.Asset, pay.Payload.Authorization.From, pay.Payload.Authorization.Nonce}, "|"))}
	g.mu.Lock()
	if _, busy := g.inflight[a.claim]; busy {
		g.mu.Unlock()
		return nil, deny(ReasonInvalidTransaction)
	}
	if g.inflight == nil {
		g.inflight = map[string]struct{}{}
	}
	g.inflight[a.claim] = struct{}{}
	g.mu.Unlock()
	admitted := false
	defer func() {
		if !admitted { // a refusal, or a facilitator that panicked: the claim is not left behind
			a.Release()
		}
	}()
	vctx, cancel := within(ctx, *offered)
	defer cancel()
	v, err := g.Facilitator.Verify(vctx, pay, *offered)
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
	admitted = true
	return a, nil
}

// Settle commits the payment once the resource has been served, and answers the
// SettlementResponse for PAYMENT-RESPONSE. A nil Refusal means the payment
// settled; otherwise the resource was served and not paid for, and the Refusal
// carries the failed settlement for the response (spec 5.3: the response rides
// the failure leg too). Call it only after a successful serve: a failed serve
// settles nothing.
func (a *Admission) Settle(ctx context.Context) (SettlementResponse, *Refusal) {
	defer a.Release()
	status := http.StatusPaymentRequired
	sctx, cancel := within(ctx, a.offered)
	defer cancel()
	s, err := a.gate.Facilitator.Settle(sctx, a.payload, a.offered)
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
