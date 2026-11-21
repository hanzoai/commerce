// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ap2

import (
	"context"
	"crypto/ecdsa"
	"crypto/subtle"
	"errors"
	"fmt"
	"time"
)

// Trust names the keys whose signature on the first token of a presentation the verifier
// accepts: a Trusted Agent Provider's, or the key of a user credential (agent_authorization.md,
// "Mandate Delegation"). It is the verifier's own list, never read from the presentation.
type Trust interface {
	// Key is the key of kid. An error, or a nil key, is a refusal of the mandate.
	Key(ctx context.Context, kid string) (*ecdsa.PublicKey, error)
}

// Keys is a fixed Trust: key ids and their keys.
type Keys map[string]*ecdsa.PublicKey

// Key implements Trust.
func (k Keys) Key(_ context.Context, kid string) (*ecdsa.PublicKey, error) {
	if pub := k[kid]; pub != nil {
		return pub, nil
	}
	return nil, fmt.Errorf("no key %q", kid)
}

// Want is what the verifier expects of the key-bound closed mandate (Delegate SD-JWT: the
// key-bound token carries `aud` and `nonce`). The audience names the verifier and the nonce
// is the one it gave the agent for this request. An empty field is not checked.
type Want struct {
	Audience string
	Nonce    string
}

// Verifier checks mandate presentations. The zero Now is time.Now.
type Verifier struct {
	Trust Trust
	Now   func() time.Time
	// Skew is how far a clock may differ and a validity period still hold. The spec names
	// none, so it is zero unless set.
	Skew time.Duration
}

func (v *Verifier) now() time.Time {
	if v.Now != nil {
		return v.Now()
	}
	return time.Now()
}

// Delegate is one verified mandate: the delegate payload with its disclosures applied, and
// the token it came in.
type Delegate struct {
	Claims map[string]any
	tok    *token
}

// Verified is a presentation whose signatures, hashes and validity periods have held.
type Verified struct {
	// Open is the user-signed open mandate; nil when the closed mandate is signed directly.
	Open *Delegate
	// Closed is the closed mandate.
	Closed *Delegate
	// Reference is the hash of the closed mandate's final SD-JWT, which a receipt names.
	Reference string
}

// isKeyBound says a JOSE `typ` is that of a key-bound delegation token.
func isKeyBound(typ string) bool {
	switch typ {
	case "kb+sd-jwt", "kb-sd-jwt", "kb+sd-jwt+kb", "kb-sd-jwt+kb":
		return true
	}
	return false
}

// verify checks a presentation (agent_authorization.md, "Verification and Processing
// Rules", step 1; Delegate SD-JWT):
//
//   - the first token is signed by a trusted key, and is not itself a key-bound token;
//   - the closed token that follows it is signed by the key in the open mandate's `cnf`,
//     carries `iat`, the expected `aud` and `nonce`, and an `sd_hash` that is the hash of the
//     open token exactly as presented;
//   - every disclosure is used, and each token's delegate payload resolves to one mandate;
//   - `exp`, `nbf` and `iat` of every token and mandate hold.
func (v *Verifier) verify(ctx context.Context, s string, want Want) (*Verified, *Error) {
	p, err := parsePresentation(s)
	if err != nil {
		return nil, err
	}
	now := v.now()
	root := p.tokens[0]
	if isKeyBound(root.jws.header.Typ) {
		return nil, credential(ErrChain, "the first token is a key-bound token, typ %q", root.jws.header.Typ)
	}
	if root.jws.header.Kid == "" {
		return nil, credential(ErrKey, "the first token names no kid")
	}
	key, kerr := v.Trust.Key(ctx, root.jws.header.Kid)
	if kerr != nil || key == nil {
		return nil, credential(ErrKey, "%q is not a trusted key", root.jws.header.Kid)
	}
	if serr := root.jws.verify(key); serr != nil {
		return nil, credential(ErrSignature, "first token: %v", serr)
	}
	first, err := v.delegate(root, now)
	if err != nil {
		return nil, err
	}
	out := &Verified{Closed: first}
	if len(p.tokens) == 2 {
		out.Open = first
		if out.Closed, err = v.closed(p, first, want, now); err != nil {
			return nil, err
		}
	} else if _, bound := first.Claims["cnf"]; bound {
		// A lone token with cnf is an open mandate: it authorizes nothing by itself.
		return nil, credential(ErrChain, "an open mandate is presented without its closed mandate")
	}
	ref, herr := p.reference()
	if herr != nil {
		return nil, &Error{Code: InvalidCredential, Err: herr}
	}
	out.Reference = ref
	return out, nil
}

// closed verifies the key-bound closed token against the open mandate it follows.
func (v *Verifier) closed(p *presentation, open *Delegate, want Want, now time.Time) (*Delegate, *Error) {
	tok := p.tokens[1]
	if typ := tok.jws.header.Typ; typ != "kb+sd-jwt" && typ != "kb-sd-jwt" {
		return nil, credential(ErrChain, "the closed token has typ %q, not a terminal key-bound token", typ)
	}
	cnf, ok := open.Claims["cnf"].(map[string]any)
	if !ok {
		return nil, credential(ErrChain, "the open mandate has no cnf to bind the closed mandate to")
	}
	jwk, err := decodeJWK(cnf["jwk"])
	if err != nil {
		return nil, credential(ErrKey, "open mandate cnf: %v", err)
	}
	key, err := jwk.Key()
	if err != nil {
		return nil, credential(ErrKey, "open mandate cnf: %v", err)
	}
	if err := tok.jws.verify(key); err != nil {
		return nil, credential(ErrSignature, "closed token is not signed by the open mandate's cnf key")
	}
	c := tok.claims
	if _, ok := c["issuer_jwt_hash"]; ok {
		return nil, credential(ErrBinding, "the closed token binds with issuer_jwt_hash; AP2 requires sd_hash")
	}
	sdHash, _ := c["sd_hash"].(string)
	wantHash, herr := p.tokens[0].hash()
	if herr != nil {
		return nil, &Error{Code: InvalidCredential, Err: herr}
	}
	if sdHash == "" || subtle.ConstantTimeCompare([]byte(sdHash), []byte(wantHash)) != 1 {
		return nil, credential(ErrBinding, "sd_hash is not the hash of the open mandate as presented")
	}
	if _, ok := integer(c["iat"]); !ok {
		return nil, credential(ErrMalformed, "the closed token needs iat")
	}
	aud, ok := c["aud"].(string)
	if !ok {
		return nil, credential(ErrAudience, "the closed token needs aud")
	}
	nonce, ok := c["nonce"].(string)
	if !ok {
		return nil, credential(ErrNonce, "the closed token needs nonce")
	}
	if want.Audience != "" && subtle.ConstantTimeCompare([]byte(aud), []byte(want.Audience)) != 1 {
		return nil, credential(ErrAudience, "aud %q is not %q", aud, want.Audience)
	}
	if want.Nonce != "" && subtle.ConstantTimeCompare([]byte(nonce), []byte(want.Nonce)) != 1 {
		return nil, credential(ErrNonce, "nonce is not the one given for this request")
	}
	closed, derr := v.delegate(tok, now)
	if derr != nil {
		return nil, derr
	}
	if _, ok := closed.Claims["cnf"]; ok {
		return nil, credential(ErrChain, "the closed mandate carries cnf")
	}
	return closed, nil
}

// delegate resolves a token's disclosures and takes its one delegate payload, after
// checking the validity period of the token and of that payload.
func (v *Verifier) delegate(t *token, now time.Time) (*Delegate, *Error) {
	claims, err := t.resolve()
	if err != nil {
		var e *Error
		if errors.As(err, &e) {
			return nil, e
		}
		return nil, &Error{Code: InvalidCredential, Err: err}
	}
	if e := v.period(claims, now); e != nil {
		return nil, e
	}
	list, ok := claims["delegate_payload"].([]any)
	if !ok || len(list) != 1 {
		return nil, credential(ErrChain, "delegate_payload must disclose exactly one mandate")
	}
	m, ok := list[0].(map[string]any)
	if !ok {
		return nil, credential(ErrChain, "delegate_payload holds no mandate")
	}
	if e := v.period(m, now); e != nil {
		return nil, e
	}
	return &Delegate{Claims: m, tok: t}, nil
}

// period checks `exp`, `nbf` and `iat` where a payload carries them.
func (v *Verifier) period(m map[string]any, now time.Time) *Error {
	for _, name := range []string{"exp", "nbf", "iat"} {
		raw, ok := m[name]
		if !ok {
			continue
		}
		n, ok := integer(raw)
		if !ok {
			return credential(ErrMalformed, "%s is not a whole number", name)
		}
		at := time.Unix(n, 0)
		switch {
		case name == "exp" && now.After(at.Add(v.Skew)):
			return credential(ErrExpired, "expired at %s", at.UTC().Format(time.RFC3339))
		case name != "exp" && at.After(now.Add(v.Skew)):
			return credential(ErrNotYet, "%s is %s, in the future", name, at.UTC().Format(time.RFC3339))
		}
	}
	return nil
}

// CheckoutProof is a verified Checkout Mandate.
type CheckoutProof struct {
	Verified
	Mandate  CheckoutMandate
	Checkout Checkout
	// OpenToken is the open Checkout Mandate's token as presented, "" for a direct mandate.
	// A Payment Mandate names it in `payment.reference`.
	OpenToken string
}

// Checkout verifies a Checkout Mandate as the merchant must (specification.md, "Merchant"):
// the chain of mandates; that the merchant's own signed checkout is the one the mandate
// hashes; and, for an autonomous mandate, that the checkout meets every constraint of the
// open mandate. merchant is the key the merchant signs checkouts with. The audience and the
// nonce are required: they are what ties an autonomous mandate to this request.
func (v *Verifier) Checkout(ctx context.Context, presented string, want Want, merchant *ecdsa.PublicKey) (*CheckoutProof, error) {
	if want.Audience == "" || want.Nonce == "" || merchant == nil {
		return nil, errors.New("ap2: Checkout needs the audience, the nonce and the merchant key")
	}
	ver, err := v.verify(ctx, presented, want)
	if err != nil {
		return nil, err
	}
	closed, err := closedCheckout(ver.Closed.Claims)
	if err != nil {
		return nil, err
	}
	co, err := checkoutOf(closed, ver.Closed.tok.alg, merchant, v.now(), v.Skew)
	if err != nil {
		return nil, err
	}
	proof := &CheckoutProof{Verified: *ver, Mandate: *closed, Checkout: *co}
	if ver.Open == nil {
		return proof, nil
	}
	open, err := openCheckout(ver.Open.Claims)
	if err != nil {
		return nil, err
	}
	if err := presetClaims(ver.Open.Claims, ver.Closed.Claims, "checkout_jwt", "checkout_hash"); err != nil {
		return nil, err
	}
	if err := open.check(co); err != nil {
		return nil, err
	}
	proof.OpenToken = ver.Open.tok.text()
	return proof, nil
}

// PaymentProof is a verified Payment Mandate.
type PaymentProof struct {
	Verified
	Mandate PaymentMandate
}

// PayWant is what a Payment Mandate is verified against.
type PayWant struct {
	// Want is the audience and nonce the mandate was bound to, where the verifier is the
	// party they were given to. A merchant payment processor that receives the mandate inside
	// a payment credential leaves them empty: they were for the credential provider.
	Want
	// CheckoutJWT is the merchant-signed checkout of the verified Checkout Mandate the
	// payment is for. The mandate's `transaction_id` must be its hash.
	CheckoutJWT string
	// OpenCheckout is the open Checkout Mandate of that mandate's chain, as presented
	// (CheckoutProof.OpenToken); empty for a direct mandate.
	OpenCheckout string
	// Spent reports what has been accepted under an open mandate. It is read only when a
	// payment.budget or payment.agent_recurrence constraint is evaluated, and such a
	// constraint cannot be evaluated without it.
	Spent Spent
}

// Payment verifies a Payment Mandate as the merchant payment processor must
// (specification.md, "Merchant Payment Processor", "Credential Provider and Network"): the
// chain of mandates; that it is bound to the checkout, by `transaction_id`; and, for an
// autonomous mandate, that it meets every constraint of the open mandate.
func (v *Verifier) Payment(ctx context.Context, presented string, want PayWant) (*PaymentProof, error) {
	if want.CheckoutJWT == "" {
		return nil, errors.New("ap2: Payment needs the checkout the payment is for")
	}
	ver, err := v.verify(ctx, presented, want.Want)
	if err != nil {
		return nil, err
	}
	closed, err := closedPayment(ver.Closed.Claims)
	if err != nil {
		return nil, err
	}
	id, herr := digest(ver.Closed.tok.alg, want.CheckoutJWT)
	if herr != nil {
		return nil, &Error{Code: InvalidCredential, Err: herr}
	}
	if subtle.ConstantTimeCompare([]byte(id), []byte(closed.TransactionID)) != 1 {
		return nil, mandate(ErrHash, "transaction_id is not the hash of the checkout this payment is for")
	}
	proof := &PaymentProof{Verified: *ver, Mandate: *closed}
	if ver.Open == nil {
		return proof, nil
	}
	open, err := openPayment(ver.Open.Claims)
	if err != nil {
		return nil, err
	}
	if err := presetClaims(ver.Open.Claims, ver.Closed.Claims, "payee", "pisp", "payment_amount", "payment_instrument", "execution_date", "risk_data", "transaction_id"); err != nil {
		return nil, err
	}
	env := payEnv{now: v.now()}
	if want.OpenCheckout != "" {
		if env.openCheckout, herr = digest(ver.Open.tok.alg, want.OpenCheckout); herr != nil {
			return nil, &Error{Code: InvalidCredential, Err: herr}
		}
	}
	var hash string
	env.spent = func() (Usage, *Error) {
		if want.Spent == nil {
			return Usage{}, unresolved("the verifier keeps no record of what an open mandate has spent")
		}
		if hash == "" {
			hash, _ = ver.Open.tok.hash()
		}
		u, err := want.Spent(ctx, hash)
		if err != nil {
			return Usage{}, unresolved("what the open mandate has spent could not be read: %v", err)
		}
		return u, nil
	}
	if err := open.check(closed, env); err != nil {
		return nil, err
	}
	return proof, nil
}
