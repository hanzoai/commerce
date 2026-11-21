// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ap2

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Config is a merchant: the store behind it, whom it trusts, how it signs, and what it is
// called. Nothing here has a default the spec does not state.
type Config struct {
	Backend Backend
	// Trust names the keys of the Trusted Surfaces and Agent Providers whose mandates the
	// merchant accepts.
	Trust Trust
	// Signer signs the checkouts and receipts the merchant issues.
	Signer Signer
	// Identity is the merchant the checkouts name and the payee a Payment Mandate must name.
	Identity Merchant
	// Issuer is the `iss` of its receipts: the verifier's name.
	Issuer string
	// Audience is the `aud` an agent binds a closed mandate to when it presents it here.
	Audience string
	// Secret keys the nonces the merchant gives agents. At least 32 bytes.
	Secret []byte
	// Links are the checkout's links (terms of service, privacy policy, ...).
	Links []Link
	// Lifetime is how long a checkout the merchant signs stays completable. Zero signs
	// checkouts without an expiry.
	Lifetime time.Duration
	// Skew is the clock difference a validity period tolerates; the spec names none.
	Skew time.Duration
	// Now is the clock; time.Now when nil.
	Now func() time.Time
	// Ledger records the open mandates a checkout was completed under.
	Ledger Ledger
	// Spent reports what an open Payment Mandate has spent, for the constraints that need
	// it. Without it such a mandate is refused as unresolved_constraint.
	Spent Spent
}

// Seller plays the Merchant role of AP2, and the Merchant Payment Processor role over the
// store's payment: it signs the checkouts it offers, and completes a checkout only against
// a Checkout Mandate and a Payment Mandate that verify and agree with the store.
type Seller struct {
	cfg Config
	v   *Verifier
}

// New makes a Seller.
func New(cfg Config) (*Seller, error) {
	switch {
	case cfg.Backend == nil || cfg.Trust == nil || cfg.Ledger == nil:
		return nil, errors.New("ap2: a merchant needs a Backend, a Trust and a Ledger")
	case cfg.Identity.ID == "" || cfg.Identity.Name == "":
		return nil, errors.New("ap2: a merchant needs an Identity with an id and a name")
	case cfg.Issuer == "" || cfg.Audience == "":
		return nil, errors.New("ap2: a merchant needs an Issuer and an Audience")
	case len(cfg.Secret) < 32:
		return nil, errors.New("ap2: the nonce Secret is at least 32 bytes")
	case cfg.Lifetime < 0 || cfg.Skew < 0:
		return nil, errors.New("ap2: Lifetime and Skew are not negative")
	}
	if err := cfg.Signer.check(); err != nil {
		return nil, err
	}
	return &Seller{cfg: cfg, v: &Verifier{Trust: cfg.Trust, Now: cfg.Now, Skew: cfg.Skew}}, nil
}

func (m *Seller) now() time.Time { return m.v.now() }

// JWK is the public key agents and disputing parties verify the merchant's checkouts and
// receipts with.
func (m *Seller) JWK() JWK { return m.cfg.Signer.JWK() }

// Issued is a checkout the merchant has signed, with what an agent needs to have a Checkout
// Mandate made for it.
type Issued struct {
	ID string
	// JWT is the merchant-signed checkout: the mandate's `checkout_jwt`.
	JWT string
	// Hash is the hash of JWT in sha-256, the `checkout_hash` of a mandate whose SD-JWT sets
	// no other `_sd_alg`.
	Hash string
	// Audience and Nonce are what the agent binds the closed mandate to.
	Audience string
	Nonce    string
	Checkout Checkout
}

// Open starts a checkout in the store for items and signs it.
func (m *Seller) Open(ctx context.Context, who Caller, items []Pick) (*Issued, error) {
	cart, err := m.cfg.Backend.Open(ctx, who, items)
	if err != nil {
		return nil, err
	}
	return m.sign(who, cart)
}

// Issue signs the store's checkout id as it is now. An agent that changes the checkout has
// it signed again: the earlier signed checkout no longer matches the store, and the
// mandate made for it is refused.
func (m *Seller) Issue(ctx context.Context, who Caller, id string) (*Issued, error) {
	cart, err := m.cfg.Backend.Read(ctx, who, id)
	if err != nil {
		return nil, err
	}
	return m.sign(who, cart)
}

func (m *Seller) sign(who Caller, cart *Cart) (*Issued, error) {
	if cart.Canceled {
		return nil, fmt.Errorf("ap2: checkout %s is canceled", cart.ID)
	}
	if err := consistent(cart); err != nil {
		return nil, err
	}
	co := Checkout{
		ID: cart.ID, Merchant: &m.cfg.Identity, Status: ReadyForComplete, Currency: cart.Currency,
		Links:  append([]Link{}, m.cfg.Links...),
		Totals: []Total{{Type: "subtotal", Amount: cart.Total}, {Type: "total", Amount: cart.Total}},
	}
	for i, l := range cart.Lines {
		line := l.Price * l.Quantity
		co.LineItems = append(co.LineItems, LineItem{
			ID: fmt.Sprintf("line_%d", i+1), Item: Product{ID: l.ID, Title: l.Title, Price: l.Price}, Quantity: l.Quantity,
			Totals: []Total{{Type: "subtotal", Amount: line}, {Type: "total", Amount: line}},
		})
	}
	if m.cfg.Lifetime > 0 {
		co.ExpiresAt = m.now().Add(m.cfg.Lifetime).UTC().Format(time.RFC3339)
	}
	jwt, err := m.cfg.Signer.Sign(co)
	if err != nil {
		return nil, err
	}
	hash, _ := digest("", jwt)
	return &Issued{ID: cart.ID, JWT: jwt, Hash: hash, Audience: m.cfg.Audience, Nonce: m.nonce(who, cart.ID), Checkout: co}, nil
}

// nonce is the nonce an agent binds a closed Checkout Mandate for checkout id to: a MAC
// under the merchant's secret, so it needs no store and cannot be made for another checkout.
func (m *Seller) nonce(who Caller, id string) string {
	h := hmac.New(sha256.New, m.cfg.Secret)
	h.Write([]byte("ap2.nonce\x00" + who.Org + "\x00" + id))
	return b64.EncodeToString(h.Sum(nil))
}

// Completion is a request to complete a checkout.
type Completion struct {
	// ID is the checkout being completed.
	ID string
	// Checkout is the presentation of the Checkout Mandate: the SD-JWT chain, as sent.
	Checkout string
	// Payment is the presentation of the Payment Mandate that goes with it.
	Payment string
	// Token is the payment credential the credential provider released for the payment.
	Token string
}

// Outcome is what a completion answers with.
type Outcome struct {
	// Order is the order the checkout became; empty when it was refused.
	Order string
	// Reference is the hash of the closed Checkout Mandate that the receipt names.
	Reference string
	// Receipt is the signed Checkout Receipt: the merchant's answer to the agent, success or
	// error.
	Receipt string
	// Payment is the signed Payment Receipt. It is empty when none could be made: a success
	// receipt carries the payment network's confirmation id, which the store may not have.
	Payment string
	// Unsettled is set when the order was placed but the open mandate could not be marked as
	// spent. The host must reconcile it; the order stands.
	Unsettled error
}

// Complete completes a checkout against its Checkout Mandate and Payment Mandate. Before
// money moves it verifies, in this order:
//
//  1. the Checkout Mandate (Verifier.Checkout), bound to this merchant's audience and the
//     nonce it gave for this checkout, and the merchant's own signed checkout inside it;
//  2. that checkout against the store now: the same checkout, the merchant, the currency,
//     every line (item, quantity, catalog price) and the total;
//  3. the Payment Mandate (Verifier.Payment) bound to that checkout, naming this merchant as
//     payee, for the checkout's total in its currency, for immediate execution, by an
//     instrument the store honours;
//  4. that the open mandate, for an autonomous mandate, has completed no other checkout.
//
// It then charges the store, keyed by the closed mandate's hash, so a mandate presented again
// is the same order. A refusal is an *Error, an *Unsupported or a *Declined; and the Outcome
// then holds the signed error receipt the agent is answered with (specification.md,
// "Merchant": "the Merchant MUST return a Checkout Receipt JWT containing the appropriate
// error message"). Where the mandate cannot be read at all there is no reference to put in
// a receipt, and the Outcome is nil.
func (m *Seller) Complete(ctx context.Context, who Caller, in Completion) (*Outcome, error) {
	if in.ID == "" || in.Checkout == "" || in.Payment == "" || in.Token == "" {
		return nil, credential(ErrMalformed, "a completion needs the checkout id, both mandates and the payment token")
	}
	c := &completion{m: m, who: who, in: in}
	for _, side := range []struct {
		presented string
		ref       *string
	}{{in.Checkout, &c.ref}, {in.Payment, &c.payRef}} {
		p, perr := parsePresentation(side.presented)
		if perr != nil {
			return nil, perr
		}
		var err error
		if *side.ref, err = p.reference(); err != nil {
			return nil, &Error{Code: InvalidCredential, Err: err}
		}
	}
	if err := c.run(ctx); err != nil {
		return c.refused(err)
	}
	out := &Outcome{Order: c.paid.Order, Reference: c.ref, Unsettled: c.unsettled}
	var err error
	rec := CheckoutReceipt{Status: Success, Issuer: m.cfg.Issuer, IssuedAt: m.now().Unix(), Reference: c.ref, OrderID: c.paid.Order}
	if out.Receipt, err = SignCheckoutReceipt(m.cfg.Signer, rec); err != nil {
		return nil, err
	}
	if c.paid.Payment != "" && c.paid.PSP != "" && c.paid.Network != "" {
		pr := PaymentReceipt{Status: Success, Issuer: m.cfg.Issuer, IssuedAt: m.now().Unix(), Reference: c.payRef,
			PaymentID: c.paid.Payment, PSPConfirmationID: c.paid.PSP, NetworkConfirmationID: c.paid.Network}
		if out.Payment, err = SignPaymentReceipt(m.cfg.Signer, pr); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// completion is one completion in progress.
type completion struct {
	m   *Seller
	who Caller
	in  Completion
	// ref and payRef are the hashes of the two closed mandates, which the receipts name.
	ref, payRef string
	// paying is set once the Payment Mandate is being weighed: from then on a refusal is
	// answered with a Payment Receipt too.
	paying    bool
	paid      *Paid
	unsettled error
}

// refused answers a refusal with its signed error receipts, when it is one to answer. A
// store that could not answer decided nothing, and gets no receipt.
func (c *completion) refused(err error) (*Outcome, error) {
	m := c.m
	code, ok := CodeOf(err)
	if !ok {
		return nil, err
	}
	out := &Outcome{Reference: c.ref}
	var serr error
	rec := CheckoutReceipt{Status: Failure, Issuer: m.cfg.Issuer, IssuedAt: m.now().Unix(), Reference: c.ref, Error: string(code), ErrorDescription: err.Error()}
	if out.Receipt, serr = SignCheckoutReceipt(m.cfg.Signer, rec); serr != nil {
		return nil, errors.Join(err, serr)
	}
	if c.paying {
		var id [16]byte
		if _, serr = rand.Read(id[:]); serr != nil {
			return nil, errors.Join(err, serr)
		}
		pr := PaymentReceipt{Status: Failure, Issuer: m.cfg.Issuer, IssuedAt: m.now().Unix(), Reference: c.payRef,
			PaymentID: "pay_" + hex.EncodeToString(id[:]), Error: string(code), ErrorDescription: err.Error()}
		if out.Payment, serr = SignPaymentReceipt(m.cfg.Signer, pr); serr != nil {
			return nil, errors.Join(err, serr)
		}
	}
	return out, err
}

func (c *completion) run(ctx context.Context) error {
	m, in := c.m, c.in
	proof, err := m.v.Checkout(ctx, in.Checkout, Want{Audience: m.cfg.Audience, Nonce: m.nonce(c.who, in.ID)}, &m.cfg.Signer.Key.PublicKey)
	if err != nil {
		return err
	}
	co := proof.Checkout
	switch {
	case co.ID != in.ID:
		return mandate(ErrCheckout, "the mandate is for checkout %q, not %q", co.ID, in.ID)
	case co.Merchant == nil || !sameMerchant(*co.Merchant, m.cfg.Identity):
		return mandate(ErrCheckout, "the checkout is not this merchant's")
	case co.Status != ReadyForComplete:
		return mandate(ErrCheckout, "the checkout is %s, not %s", co.Status, ReadyForComplete)
	}
	cart, err := m.cfg.Backend.Read(ctx, c.who, in.ID)
	if err != nil {
		return err
	}
	if err := agrees(&co, cart); err != nil {
		return err
	}

	c.paying = true
	pp, err := m.v.Payment(ctx, in.Payment, PayWant{CheckoutJWT: proof.Mandate.CheckoutJWT, OpenCheckout: proof.OpenToken, Spent: m.cfg.Spent})
	if err != nil {
		return err
	}
	if err := m.payable(ctx, c.who, &pp.Mandate, cart); err != nil {
		return err
	}

	var lease Lease
	if proof.Open != nil {
		open, herr := proof.Open.tok.hash()
		if herr != nil {
			return &Error{Code: InvalidCredential, Err: herr}
		}
		verdict, l, lerr := m.cfg.Ledger.Begin(ctx, m.cfg.Identity.ID+"\x00"+open, c.ref)
		if lerr != nil {
			return fmt.Errorf("%w: %v", ErrUnavailable, lerr)
		}
		switch verdict {
		case Taken:
			return mandate(ErrSpent, "another closed mandate was completed under this open mandate")
		case Busy:
			return mandate(ErrBusy, "a completion under this open mandate is in progress")
		}
		lease = l
	}
	paid, err := m.cfg.Backend.Pay(ctx, c.who, in.ID, "ap2:"+c.ref, Payment{Token: in.Token, Instrument: pp.Mandate.Instrument})
	if err != nil {
		if lease != nil {
			c.unsettled = lease.Drop(ctx)
		}
		return err
	}
	if lease != nil {
		c.unsettled = lease.Keep(ctx)
	}
	c.paid = paid
	return nil
}

// agrees says the signed checkout is the store's checkout now, line for line.
func agrees(co *Checkout, cart *Cart) *Error {
	total, _ := co.Total("total")
	switch {
	case cart.Canceled:
		return mandate(ErrStale, "the checkout was canceled")
	case !strings.EqualFold(co.Currency, cart.Currency):
		return mandate(ErrStale, "the checkout is in %s, the store's in %s", co.Currency, cart.Currency)
	case len(co.LineItems) != len(cart.Lines):
		return mandate(ErrStale, "the checkout has %d lines, the store's %d", len(co.LineItems), len(cart.Lines))
	case total != cart.Total:
		return mandate(ErrStale, "the checkout totals %d, the store's %d", total, cart.Total)
	}
	for i, l := range co.LineItems {
		c := cart.Lines[i]
		if l.Item.ID != c.ID || l.Quantity != c.Quantity || l.Item.Price != c.Price {
			return mandate(ErrStale, "line %d is %d x %s at %d, the store's %d x %s at %d", i+1, l.Quantity, l.Item.ID, l.Item.Price, c.Quantity, c.ID, c.Price)
		}
	}
	return consistent(cart)
}

// consistent says the store's lines, priced from its catalog now, come to the total it
// charges. They do not when the catalog has moved since the checkout was opened.
func consistent(cart *Cart) *Error {
	var sum int64
	for _, l := range cart.Lines {
		sum += l.Price * l.Quantity
	}
	if sum != cart.Total {
		return mandate(ErrStale, "the store's lines come to %d, its total is %d", sum, cart.Total)
	}
	return nil
}

// payable says a verified Payment Mandate pays this cart, to this merchant, now, by a
// means the store honours.
func (m *Seller) payable(ctx context.Context, who Caller, pm *PaymentMandate, cart *Cart) error {
	switch {
	case !sameMerchant(pm.Payee, m.cfg.Identity):
		return mandate(ErrPayment, "the payee is not this merchant")
	case pm.PaymentAmount.Amount != cart.Total || !strings.EqualFold(pm.PaymentAmount.Currency, cart.Currency):
		return mandate(ErrPayment, "the mandate pays %d %s, the checkout is %d %s", pm.PaymentAmount.Amount, pm.PaymentAmount.Currency, cart.Total, cart.Currency)
	}
	if pm.ExecutionDate != "" {
		at, err := instant(pm.ExecutionDate)
		if err != nil {
			return mandate(ErrMandate, "execution_date %q: %v", pm.ExecutionDate, err)
		}
		if at.After(m.now()) {
			return mandate(ErrPayment, "the payment is set for %s; the store charges at once", pm.ExecutionDate)
		}
	}
	return m.cfg.Backend.Honours(ctx, who, pm.Instrument)
}
