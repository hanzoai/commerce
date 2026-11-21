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
}

func newStore() *store {
	return &store{prices: map[string]int64{"credit-500": 500, "credit-1000": 1000}, carts: map[string]*Cart{}, owner: map[string]string{}, orders: map[string]string{}, network: "net-1"}
}

func (s *store) Open(_ context.Context, who Caller, items []Pick) (*Cart, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	c := &Cart{ID: fmt.Sprintf("chk_%d", s.seq), Currency: "USD"}
	for _, it := range items {
		price, ok := s.prices[it.ID]
		if !ok {
			return nil, ErrNoItem
		}
		c.Lines = append(c.Lines, Line{ID: it.ID, Title: it.ID, Price: price, Quantity: it.Quantity})
		c.Total += price * it.Quantity
	}
	s.carts[c.ID], s.owner[c.ID] = c, who.Subject
	return c, nil
}

// Read prices the lines from the catalog as it is now.
func (s *store) Read(_ context.Context, who Caller, id string) (*Cart, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.carts[id]
	if !ok || s.owner[id] != who.Subject {
		return nil, ErrNotFound
	}
	out := *c
	out.Lines = nil
	for _, l := range c.Lines {
		l.Price = s.prices[l.ID]
		out.Lines = append(out.Lines, l)
	}
	return &out, nil
}

func (s *store) Pay(_ context.Context, who Caller, id, key string, pay Payment) (*Paid, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.carts[id]; !ok || s.owner[id] != who.Subject {
		return nil, ErrNotFound
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
	s.orders[id] = key
	return &Paid{Order: "ord_" + id, Payment: "pay_" + id, PSP: "psp_" + id, Network: s.network}, nil
}

func (s *store) Honours(_ context.Context, _ Caller, in Instrument) error {
	if in.Type != "card" {
		return &Unsupported{Instrument: in, Why: "only cards are taken"}
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
		Issuer:   "https://demo-merchant.example", Audience: w.aud, Secret: []byte("0123456789abcdef0123456789abcdef"),
		Now: w.v.Now, Ledger: &Memory{},
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
	issued   *Issued
	checkout *autonomous
	payment  *autonomous
	token    string
}

func (s *shop) buy(t *testing.T) *purchase {
	t.Helper()
	iss, err := s.m.Open(context.Background(), s.who, []Pick{{ID: "credit-500", Quantity: 1}})
	if err != nil {
		t.Fatal(err)
	}
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
	if err != nil || r.Status != Success || r.OrderID != out.Order || r.Issuer != "https://demo-merchant.example" {
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

// A store that cannot answer has decided nothing: no receipt, and the open mandate is free.
func TestAnUnavailableStoreDecidesNothing(t *testing.T) {
	s := newShop(t)
	p := s.buy(t)
	s.store.decline = ErrUnavailable
	out, err := s.complete(p)
	if !errors.Is(err, ErrUnavailable) || out != nil {
		t.Fatalf("out = %+v err = %v", out, err)
	}
	s.store.decline = nil
	if out, err = s.complete(p); err != nil || out.Order == "" {
		t.Fatalf("retry = %+v %v", out, err)
	}
}

func TestADirectMandateCompletes(t *testing.T) {
	s := newShop(t)
	iss, err := s.m.Open(context.Background(), s.who, []Pick{{ID: "credit-500", Quantity: 1}})
	if err != nil {
		t.Fatal(err)
	}
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
	if p.issued.Checkout.ExpiresAt != s.now.Add(time.Minute).UTC().Format(time.RFC3339) {
		t.Fatalf("expires_at %q", p.issued.Checkout.ExpiresAt)
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
	if _, err := s.m.Open(context.Background(), s.who, []Pick{{ID: "nope", Quantity: 1}}); !errors.Is(err, ErrNoItem) {
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
		Issuer: "iss", Audience: "aud", Secret: make([]byte, 32), Ledger: &Memory{},
	}
	if _, err := New(good); err != nil {
		t.Fatal(err)
	}
	for name, f := range map[string]func(c *Config){
		"no backend":           func(c *Config) { c.Backend = nil },
		"no trust":             func(c *Config) { c.Trust = nil },
		"no ledger":            func(c *Config) { c.Ledger = nil },
		"no identity":          func(c *Config) { c.Identity = Merchant{} },
		"no issuer":            func(c *Config) { c.Issuer = "" },
		"no audience":          func(c *Config) { c.Audience = "" },
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
	var m Memory
	ctx := context.Background()
	v, l, err := m.Begin(ctx, "open", "a")
	if v != Fresh || l == nil || err != nil {
		t.Fatalf("%v %v %v", v, l, err)
	}
	if v, _, _ = m.Begin(ctx, "open", "a"); v != Busy {
		t.Fatalf("a claim in progress: %v", v)
	}
	if err := l.Keep(ctx); err != nil {
		t.Fatal(err)
	}
	if v, _, _ = m.Begin(ctx, "open", "a"); v != Replay {
		t.Fatalf("the same closed mandate: %v", v)
	}
	if v, _, _ = m.Begin(ctx, "open", "b"); v != Taken {
		t.Fatalf("another closed mandate: %v", v)
	}
	if v, l, _ = m.Begin(ctx, "other-open", "b"); v != Fresh {
		t.Fatalf("another open mandate: %v", v)
	}
	l.Drop(ctx)
	if v, _, _ = m.Begin(ctx, "other-open", "c"); v != Fresh {
		t.Fatalf("a dropped claim: %v", v)
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
