// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ap2

import (
	"context"
	"errors"
	"testing"

	"github.com/hanzoai/commerce/api/billing"
	"github.com/hanzoai/commerce/models/checkoutsession"
	orgpkg "github.com/hanzoai/commerce/pkg/org"
	"github.com/hanzoai/commerce/util/test/ae"
)

// The merchant over the real checkout surface of commerce: a checkout is opened at the
// catalog's price, signed, and completed against mandates that must agree with the catalog
// and the checkout before anything is charged.
func TestCommerceBehindAMerchant(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	who := Caller{Org: "ap2-seller", Subject: "agent-buyer"}
	org, err := orgpkg.Resolve(ctx, who.Org)
	if err != nil {
		t.Fatal(err)
	}
	cat, err := billing.ReadCatalog(ctx, org, who.Host)
	if err != nil || len(cat.CreditPacks) == 0 || len(cat.Plans) == 0 {
		t.Fatalf("catalog = %+v %v", cat, err)
	}
	pack, plan := cat.CreditPacks[0], cat.Plans[0]

	w := newWorld(t)
	m, err := New(Config{
		Backend: Commerce{}, Trust: w.v.Trust, Signer: w.merchant,
		Identity: Merchant{ID: "merchant_1", Name: "Demo Merchant", Website: "https://demo-merchant.example"},
		Secret:   []byte("0123456789abcdef0123456789abcdef"), Now: w.v.Now, Ledger: Durable{},
	})
	if err != nil {
		t.Fatal(err)
	}

	// A credit pack's checkout is signed at the catalog's price, under the catalog's id, in USD.
	cv, f := billing.CreateCheckout(ctx, org, who.Subject, who.Host, []billing.ItemIn{{Kind: "credit", UsdCents: pack.UsdCents}})
	if f != nil {
		t.Fatal(f)
	}
	iss, err := m.Issue(ctx, who, cv.ID)
	if err != nil {
		t.Fatal(err)
	}
	j, _ := parseJWS(iss.JWT)
	co, err := decodeCheckout(j.payload)
	if total, _ := co.Total("total"); err != nil || co.Currency != "USD" || len(co.LineItems) != 1 || co.LineItems[0].Item.ID != pack.ID || co.LineItems[0].Item.Price != pack.UsdCents || total != pack.UsdCents {
		t.Fatalf("checkout = %+v %v", co, err)
	}
	// A plan is not sold through mandates: one payment mandate authorizes one payment, and a plan
	// would renew on its token with none. Its checkout is not signed, and cannot be paid.
	var unsupported *Unsupported
	pv, f := billing.CreateCheckout(ctx, org, who.Subject, who.Host, []billing.ItemIn{{Kind: "plan", Plan: plan.Slug}})
	if f != nil {
		t.Fatal(f)
	}
	if _, err := m.Issue(ctx, who, pv.ID); !errors.As(err, &unsupported) || unsupported.Item != plan.Slug {
		t.Fatalf("issue of a plan checkout = %v", err)
	}
	if code, ok := CodeOf(unsupported); !ok || code != InvalidMandate {
		t.Fatalf("code %s", code)
	}
	if _, err := (Commerce{}).Pay(ctx, who, pv.ID, "k-plan", Payment{Token: "spt_1Test", Cart: Cart{Lines: []Line{{ID: plan.Slug, Quantity: 1}}, Total: plan.Price}}); !errors.As(err, &unsupported) {
		t.Fatalf("pay of a plan checkout = %v", err)
	}
	if s, _, _ := checkoutsession.Get(pv.ID); s.Status != checkoutsession.Open {
		t.Fatalf("a refused plan checkout is %s", s.Status)
	}
	// More than one line is not a checkout commerce sells.
	two := Cart{Lines: []Line{{ID: pack.ID, Quantity: 1}, {ID: pack.ID, Quantity: 1}}}
	if _, err := (Commerce{}).Pay(ctx, who, cv.ID, "k-two", Payment{Token: "spt_1Test", Cart: two}); !errors.Is(err, ErrNoItem) {
		t.Fatalf("two lines = %v", err)
	}

	// Another buyer's checkout is not found.
	if _, err := m.Issue(ctx, Caller{Org: who.Org, Subject: "someone-else"}, cv.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("issue to another buyer = %v", err)
	}

	// Mandates for the signed checkout.
	w.nonce, w.aud = iss.Nonce, iss.Audience
	cm := w.checkoutMandate(iss.JWT, digest(iss.JWT))
	cm.constraints(func(b *sd) []any {
		return []any{w.lineItems(b, 1, Item{pack.ID, "pack"}), w.allowedMerchants(b, Merchant{ID: "merchant_1", Name: "Demo Merchant"})}
	})
	pm := w.paymentMandate(digest(iss.JWT), pack.UsdCents, cm.openText())
	in := Completion{ID: cv.ID, Checkout: cm.text(), Payment: pm.text(), Token: "spt_1Test"}

	// The deployment has no Stripe, so the token cannot be read: the payment is declined, with
	// the code commerce gives, the answer is a signed error receipt, and nothing was taken.
	out, err := m.Complete(ctx, who, in)
	var d *Declined
	if !errors.As(err, &d) || d.Code != "payments_unavailable" || out == nil || out.Order != "" {
		t.Fatalf("complete = %+v %v", out, err)
	}
	r, rerr := ParseCheckoutReceipt(out.Receipt, &w.merchant.Key.PublicKey, in.Checkout)
	if rerr != nil || r.Status != Failure || r.Error != string(PaymentDeclined) {
		t.Fatalf("receipt = %+v %v", r, rerr)
	}
	// The open mandate was freed with it: the same mandates, tried again, reach the store
	// again (not Busy, not Taken), and the checkout is still there to be paid.
	if _, err = m.Complete(ctx, who, in); !errors.As(err, &d) {
		t.Fatalf("complete again = %v", err)
	}
	if got, err := (Commerce{}).Read(ctx, who, cv.ID); err != nil || got.Total != pack.UsdCents || got.Canceled {
		t.Fatalf("read = %+v %v", got, err)
	}

	// The catalog moving under an open checkout: the checkout keeps the total it was opened at,
	// the lines are priced as the catalog stands, and the merchant will neither sign nor
	// accept a checkout in which they differ. Nothing is charged.
	if _, _, err := checkoutsession.Change(cv.ID, func(s *checkoutsession.Session) bool { s.TotalCents += 100; return true }); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Issue(ctx, who, cv.ID); !errors.Is(err, ErrStale) {
		t.Fatalf("issue after the total moved = %v", err)
	}
	if _, err := m.Complete(ctx, who, in); !errors.Is(err, ErrStale) {
		t.Fatalf("complete after the total moved = %v", err)
	}
}

func TestCommerceRefusesAPaymentMethodItCannotHonour(t *testing.T) {
	c := Commerce{}
	if err := c.Honours(context.Background(), Caller{}, Instrument{ID: "pi_1", Type: "card"}, false); err != nil {
		t.Fatal(err)
	}
	if err := c.Honours(context.Background(), Caller{}, Instrument{ID: "pi_1", Type: "CARD"}, false); err != nil {
		t.Fatal(err)
	}
	// A card whose use the mandate limits to an instrument is refused as well: commerce cannot
	// tell which instrument a shared payment token stands for.
	var pinned *Unsupported
	if err := c.Honours(context.Background(), Caller{}, Instrument{ID: "pi_1", Type: "card"}, true); !errors.As(err, &pinned) {
		t.Errorf("a pinned card = %v", err)
	}
	for _, typ := range []string{"UPI", "dpc", "stablecoin", "bank", ""} {
		var u *Unsupported
		err := c.Honours(context.Background(), Caller{}, Instrument{ID: "pi_2", Type: typ}, false)
		if !errors.As(err, &u) || u.Instrument.Type != typ {
			t.Errorf("%q: %v", typ, err)
		}
		if code, ok := CodeOf(err); !ok || code != InvalidMandate {
			t.Errorf("%q: code %s", typ, code)
		}
	}
}

// What commerce answers a completion with, in the merchant's vocabulary.
func TestCommerceAnswersAreStatedAsTheMerchantUnderstandsThem(t *testing.T) {
	view := func(status string, e *billing.AgentError) *billing.CheckoutView {
		return &billing.CheckoutView{ID: "chk_1", Status: status, TotalCents: 500, Error: e, Items: []billing.ItemIn{{Kind: "credit", UsdCents: 500}}}
	}
	for _, tc := range []struct {
		name string
		out  billing.CompleteOut
		is   error
		code string // of a *Declined
	}{
		{"a declined card", billing.CompleteOut{Status: 402, View: view("failed", &billing.AgentError{Code: "declined", Message: "declined"})}, nil, "declined"},
		{"a revoked token", billing.CompleteOut{Status: 402, View: view("failed", &billing.AgentError{Code: "token_revoked", Message: "revoked"})}, nil, "token_revoked"},
		{"a bank that wants authentication", billing.CompleteOut{Status: 402, View: view("failed", &billing.AgentError{Code: "authentication_required", Message: "3ds"})}, nil, "authentication_required"},
		{"a checkout completed under another key", billing.CompleteOut{Status: 409, View: view("complete", &billing.AgentError{Code: "already_completed", Message: "another key"})}, ErrSpent, ""},
		{"a checkout being completed", billing.CompleteOut{Status: 409, View: view("pending", &billing.AgentError{Code: "in_progress", Message: "being completed"})}, ErrBusy, ""},
		{"a checkout that is gone", billing.CompleteOut{Status: 404, View: view("failed", &billing.AgentError{Code: "not_found", Message: "gone"})}, ErrNotFound, ""},
		{"billing unavailable", billing.CompleteOut{Status: 503, View: view("failed", &billing.AgentError{Code: "unavailable", Message: "later"})}, ErrUnavailable, ""},
		{"a processor outage", billing.CompleteOut{Status: 502, View: view("failed", &billing.AgentError{Code: "payment_failed", Message: "down"})}, ErrUnavailable, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := paid(tc.out)
			if p != nil {
				t.Fatalf("paid = %+v", p)
			}
			if tc.is != nil {
				if !errors.Is(err, tc.is) {
					t.Fatalf("err = %v; want %v", err, tc.is)
				}
				return
			}
			var d *Declined
			if !errors.As(err, &d) || d.Code != tc.code {
				t.Fatalf("err = %v; want a *Declined %s", err, tc.code)
			}
		})
	}
	done := view("complete", nil)
	done.Order = &billing.OrderView{ID: "chk_1", CheckoutID: "chk_1", Status: "paid", TotalCents: 500, PaymentID: "pi_1"}
	p, err := paid(billing.CompleteOut{Status: 200, View: done})
	if err != nil || p.Order != "chk_1" || p.Payment != "pi_1" || p.Network != "" || p.PSP != "" {
		t.Fatalf("paid = %+v %v", p, err)
	}
	// What commerce must not answer: a request this package never makes, or no checkout.
	for _, bad := range []billing.CompleteOut{
		{Status: 400, View: view("failed", &billing.AgentError{Code: "idempotency_key_required"})},
		{Status: 400, View: view("failed", &billing.AgentError{Code: "payment_type_unknown"})},
		{Status: 200, View: view("open", nil)},
		{Status: 202, View: view("pending", nil)},
		{Status: 200},
	} {
		if p, err := paid(bad); err == nil || p != nil {
			t.Errorf("%+v accepted", bad)
		}
	}
}

func TestTheDurableLedger(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	var l Durable
	v, c, err := l.Begin(ctx, "open-1", "closed-a", "chk_a")
	if v != Fresh || c.Ref != "closed-a" || c.Checkout != "chk_a" || c.At.IsZero() || err != nil {
		t.Fatalf("%v %+v %v", v, c, err)
	}
	// A claim begun and not ended is the holder's, whoever asks.
	for _, ref := range []string{"closed-a", "closed-b"} {
		if v, c, _ = l.Begin(ctx, "open-1", ref, "chk_x"); v != Busy || c.Ref != "closed-a" || c.Checkout != "chk_a" {
			t.Fatalf("a claim in progress, asked by %s: %v %+v", ref, v, c)
		}
	}
	// Only the holder settles it.
	if err := l.Settle(ctx, "open-1", "closed-b", true); err != nil {
		t.Fatal(err)
	}
	if v, _, _ = l.Begin(ctx, "open-1", "closed-a", "chk_a"); v != Busy {
		t.Fatalf("another closed mandate settled the claim: %v", v)
	}
	if err := l.Settle(ctx, "open-1", "closed-a", false); err != nil {
		t.Fatal(err)
	}
	if v, _, _ = l.Begin(ctx, "open-1", "closed-a", "chk_a"); v != Fresh {
		t.Fatalf("a given-up claim: %v", v)
	}
	if err := l.Settle(ctx, "open-1", "closed-a", true); err != nil {
		t.Fatal(err)
	}
	if v, _, _ = l.Begin(ctx, "open-1", "closed-a", "chk_a"); v != Replay {
		t.Fatalf("the same closed mandate: %v", v)
	}
	if v, c, _ = l.Begin(ctx, "open-1", "closed-b", "chk_b"); v != Taken || c.Ref != "closed-a" {
		t.Fatalf("another closed mandate: %v %+v", v, c)
	}
	// A kept claim is not given up.
	if err := l.Settle(ctx, "open-1", "closed-a", false); err != nil {
		t.Fatal(err)
	}
	if v, _, _ = l.Begin(ctx, "open-1", "closed-b", "chk_b"); v != Taken {
		t.Fatalf("a kept claim was given up: %v", v)
	}
	if v, _, _ = l.Begin(ctx, "open-2", "closed-b", "chk_b"); v != Fresh {
		t.Fatalf("another open mandate: %v", v)
	}
	if err := l.Settle(ctx, "no-such-open", "closed-a", true); err != nil {
		t.Fatalf("settling nothing: %v", err)
	}
}

// Commerce charges the checkout as the mandates were verified, or not at all: a buyer who
// swaps the line through UpdateCheckout after the verification is refused where the checkout
// is taken for payment, and the checkout is left as the buyer made it.
func TestCommerceChargesTheCheckoutAsVerifiedOrNotAtAll(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	who := Caller{Org: "ap2-swap-seller", Subject: "agent-buyer"}
	org, err := orgpkg.Resolve(ctx, who.Org)
	if err != nil {
		t.Fatal(err)
	}
	cat, err := billing.ReadCatalog(ctx, org, who.Host)
	if err != nil || len(cat.CreditPacks) < 2 {
		t.Fatalf("catalog = %+v %v", cat, err)
	}
	c := Commerce{}
	cv, f := billing.CreateCheckout(ctx, org, who.Subject, who.Host, []billing.ItemIn{{Kind: "credit", UsdCents: cat.CreditPacks[0].UsdCents}})
	if f != nil {
		t.Fatal(f)
	}
	cart, err := c.Read(ctx, who, cv.ID)
	if err != nil {
		t.Fatal(err)
	}
	other := cat.CreditPacks[1]
	if _, f := billing.UpdateCheckout(ctx, org, who.Subject, who.Host, cart.ID, []billing.ItemIn{{Kind: "credit", UsdCents: other.UsdCents}}); f != nil {
		t.Fatal(f)
	}
	_, err = c.Pay(ctx, who, cart.ID, "k-swap", Payment{Token: "spt_1Test", Instrument: Instrument{ID: "pi", Type: "card"}, Cart: *cart})
	refusedWith(t, err, InvalidMandate, ErrStale)
	got, err := c.Read(ctx, who, cart.ID)
	if err != nil || got.Total != other.UsdCents || got.Paid || got.Pending {
		t.Fatalf("the swapped checkout = %+v %v", got, err)
	}
}
