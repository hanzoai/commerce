// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ap2

import (
	"context"
	"crypto/ecdsa"
	"crypto/subtle"
	"errors"
	"time"
)

// Trust names the keys whose signature on the first token of a presentation the verifier
// accepts: a Trusted Agent Provider's (agent_authorization.md, "Mandate Delegation").
type Trust interface {
	// Key is the key of kid. An error, or a nil key, refuses the mandate.
	Key(ctx context.Context, kid string) (*ecdsa.PublicKey, error)
}

// Want is what the verifier expects of the key-bound closed token: its `aud` and `nonce`.
// An empty field is not checked.
type Want struct {
	Audience string
	Nonce    string
}

// Verifier checks mandate presentations. Skew is the clock difference a validity period
// tolerates; the spec names none. Now is time.Now when nil.
type Verifier struct {
	Trust Trust
	Now   func() time.Time
	Skew  time.Duration
}

func (v *Verifier) now() time.Time {
	if v.Now != nil {
		return v.Now()
	}
	return time.Now()
}

// presented is a presentation whose signatures, hashes and validity periods have held: the
// delegate payloads of the open mandate (nil when the closed mandate is signed directly) and of
// the closed one, the open token, and the hash of the closed token a receipt names.
type presented struct {
	open, closed map[string]any
	openTok      *token
	reference    string
}

// verify checks a presentation (agent_authorization.md, "Verification and Processing Rules";
// Delegate SD-JWT): the first token is signed by a trusted key; the closed token that follows it
// is signed by the key in the open mandate's `cnf`, carries `iat`, `aud`, `nonce` and an
// `sd_hash` that is the hash of the open token as presented; every disclosure is used; each
// token resolves to one mandate; `exp`, `nbf` and `iat` hold.
func (v *Verifier) verify(ctx context.Context, s string, want Want) (*presented, *Error) {
	p, err := parsePresentation(s)
	if err != nil {
		return nil, err
	}
	root := p.tokens[0]
	if root.jws.header.Typ == typClosed {
		return nil, credential(ErrChain, "the first token is a key-bound token")
	}
	var key *ecdsa.PublicKey
	if kid := root.jws.header.Kid; kid != "" {
		key, _ = v.Trust.Key(ctx, kid)
	}
	if key == nil {
		return nil, credential(ErrKey, "the first token's key %q is not trusted", root.jws.header.Kid)
	}
	if serr := root.jws.verify(key); serr != nil {
		return nil, credential(ErrSignature, "first token: %v", serr)
	}
	first, err := v.delegate(root)
	if err != nil {
		return nil, err
	}
	out := &presented{closed: first, reference: p.reference()}
	if len(p.tokens) == 1 {
		if _, open := first["cnf"]; open {
			return nil, credential(ErrChain, "an open mandate is presented without its closed mandate")
		}
		return out, nil
	}
	out.open, out.openTok = first, root
	if out.closed, err = v.keyBound(p.tokens[1], root, first, want); err != nil {
		return nil, err
	}
	return out, nil
}

// typClosed is the `typ` of the key-bound token that carries a closed mandate.
const typClosed = "kb+sd-jwt"

// keyBound verifies the closed token against the open one it follows.
func (v *Verifier) keyBound(tok, open *token, openPayload map[string]any, want Want) (map[string]any, *Error) {
	if tok.jws.header.Typ != typClosed {
		return nil, credential(ErrChain, "the closed token has typ %q, not %q", tok.jws.header.Typ, typClosed)
	}
	cnf, ok := openPayload["cnf"].(map[string]any)
	if !ok {
		return nil, credential(ErrChain, "the open mandate has no cnf to bind the closed mandate to")
	}
	jwk, err := decodeJWK(cnf["jwk"])
	if err == nil {
		var key *ecdsa.PublicKey
		if key, err = jwk.Key(); err == nil && tok.jws.verify(key) != nil {
			return nil, credential(ErrSignature, "the closed token is not signed by the open mandate's cnf key")
		}
	}
	if err != nil {
		return nil, credential(ErrKey, "the open mandate's cnf: %v", err)
	}
	c := tok.claims
	if _, ok := c["issuer_jwt_hash"]; ok {
		return nil, credential(ErrBinding, "the closed token binds with issuer_jwt_hash; AP2 requires sd_hash")
	}
	if sdHash, _ := c["sd_hash"].(string); !same(sdHash, open.hash()) {
		return nil, credential(ErrBinding, "sd_hash is not the hash of the open mandate as presented")
	}
	if _, ok := integer(c["iat"]); !ok {
		return nil, credential(ErrMalformed, "the closed token needs iat")
	}
	if aud, ok := c["aud"].(string); !ok || want.Audience != "" && !same(aud, want.Audience) {
		return nil, credential(ErrAudience, "the closed token is not for %q", want.Audience)
	}
	if nonce, ok := c["nonce"].(string); !ok || want.Nonce != "" && !same(nonce, want.Nonce) {
		return nil, credential(ErrNonce, "the closed token does not carry the nonce given for this request")
	}
	closed, derr := v.delegate(tok)
	if derr != nil {
		return nil, derr
	}
	if _, ok := closed["cnf"]; ok {
		return nil, credential(ErrChain, "the closed mandate carries cnf")
	}
	return closed, nil
}

// same compares two strings in constant time.
func same(a, b string) bool { return a != "" && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }

// delegate resolves a token's disclosures and returns its one delegate payload, after
// checking the validity period of the token and of that payload.
func (v *Verifier) delegate(t *token) (map[string]any, *Error) {
	claims, err := t.resolve()
	if err != nil {
		return nil, err
	}
	if e := v.period(claims); e != nil {
		return nil, e
	}
	list, _ := claims["delegate_payload"].([]any)
	if len(list) != 1 {
		return nil, credential(ErrChain, "delegate_payload must disclose exactly one mandate")
	}
	m, ok := list[0].(map[string]any)
	if !ok {
		return nil, credential(ErrChain, "delegate_payload holds no mandate")
	}
	if e := v.period(m); e != nil {
		return nil, e
	}
	return m, nil
}

// period checks `exp`, `nbf` and `iat` where a payload carries them, in whole seconds so that no
// date wraps.
func (v *Verifier) period(m map[string]any) *Error {
	now, skew := v.now().Unix(), int64(v.Skew/time.Second)
	for _, name := range []string{"exp", "nbf", "iat"} {
		raw, ok := m[name]
		if !ok {
			continue
		}
		n, ok := integer(raw)
		switch {
		case !ok:
			return credential(ErrMalformed, "%s is not a whole number", name)
		case name == "exp" && n < now-skew:
			return credential(ErrExpired, "the mandate has expired")
		case name != "exp" && n > now+skew:
			return credential(ErrNotYet, "%s is in the future", name)
		}
	}
	return nil
}

// CheckoutProof is a verified Checkout Mandate.
type CheckoutProof struct {
	// Reference is the hash of the closed mandate's final SD-JWT, which a receipt names.
	Reference string
	Mandate   CheckoutMandate
	Checkout  Checkout
	// OpenToken is the open Checkout Mandate as presented, "" for a direct mandate. A Payment
	// Mandate names its hash in `payment.reference`.
	OpenToken string
	openID    string
}

// Checkout verifies a Checkout Mandate as the merchant must (specification.md, "Merchant"): the
// chain of mandates; that the checkout the merchant signed, under key merchant, is the one the
// mandate hashes; and, for an autonomous mandate, that it meets every constraint of the open
// mandate. The audience and the nonce tie an autonomous mandate to this request.
func (v *Verifier) Checkout(ctx context.Context, presentation string, want Want, merchant *ecdsa.PublicKey) (*CheckoutProof, error) {
	if want.Audience == "" || want.Nonce == "" || merchant == nil {
		return nil, errors.New("ap2: Checkout needs the audience, the nonce and the merchant key")
	}
	ver, err := v.verify(ctx, presentation, want)
	if err != nil {
		return nil, err
	}
	closed, err := closedCheckout(ver.closed)
	if err != nil {
		return nil, err
	}
	co, err := checkoutOf(closed, merchant, v.now())
	if err != nil {
		return nil, err
	}
	proof := &CheckoutProof{Reference: ver.reference, Mandate: *closed, Checkout: *co}
	if ver.open == nil {
		return proof, nil
	}
	open, err := openCheckout(ver.open)
	if err != nil {
		return nil, err
	}
	if err := presetClaims(ver.open, ver.closed, "checkout_jwt", "checkout_hash"); err != nil {
		return nil, err
	}
	if err := open.check(co); err != nil {
		return nil, err
	}
	proof.OpenToken, proof.openID = ver.openTok.text(), ver.openTok.id()
	return proof, nil
}

// PaymentProof is a verified Payment Mandate.
type PaymentProof struct {
	Reference string
	Mandate   PaymentMandate
	// Pinned says the open mandate limits which instrument may pay: an allowed-instruments
	// constraint, or a preset payment_instrument.
	Pinned bool
}

// PayWant is what a Payment Mandate is verified against.
type PayWant struct {
	// Want is the audience and nonce the mandate was bound to, when the verifier is who they were
	// given to; a processor that receives the mandate inside a payment credential leaves them
	// empty: they were for the credential provider.
	Want
	// CheckoutJWT is the merchant-signed checkout of the verified Checkout Mandate. The
	// mandate's `transaction_id` must be its hash.
	CheckoutJWT string
	// OpenCheckout is that mandate's open Checkout Mandate (CheckoutProof.OpenToken), "" when
	// it has none.
	OpenCheckout string
	// Spent reports what has been accepted under an open mandate, for payment.budget and
	// payment.agent_recurrence, which cannot be evaluated without it.
	Spent Spent
}

// Payment verifies a Payment Mandate as the merchant payment processor must (specification.md,
// "Merchant Payment Processor"): the chain of mandates; that `transaction_id` binds it to the
// checkout; and, for an autonomous mandate, that it meets every constraint of the open mandate.
func (v *Verifier) Payment(ctx context.Context, presentation string, want PayWant) (*PaymentProof, error) {
	if want.CheckoutJWT == "" {
		return nil, errors.New("ap2: Payment needs the checkout the payment is for")
	}
	ver, err := v.verify(ctx, presentation, want.Want)
	if err != nil {
		return nil, err
	}
	closed, err := closedPayment(ver.closed)
	if err != nil {
		return nil, err
	}
	if !same(closed.TransactionID, digest(want.CheckoutJWT)) {
		return nil, mandate(ErrHash, "transaction_id is not the hash of the checkout this payment is for")
	}
	proof := &PaymentProof{Reference: ver.reference, Mandate: *closed}
	if ver.open == nil {
		return proof, nil
	}
	open, err := openPayment(ver.open)
	if err != nil {
		return nil, err
	}
	if err := presetClaims(ver.open, ver.closed, "payee", "pisp", "payment_amount", "payment_instrument", "execution_date", "risk_data", "transaction_id"); err != nil {
		return nil, err
	}
	env := payEnv{now: v.now()}
	if want.OpenCheckout != "" {
		env.openCheckout = digest(want.OpenCheckout)
	}
	env.spent = func() (Usage, *Error) {
		if want.Spent == nil {
			return Usage{}, unresolved("the verifier keeps no record of what an open mandate has spent")
		}
		u, err := want.Spent(ctx, ver.openTok.id())
		if err != nil {
			return Usage{}, unresolved("what the open mandate has spent could not be read")
		}
		return u, nil
	}
	if err := open.check(closed, env); err != nil {
		return nil, err
	}
	_, proof.Pinned = ver.open["payment_instrument"]
	proof.Pinned = proof.Pinned || has(open.Constraints, "payment.allowed_payment_instruments")
	return proof, nil
}
