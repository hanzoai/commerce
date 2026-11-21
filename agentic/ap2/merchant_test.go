// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ap2

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// store is a Backend for the tests: a catalog, carts that belong to a buyer, and a charge that
// is idempotent in its key as a real store's is. It counts what it charges, which is what a
// refusal must leave at zero.
type store struct {
	mu      sync.Mutex
	prices  map[string]int64
	carts   map[string]*Cart
	owner   map[string]string
	orders  map[string]string // checkout -> idempotency key of the charge that paid it
	charges int
	decline error
	network string
	seq     int
	// pinned is what Honours was last told; refusePinned makes the store refuse such mandates.
	pinned       bool
	refusePinned bool
	// lost makes a charge go through and its answer not arrive; pending marks checkouts whose
	// payment is under way.
	lost    bool
	pending map[string]bool
	// instrument is the instrument the last charge was told to use.
	instrument string
}

func newStore() *store {
	return &store{prices: map[string]int64{"credit-500": 500, "credit-1000": 1000}, carts: map[string]*Cart{}, owner: map[string]string{}, orders: map[string]string{}, pending: map[string]bool{}, network: "net-1"}
}

// open starts a checkout of one item for a buyer, as the store's own surface does.
func (s *store) open(who Caller, item string, qty int64) (*Cart, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	price, ok := s.prices[item]
	if !ok {
		return nil, ErrNoItem
	}
	s.seq++
	c := &Cart{ID: fmt.Sprintf("chk_%d", s.seq), Currency: "USD", Lines: []Line{{ID: item, Title: item, Price: price, Quantity: qty}}, Total: price * qty}
	s.carts[c.ID], s.owner[c.ID] = c, who.Subject
	return c, nil
}

// Read prices the lines from the catalog as it is now.
func (s *store) Read(_ context.Context, who Caller, id string) (*Cart, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.owner[id] != who.Subject {
		return nil, ErrNotFound
	}
	c, ok := s.priced(id)
	if !ok {
		return nil, ErrNotFound
	}
	return &c, nil
}

// priced is the cart with its lines priced from the catalog as it is now; the caller holds s.mu.
func (s *store) priced(id string) (Cart, bool) {
	c, ok := s.carts[id]
	if !ok {
		return Cart{}, false
	}
	out := *c
	out.Lines = nil
	for _, l := range c.Lines {
		l.Price = s.prices[l.ID]
		out.Lines = append(out.Lines, l)
	}
	_, out.Paid = s.orders[id]
	out.Pending = s.pending[id]
	return out, true
}

func sameCart(a, b Cart) bool {
	if a.Currency != b.Currency || a.Total != b.Total || len(a.Lines) != len(b.Lines) {
		return false
	}
	for i := range a.Lines {
		if a.Lines[i] != b.Lines[i] {
			return false
		}
	}
	return true
}

func (s *store) Pay(_ context.Context, who Caller, id, key string, pay Payment) (*Paid, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.carts[id]; !ok || s.owner[id] != who.Subject {
		return nil, ErrNotFound
	}
	if cur, _ := s.priced(id); !sameCart(pay.Cart, cur) {
		return nil, mandate(ErrStale, "the checkout is no longer the one that was verified")
	}
	if k, paid := s.orders[id]; paid {
		if k != key {
			return nil, &Declined{Code: "already_completed", Message: "this checkout was completed under another idempotency key"}
		}
		return &Paid{Order: "ord_" + id, Payment: "pay_" + id, PSP: "psp_" + id, Network: s.network}, nil
	}
	if s.decline != nil {
		return nil, s.decline
	}
	s.charges++
	s.instrument = pay.Instrument.ID
	s.orders[id] = key
	if s.lost {
		return nil, ErrUnavailable
	}
	return &Paid{Order: "ord_" + id, Payment: "pay_" + id, PSP: "psp_" + id, Network: s.network}, nil
}

func (s *store) Honours(_ context.Context, _ Caller, in Instrument, pinned bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pinned = pinned
	switch {
	case in.Type != "card":
		return &Unsupported{Instrument: in, Why: "only cards are taken"}
	case pinned && s.refusePinned:
		return &Unsupported{Instrument: in, Why: "the store cannot tie a token to an instrument"}
	}
	return nil
}

func (s *store) count() int { s.mu.Lock(); defer s.mu.Unlock(); return s.charges }

// shop is a merchant over a store, with the world that signs for it.
type shop struct {
	*world
	m     *Seller
	store *store
	who   Caller
}

func newShop(t *testing.T, mutate ...func(*Config)) *shop {
	t.Helper()
	w := newWorld(t)
	st := newStore()
	cfg := Config{
		Backend: st, Trust: w.v.Trust, Signer: w.merchant,
		Identity: Merchant{ID: "merchant_1", Name: "Demo Merchant", Website: "https://demo-merchant.example"},
		Secret:   []byte("0123456789abcdef0123456789abcdef"), Now: w.v.Now, Ledger: &memory{now: w.v.Now},
	}
	for _, f := range mutate {
		f(&cfg)
	}
	m, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return &shop{world: w, m: m, store: st, who: Caller{Org: "demo", Subject: "buyer-1"}}
}

// buy is an agent buying one credit-500: the merchant signs the checkout, the agent has the
// mandates made for it (autonomously) and presents them.
type purchase struct {
	issued   *signed
	checkout *autonomous
	payment  *autonomous
	token    string
}

// signed is a checkout the store opened and the merchant signed.
type signed struct {
	*Issued
	ID   string
	Hash string
}

// issue has the store open a checkout of credit-500 for the buyer and the merchant sign it.
func (s *shop) issue(t *testing.T) *signed {
	t.Helper()
	cart, err := s.store.open(s.who, "credit-500", 1)
	if err != nil {
		t.Fatal(err)
	}
	iss, err := s.m.Issue(context.Background(), s.who, cart.ID)
	if err != nil {
		t.Fatal(err)
	}
	return &signed{Issued: iss, ID: cart.ID, Hash: digest(iss.JWT)}
}

func (s *shop) buy(t *testing.T) *purchase {
	t.Helper()
	iss := s.issue(t)
	s.world.nonce = iss.Nonce
	s.world.aud = iss.Audience
	cm := s.checkoutMandate(iss.JWT, iss.Hash)
	pm := s.paymentMandate(iss.Hash, 500, cm.openText())
	return &purchase{issued: iss, checkout: cm, payment: pm, token: "spt_test"}
}

func (s *shop) complete(p *purchase) (*Outcome, error) {
	return s.m.Complete(context.Background(), s.who, Completion{ID: p.issued.ID, Checkout: p.checkout.text(), Payment: p.payment.text(), Token: p.token})
}

func TestACheckoutIsSignedAndCompleted(t *testing.T) {
	s := newShop(t)
	p := s.buy(t)

	// What the merchant signs is a UCP checkout the agent can verify under the merchant's key.
	j, err := parseJWS(p.issued.JWT)
	if err != nil || j.verify(&s.merchant.Key.PublicKey) != nil {
		t.Fatalf("the checkout does not verify: %v", err)
	}
	co, err := decodeCheckout(j.payload)
	if err != nil || co.ID != p.issued.ID || co.Status != ReadyForComplete || co.Currency != "USD" ||
		len(co.LineItems) != 1 || co.LineItems[0].Item.ID != "credit-500" || co.LineItems[0].Item.Price != 500 {
		t.Fatalf("checkout = %+v %v", co, err)
	}
	if total, _ := co.Total("total"); total != 500 {
		t.Fatalf("total %d", total)
	}
	if k := s.m.JWK(); k.Kid != "merchant-key-1" {
		t.Fatalf("jwk %+v", k)
	} else if pub, err := k.Key(); err != nil || !pub.Equal(&s.merchant.Key.PublicKey) {
		t.Fatalf("the published key is not the signing key: %v", err)
	}

	out, err := s.complete(p)
	if err != nil {
		t.Fatal(err)
	}
	if out.Order != "ord_"+p.issued.ID || s.store.count() != 1 || out.Unsettled != nil {
		t.Fatalf("outcome = %+v, charges %d", out, s.store.count())
	}
	pub := &s.merchant.Key.PublicKey
	r, err := ParseCheckoutReceipt(out.Receipt, pub, p.checkout.text())
	if err != nil || r.Status != Success || r.OrderID != out.Order || r.Issuer != "merchant_1" {
		t.Fatalf("receipt = %+v %v", r, err)
	}
	pr, err := ParsePaymentReceipt(out.Payment, pub, p.payment.text())
	if err != nil || pr.Status != Success || pr.PSPConfirmationID != "psp_"+p.issued.ID || pr.NetworkConfirmationID != "net-1" {
		t.Fatalf("payment receipt = %+v %v", pr, err)
	}
}

// The same mandates presented again are the same order, not a second charge.
func TestAMandatePresentedAgainIsTheSameOrder(t *testing.T) {
	s := newShop(t)
	p := s.buy(t)
	first, err := s.complete(p)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.complete(p)
	if err != nil || again.Order != first.Order || s.store.count() != 1 {
		t.Fatalf("again = %+v %v, charges %d", again, err, s.store.count())
	}
}

// An open mandate buys one checkout: a second closed mandate under it is refused.
func TestAnOpenMandateBuysOneCheckout(t *testing.T) {
	s := newShop(t)
	p := s.buy(t)
	if _, err := s.complete(p); err != nil {
		t.Fatal(err)
	}
	// the agent signs a second closed mandate under the same open one, for the same checkout
	p.checkout.closed.claims["iat"] = s.unix(-30 * time.Second)
	p.checkout.resign()
	out, err := s.complete(p)
	refusedWith(t, err, InvalidMandate, ErrSpent)
	if out == nil || out.Order != "" || s.store.count() != 1 {
		t.Fatalf("outcome %+v charges %d", out, s.store.count())
	}
	r, rerr := ParseCheckoutReceipt(out.Receipt, &s.merchant.Key.PublicKey, "")
	if rerr != nil || r.Status != Failure || r.Error != string(InvalidMandate) {
		t.Fatalf("receipt = %+v %v", r, rerr)
	}
}

// A refusal comes with the signed error receipt the spec requires, and moves no money.
func TestRefusalsAreAnsweredWithAnErrorReceiptAndMoveNoMoney(t *testing.T) {
	other := newParty(t, "other")
	for _, tc := range []struct {
		name   string
		change func(s *shop, p *purchase) Completion
		code   Code
		reason error
		pay    bool // a payment receipt too
	}{
		{"a closed mandate for another nonce", func(s *shop, p *purchase) Completion {
			p.checkout.closed.claims["nonce"] = "another-checkouts-nonce"
			return Completion{ID: p.issued.ID, Checkout: p.checkout.text(), Payment: p.payment.text(), Token: p.token}
		}, InvalidCredential, ErrNonce, false},
		{"a closed mandate for another merchant", func(s *shop, p *purchase) Completion {
			p.checkout.closed.claims["aud"] = "another-merchant"
			return Completion{ID: p.issued.ID, Checkout: p.checkout.text(), Payment: p.payment.text(), Token: p.token}
		}, InvalidCredential, ErrAudience, false},
		{"a closed mandate signed by another agent", func(s *shop, p *purchase) Completion {
			p.checkout.closed.key = other.key
			return Completion{ID: p.issued.ID, Checkout: p.checkout.text(), Payment: p.payment.text(), Token: p.token}
		}, InvalidCredential, ErrSignature, false},
		{"a completion of another checkout", func(s *shop, p *purchase) Completion {
			return Completion{ID: "chk_other", Checkout: p.checkout.text(), Payment: p.payment.text(), Token: p.token}
		}, InvalidCredential, ErrNonce, false},
		{"an expired mandate", func(s *shop, p *purchase) Completion {
			p.checkout.closedItem["exp"] = s.unix(-time.Second)
			return Completion{ID: p.issued.ID, Checkout: p.checkout.text(), Payment: p.payment.text(), Token: p.token}
		}, InvalidCredential, ErrExpired, false},
		{"a checkout the store has since repriced", func(s *shop, p *purchase) Completion {
			s.store.prices["credit-500"] = 600
			return Completion{ID: p.issued.ID, Checkout: p.checkout.text(), Payment: p.payment.text(), Token: p.token}
		}, InvalidMandate, ErrStale, false},
		{"a checkout the store has since canceled", func(s *shop, p *purchase) Completion {
			s.store.carts[p.issued.ID].Canceled = true
			return Completion{ID: p.issued.ID, Checkout: p.checkout.text(), Payment: p.payment.text(), Token: p.token}
		}, InvalidMandate, ErrStale, false},
		{"a checkout the store has since changed to another item", func(s *shop, p *purchase) Completion {
			s.store.carts[p.issued.ID].Lines[0].ID = "credit-1000"
			s.store.carts[p.issued.ID].Total = 1000
			return Completion{ID: p.issued.ID, Checkout: p.checkout.text(), Payment: p.payment.text(), Token: p.token}
		}, InvalidMandate, ErrStale, false},
		{"a checkout the store has since changed in quantity", func(s *shop, p *purchase) Completion {
			s.store.carts[p.issued.ID].Lines[0].Quantity = 2
			s.store.carts[p.issued.ID].Total = 1000
			return Completion{ID: p.issued.ID, Checkout: p.checkout.text(), Payment: p.payment.text(), Token: p.token}
		}, InvalidMandate, ErrStale, false},
		{"a checkout the store has since changed in currency", func(s *shop, p *purchase) Completion {
			s.store.carts[p.issued.ID].Currency = "EUR"
			return Completion{ID: p.issued.ID, Checkout: p.checkout.text(), Payment: p.payment.text(), Token: p.token}
		}, InvalidMandate, ErrStale, false},
		{"a checkout the store has since changed in total", func(s *shop, p *purchase) Completion {
			s.store.carts[p.issued.ID].Total = 499
			return Completion{ID: p.issued.ID, Checkout: p.checkout.text(), Payment: p.payment.text(), Token: p.token}
		}, InvalidMandate, ErrStale, false},
		{"a checkout of another buyer", func(s *shop, p *purchase) Completion {
			s.store.owner[p.issued.ID] = "someone-else"
			return Completion{ID: p.issued.ID, Checkout: p.checkout.text(), Payment: p.payment.text(), Token: p.token}
		}, InvalidMandate, nil, false},
		{"a payment for less than the checkout", func(s *shop, p *purchase) Completion {
			p.payment.closedItem["payment_amount"] = map[string]any{"amount": 499, "currency": "USD"}
			return Completion{ID: p.issued.ID, Checkout: p.checkout.text(), Payment: p.payment.text(), Token: p.token}
		}, InvalidMandate, ErrPayment, true},
		{"a payment in another currency", func(s *shop, p *purchase) Completion {
			p.payment.closedItem["payment_amount"] = map[string]any{"amount": 500, "currency": "EUR"}
			return Completion{ID: p.issued.ID, Checkout: p.checkout.text(), Payment: p.payment.text(), Token: p.token}
		}, InvalidMandate, ErrPayment, true},
		{"a payment to another payee", func(s *shop, p *purchase) Completion {
			p.payment.closedItem["payee"] = map[string]any{"id": "merchant_2", "name": "Another"}
			return Completion{ID: p.issued.ID, Checkout: p.checkout.text(), Payment: p.payment.text(), Token: p.token}
		}, InvalidMandate, ErrPayment, true},
		{"a payment for another checkout", func(s *shop, p *purchase) Completion {
			p.payment.closedItem["transaction_id"] = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
			return Completion{ID: p.issued.ID, Checkout: p.checkout.text(), Payment: p.payment.text(), Token: p.token}
		}, InvalidMandate, ErrHash, true},
		{"a payment set for a later date", func(s *shop, p *purchase) Completion {
			p.payment.closedItem["execution_date"] = "2099-01-01"
			return Completion{ID: p.issued.ID, Checkout: p.checkout.text(), Payment: p.payment.text(), Token: p.token}
		}, InvalidMandate, ErrPayment, true},
		{"a payment mandate from an untrusted key", func(s *shop, p *purchase) Completion {
			p.payment.open.key = other.key
			return Completion{ID: p.issued.ID, Checkout: p.checkout.text(), Payment: p.payment.text(), Token: p.token}
		}, InvalidCredential, ErrSignature, true},
		{"a payment mandate over an amount it may not pay", func(s *shop, p *purchase) Completion {
			p.payment.openItem["constraints"] = append(p.payment.openItem["constraints"].([]any), map[string]any{"type": "payment.amount_range", "currency": "USD", "max": 499})
			return Completion{ID: p.issued.ID, Checkout: p.checkout.text(), Payment: p.payment.text(), Token: p.token}
		}, UnresolvedConstraint, ErrConstraint, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newShop(t)
			p := s.buy(t)
			in := tc.change(s, p)
			out, err := s.m.Complete(context.Background(), s.who, in)
			var e *Error
			if tc.reason != nil {
				refusedWith(t, err, tc.code, tc.reason)
			} else if code, ok := CodeOf(err); !ok || code != tc.code || errors.As(err, &e) {
				t.Fatalf("err = %v", err)
			}
			if out == nil || out.Order != "" || out.Receipt == "" || s.store.count() != 0 {
				t.Fatalf("outcome = %+v, charges %d", out, s.store.count())
			}
			r, rerr := ParseCheckoutReceipt(out.Receipt, &s.merchant.Key.PublicKey, "")
			if rerr != nil || r.Status != Failure || r.Error != string(tc.code) || r.ErrorDescription == "" || r.OrderID != "" {
				t.Fatalf("receipt = %+v %v", r, rerr)
			}
			if (out.Payment != "") != tc.pay {
				t.Fatalf("payment receipt %q, want one: %v", out.Payment, tc.pay)
			}
			if tc.pay {
				pr, perr := ParsePaymentReceipt(out.Payment, &s.merchant.Key.PublicKey, "")
				if perr != nil || pr.Status != Failure || pr.Error != string(tc.code) {
					t.Fatalf("payment receipt = %+v %v", pr, perr)
				}
			}
		})
	}
}

func TestAPaymentTheStoreCannotHonourIsRefusedWithATypedError(t *testing.T) {
	s := newShop(t)
	p := s.buy(t)
	p.payment.closedItem["payment_instrument"] = map[string]any{"id": "upi_1", "type": "UPI", "description": "user****@bank"}
	out, err := s.complete(p)
	var u *Unsupported
	if !errors.As(err, &u) || u.Instrument.Type != "UPI" {
		t.Fatalf("err = %v", err)
	}
	if code, _ := CodeOf(err); code != InvalidMandate {
		t.Fatalf("code %s", code)
	}
	if out == nil || out.Receipt == "" || out.Payment == "" || s.store.count() != 0 {
		t.Fatalf("outcome = %+v charges %d", out, s.store.count())
	}
}

// A payment the issuer declines is answered with a receipt, and frees the open mandate.
func TestADeclinedPaymentFreesTheOpenMandate(t *testing.T) {
	s := newShop(t)
	p := s.buy(t)
	s.store.decline = &Declined{Code: "declined", Message: "Your card was declined."}
	out, err := s.complete(p)
	var d *Declined
	if !errors.As(err, &d) || out == nil {
		t.Fatalf("err = %v out = %+v", err, out)
	}
	r, rerr := ParseCheckoutReceipt(out.Receipt, &s.merchant.Key.PublicKey, "")
	if rerr != nil || r.Error != string(PaymentDeclined) || r.ErrorDescription == "" {
		t.Fatalf("receipt = %+v %v", r, rerr)
	}
	if pr, perr := ParsePaymentReceipt(out.Payment, &s.merchant.Key.PublicKey, ""); perr != nil || pr.Status != Failure || pr.PaymentID == "" {
		t.Fatalf("payment receipt = %+v %v", pr, perr)
	}
	// the agent tries again: the open mandate was not spent
	s.store.decline = nil
	if out, err = s.complete(p); err != nil || out.Order == "" {
		t.Fatalf("retry = %+v %v", out, err)
	}
}

// A store that cannot answer has decided nothing: no receipt. But a charge whose outcome is not
// known may have moved money, so the open mandate stays claimed: the same mandates, tried again
// at once, are told the completion has not ended, and only a claim that is stale, on a checkout
// the store shows unpaid and not being paid, is given up.
func TestAnUnavailableStoreDecidesNothingAndKeepsTheClaim(t *testing.T) {
	s := newShop(t, func(c *Config) { c.Stale = 30 * time.Minute })
	var open string
	p := s.buyUnder(t, &open)
	s.store.decline = ErrUnavailable
	out, err := s.complete(p)
	if !errors.Is(err, ErrUnavailable) || out != nil {
		t.Fatalf("out = %+v err = %v", out, err)
	}
	s.store.decline = nil
	if _, err = s.complete(p); !errors.Is(err, ErrBusy) {
		t.Fatalf("at once: %v", err)
	}
	if _, err = s.complete(s.buyUnder(t, &open)); !errors.Is(err, ErrBusy) {
		t.Fatalf("another checkout under the open mandate, at once: %v", err)
	}
	later := func() time.Time { return s.now.Add(31 * time.Minute) }
	s.v.Now, s.m.v.Now = later, later
	// A payment under way is never given up, however old the claim.
	s.store.pending[p.issued.ID] = true
	if _, err = s.complete(p); !errors.Is(err, ErrBusy) {
		t.Fatalf("a payment under way: %v", err)
	}
	s.store.pending[p.issued.ID] = false
	if out, err = s.complete(p); err != nil || out.Order == "" || s.store.count() != 1 {
		t.Fatalf("a stale claim on an unpaid checkout: %+v %v, charges %d", out, err, s.store.count())
	}
}

// Without a Stale a claim is never released by the clock: the host reconciles.
func TestAClaimIsNotReleasedByTheClockWithoutAStale(t *testing.T) {
	s := newShop(t)
	p := s.buy(t)
	s.store.decline = ErrUnavailable
	s.complete(p)
	s.store.decline = nil
	later := func() time.Time { return s.now.Add(30 * time.Minute) }
	s.v.Now, s.m.v.Now = later, later
	if _, err := s.complete(p); !errors.Is(err, ErrBusy) {
		t.Fatalf("err = %v", err)
	}
}

// A charge that went through and whose answer was lost has moved money: no other closed mandate
// under the open mandate is charged for another checkout, and the same one is the same order.
func TestAChargeWhoseAnswerWasLostIsNotChargedAgain(t *testing.T) {
	s := newShop(t)
	var open string
	p1 := s.buyUnder(t, &open)
	s.store.lost = true
	if _, err := s.complete(p1); !errors.Is(err, ErrUnavailable) || s.store.count() != 1 {
		t.Fatalf("err = %v, charges %d", err, s.store.count())
	}
	s.store.lost = false
	p2 := s.buyUnder(t, &open)
	if _, err := s.complete(p2); !errors.Is(err, ErrSpent) || s.store.count() != 1 {
		t.Fatalf("another checkout under the open mandate: %v, charges %d", err, s.store.count())
	}
	out, err := s.complete(p1)
	if err != nil || out.Order == "" || s.store.count() != 1 {
		t.Fatalf("the same mandates again: %+v %v, charges %d", out, err, s.store.count())
	}
}

// ledgerDown fails to record a completion as kept.
type ledgerDown struct {
	Ledger
	down bool
}

func (l *ledgerDown) Settle(ctx context.Context, open, ref string, kept bool) error {
	if kept && l.down {
		return errors.New("the ledger is down")
	}
	return l.Ledger.Settle(ctx, open, ref, kept)
}

// An order that is placed but cannot be recorded against the open mandate stands, says so, and
// is found paid when another closed mandate tries.
func TestAnOrderThatCannotBeRecordedAgainstTheOpenMandateStands(t *testing.T) {
	var l *ledgerDown
	s := newShop(t, func(c *Config) { l = &ledgerDown{Ledger: c.Ledger, down: true}; c.Ledger = l })
	var open string
	out, err := s.complete(s.buyUnder(t, &open))
	if err != nil || out.Order == "" || out.Unsettled == nil {
		t.Fatalf("out = %+v err = %v", out, err)
	}
	if _, err := s.complete(s.buyUnder(t, &open)); !errors.Is(err, ErrSpent) || s.store.count() != 1 {
		t.Fatalf("another checkout: %v, charges %d", err, s.store.count())
	}
}

// buyUnder is buy, with an open Checkout Mandate the user signed once and the agent presents
// every time: *open is empty for the first and holds it after.
func (s *shop) buyUnder(t *testing.T, open *string) *purchase {
	t.Helper()
	iss := s.issue(t)
	s.world.nonce, s.world.aud = iss.Nonce, iss.Audience
	cm := s.checkoutMandate(iss.JWT, iss.Hash)
	if *open == "" {
		*open = cm.openText()
	}
	cm.openOut = *open
	pm := s.paymentMandate(iss.Hash, 500, cm.openText())
	return &purchase{issued: iss, checkout: cm, payment: pm, token: "spt_test"}
}

func TestADirectMandateCompletes(t *testing.T) {
	s := newShop(t)
	iss := s.issue(t)
	b1, b2 := s.sd(), s.sd()
	co := s.rootTok(b1, s.closedCheckoutItem(b1, iss.JWT, iss.Hash))
	pay := s.rootTok(b2, paymentItem(iss.Hash, 500))
	coText, payText := co.text(t), pay.text(t)
	for i := 0; i < 2; i++ { // and again: a direct mandate has no open mandate to spend
		out, err := s.m.Complete(context.Background(), s.who, Completion{ID: iss.ID, Checkout: coText, Payment: payText, Token: "spt_test"})
		if err != nil || out.Order == "" || s.store.count() != 1 {
			t.Fatalf("out = %+v err = %v charges %d", out, err, s.store.count())
		}
	}
}

func TestACheckoutThatExpiredIsRefused(t *testing.T) {
	s := newShop(t, func(c *Config) { c.Lifetime = time.Minute })
	p := s.buy(t)
	j, _ := parseJWS(p.issued.JWT)
	if co, err := decodeCheckout(j.payload); err != nil || co.ExpiresAt != s.now.Add(time.Minute).UTC().Format(time.RFC3339) {
		t.Fatalf("expires_at %+v %v", co, err)
	}
	s.v.Now = func() time.Time { return s.now.Add(2 * time.Minute) }
	s.m.v.Now = s.v.Now
	_, err := s.complete(p)
	if !errors.Is(err, ErrExpired) {
		t.Fatalf("err = %v", err)
	}
	if s.store.count() != 0 {
		t.Fatal("charged")
	}
}

func TestAStoreThatRepricedHasNoCheckoutToSign(t *testing.T) {
	s := newShop(t)
	p := s.buy(t)
	s.store.prices["credit-500"] = 700
	// the cart's lines are priced from the catalog as it is now and no longer come to its total
	if _, err := s.m.Issue(context.Background(), s.who, p.issued.ID); !errors.Is(err, ErrStale) {
		t.Fatalf("err = %v", err)
	}
	// the mandate made for the checkout signed before the change fails against the store
	if _, err := s.complete(p); !errors.Is(err, ErrStale) {
		t.Fatalf("err = %v", err)
	}
	if s.store.count() != 0 {
		t.Fatal("charged")
	}
}

func TestOnlyTheCallersCheckoutIsIssued(t *testing.T) {
	s := newShop(t)
	p := s.buy(t)
	if _, err := s.m.Issue(context.Background(), Caller{Org: "demo", Subject: "someone-else"}, p.issued.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
	s.store.carts[p.issued.ID].Canceled = true
	if _, err := s.m.Issue(context.Background(), s.who, p.issued.ID); err == nil {
		t.Fatal("a canceled checkout was signed")
	}
	if _, err := s.store.open(s.who, "nope", 1); !errors.Is(err, ErrNoItem) {
		t.Fatalf("err = %v", err)
	}
}

func TestANonceIsTheMerchantsAndTheCheckoutsAndTheOrganizations(t *testing.T) {
	s := newShop(t)
	a := s.m.nonce(Caller{Org: "demo"}, "chk_1")
	if a != s.m.nonce(Caller{Org: "demo", Subject: "anyone"}, "chk_1") {
		t.Fatal("a nonce depends on the buyer")
	}
	if a == s.m.nonce(Caller{Org: "demo"}, "chk_2") || a == s.m.nonce(Caller{Org: "other"}, "chk_1") {
		t.Fatal("a nonce is shared between checkouts or organizations")
	}
	t2 := newShop(t, func(c *Config) { c.Secret = []byte("fedcba9876543210fedcba9876543210") })
	if a == t2.m.nonce(Caller{Org: "demo"}, "chk_1") {
		t.Fatal("a nonce is shared between merchants")
	}
}

func TestAMerchantNeedsWhatItIsMadeOf(t *testing.T) {
	w := newWorld(t)
	good := Config{
		Backend: newStore(), Trust: w.v.Trust, Signer: w.merchant, Identity: Merchant{ID: "m", Name: "M"},
		Secret: make([]byte, 32), Ledger: &memory{},
	}
	if _, err := New(good); err != nil {
		t.Fatal(err)
	}
	for name, f := range map[string]func(c *Config){
		"no backend":           func(c *Config) { c.Backend = nil },
		"no trust":             func(c *Config) { c.Trust = nil },
		"no ledger":            func(c *Config) { c.Ledger = nil },
		"no identity":          func(c *Config) { c.Identity = Merchant{} },
		"a short secret":       func(c *Config) { c.Secret = make([]byte, 31) },
		"a negative lifetime":  func(c *Config) { c.Lifetime = -1 },
		"a signer with no kid": func(c *Config) { c.Signer.Kid = "" },
		"a signer with no key": func(c *Config) { c.Signer.Key = nil },
	} {
		c := good
		f(&c)
		if _, err := New(c); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestACompletionNeedsItsParts(t *testing.T) {
	s := newShop(t)
	p := s.buy(t)
	full := Completion{ID: p.issued.ID, Checkout: p.checkout.text(), Payment: p.payment.text(), Token: "spt_x"}
	for name, f := range map[string]func(c *Completion){
		"no id":                            func(c *Completion) { c.ID = "" },
		"no checkout":                      func(c *Completion) { c.Checkout = "" },
		"no payment":                       func(c *Completion) { c.Payment = "" },
		"no token":                         func(c *Completion) { c.Token = "" },
		"a checkout that is not a mandate": func(c *Completion) { c.Checkout = "junk~" },
		"a payment that is not a mandate":  func(c *Completion) { c.Payment = "junk~" },
	} {
		in := full
		f(&in)
		out, err := s.m.Complete(context.Background(), s.who, in)
		var e *Error
		if !errors.As(err, &e) || e.Code != InvalidCredential || out != nil {
			t.Errorf("%s: out = %+v err = %v", name, out, err)
		}
	}
	if s.store.count() != 0 {
		t.Fatal("charged")
	}
}

func TestTheMemoryLedger(t *testing.T) {
	var m memory
	ctx := context.Background()
	v, c, err := m.Begin(ctx, "open", "a", "chk_a")
	if v != Fresh || c.Ref != "a" || c.Checkout != "chk_a" || c.At.IsZero() || err != nil {
		t.Fatalf("%v %+v %v", v, c, err)
	}
	if v, c, _ = m.Begin(ctx, "open", "a", "chk_a"); v != Busy || c.Ref != "a" {
		t.Fatalf("a claim begun and not ended: %v %+v", v, c)
	}
	// Only the holder settles a claim.
	m.Settle(ctx, "open", "b", true)
	if v, _, _ = m.Begin(ctx, "open", "a", "chk_a"); v != Busy {
		t.Fatalf("another closed mandate settled the claim: %v", v)
	}
	m.Settle(ctx, "open", "a", true)
	if v, _, _ = m.Begin(ctx, "open", "a", "chk_a"); v != Replay {
		t.Fatalf("the same closed mandate: %v", v)
	}
	if v, c, _ = m.Begin(ctx, "open", "b", "chk_b"); v != Taken || c.Ref != "a" {
		t.Fatalf("another closed mandate: %v %+v", v, c)
	}
	// A kept claim is not given up.
	m.Settle(ctx, "open", "a", false)
	if v, _, _ = m.Begin(ctx, "open", "b", "chk_b"); v != Taken {
		t.Fatalf("a kept claim was given up: %v", v)
	}
	if v, _, _ = m.Begin(ctx, "other-open", "b", "chk_b"); v != Fresh {
		t.Fatalf("another open mandate: %v", v)
	}
	m.Settle(ctx, "other-open", "b", false)
	if v, _, _ = m.Begin(ctx, "other-open", "c", "chk_c"); v != Fresh {
		t.Fatalf("a given-up claim: %v", v)
	}
}

func TestSpentIsReadOnlyWhenAConstraintNeedsIt(t *testing.T) {
	var asked int
	s := newShop(t, func(c *Config) {
		c.Spent = func(context.Context, string) (Usage, error) { asked++; return Usage{}, nil }
	})
	p := s.buy(t)
	if _, err := s.complete(p); err != nil || asked != 0 {
		t.Fatalf("err = %v, asked %d", err, asked)
	}
	// a budget on the open payment mandate needs it, and the merchant gives it
	p = s.buy(t)
	p.payment.openItem["constraints"] = append(p.payment.openItem["constraints"].([]any), map[string]any{"type": "payment.budget", "max": 1000, "currency": "USD"})
	if _, err := s.complete(p); err != nil || asked != 1 {
		t.Fatalf("err = %v, asked %d", err, asked)
	}
}

func TestADisputeStandsOnTheEvidence(t *testing.T) {
	ctx := context.Background()
	s := newShop(t)
	p := s.buy(t)
	out, err := s.complete(p)
	if err != nil {
		t.Fatal(err)
	}
	pub := &s.merchant.Key.PublicKey
	ev := Evidence{
		Checkout: p.checkout.text(), Payment: p.payment.text(), CheckoutReceipt: out.Receipt, PaymentReceipt: out.Payment,
		Merchant: pub, Processor: pub, Want: Want{Audience: s.aud, Nonce: p.issued.Nonce},
	}
	if err := s.v.Dispute(ctx, ev); err != nil {
		t.Fatal(err)
	}
	// Months on, the mandates have expired: checked now they fail, checked as of the receipts,
	// which is what a dispute does, they hold.
	later := *s.v
	later.Now = func() time.Time { return s.now.Add(90 * 24 * time.Hour) }
	if _, err := later.Checkout(ctx, ev.Checkout, ev.Want, pub); !errors.Is(err, ErrExpired) {
		t.Fatalf("checked today: %v", err)
	}
	if err := later.Dispute(ctx, ev); err != nil {
		t.Fatalf("checked as of the receipts: %v", err)
	}

	// A second purchase, whose receipts are not this one's.
	p2 := s.buy(t)
	out2, err := s.complete(p2)
	if err != nil {
		t.Fatal(err)
	}
	other := newParty(t, "other")
	for name, change := range map[string]func(e *Evidence){
		"another purchase's checkout receipt": func(e *Evidence) { e.CheckoutReceipt = out2.Receipt },
		"another purchase's payment receipt":  func(e *Evidence) { e.PaymentReceipt = out2.Payment },
		"a checkout receipt of another key":   func(e *Evidence) { e.Merchant = &other.key.PublicKey },
		"a payment receipt of another key":    func(e *Evidence) { e.Processor = &other.key.PublicKey },
		"a checkout mandate that was altered": func(e *Evidence) { e.Checkout = flip(e.Checkout) },
		"another payment mandate":             func(e *Evidence) { e.Payment = p2.payment.text() },
		"another nonce":                       func(e *Evidence) { e.Want.Nonce = "another" },
		"no merchant key":                     func(e *Evidence) { e.Merchant = nil },
		"no processor key":                    func(e *Evidence) { e.Processor = nil },
	} {
		bad := ev
		change(&bad)
		if err := s.v.Dispute(ctx, bad); err == nil {
			t.Errorf("%s accepted", name)
		}
	}

	// Receipts that say the purchase did not happen are not evidence that it did.
	s3 := newShop(t)
	p3 := s3.buy(t)
	s3.store.decline = &Declined{Code: "declined", Message: "no"}
	refused, _ := s3.complete(p3)
	ev3 := Evidence{
		Checkout: p3.checkout.text(), Payment: p3.payment.text(), CheckoutReceipt: refused.Receipt, PaymentReceipt: refused.Payment,
		Merchant: &s3.merchant.Key.PublicKey, Processor: &s3.merchant.Key.PublicKey, Want: Want{Audience: s3.aud, Nonce: p3.issued.Nonce},
	}
	if err := s3.v.Dispute(ctx, ev3); !errors.Is(err, ErrMandate) {
		t.Fatalf("error receipts as evidence: %v", err)
	}
}

// Many closed mandates under one open mandate, presented at once: one is completed and the
// store is charged once.
func TestClosedMandatesUnderOneOpenMandateAreCompletedOnce(t *testing.T) {
	s := newShop(t)
	p := s.buy(t)
	const n = 12
	texts := make([]string, n)
	for i := range texts {
		p.checkout.closed.claims["iat"] = s.unix(-time.Duration(i+1) * time.Second)
		p.checkout.resign()
		texts[i] = p.checkout.text()
	}
	pay := p.payment.text()
	var wg sync.WaitGroup
	var mu sync.Mutex
	done, refused := 0, 0
	for _, text := range texts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := s.m.Complete(context.Background(), s.who, Completion{ID: p.issued.ID, Checkout: text, Payment: pay, Token: "spt_x"})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil && out.Order != "":
				done++
			case errors.Is(err, ErrSpent) || errors.Is(err, ErrBusy):
				refused++
			default:
				t.Errorf("out = %+v err = %v", out, err)
			}
		}()
	}
	wg.Wait()
	if done != 1 || refused != n-1 || s.store.count() != 1 {
		t.Fatalf("completed %d, refused %d, charged %d", done, refused, s.store.count())
	}
}

// racy is a Backend whose checkout is changed by its buyer between the merchant's verification
// and the charge, as UpdateCheckout lets the buyer do in commerce. Honours runs after the cart
// has been agreed with the mandates and before Pay.
type racy struct {
	*store
	to string
}

func (r *racy) Honours(ctx context.Context, who Caller, in Instrument, pinned bool) error {
	r.store.mu.Lock()
	for _, c := range r.store.carts {
		c.Lines[0].ID, c.Total = r.to, r.store.prices[r.to]
	}
	r.store.mu.Unlock()
	return r.store.Honours(ctx, who, in, pinned)
}

// The store is charged for the checkout the mandates were verified against, or not at all.
func TestACheckoutChangedBetweenVerificationAndChargeIsNotCharged(t *testing.T) {
	var r *racy
	s := newShop(t, func(c *Config) { r = &racy{store: c.Backend.(*store), to: "credit-1000"}; c.Backend = r })
	p := s.buy(t) // mandates for credit-500 at 500
	out, err := s.complete(p)
	refusedWith(t, err, InvalidMandate, ErrStale)
	if out == nil || out.Order != "" || s.store.count() != 0 {
		t.Fatalf("outcome %+v, charges %d under mandates for 500", out, s.store.count())
	}
}

func allowInstruments(t *testing.T, is ...Instrument) func(b *sd) map[string]any {
	return func(b *sd) map[string]any {
		var l []any
		for _, i := range is {
			l = append(l, b.elem(t, i))
		}
		return map[string]any{"type": "payment.allowed_payment_instruments", "allowed": l}
	}
}

// A mandate that limits the instrument reaches the store as such: the store may tie its
// credential to an instrument and honour the limit, and one that cannot refuses the mandate
// with a typed error rather than ignore it.
func TestAMandateThatLimitsTheInstrumentIsNotIgnored(t *testing.T) {
	card := Instrument{ID: "pi_1", Type: "card", Description: "Card 4242"}
	for _, tc := range []struct {
		name   string
		limit  func(a *autonomous)
		pinned bool
	}{
		{"no limit", func(a *autonomous) {}, false},
		{"allowed_payment_instruments", func(a *autonomous) {
			a.constraints(func(b *sd) []any {
				return []any{a.openItem["constraints"].([]any)[0], allowInstruments(t, card)(b)}
			})
		}, true},
		{"a preset instrument", func(a *autonomous) {
			a.openItem["payment_instrument"] = map[string]any{"id": "pi_1", "type": "card", "description": "Card 4242"}
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newShop(t)
			p := s.buy(t)
			tc.limit(p.payment)
			if _, err := s.complete(p); err != nil || s.store.pinned != tc.pinned {
				t.Fatalf("err = %v, the store was told pinned=%v; want %v", err, s.store.pinned, tc.pinned)
			}
			// A store that cannot honour a limit refuses it, typed, and charges nothing.
			if tc.pinned {
				s = newShop(t)
				s.store.refusePinned = true
				p = s.buy(t)
				tc.limit(p.payment)
				out, err := s.complete(p)
				var u *Unsupported
				if !errors.As(err, &u) || out == nil || out.Order != "" || s.store.count() != 0 {
					t.Fatalf("out = %+v err = %v charges %d", out, err, s.store.count())
				}
			}
		})
	}
}
