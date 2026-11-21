// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ap2

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

func basket(lines ...LineItem) *Checkout {
	return &Checkout{ID: "c", Merchant: &Merchant{ID: "m", Name: "M"}, LineItems: lines}
}

func line(id string, qty int64) LineItem {
	return LineItem{ID: "l-" + id, Item: Product{ID: id}, Quantity: qty}
}

func req(qty int64, ids ...string) LineRequirement {
	r := LineRequirement{ID: "r", Quantity: qty}
	for _, id := range ids {
		r.AcceptableItems = append(r.AcceptableItems, Item{ID: id, Title: id})
	}
	return r
}

// The examples of checkout_mandate.md: one pair of shoes, one pair of socks.
func TestLineItemsAsTheSpecExplainsThem(t *testing.T) {
	c := LineItems{Type: "checkout.line_items", Items: []LineRequirement{req(1, "BAB1234", "FAF1234"), req(1, "QRT1234")}}
	for name, tc := range map[string]struct {
		cart *Checkout
		ok   bool
	}{
		"Red Style and The Best Socks":  {basket(line("BAB1234", 1), line("QRT1234", 1)), true},
		"Blue Style and The Best Socks": {basket(line("FAF1234", 1), line("QRT1234", 1)), true},
		"Red Style and Blue Style":      {basket(line("BAB1234", 1), line("FAF1234", 1)), false},
		"Red Style":                     {basket(line("BAB1234", 1)), false},
		"Blue Style":                    {basket(line("FAF1234", 1)), false},
		"The Best Socks":                {basket(line("QRT1234", 1)), false},
		"two pairs of socks":            {basket(line("QRT1234", 2), line("BAB1234", 1)), false},
		"socks on two lines":            {basket(line("QRT1234", 1), line("QRT1234", 1), line("BAB1234", 1)), false},
		"an item nobody accepts":        {basket(line("BAB1234", 1), line("QRT1234", 1), line("XXX", 1)), false},
		"an empty checkout":             {basket(), false},
	} {
		t.Run(name, func(t *testing.T) {
			err := c.fit(tc.cart)
			if (err == nil) != tc.ok {
				t.Fatalf("fit = %v; want ok=%v", err, tc.ok)
			}
			if err != nil && (err.Code != UnresolvedConstraint || !errors.Is(err, ErrConstraint)) {
				t.Fatalf("code = %s", err.Code)
			}
		})
	}
}

// Overlapping requirements are the case a greedy match gets wrong and a maximum flow gets
// right: the item both accept must go to the requirement that has no other.
func TestLineItemsAreMatchedAsAMaximumFlow(t *testing.T) {
	c := LineItems{Items: []LineRequirement{req(1, "A", "B"), req(1, "A")}}
	if err := c.fit(basket(line("A", 1), line("B", 1))); err != nil {
		t.Fatalf("A and B: %v", err)
	}
	if err := c.fit(basket(line("B", 2))); err == nil {
		t.Fatal("two of B fills one requirement twice")
	}
	big := LineItems{Items: []LineRequirement{req(3, "A", "B"), req(2, "B", "C")}}
	if err := big.fit(basket(line("A", 3), line("C", 2))); err != nil {
		t.Fatal(err)
	}
	if err := big.fit(basket(line("A", 4), line("C", 1))); err == nil {
		t.Fatal("4 of A is more than the first requirement takes")
	}
}

func TestLineItemsRefuseWhatTheyCannotEvaluate(t *testing.T) {
	for name, c := range map[string]LineItems{
		"no requirement":             {},
		"a requirement of none":      {Items: []LineRequirement{req(0, "A")}},
		"no acceptable item":         {Items: []LineRequirement{req(1)}},
		"requirements that overflow": {Items: []LineRequirement{req(math.MaxInt64, "A"), req(1, "A")}},
		"a requirement below zero":   {Items: []LineRequirement{req(-1, "A")}},
	} {
		if err := c.fit(basket(line("A", 1))); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if err := (LineItems{Items: []LineRequirement{req(1, "A")}}).fit(basket(line("A", math.MaxInt64), line("A", 1))); err == nil {
		t.Error("a cart that overflows accepted")
	}
}

func TestAllowedMerchants(t *testing.T) {
	m := Merchant{ID: "merchant_1", Name: "Demo Merchant", Website: "https://demo-merchant.example"}
	open := func(allowed ...Merchant) *OpenCheckoutMandate {
		return &OpenCheckoutMandate{Constraints: Constraints{AllowedMerchants{Allowed: allowed}}}
	}
	co := &Checkout{Merchant: &m}
	for name, tc := range map[string]struct {
		o  *OpenCheckoutMandate
		co *Checkout
		ok bool
	}{
		"by id":                             {open(Merchant{ID: "merchant_1", Name: "other name"}), co, true},
		"by name and website":               {open(Merchant{Name: "Demo Merchant", Website: "https://demo-merchant.example"}), co, true},
		"by name alone is not enough":       {open(Merchant{Name: "Demo Merchant"}), co, false},
		"another merchant":                  {open(Merchant{ID: "merchant_2", Name: "Demo Merchant"}), co, false},
		"none revealed":                     {open(), co, false},
		"a checkout that names no merchant": {open(m), &Checkout{}, false},
	} {
		t.Run(name, func(t *testing.T) {
			if err := tc.o.check(tc.co); (err == nil) != tc.ok {
				t.Fatalf("check = %v", err)
			}
		})
	}
}

func TestConstraintsOfOpenCheckoutMandates(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(w *world, a *autonomous)
		code   Code
		reason error
	}{
		{"more than the mandate asks for", func(w *world, a *autonomous) {
			_, jwt, hash := w.checkout("chk_1", 2, 500)
			a.closing(func(b *sd) map[string]any { return w.closedCheckoutItem(b, jwt, hash) })
		}, UnresolvedConstraint, ErrConstraint},
		{"an item the mandate does not accept", func(w *world, a *autonomous) {
			co, _, _ := w.checkout("chk_1", 1, 500)
			co.LineItems[0].Item.ID = "credit-9999"
			jwt, _ := w.merchant.Sign(co)
			h := digest(jwt)
			a.closing(func(b *sd) map[string]any { return w.closedCheckoutItem(b, jwt, h) })
		}, UnresolvedConstraint, ErrConstraint},
		{"a merchant the mandate does not allow", func(w *world, a *autonomous) {
			a.constraints(func(b *sd) []any {
				return []any{w.lineItems(b, 1, Item{"credit-500", "x"}), w.allowedMerchants(b, Merchant{ID: "merchant_2", Name: "Another"})}
			})
		}, UnresolvedConstraint, ErrConstraint},
		{"no merchant revealed", func(w *world, a *autonomous) {
			a.constraints(func(b *sd) []any {
				return []any{w.lineItems(b, 1, Item{"credit-500", "x"}), map[string]any{"type": "checkout.allowed_merchants", "allowed": []any{}}}
			})
		}, UnresolvedConstraint, ErrConstraint},
		{"no acceptable item revealed", func(w *world, a *autonomous) {
			a.constraints(func(b *sd) []any {
				return []any{map[string]any{"type": "checkout.line_items", "items": []any{map[string]any{"id": "line_1", "acceptable_items": []any{}, "quantity": 1}}}}
			})
		}, UnresolvedConstraint, ErrConstraint},
		{"a constraint nobody knows", func(w *world, a *autonomous) {
			a.openItem["constraints"] = append(a.openItem["constraints"].([]any), map[string]any{"type": "checkout.shipping_country", "allowed": []any{"US"}})
		}, UnresolvedConstraint, ErrConstraint},
		{"a payment constraint in a checkout mandate", func(w *world, a *autonomous) {
			a.openItem["constraints"] = append(a.openItem["constraints"].([]any), map[string]any{"type": "payment.amount_range", "currency": "USD", "max": 1})
		}, UnresolvedConstraint, ErrConstraint},
		{"a known constraint with a member it does not know", func(w *world, a *autonomous) {
			a.openItem["constraints"].([]any)[0].(map[string]any)["max_total"] = 1
		}, UnresolvedConstraint, ErrConstraint},
		{"a constraint that is not an object", func(w *world, a *autonomous) {
			a.openItem["constraints"] = append(a.openItem["constraints"].([]any), "line_items")
		}, UnresolvedConstraint, ErrConstraint},
		{"a quantity that is not a whole number", func(w *world, a *autonomous) {
			a.openItem["constraints"].([]any)[0].(map[string]any)["items"].([]any)[0].(map[string]any)["quantity"] = 1.5
		}, UnresolvedConstraint, ErrConstraint},
		{"no line_items constraint at all", func(w *world, a *autonomous) {
			a.constraints(func(b *sd) []any {
				return []any{w.allowedMerchants(b, Merchant{ID: "merchant_1", Name: "Demo Merchant"})}
			})
		}, InvalidMandate, ErrMandate},
		{"a preset checkout_hash the closed mandate changes", func(w *world, a *autonomous) {
			a.openItem["checkout_hash"] = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
		}, UnresolvedConstraint, ErrConstraint},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			_, jwt, hash := w.checkout("chk_1", 1, 500)
			a := w.checkoutMandate(jwt, hash)
			tc.change(w, a)
			_, err := w.check(a.text())
			refusedWith(t, err, tc.code, tc.reason)
		})
	}
}

// ---- payment ----

// payment verifies an autonomous Payment Mandate for a checkout of 500, after the Checkout
// Mandate it names has verified.
func (w *world) payment(t *testing.T, amount int64, more ...func(b *sd) map[string]any) (*autonomous, PayWant) {
	t.Helper()
	_, jwt, hash := w.checkout("chk_1", 1, 500)
	proof, err := w.check(w.checkoutMandate(jwt, hash).text())
	if err != nil {
		t.Fatal(err)
	}
	return w.paymentMandate(hash, amount, proof.OpenToken, more...), PayWant{CheckoutJWT: jwt, OpenCheckout: proof.OpenToken}
}

func (w *world) verifyPayment(text string, want PayWant) (*PaymentProof, error) {
	return w.v.Payment(context.Background(), text, want)
}

func constraint(m map[string]any) func(*sd) map[string]any {
	return func(*sd) map[string]any { return m }
}

func TestPaymentMandatesVerify(t *testing.T) {
	w := newWorld(t)
	a, want := w.payment(t, 500)
	p, err := w.verifyPayment(a.text(), want)
	if err != nil || p.Mandate.PaymentAmount.Amount != 500 {
		t.Fatalf("proof = %+v %v", p, err)
	}
	// signed by the Trusted Agent Provider directly: no open mandate, no constraints
	_, jwt, hash := w.checkout("chk_1", 1, 500)
	b := w.sd()
	direct := w.rootTok(b, paymentItem(hash, 500))
	p, err = w.verifyPayment(direct.text(t), PayWant{CheckoutJWT: jwt})
	if err != nil {
		t.Fatalf("direct = %+v %v", p, err)
	}
}

func TestPaymentMandateRefusals(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(w *world, a *autonomous, want *PayWant)
		code   Code
		reason error
	}{
		{"bound to another checkout", func(w *world, a *autonomous, _ *PayWant) {
			a.closedItem["transaction_id"] = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
		}, InvalidMandate, ErrHash},
		{"presented for another checkout", func(w *world, a *autonomous, want *PayWant) { want.CheckoutJWT = "another.checkout.jwt" }, InvalidMandate, ErrHash},
		{"a Checkout Mandate where a Payment Mandate belongs", func(w *world, a *autonomous, _ *PayWant) { a.closedItem["vct"] = VCTCheckout }, InvalidMandate, ErrType},
		{"a closed mandate of the next version", func(w *world, a *autonomous, _ *PayWant) { a.closedItem["vct"] = "mandate.payment.2" }, InvalidMandate, ErrType},
		{"no payee", func(w *world, a *autonomous, _ *PayWant) { delete(a.closedItem, "payee") }, InvalidMandate, ErrMandate},
		{"a payee with no name", func(w *world, a *autonomous, _ *PayWant) { a.closedItem["payee"] = map[string]any{"id": "m"} }, InvalidMandate, ErrMandate},
		{"no amount", func(w *world, a *autonomous, _ *PayWant) {
			a.closedItem["payment_amount"] = map[string]any{"currency": "USD"}
		}, InvalidMandate, ErrMandate},
		{"an amount in dollars and cents", func(w *world, a *autonomous, _ *PayWant) {
			a.closedItem["payment_amount"] = map[string]any{"amount": 5.25, "currency": "USD"}
		}, InvalidMandate, ErrMandate},
		{"an instrument with no type", func(w *world, a *autonomous, _ *PayWant) {
			a.closedItem["payment_instrument"] = map[string]any{"id": "pi"}
		}, InvalidMandate, ErrMandate},
		{"no payment.reference constraint", func(w *world, a *autonomous, _ *PayWant) { a.openItem["constraints"] = []any{} }, InvalidMandate, ErrMandate},
		{"a payment.reference to another open mandate", func(w *world, a *autonomous, _ *PayWant) {
			a.openItem["constraints"] = []any{map[string]any{"type": "payment.reference", "conditional_transaction_id": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}}
		}, UnresolvedConstraint, ErrConstraint},
		{"a payment.reference with no open checkout mandate to name", func(w *world, a *autonomous, want *PayWant) { want.OpenCheckout = "" }, UnresolvedConstraint, ErrConstraint},
		{"the wrong audience when one is expected", func(w *world, a *autonomous, want *PayWant) { want.Audience = "someone-else" }, InvalidCredential, ErrAudience},
		{"the wrong nonce when one is expected", func(w *world, a *autonomous, want *PayWant) {
			want.Audience, want.Nonce = "credential-provider", "replayed"
		}, InvalidCredential, ErrNonce},
		{"a preset payee the closed mandate changes", func(w *world, a *autonomous, _ *PayWant) {
			a.openItem["payee"] = map[string]any{"id": "merchant_2", "name": "Another"}
		}, UnresolvedConstraint, ErrConstraint},
		{"a preset amount the closed mandate changes", func(w *world, a *autonomous, _ *PayWant) {
			a.openItem["payment_amount"] = map[string]any{"amount": 400, "currency": "USD"}
		}, UnresolvedConstraint, ErrConstraint},
		{"a preset instrument the closed mandate changes", func(w *world, a *autonomous, _ *PayWant) {
			a.openItem["payment_instrument"] = map[string]any{"id": "pi_2", "type": "card"}
		}, UnresolvedConstraint, ErrConstraint},
		{"the open mandate expired", func(w *world, a *autonomous, _ *PayWant) { a.openItem["exp"] = w.unix(-time.Second) }, InvalidCredential, ErrExpired},
		{"a constraint nobody knows", func(w *world, a *autonomous, _ *PayWant) {
			a.openItem["constraints"] = append(a.openItem["constraints"].([]any), map[string]any{"type": "payment.country", "allowed": []any{"US"}})
		}, UnresolvedConstraint, ErrConstraint},
		{"a known constraint with a member it does not know", func(w *world, a *autonomous, _ *PayWant) {
			a.openItem["constraints"] = append(a.openItem["constraints"].([]any), map[string]any{"type": "payment.amount_range", "currency": "USD", "max": 9999, "per_day": 1})
		}, UnresolvedConstraint, ErrConstraint},
		{"a checkout constraint in a payment mandate", func(w *world, a *autonomous, _ *PayWant) {
			a.openItem["constraints"] = append(a.openItem["constraints"].([]any), map[string]any{"type": "checkout.line_items", "items": []any{}})
		}, UnresolvedConstraint, ErrConstraint},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			a, want := w.payment(t, 500)
			tc.change(w, a, &want)
			_, err := w.verifyPayment(a.text(), want)
			refusedWith(t, err, tc.code, tc.reason)
		})
	}
}

func TestPaymentAudienceAndNonceWhereTheyAreExpected(t *testing.T) {
	w := newWorld(t)
	a, want := w.payment(t, 500)
	want.Audience, want.Nonce = "credential-provider", "nonce-1"
	if _, err := w.verifyPayment(a.text(), want); err != nil {
		t.Fatal(err)
	}
}

func TestPaymentConstraints(t *testing.T) {
	merchant := Merchant{ID: "merchant_1", Name: "Demo Merchant", Website: "https://demo-merchant.example"}
	card := Instrument{ID: "pi_1", Type: "card"}
	spent := func(u Usage) Spent { return func(context.Context, string) (Usage, error) { return u, nil } }
	rng := func(max, min int64, cur string) func(*sd) map[string]any {
		return constraint(map[string]any{"type": "payment.amount_range", "currency": cur, "max": max, "min": min})
	}
	payees := func(ms ...Merchant) func(*sd) map[string]any {
		return func(b *sd) map[string]any {
			var l []any
			for _, m := range ms {
				l = append(l, b.elem(t, m))
			}
			return map[string]any{"type": "payment.allowed_payees", "allowed": l}
		}
	}
	instruments := func(is ...Instrument) func(*sd) map[string]any {
		return func(b *sd) map[string]any {
			var l []any
			for _, i := range is {
				l = append(l, b.elem(t, i))
			}
			return map[string]any{"type": "payment.allowed_payment_instruments", "allowed": l}
		}
	}
	pisps := func(ps ...Pisp) func(*sd) map[string]any {
		return func(b *sd) map[string]any {
			var l []any
			for _, p := range ps {
				l = append(l, p)
			}
			return map[string]any{"type": "payment.allowed_pisps", "allowed": l}
		}
	}
	date := func(from, to string) func(*sd) map[string]any {
		m := map[string]any{"type": "payment.execution_date"}
		if from != "" {
			m["not_before"] = from
		}
		if to != "" {
			m["not_after"] = to
		}
		return constraint(m)
	}
	budget := func(max any, cur string) func(*sd) map[string]any {
		return constraint(map[string]any{"type": "payment.budget", "max": max, "currency": cur})
	}
	recur := func(freq string, max int64) func(*sd) map[string]any {
		return constraint(map[string]any{"type": "payment.agent_recurrence", "frequency": freq, "max_occurrences": max})
	}
	pisp := Pisp{LegalName: "Example Payment Services Ltd.", BrandName: "ExamplePay", DomainName: "examplepay.com"}
	now := newWorld(t).now
	for _, tc := range []struct {
		name  string
		c     func(*sd) map[string]any
		item  map[string]any // members added to the closed mandate
		spent Spent
		ok    bool
	}{
		{"amount in range", rng(1000, 100, "USD"), nil, nil, true},
		{"amount at the maximum", rng(500, 0, "USD"), nil, nil, true},
		{"amount above the maximum", rng(499, 0, "USD"), nil, nil, false},
		{"amount below the minimum", rng(1000, 501, "USD"), nil, nil, false},
		{"amount in another currency", rng(1000, 0, "EUR"), nil, nil, false},
		{"allowed payee", payees(merchant), nil, nil, true},
		{"another payee", payees(Merchant{ID: "other", Name: "Other"}), nil, nil, false},
		{"no payee revealed", payees(), nil, nil, false},
		{"allowed instrument", instruments(card), nil, nil, true},
		{"another instrument of the same id and another type", instruments(Instrument{ID: "pi_1", Type: "UPI"}), nil, nil, false},
		{"no instrument revealed", instruments(), nil, nil, false},
		{"allowed pisp", pisps(pisp), map[string]any{"pisp": pisp}, nil, true},
		{"a pisp that is not allowed", pisps(pisp), map[string]any{"pisp": Pisp{LegalName: "X", BrandName: "X", DomainName: "x.example"}}, nil, false},
		{"no pisp in the closed mandate", pisps(pisp), nil, nil, false},
		{"immediate execution inside a window that is open now", date("2026-01-01", ""), nil, nil, true},
		{"immediate execution before the window opens", date("2099-01-01T00:00:00Z", ""), nil, nil, false},
		{"immediate execution after the window closed", date("", "2026-01-01"), nil, nil, false},
		{"a date inside the window", date("2026-03-31T00:00:00Z", "2026-04-30T23:59:59Z"), map[string]any{"execution_date": "2026-04-15"}, nil, true},
		{"a date before the window", date("2026-03-31T00:00:00Z", "2026-04-30T23:59:59Z"), map[string]any{"execution_date": "2026-03-30T23:59:59Z"}, nil, false},
		{"a date after the window", date("2026-03-31T00:00:00Z", "2026-04-30T23:59:59Z"), map[string]any{"execution_date": "2026-05-01"}, nil, false},
		{"a date that is not a date", date("2026-03-31", ""), map[string]any{"execution_date": "next tuesday"}, nil, false},
		{"within the budget", budget(1000, "USD"), nil, spent(Usage{Total: 500}), true},
		{"over the budget", budget(1000, "USD"), nil, spent(Usage{Total: 501}), false},
		{"a budget in another currency", budget(1000, "EUR"), nil, spent(Usage{}), false},
		{"a budget that is not whole minor units", budget(10.5, "USD"), nil, spent(Usage{}), false},
		{"a budget with no record of spending", budget(1000, "USD"), nil, nil, false},
		{"a budget whose record cannot be read", budget(1000, "USD"), nil, func(context.Context, string) (Usage, error) { return Usage{}, errors.New("down") }, false},
		{"a first monthly payment", recur("MONTHLY", 12), nil, spent(Usage{}), true},
		{"a monthly payment after a month", recur("MONTHLY", 12), nil, spent(Usage{Count: 1, Last: now.AddDate(0, -1, -1)}), true},
		{"a monthly payment within a month", recur("MONTHLY", 12), nil, spent(Usage{Count: 1, Last: now.AddDate(0, 0, -10)}), false},
		{"a payment past max_occurrences", recur("DAILY", 3), nil, spent(Usage{Count: 3, Last: now.AddDate(0, 0, -5)}), false},
		{"the last occurrence", recur("DAILY", 3), nil, spent(Usage{Count: 2, Last: now.AddDate(0, 0, -5)}), true},
		{"on demand, twice in a minute", recur("ON_DEMAND", 5), nil, spent(Usage{Count: 1, Last: now.Add(-time.Minute)}), true},
		{"daily, twice in an hour", recur("DAILY", 5), nil, spent(Usage{Count: 1, Last: now.Add(-time.Hour)}), false},
		{"a frequency nobody defined", recur("HOURLY", 5), nil, spent(Usage{}), false},
		{"recurrence with no record of spending", recur("MONTHLY", 12), nil, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			a, want := w.payment(t, 500, tc.c)
			for k, v := range tc.item {
				a.closedItem[k] = v
			}
			want.Spent = tc.spent
			_, err := w.verifyPayment(a.text(), want)
			if (err == nil) != tc.ok {
				t.Fatalf("err = %v; want ok=%v", err, tc.ok)
			}
			if err != nil {
				refusedWith(t, err, UnresolvedConstraint, ErrConstraint)
			}
		})
	}
}

// The usage a verifier is asked about is the open mandate, by its hash.
func TestSpentIsAskedAboutTheOpenMandate(t *testing.T) {
	w := newWorld(t)
	a, want := w.payment(t, 500, constraint(map[string]any{"type": "payment.budget", "max": 1000, "currency": "USD"}))
	var asked string
	want.Spent = func(_ context.Context, open string) (Usage, error) { asked = open; return Usage{}, nil }
	if _, err := w.verifyPayment(a.text(), want); err != nil {
		t.Fatal(err)
	}
	jwt := strings.SplitN(a.openText(), "~", 2)[0]
	if h := digest(jwt[:strings.LastIndex(jwt, ".")]); asked != h {
		t.Fatalf("asked about %q, want %q (the hash of what the issuer signed)", asked, h)
	}
}
