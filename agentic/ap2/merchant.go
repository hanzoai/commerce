// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ap2

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Config is a merchant. Every field is the host's: nothing has a default the spec does not state.
type Config struct {
	Backend Backend
	// Trust names the keys of the Agent Providers whose mandates the merchant accepts.
	Trust Trust
	// Signer signs the checkouts and receipts the merchant issues.
	Signer Signer
	// Identity is the merchant the checkouts name, the payee a Payment Mandate must name, the `iss`
	// of its receipts and the `aud` an agent binds a closed mandate to.
	Identity Merchant
	// Secret keys the nonces the merchant gives agents; at least a hash's length.
	Secret []byte
	// Links are the checkout's links (terms of service, privacy policy, ...).
	Links []Link
	// Lifetime is how long a signed checkout stays completable; zero signs no expiry.
	Lifetime time.Duration
	// Skew is the clock difference a validity period tolerates; the spec names none.
	Skew time.Duration
	// Now is the clock; time.Now when nil.
	Now func() time.Time
	// Ledger records the open mandates a checkout was completed under.
	Ledger Ledger
	// Spent reports what an open Payment Mandate has spent; without it a mandate with a budget or
	// a recurrence is refused as unresolved_constraint.
	Spent Spent
	// Stale is how long a claim begun and never settled is presumed abandoned, when the store shows
	// its checkout unpaid and no payment under way; zero never releases one.
	Stale time.Duration
}

// Seller plays the Merchant role of AP2, and the Merchant Payment Processor role over the store's
// payment: it signs the checkouts it offers and completes one only against a Checkout Mandate and
// a Payment Mandate that verify and agree with the store.
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
	case len(cfg.Secret) < sha256.Size:
		return nil, errors.New("ap2: the nonce Secret is at least a hash long")
	case cfg.Lifetime < 0 || cfg.Skew < 0 || cfg.Stale < 0:
		return nil, errors.New("ap2: Lifetime, Skew and Stale are not negative")
	}
	if err := cfg.Signer.check(); err != nil {
		return nil, err
	}
	return &Seller{cfg: cfg, v: &Verifier{Trust: cfg.Trust, Now: cfg.Now, Skew: cfg.Skew}}, nil
}

func (m *Seller) now() time.Time { return m.v.now() }

// JWK is the key agents verify the merchant's checkouts and receipts with.
func (m *Seller) JWK() JWK { return NewJWK(&m.cfg.Signer.Key.PublicKey, m.cfg.Signer.Kid) }

// Issued is a signed checkout and what an agent binds its closed Checkout Mandate to.
type Issued struct {
	// JWT is the merchant-signed checkout: the mandate's `checkout_jwt`.
	JWT      string
	Audience string
	Nonce    string
}

// Issue signs the store's checkout id as it is now. A checkout the store has since changed no
// longer matches it, and a mandate made for the signed one is refused.
func (m *Seller) Issue(ctx context.Context, who Caller, id string) (*Issued, error) {
	cart, err := m.cfg.Backend.Read(ctx, who, id)
	if err != nil {
		return nil, err
	}
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
		amount := l.Price * l.Quantity
		co.LineItems = append(co.LineItems, LineItem{
			ID: fmt.Sprintf("line_%d", i+1), Item: Product{ID: l.ID, Title: l.Title, Price: l.Price}, Quantity: l.Quantity,
			Totals: []Total{{Type: "subtotal", Amount: amount}, {Type: "total", Amount: amount}},
		})
	}
	if m.cfg.Lifetime > 0 {
		co.ExpiresAt = m.now().Add(m.cfg.Lifetime).UTC().Format(time.RFC3339)
	}
	jwt, err := m.cfg.Signer.Sign(co)
	if err != nil {
		return nil, err
	}
	return &Issued{JWT: jwt, Audience: m.cfg.Identity.ID, Nonce: m.nonce(who, cart.ID)}, nil
}

// nonce is the nonce an agent binds a closed Checkout Mandate for checkout id to: a MAC under the
// merchant's secret, so it needs no store and cannot be made for another checkout.
func (m *Seller) nonce(who Caller, id string) string {
	h := hmac.New(sha256.New, m.cfg.Secret)
	h.Write([]byte("ap2.nonce\x00" + who.Org + "\x00" + id))
	return b64.EncodeToString(h.Sum(nil))
}

// Completion is a request to complete a checkout: its id, the presentations of the Checkout
// Mandate and the Payment Mandate as sent, and the payment credential the credential provider
// released.
type Completion struct {
	ID       string
	Checkout string
	Payment  string
	Token    string
}

// Outcome is the answer to a completion.
type Outcome struct {
	// Order is the order the checkout became; empty when it was refused.
	Order string
	// Receipt is the signed Checkout Receipt, success or error.
	Receipt string
	// Payment is the signed Payment Receipt. A success receipt carries the network's confirmation
	// id, so it is empty when the store did not supply one.
	Payment string
	// Unsettled is set when the order was placed but the ledger could not record the open mandate
	// as spent. The host reconciles it; the order stands.
	Unsettled error
}

// Complete completes a checkout against its Checkout Mandate and Payment Mandate. Before money
// moves it verifies the Checkout Mandate (Verifier.Checkout) for this merchant's audience and the
// nonce it gave for this checkout; that checkout against the store now (the same checkout, the
// currency, every line and the total); the Payment Mandate (Verifier.Payment) bound to it, naming
// this merchant as payee, for the total, to be paid now, by an instrument the store honours; and
// that an open mandate has completed no other checkout. It then charges the store, keyed by the
// closed mandate's hash, so a mandate presented again is the same order.
//
// A refusal is an *Error, *Unsupported or *Declined, and the Outcome holds the signed error
// receipt the agent is answered with (specification.md, "Merchant"). A mandate that cannot be read
// has no reference for a receipt: the Outcome is nil. A store that could not answer decided
// nothing: no receipt.
func (m *Seller) Complete(ctx context.Context, who Caller, in Completion) (*Outcome, error) {
	if in.ID == "" || in.Checkout == "" || in.Payment == "" || in.Token == "" {
		return nil, credential(ErrMalformed, "a completion needs the checkout id, both mandates and the payment token")
	}
	c := &completion{m: m, who: who, in: in}
	for _, side := range []struct {
		presented string
		ref       *string
	}{{in.Checkout, &c.ref}, {in.Payment, &c.payRef}} {
		p, err := parsePresentation(side.presented)
		if err != nil {
			return nil, err
		}
		*side.ref = p.reference()
	}
	if err := c.run(ctx); err != nil {
		return c.refused(err)
	}
	out := &Outcome{Order: c.paid.Order, Unsettled: c.unsettled}
	var err error
	if out.Receipt, err = m.cfg.Signer.receipt(&CheckoutReceipt{Status: Success, Issuer: m.cfg.Identity.ID, IssuedAt: m.now().Unix(), Reference: c.ref, OrderID: c.paid.Order}); err != nil {
		return nil, err
	}
	if p := c.paid; p.Payment != "" && p.PSP != "" && p.Network != "" {
		out.Payment, err = m.cfg.Signer.receipt(&PaymentReceipt{Status: Success, Issuer: m.cfg.Identity.ID, IssuedAt: m.now().Unix(), Reference: c.payRef,
			PaymentID: p.Payment, PSPConfirmationID: p.PSP, NetworkConfirmationID: p.Network})
	}
	return out, err
}

// completion is one completion in progress.
type completion struct {
	m   *Seller
	who Caller
	in  Completion
	// ref and payRef are the hashes of the two closed mandates, which the receipts name.
	ref, payRef string
	// paying is set once the Payment Mandate is weighed: a refusal then has a Payment Receipt too.
	paying bool
	// claimed says this completion holds the claim on the open mandate.
	claimed   bool
	paid      *Paid
	unsettled error
}

// refused answers a refusal with its signed error receipts.
func (c *completion) refused(err error) (*Outcome, error) {
	code, ok := CodeOf(err)
	if !ok {
		return nil, err
	}
	m := c.m
	out := &Outcome{}
	var serr error
	if out.Receipt, serr = m.cfg.Signer.receipt(&CheckoutReceipt{Status: Failure, Issuer: m.cfg.Identity.ID, IssuedAt: m.now().Unix(), Reference: c.ref, Error: string(code), ErrorDescription: err.Error()}); serr != nil {
		return nil, errors.Join(err, serr)
	}
	if c.paying {
		if out.Payment, serr = m.cfg.Signer.receipt(&PaymentReceipt{Status: Failure, Issuer: m.cfg.Identity.ID, IssuedAt: m.now().Unix(), Reference: c.payRef,
			PaymentID: c.payRef, Error: string(code), ErrorDescription: err.Error()}); serr != nil {
			return nil, errors.Join(err, serr)
		}
	}
	return out, err
}

func (c *completion) run(ctx context.Context) error {
	m, in := c.m, c.in
	proof, err := m.v.Checkout(ctx, in.Checkout, Want{Audience: m.cfg.Identity.ID, Nonce: m.nonce(c.who, in.ID)}, &m.cfg.Signer.Key.PublicKey)
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
	if err := m.payable(ctx, c.who, &pp.Mandate, pp.Pinned, cart); err != nil {
		return err
	}

	key := m.cfg.Identity.ID + "\x00" + proof.openID
	if proof.openID != "" {
		switch verdict, err := c.claim(ctx, key); {
		case err != nil:
			return err
		case verdict == Taken:
			return mandate(ErrSpent, "another closed mandate was completed under this open mandate")
		case verdict == Busy:
			return mandate(ErrBusy, "a completion under this open mandate has not ended")
		}
	}
	paid, err := m.cfg.Backend.Pay(ctx, c.who, in.ID, "ap2:"+c.ref, Payment{Token: in.Token, Instrument: pp.Mandate.Instrument, Cart: *cart})
	if err != nil {
		// Only a refusal frees the open mandate: a charge of unknown outcome may have moved money.
		if _, refused := CodeOf(err); refused && c.claimed {
			c.unsettled = m.cfg.Ledger.Settle(ctx, key, c.ref, false)
		}
		return err
	}
	if c.claimed {
		c.unsettled = m.cfg.Ledger.Settle(ctx, key, c.ref, true)
	}
	c.paid = paid
	return nil
}

// claim takes the open mandate for this closed mandate. A claim left begun and not ended is
// reconciled against the store: its checkout paid settles it as taken; payment under way, or a
// claim younger than Stale, keeps it; otherwise nothing was charged and it is given up.
func (c *completion) claim(ctx context.Context, key string) (Verdict, error) {
	m := c.m
	for range 2 {
		verdict, held, err := m.cfg.Ledger.Begin(ctx, key, c.ref, c.in.ID)
		if err != nil {
			return 0, fmt.Errorf("%w: the ledger of open mandates", ErrUnavailable)
		}
		if verdict != Busy {
			c.claimed = verdict == Fresh
			return verdict, nil
		}
		cart, err := m.cfg.Backend.Read(ctx, c.who, held.Checkout)
		switch {
		case err != nil || cart.Pending:
			return Busy, nil
		case cart.Paid:
			// The open mandate is spent whether or not the ledger can be told now.
			_ = m.cfg.Ledger.Settle(ctx, key, held.Ref, true)
			if held.Ref == c.ref {
				return Replay, nil
			}
			return Taken, nil
		case m.cfg.Stale > 0 && m.now().Sub(held.At) >= m.cfg.Stale:
			if m.cfg.Ledger.Settle(ctx, key, held.Ref, false) != nil {
				return Busy, nil
			}
			continue
		}
		return Busy, nil
	}
	return Busy, nil
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
		if c := cart.Lines[i]; l.Item.ID != c.ID || l.Quantity != c.Quantity || l.Item.Price != c.Price {
			return mandate(ErrStale, "line %d is %d x %s at %d, the store's %d x %s at %d", i+1, l.Quantity, l.Item.ID, l.Item.Price, c.Quantity, c.ID, c.Price)
		}
	}
	return consistent(cart)
}

// consistent says the store's lines, priced from its catalog now, come to the total it charges:
// they do not once the catalog has moved since the checkout was opened.
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

// payable says a verified Payment Mandate pays this cart, to this merchant, now, by an instrument
// the store honours.
func (m *Seller) payable(ctx context.Context, who Caller, pm *PaymentMandate, pinned bool, cart *Cart) error {
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
	return m.cfg.Backend.Honours(ctx, who, pm.Instrument, pinned)
}
