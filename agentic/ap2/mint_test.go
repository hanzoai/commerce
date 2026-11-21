// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ap2

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// The Trusted Surface and the Shopping Agent are not roles this package plays: the tests play
// them, with real keys and real SD-JWTs, so that every check of the verifier has something to
// refuse.

type party struct {
	key *ecdsa.PrivateKey
	kid string
}

func newParty(t *testing.T, kid string) party {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return party{k, kid}
}

func (p party) cnf() map[string]any {
	j := NewJWK(&p.key.PublicKey, "")
	return map[string]any{"jwk": map[string]any{"kty": j.Kty, "crv": j.Crv, "x": j.X, "y": j.Y}}
}

// sd collects the disclosures of one token.
type sd struct{ discs []string }

func (b *sd) add(t *testing.T, parts ...any) string {
	t.Helper()
	salt := make([]byte, 16)
	rand.Read(salt)
	raw, err := json.Marshal(append([]any{b64.EncodeToString(salt)}, parts...))
	if err != nil {
		t.Fatal(err)
	}
	d := b64.EncodeToString(raw)
	b.discs = append(b.discs, d)
	dg := digest(d)
	return dg
}

// elem is an array element that is disclosed: {"...": digest}.
func (b *sd) elem(t *testing.T, v any) map[string]any {
	return map[string]any{"...": b.add(t, v)}
}

// prop is an object member that is disclosed; the digest goes in the object's _sd.
func (b *sd) prop(t *testing.T, name string, v any) string { return b.add(t, name, v) }

// tok is a token before it is signed, every part of it open to a test's change.
type tok struct {
	hdr    jwsHeader
	claims map[string]any
	disc   *sd
	key    *ecdsa.PrivateKey
}

func (k *tok) text(t *testing.T) string {
	t.Helper()
	raw, err := json.Marshal(k.claims)
	if err != nil {
		t.Fatal(err)
	}
	jwt, err := sign(k.hdr, raw, k.key)
	if err != nil {
		t.Fatal(err)
	}
	if len(k.disc.discs) == 0 {
		return jwt + "~"
	}
	return jwt + "~" + strings.Join(k.disc.discs, "~") + "~"
}

// join is a presentation of an open token and the closed token after it.
func join(open, closed string) string { return open + "~" + closed }

// world is a verifier, the keys it trusts, and the parties that sign for it.
type world struct {
	t        *testing.T
	now      time.Time
	root     party // the Trusted Agent Provider
	agent    party // the Shopping Agent
	merchant Signer
	v        *Verifier
	aud      string
	nonce    string
}

func newWorld(t *testing.T) *world {
	t.Helper()
	root, agent, mk := newParty(t, "agent-provider-key-1"), newParty(t, "agent-key"), newParty(t, "merchant-key-1")
	now := time.Unix(1777342400, 0)
	return &world{
		t: t, now: now, root: root, agent: agent, merchant: Signer{Key: mk.key, Kid: mk.kid},
		v:   &Verifier{Trust: keys{root.kid: &root.key.PublicKey}, Now: func() time.Time { return now }},
		aud: "merchant", nonce: "nonce-1",
	}
}

func (w *world) sd() *sd { return &sd{} }

func (w *world) unix(d time.Duration) int64 { return w.now.Add(d).Unix() }

// checkout is the merchant's signed checkout of one item.
func (w *world) checkout(id string, qty int64, price int64) (Checkout, string, string) {
	w.t.Helper()
	co := Checkout{
		ID: id, Merchant: &Merchant{ID: "merchant_1", Name: "Demo Merchant", Website: "https://demo-merchant.example"},
		Status: ReadyForComplete, Currency: "USD", Links: []Link{},
		LineItems: []LineItem{{ID: "line_1", Item: Product{ID: "credit-500", Title: "$5.00 USD credit", Price: price}, Quantity: qty,
			Totals: []Total{{Type: "subtotal", Amount: price * qty}, {Type: "total", Amount: price * qty}}}},
		Totals: []Total{{Type: "subtotal", Amount: price * qty}, {Type: "total", Amount: price * qty}},
	}
	jwt, err := w.merchant.Sign(co)
	if err != nil {
		w.t.Fatal(err)
	}
	h := digest(jwt)
	return co, jwt, h
}

// closedCheckoutItem is the delegate payload of a closed Checkout Mandate, its checkout_jwt a
// disclosed member.
func (w *world) closedCheckoutItem(b *sd, jwt, hash string) map[string]any {
	return map[string]any{"vct": VCTCheckout, "checkout_hash": hash, "_sd": []any{b.prop(w.t, "checkout_jwt", jwt)}}
}

// rootTok is a token signed with the Trusted Agent Provider's key.
func (w *world) rootTok(b *sd, item map[string]any) *tok {
	return &tok{hdr: jwsHeader{Alg: "ES256", Typ: "example+sd-jwt", Kid: w.root.kid}, key: w.root.key, disc: b,
		claims: map[string]any{"delegate_payload": []any{b.elem(w.t, item)}, "_sd_alg": "sha-256"}}
}

// lineItems is a checkout.line_items constraint asking for qty of one of items.
func (w *world) lineItems(b *sd, qty int, items ...Item) map[string]any {
	var acc []any
	for _, it := range items {
		acc = append(acc, b.elem(w.t, map[string]any{"id": it.ID, "title": it.Title}))
	}
	return map[string]any{"type": "checkout.line_items", "items": []any{map[string]any{"id": "line_1", "acceptable_items": acc, "quantity": qty}}}
}

func (w *world) allowedMerchants(b *sd, ms ...Merchant) map[string]any {
	var list []any
	for _, m := range ms {
		list = append(list, b.elem(w.t, m))
	}
	return map[string]any{"type": "checkout.allowed_merchants", "allowed": list}
}

// autonomous is a mandate presentation of an open token and the closed token the agent signs
// after it, every part open to a test's change before it is written out.
type autonomous struct {
	w                    *world
	b1, b2               *sd
	n1, n2               int
	openItem, closedItem map[string]any
	open, closed         *tok
	openOut, closedOut   string
	// bind sets the closed token's binding claim from the open token as presented.
	bind func(claims map[string]any, open string)
}

func (w *world) autonomous(b1, b2 *sd, openItem, closedItem map[string]any) *autonomous {
	return &autonomous{
		w: w, b1: b1, b2: b2, n1: len(b1.discs), n2: len(b2.discs), openItem: openItem, closedItem: closedItem,
		open:   &tok{hdr: jwsHeader{Alg: "ES256", Typ: "example+sd-jwt", Kid: w.root.kid}, key: w.root.key, disc: b1, claims: map[string]any{"_sd_alg": "sha-256"}},
		closed: &tok{hdr: jwsHeader{Alg: "ES256", Typ: "kb+sd-jwt"}, key: w.agent.key, disc: b2, claims: map[string]any{"_sd_alg": "sha-256", "iat": w.unix(-time.Minute), "aud": w.aud, "nonce": w.nonce}},
		bind: func(c map[string]any, open string) {
			h := digest(open)
			c["sd_hash"] = h
		},
	}
}

// openText is the open token as it is presented. It is signed once, as the user signs an open
// mandate once, and the agent presents the same bytes every time.
func (a *autonomous) openText() string {
	if a.openOut == "" {
		a.b1.discs = a.b1.discs[:a.n1]
		a.open.claims["delegate_payload"] = []any{a.b1.elem(a.w.t, a.openItem)}
		a.openOut = a.open.text(a.w.t)
	}
	return a.openOut
}

// text is the presentation, signed the first time it is asked for.
func (a *autonomous) text() string {
	o := a.openText()
	if a.closedOut == "" {
		a.b2.discs = a.b2.discs[:a.n2]
		a.closed.claims["delegate_payload"] = []any{a.b2.elem(a.w.t, a.closedItem)}
		a.bind(a.closed.claims, o)
		a.closedOut = a.closed.text(a.w.t)
	}
	return join(o, a.closedOut)
}

// constraints replaces the open mandate's constraints with ones built over fresh disclosures.
func (a *autonomous) constraints(build func(b *sd) []any) {
	a.b1.discs = nil
	a.openItem["constraints"] = build(a.b1)
	a.n1 = len(a.b1.discs)
	a.openOut = ""
}

// closing replaces the closed mandate's delegate payload with one built over fresh disclosures.
func (a *autonomous) closing(build func(b *sd) map[string]any) {
	a.b2.discs = nil
	a.closedItem = build(a.b2)
	a.n2 = len(a.b2.discs)
	a.closedOut = ""
}

// resign has the agent sign its closed mandate again, as another mandate under the same open one.
func (a *autonomous) resign() { a.closedOut = "" }

// openCheckoutItem is the delegate payload of an open Checkout Mandate.
func (w *world) openCheckoutItem(constraints ...any) map[string]any {
	return map[string]any{"vct": VCTCheckoutOpen, "constraints": constraints, "cnf": w.agent.cnf(), "iat": w.unix(-time.Hour), "exp": w.unix(time.Hour)}
}

// checkoutMandate is an autonomous Checkout Mandate for the merchant's checkout: the open
// mandate allows the item and the merchant, the closed one is the agent's.
func (w *world) checkoutMandate(jwt, hash string) *autonomous {
	b1, b2 := w.sd(), w.sd()
	open := w.openCheckoutItem(
		w.lineItems(b1, 1, Item{"credit-500", "$5.00 USD credit"}),
		w.allowedMerchants(b1, Merchant{ID: "merchant_1", Name: "Demo Merchant", Website: "https://demo-merchant.example"}),
	)
	return w.autonomous(b1, b2, open, w.closedCheckoutItem(b2, jwt, hash))
}

// directCheckout is a Checkout Mandate the Trusted Agent Provider signs itself.
func (w *world) directCheckout(jwt, hash string) *tok {
	b := w.sd()
	return w.rootTok(b, w.closedCheckoutItem(b, jwt, hash))
}

// paymentItem is the delegate payload of a closed Payment Mandate.
func paymentItem(txn string, amount int64) map[string]any {
	return map[string]any{
		"vct": VCTPayment, "transaction_id": txn,
		"payee":              map[string]any{"id": "merchant_1", "name": "Demo Merchant", "website": "https://demo-merchant.example"},
		"payment_amount":     map[string]any{"amount": amount, "currency": "USD"},
		"payment_instrument": map[string]any{"id": "pi_1", "type": "card", "description": "Card 4242"},
	}
}

// paymentMandate is an autonomous Payment Mandate: open with its payment.reference and the
// given constraints, closed by the agent, bound to the open Checkout Mandate openCheckout.
func (w *world) paymentMandate(txn string, amount int64, openCheckout string, more ...func(b *sd) map[string]any) *autonomous {
	b1, b2 := w.sd(), w.sd()
	ref := digest(openCheckout)
	cs := []any{map[string]any{"type": "payment.reference", "conditional_transaction_id": ref}}
	for _, c := range more {
		cs = append(cs, c(b1))
	}
	open := map[string]any{"vct": VCTPaymentOpen, "constraints": cs, "cnf": w.agent.cnf(), "iat": w.unix(-time.Hour), "exp": w.unix(time.Hour)}
	a := w.autonomous(b1, b2, open, paymentItem(txn, amount))
	a.closed.claims["aud"] = "credential-provider"
	return a
}

// keys is a Trust of fixed keys by id.
type keys map[string]*ecdsa.PublicKey

func (k keys) Key(_ context.Context, kid string) (*ecdsa.PublicKey, error) {
	if pub := k[kid]; pub != nil {
		return pub, nil
	}
	return nil, errors.New("no such key")
}

// memory is a Ledger in the process.
type memory struct {
	now  func() time.Time
	mu   sync.Mutex
	held map[string]*held
}

type held struct {
	Claim
	kept bool
}

func (m *memory) Begin(_ context.Context, open, ref, checkout string) (Verdict, Claim, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.held == nil {
		m.held = map[string]*held{}
	}
	h, ok := m.held[open]
	switch {
	case !ok:
		now := time.Now()
		if m.now != nil {
			now = m.now()
		}
		h = &held{Claim: Claim{Ref: ref, Checkout: checkout, At: now}}
		m.held[open] = h
		return Fresh, h.Claim, nil
	case !h.kept:
		return Busy, h.Claim, nil
	case h.Ref == ref:
		return Replay, h.Claim, nil
	}
	return Taken, h.Claim, nil
}

func (m *memory) Settle(_ context.Context, open, ref string, kept bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, ok := m.held[open]
	switch {
	case !ok || h.Ref != ref || h.kept:
	case kept:
		h.kept = true
	default:
		delete(m.held, open)
	}
	return nil
}
