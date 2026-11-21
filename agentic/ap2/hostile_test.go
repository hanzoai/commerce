// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ap2

import (
	"context"
	"encoding/json"
	"math"
	"math/rand"
	"strings"
	"testing"
	"time"
)

func resignJWT(t *testing.T, hdr jwsHeader, claims map[string]any, k party) string {
	t.Helper()
	raw, _ := json.Marshal(claims)
	j, err := sign(hdr, raw, k.key)
	if err != nil {
		t.Fatal(err)
	}
	return j
}

// Each of these must be refused and move no money.
func TestHostilePresentationsAreRefusedAndMoveNoMoney(t *testing.T) {
	std := func(p *purchase) Completion {
		return Completion{ID: p.issued.ID, Checkout: p.checkout.text(), Payment: p.payment.text(), Token: "spt"}
	}
	for _, tc := range []struct {
		name string
		f    func(s *shop, p *purchase) Completion
	}{
		{"open/open reorder breaks payment.reference", func(s *shop, p *purchase) Completion {
			p.checkout.openOut = reorder(p.checkout.openText())
			p.checkout.resign()
			return std(p)
		}},
		{"swapped mandates", func(s *shop, p *purchase) Completion {
			return Completion{ID: p.issued.ID, Checkout: p.payment.text(), Payment: p.checkout.text(), Token: "spt"}
		}},
		{"float amount", func(s *shop, p *purchase) Completion {
			p.payment.closedItem["payment_amount"] = json.RawMessage(`{"amount":500.0,"currency":"USD"}`)
			return std(p)
		}},
		{"exp amount", func(s *shop, p *purchase) Completion {
			p.payment.closedItem["payment_amount"] = json.RawMessage(`{"amount":5e2,"currency":"USD"}`)
			return std(p)
		}},
		{"huge amount", func(s *shop, p *purchase) Completion {
			p.payment.closedItem["payment_amount"] = json.RawMessage(`{"amount":100000000000000000000,"currency":"USD"}`)
			return std(p)
		}},
		{"string amount", func(s *shop, p *purchase) Completion {
			p.payment.closedItem["payment_amount"] = map[string]any{"amount": "500", "currency": "USD"}
			return std(p)
		}},
		{"checkout jwt of another merchant key", func(s *shop, p *purchase) Completion {
			o := newParty(t, "evil")
			j, _ := parseJWS(p.issued.JWT)
			var claims map[string]any
			json.Unmarshal(j.payload, &claims)
			evil := resignJWT(t, jwsHeader{Alg: "ES256", Typ: "JWT", Kid: "merchant-key-1"}, claims, o)
			h := digest(evil)
			cm := s.checkoutMandate(evil, h)
			return Completion{ID: p.issued.ID, Checkout: cm.text(), Payment: p.payment.text(), Token: "spt"}
		}},
		{"closed token signed by the trusted root, not by cnf", func(s *shop, p *purchase) Completion {
			p.checkout.closed.key = s.root.key
			return std(p)
		}},
		{"closed token alg HS256", func(s *shop, p *purchase) Completion {
			p.checkout.closed.hdr.Alg = "HS256"
			return std(p)
		}},
		{"root alg none", func(s *shop, p *purchase) Completion {
			p.checkout.open.hdr.Alg = "none"
			p.checkout.openOut = ""
			return std(p)
		}},
		{"unknown kid", func(s *shop, p *purchase) Completion {
			p.checkout.open.hdr.Kid = "nope"
			p.checkout.openOut = ""
			return std(p)
		}},
		{"closed typ not kb", func(s *shop, p *purchase) Completion {
			p.checkout.closed.hdr.Typ = "JWT"
			return std(p)
		}},
		{"sd_hash of another open", func(s *shop, p *purchase) Completion {
			p.checkout.bind = func(c map[string]any, open string) { c["sd_hash"] = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" }
			p.checkout.resign()
			return std(p)
		}},
		{"issuer_jwt_hash instead of sd_hash", func(s *shop, p *purchase) Completion {
			p.checkout.bind = func(c map[string]any, open string) {
				h := digest(open)
				c["issuer_jwt_hash"] = h
			}
			p.checkout.resign()
			return std(p)
		}},
		{"disclosure tampered", func(s *shop, p *purchase) Completion {
			parts := strings.Split(p.checkout.text(), "~")
			raw, _ := b64.DecodeString(parts[1])
			parts[1] = b64.EncodeToString([]byte(strings.Replace(string(raw), "credit-500", "credit-501", 1)))
			return Completion{ID: p.issued.ID, Checkout: strings.Join(parts, "~"), Payment: p.payment.text(), Token: "spt"}
		}},
		{"trailing junk after the final ~", func(s *shop, p *purchase) Completion {
			return Completion{ID: p.issued.ID, Checkout: p.checkout.text() + "x", Payment: p.payment.text(), Token: "spt"}
		}},
		{"a three-token chain", func(s *shop, p *purchase) Completion {
			return Completion{ID: p.issued.ID, Checkout: p.checkout.text() + "~" + p.checkout.closedOut, Payment: p.payment.text(), Token: "spt"}
		}},
		{"a payment for another checkout of the store", func(s *shop, p *purchase) Completion {
			i2 := s.issue(t)
			return Completion{ID: i2.ID, Checkout: p.checkout.text(), Payment: p.payment.text(), Token: "spt"}
		}},
		{"a closed payment with a repeated member", func(s *shop, p *purchase) Completion {
			p.payment.closedItem["payee"] = json.RawMessage(`{"id":"merchant_1","name":"x","id":"merchant_2"}`)
			return std(p)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newShop(t)
			p := s.buy(t)
			in := tc.f(s, p)
			out, err := s.m.Complete(context.Background(), s.who, in)
			if err == nil || s.store.count() != 0 {
				t.Fatalf("NOT REFUSED: out=%+v err=%v charges=%d", out, err, s.store.count())
			}
		})
	}
}

// Hostile bytes must neither panic nor hang the verifier, and a mutated mandate is never
// accepted.
func TestMutatedAndHugePresentationsAreRefusedInBoundedTime(t *testing.T) {
	s := newShop(t)
	p := s.buy(t)
	good := p.checkout.text()
	pay := p.payment.text()
	start := time.Now()
	for _, in := range []string{
		strings.Repeat("~", 1<<20), strings.Repeat("a~", 1<<19), strings.Repeat("x.y.z~~", 100000),
		good + good, good + "~", strings.Repeat("[", 1<<19),
	} {
		_, _ = s.m.Complete(context.Background(), s.who, Completion{ID: p.issued.ID, Checkout: in, Payment: pay, Token: "t"})
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("hostile inputs took %s", d)
	}
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 1000; i++ {
		b := []byte(good)
		for k := 0; k < 1+rng.Intn(3); k++ {
			b[rng.Intn(len(b))] = "~.=-_ \"{}[]0aA"[rng.Intn(14)]
		}
		if string(b) == good {
			continue
		}
		if out, err := s.m.Complete(context.Background(), s.who, Completion{ID: p.issued.ID, Checkout: string(b), Payment: pay, Token: "t"}); err == nil {
			t.Fatalf("a mutated mandate was accepted: %q (%+v)", b[:80], out)
		}
	}
}

// The open mandate fixes the payment instrument. encoding/json folds member names (U+017F, the
// long s, folds to s), so a closed mandate can carry a twin of payment_instrument that sorts
// after the real one and is the one the struct takes, while presetClaims compares exact keys.
// A mandate whose members are twins of the schema's is refused, wherever they are.
func TestACaseFoldedTwinOfAMemberIsRefused(t *testing.T) {
	s := newShop(t)
	p := s.buy(t)
	pi1 := map[string]any{"id": "pi_1", "type": "card", "description": "Card 4242"}
	p.payment.openItem["payment_instrument"] = pi1
	p.payment.closedItem["payment_instrument"] = pi1
	p.payment.closedItem["payment_inſtrument"] = map[string]any{"id": "pi_ATTACKER", "type": "card"}
	out, err := s.complete(p)
	refusedWith(t, err, InvalidMandate, ErrMandate)
	if out == nil || s.store.count() != 0 || s.store.instrument != "" {
		t.Fatalf("out = %+v charges %d, the store was told to charge %q", out, s.store.count(), s.store.instrument)
	}

	for name, change := range map[string]func(a *autonomous){
		"a twin of transaction_id": func(a *autonomous) { a.closedItem["tranſaction_id"] = "AAAA" },
		"a twin of payee":          func(a *autonomous) { a.closedItem["PAYEE"] = map[string]any{"id": "x", "name": "x"} },
		"a twin of a member of the payee": func(a *autonomous) {
			a.closedItem["payee"] = map[string]any{"id": "merchant_1", "name": "Demo Merchant", "ID": "merchant_2"}
		},
		"a twin of a member of the amount": func(a *autonomous) {
			a.closedItem["payment_amount"] = map[string]any{"amount": 500, "currency": "USD", "Amount": 1}
		},
		"a twin of pisp": func(a *autonomous) {
			a.closedItem["PISP"] = map[string]any{"legal_name": "x", "brand_name": "x", "domain_name": "x"}
		},
	} {
		w := newWorld(t)
		a, want := w.payment(t, 500)
		change(a)
		_, err := w.verifyPayment(a.text(), want)
		if err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	// ... and of the checkout mandate's members (the Kelvin sign folds to k).
	w := newWorld(t)
	_, jwt, hash := w.checkout("chk_1", 1, 500)
	for _, name := range []string{"checKout_hash", "CHECKOUT_JWT"} {
		a := w.checkoutMandate(jwt, hash)
		a.closedItem[name] = "x"
		if _, err := w.check(a.text()); err == nil {
			t.Errorf("a twin %q accepted", name)
		}
	}
}

// withhold has the agent leave out the disclosures of the open token whose content has any of
// subs.
func withhold(t *testing.T, a *autonomous, subs ...string) {
	t.Helper()
	var keep []string
	dropped := 0
	for _, d := range a.b1.discs[:a.n1] {
		raw, _ := b64.DecodeString(d)
		hit := false
		for _, sub := range subs {
			hit = hit || strings.Contains(string(raw), sub)
		}
		if hit {
			dropped++
			continue
		}
		keep = append(keep, d)
	}
	if dropped == 0 {
		t.Fatalf("no disclosure holds any of %q", subs)
	}
	a.b1.discs, a.n1 = keep, len(keep)
}

// A constraint the agent does not show is not a constraint that is not there. The elements of an
// allowed list are the holder's to withhold (they are meant to be); a constraint, a line item
// requirement, and an optional member of one are not, and a mandate from which any has been left
// out is refused instead of read as a wider mandate.
func TestAConstraintTheAgentWithholdsIsRefused(t *testing.T) {
	rng := func(b *sd) map[string]any {
		return b.elem(t, map[string]any{"type": "payment.amount_range", "currency": "USD", "max": 100})
	}
	optional := func(min int64) func(b *sd) map[string]any {
		return func(b *sd) map[string]any {
			return map[string]any{"type": "payment.amount_range", "currency": "USD", "max": 1000, "_sd": []any{b.prop(t, "min", min)}}
		}
	}
	payees := func(b *sd) map[string]any {
		return map[string]any{"type": "payment.allowed_payees", "allowed": []any{
			b.elem(t, Merchant{ID: "merchant_2", Name: "Another"}), b.elem(t, Merchant{ID: "merchant_1", Name: "Demo Merchant"}),
		}}
	}
	for _, tc := range []struct {
		name string
		more func(b *sd) map[string]any
		drop string
		ok   bool
	}{
		{"a constraint minted as an array element, withheld", rng, `"max":100`, false},
		{"an optional member minted as _sd, withheld", optional(100), `"min"`, false},
		{"an optional member minted as _sd, shown", optional(100), "", true},
		{"an element of an allowed list, withheld, the other matches", payees, "Another", true},
		{"the element of an allowed list that matches, withheld", payees, "merchant_1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			a, want := w.payment(t, 500, tc.more)
			if tc.drop != "" {
				withhold(t, a, tc.drop)
			}
			_, err := w.verifyPayment(a.text(), want)
			if (err == nil) != tc.ok {
				t.Fatalf("err = %v; want ok=%v", err, tc.ok)
			}
			if err != nil {
				refusedWith(t, err, UnresolvedConstraint, ErrConstraint)
			}
		})
	}

	// A checkout mandate's constraints, each minted as an element, the agent leaving out one and
	// the disclosures under it.
	for name, drop := range map[string][]string{
		"checkout.line_items":        {"checkout.line_items", "credit-500"},
		"checkout.allowed_merchants": {"checkout.allowed_merchants", "merchant_1"},
	} {
		w := newWorld(t)
		_, jwt, hash := w.checkout("chk_1", 1, 500)
		a := w.checkoutMandate(jwt, hash)
		a.constraints(func(b *sd) []any {
			return []any{
				b.elem(t, w.lineItems(b, 1, Item{"credit-500", "x"})),
				b.elem(t, w.allowedMerchants(b, Merchant{ID: "merchant_1", Name: "Demo Merchant"})),
			}
		})
		withhold(t, a, drop...)
		_, err := w.check(a.text())
		if err == nil {
			t.Errorf("%s withheld: the mandate was accepted", name)
			continue
		}
		refusedWith(t, err, UnresolvedConstraint, ErrConstraint)
	}
}

// A payment is of something, and sums do not wrap.
func TestPaymentAmountsAreAboveZeroAndSumsDoNotWrap(t *testing.T) {
	spent := func(u Usage) Spent { return func(context.Context, string) (Usage, error) { return u, nil } }
	budget := func(b *sd) map[string]any {
		return map[string]any{"type": "payment.budget", "currency": "USD", "max": 500}
	}
	rng := func(b *sd) map[string]any {
		return map[string]any{"type": "payment.amount_range", "currency": "USD", "max": 500}
	}
	recur := func(b *sd) map[string]any {
		return map[string]any{"type": "payment.agent_recurrence", "frequency": "ON_DEMAND", "max_occurrences": 5}
	}
	for _, tc := range []struct {
		name   string
		amount int64
		more   func(b *sd) map[string]any
		spent  Usage
		ok     bool
	}{
		{"a negative amount within amount_range max", -1000, rng, Usage{}, false},
		{"a zero amount", 0, rng, Usage{}, false},
		{"a budget sum that wraps", math.MaxInt64, budget, Usage{Total: 1}, false},
		{"a budget already overspent by a negative total", 100, budget, Usage{Total: -1}, false},
		{"a budget exactly spent", 500, budget, Usage{}, true},
		{"a budget one over", 500, budget, Usage{Total: 1}, false},
		{"a recurrence count at the limit of an int64", 100, recur, Usage{Count: math.MaxInt64}, false},
		{"a recurrence count below zero", 100, recur, Usage{Count: -1}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			a, want := w.payment(t, tc.amount, tc.more)
			want.Spent = spent(tc.spent)
			_, err := w.verifyPayment(a.text(), want)
			if (err == nil) != tc.ok {
				t.Fatalf("err = %v; want ok=%v", err, tc.ok)
			}
		})
	}
}

// A date is whole seconds compared as they are, so none wraps: one at the end of an int64 is the
// far future, or the far past, and is neither a panic nor a pass.
func TestDatesAtTheEdgeOfAnInt64DoNotWrap(t *testing.T) {
	for _, tc := range []struct {
		claim  string
		n      int64
		reason error
	}{
		{"nbf", math.MaxInt64, ErrNotYet}, {"iat", math.MaxInt64 - 1000, ErrNotYet}, {"nbf", math.MinInt64, nil},
		{"exp", math.MinInt64, ErrExpired}, {"exp", math.MaxInt64, nil},
	} {
		w := newWorld(t)
		_, jwt, hash := w.checkout("chk_1", 1, 500)
		a := w.checkoutMandate(jwt, hash)
		a.closedItem[tc.claim] = tc.n
		_, err := w.check(a.text())
		if tc.reason == nil {
			if err != nil {
				t.Errorf("%s %d: %v", tc.claim, tc.n, err)
			}
			continue
		}
		refusedWith(t, err, InvalidCredential, tc.reason)
	}
	w := newWorld(t)
	_, jwt, hash := w.checkout("chk_1", 1, 500)
	a := w.checkoutMandate(jwt, hash)
	a.closedItem["exp"] = 1.5
	_, err := w.check(a.text())
	refusedWith(t, err, InvalidCredential, ErrMalformed)
}
