// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package gateway

import (
	"context"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/luxfi/crypto"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/remittance"
	"github.com/hanzoai/commerce/payment/providers/circle"
)

// ── the happy path, end to end ───────────────────────────────────────────────

func TestAPaymentIsAcceptedAndCreditedOnce(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	sub := r.pay(2500, nil) // $25.00

	v, err := r.submit(sub)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if v.Status != remittance.Accepted || !v.Credited || v.AmountCents != 2500 || v.Payment == "" {
		t.Fatalf("view = %+v, want accepted, credited, 2500 cents, with a payment id", v)
	}
	// Gateway locked the payer's money.
	if p := r.gw.payment(v.Payment); p.Amount != "25000000" || p.To != treasury || p.From != r.from {
		t.Fatalf("gateway holds %+v", p)
	}

	// The host ledger got ONE cash credit at the caller's address.
	posted := r.book.posted()
	if len(posted) != 1 {
		t.Fatalf("%d credits posted, want 1", len(posted))
	}
	c := posted[0]
	if c.Org != strings.ToLower(r.org.Name) || c.Subject != r.payer || c.AmountCents != 2500 || !c.Cash ||
		c.Test || c.Tag != "topup" || c.Currency != "usd" || c.IdempotencyKey != "remittance:"+v.ID {
		t.Fatalf("credit = %+v", c)
	}
	// And commerce's own receipt — the balance standalone — is real money.
	split := r.receipts()
	if int64(split.Balance) != 2500 || int64(split.PrepaidBalance) != 2500 || split.CreditsRemaining != 0 {
		t.Fatalf("receipts: balance %d prepaid %d credits %d, want 2500 prepaid", split.Balance, split.PrepaidBalance, split.CreditsRemaining)
	}

	// Read back.
	got, err := r.s.Get(context.Background(), r.org, r.payer, v.ID)
	if err != nil || got.Status != remittance.Accepted || got.ID != v.ID {
		t.Fatalf("get = %+v, %v", got, err)
	}
	// What Gateway received carried the canonical signature (v = 27/28) and
	// the record keeps the canonical spellings.
	rec := r.stored(sub)
	if !strings.HasSuffix(rec.Signature, "1b") && !strings.HasSuffix(rec.Signature, "1c") {
		t.Fatalf("stored signature %s does not end in v=27/28", rec.Signature)
	}
	if rec.From != r.from || rec.To != treasury || rec.Contract != wallet || rec.Asset != usdc {
		t.Fatalf("record = %+v", rec)
	}
}

func TestTermsTellAPayerEverythingToSign(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	tm := r.terms()
	if tm.PayTo != treasury || tm.Scheme != "exact" || tm.Version != 2 || tm.Primary != "TransferWithAuthorization" {
		t.Fatalf("terms = %+v", tm)
	}
	if len(tm.Accepts) != 1 {
		t.Fatalf("accepts = %+v", tm.Accepts)
	}
	a := tm.Accepts[0]
	if a.Network != caip || a.Asset != usdc || a.Decimals != 6 || a.MinValidity != int64(week/time.Second) ||
		a.MinAmount != "1000000" || a.MaxAmount != "1000000000" ||
		a.Domain != (circle.Domain{Name: "GatewayWalletBatched", Version: "1", ChainID: 84532, VerifyingContract: wallet}) {
		t.Fatalf("accept = %+v", a)
	}
	if len(tm.Types["TransferWithAuthorization"]) != 6 || len(tm.Types["EIP712Domain"]) != 4 {
		t.Fatalf("types = %+v", tm.Types)
	}
	// Mutating what a payer was handed cannot change what is verified.
	tm.Types["TransferWithAuthorization"][0].Name = "attacker"
	if circle.Types()["TransferWithAuthorization"][0].Name != "from" {
		t.Fatal("the type table a caller received is the one digests are computed from")
	}
	// Every call issues a fresh nonce, bound to the caller.
	again := r.terms()
	if again.Nonce == tm.Nonce || again.Salt == tm.Salt {
		t.Fatal("two calls issued the same nonce")
	}
	salt, _ := circle.Nonce(tm.Salt)
	if want := bind(r.org.Name, r.payer, salt); circle.Hex(want[:]) != tm.Nonce {
		t.Fatal("the nonce is not bound to the caller")
	}
}

// ── refusals before anything is sent ─────────────────────────────────────────

// Each of these is refused by the rail itself: Gateway is never asked, no
// remittance is recorded, nothing is credited.
func TestRefusedBeforeGatewayIsAsked(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	other, _ := crypto.GenerateKey()

	cases := []struct {
		name   string
		reason string
		make   func() Submission
	}{
		{"wrong recipient", ReasonRecipient, func() Submission {
			return r.pay(500, func(a *circle.Authorization) { a.To = "0x" + strings.Repeat("99", 20) })
		}},
		{"expired", ReasonExpired, func() Submission {
			return r.pay(500, func(a *circle.Authorization) { a.ValidBefore = strconv.FormatInt(r.now.Add(-time.Second).Unix(), 10) })
		}},
		{"expires now", ReasonExpired, func() Submission {
			return r.pay(500, func(a *circle.Authorization) { a.ValidBefore = strconv.FormatInt(r.now.Unix(), 10) })
		}},
		{"not yet valid", ReasonEarly, func() Submission {
			return r.pay(500, func(a *circle.Authorization) { a.ValidAfter = strconv.FormatInt(r.now.Add(time.Second).Unix(), 10) })
		}},
		{"validity shorter than Gateway's floor", ReasonShort, func() Submission {
			return r.pay(500, func(a *circle.Authorization) {
				a.ValidBefore = strconv.FormatInt(r.now.Add(week-time.Second).Unix(), 10)
			})
		}},
		{"validity past the ceiling", ReasonLong, func() Submission {
			return r.pay(500, func(a *circle.Authorization) {
				a.ValidBefore = strconv.FormatInt(r.now.Add(MaxValidity+time.Second).Unix(), 10)
			})
		}},
		{"below minimum", ReasonLow, func() Submission { return r.pay(MinCents-1, nil) }},
		{"zero", ReasonLow, func() Submission { return r.pay(0, nil) }},
		{"above maximum", ReasonHigh, func() Submission { return r.pay(MaxCents+1, nil) }},
		{"not whole cents", ReasonCents, func() Submission {
			return r.pay(500, func(a *circle.Authorization) { a.Value = "5000001" })
		}},
		{"a value past uint64", ReasonHigh, func() Submission {
			return r.pay(500, func(a *circle.Authorization) {
				a.Value = new(big.Int).Mul(new(big.Int).Lsh(big.NewInt(1), 200), big.NewInt(10_000)).String()
			})
		}},
		{"unsupported network", ReasonNetwork, func() Submission {
			s := r.pay(500, nil)
			s.Network = "eip155:8453"
			return s
		}},
		{"malformed authorization", ReasonPayload, func() Submission {
			s := r.pay(500, nil)
			s.Authorization.Value = "5e6"
			return s
		}},
		{"signed by someone else", ReasonSignature, func() Submission {
			tm := r.terms()
			s := r.sign(tm, 500, nil)
			s.Signature = r.signWith(other, tm.Accepts[0].Domain, s.Authorization)
			return s
		}},
		{"signature over different terms", ReasonSignature, func() Submission {
			s := r.pay(500, nil)
			s.Authorization.Value = "6000000"
			return s
		}},
		{"signature for another chain", ReasonSignature, func() Submission {
			tm := r.terms()
			s := r.sign(tm, 500, nil)
			d := tm.Accepts[0].Domain
			d.ChainID = 8453
			s.Signature = r.signWith(r.key, d, s.Authorization)
			return s
		}},
		{"garbage signature", ReasonSignature, func() Submission {
			s := r.pay(500, nil)
			s.Signature = "0x" + strings.Repeat("00", 65)
			return s
		}},
		{"a nonce the rail never issued", ReasonUnbound, func() Submission {
			return r.pay(500, func(a *circle.Authorization) { a.Nonce = random32() })
		}},
		{"the right nonce with the wrong salt", ReasonUnbound, func() Submission {
			s := r.pay(500, nil)
			s.Salt = random32()
			return s
		}},
		{"no salt", ReasonUnbound, func() Submission {
			s := r.pay(500, nil)
			s.Salt = ""
			return s
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before, _ := r.gw.count()
			sub := tc.make()
			_, err := r.submit(sub)
			if err == nil {
				t.Fatal("accepted")
			}
			if ReasonOf(err) != tc.reason || StatusOf(err) != http.StatusBadRequest {
				t.Fatalf("refused as %d %q (%v), want 400 %q", StatusOf(err), ReasonOf(err), err, tc.reason)
			}
			if after, _ := r.gw.count(); after != before {
				t.Fatal("Gateway was asked to settle a payment the rail refused")
			}
			if sub.Authorization.From != "" && sub.Network != "" && strings.HasPrefix(sub.Authorization.Nonce, "0x") && r.exists(sub) {
				t.Fatal("a refused payment was recorded")
			}
		})
	}
	if len(r.book.posted()) != 0 {
		t.Fatal("a refused payment was credited")
	}
}

// A payment issued to one caller cannot be presented by another, whoever
// captured it: the nonce binds the caller, and the signature binds the nonce.
func TestAPaymentCannotBePresentedByAnotherCaller(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	sub := r.pay(1000, nil)

	thief := &organization.Organization{}
	thief.Name, thief.Live = "thief"+r.org.Name, true
	for _, who := range []struct {
		org   *organization.Organization
		payer string
	}{
		{thief, thief.Name},       // another org
		{r.org, r.payer + "/bob"}, // another member of the same org
	} {
		_, err := r.s.Submit(context.Background(), who.org, who.payer, sub)
		if ReasonOf(err) != ReasonUnbound {
			t.Fatalf("%s/%s presented another caller's payment: %v", who.org.Name, who.payer, err)
		}
	}
	if settles, _ := r.gw.count(); settles != 0 {
		t.Fatal("Gateway was asked to settle a captured payment")
	}
	// The real caller still can.
	if v, err := r.submit(sub); err != nil || v.Status != remittance.Accepted {
		t.Fatalf("owner: %+v, %v", v, err)
	}
}

func TestTheRailIsShutUnlessArmedAndBooksMatch(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	sub := r.pay(500, nil)

	shutRail := New(circle.Config{}, nil)
	if _, err := shutRail.Terms(context.Background(), r.org, r.payer); ReasonOf(err) != ReasonClosed || StatusOf(err) != 503 {
		t.Fatalf("unarmed terms: %v", err)
	}
	if _, err := shutRail.Submit(context.Background(), r.org, r.payer, sub); ReasonOf(err) != ReasonClosed || StatusOf(err) != 503 {
		t.Fatalf("unarmed submit: %v", err)
	}
	if shutRail.Networks(context.Background()) != nil {
		t.Fatal("an unarmed rail offers networks")
	}

	// Mainnet USDC is real money: it funds LIVE books only.
	test := &organization.Organization{}
	test.Name, test.Live = r.org.Name, false
	if _, err := r.s.Submit(context.Background(), test, r.payer, sub); ReasonOf(err) != ReasonBooks || StatusOf(err) != 503 {
		t.Fatalf("mainnet into test books: %v", err)
	}

	// Testnet USDC is free: it funds TEST books only.
	tr := newRig(t, circle.Testnet, true)
	if _, err := tr.s.Terms(context.Background(), tr.org, tr.payer); ReasonOf(err) != ReasonBooks {
		t.Fatalf("testnet into live books: %v", err)
	}
	tt := newRig(t, circle.Testnet, false)
	v, err := tt.submit(tt.pay(500, nil))
	if err != nil || v.Status != remittance.Accepted {
		t.Fatalf("testnet into test books: %+v %v", v, err)
	}
	if c := tt.book.posted(); len(c) != 1 || !c[0].Test {
		t.Fatalf("testnet credit = %+v, want one credit in the TEST books", c)
	}

	// No caller.
	if _, err := r.s.Submit(context.Background(), nil, "", sub); StatusOf(err) != 401 {
		t.Fatalf("anonymous: %v", err)
	}
	if settles, _ := r.gw.count(); settles != 0 {
		t.Fatal("Gateway was asked to settle")
	}
}

// ── Gateway's own answers ────────────────────────────────────────────────────

func TestGatewaysRefusalIsRecordedAndNothingIsCredited(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	r.gw.set(func(f *fake) { f.held[r.from] = 0 })
	sub := r.pay(2000, nil)

	_, err := r.submit(sub)
	if ReasonOf(err) != "insufficient_balance" || StatusOf(err) != 400 {
		t.Fatalf("submit: %v", err)
	}
	if rec := r.stored(sub); rec.Status != remittance.Refused || rec.Reason != "insufficient_balance" || rec.Credited {
		t.Fatalf("record = %+v", rec)
	}
	if len(r.book.posted()) != 0 {
		t.Fatal("credited a refused payment")
	}
	// The reconciler leaves a refused payment alone.
	r.s.Reconcile(context.Background())
	if settles, _ := r.gw.count(); settles != 1 {
		t.Fatalf("the reconciler re-sent a refused payment (%d settles)", settles)
	}

	// The payer deposits and presents the SAME signed payment again.
	r.gw.fund(r.from, 2000)
	v, err := r.submit(sub)
	if err != nil || v.Status != remittance.Accepted || !v.Credited {
		t.Fatalf("second presentation: %+v %v", v, err)
	}
	if len(r.book.posted()) != 1 {
		t.Fatalf("%d credits", len(r.book.posted()))
	}
}

func TestGatewayRejectingTheRequestIsARefusal(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	r.gw.set(func(f *fake) { f.down = http.StatusBadRequest })
	sub := r.pay(500, nil)
	_, err := r.submit(sub)
	if ReasonOf(err) != ReasonRejected || StatusOf(err) != 400 {
		t.Fatalf("submit: %v", err)
	}
	if r.stored(sub).Status != remittance.Refused {
		t.Fatal("not recorded as refused")
	}
}

// A 5xx, a 429 and a timeout say nothing about whether the money moved. The
// payment stays pending, is not credited, and resolves to exactly one credit
// once Gateway shows it.
func TestAnUnknownOutcomeIsNeverRefusedAndResolvesOnce(t *testing.T) {
	for _, tc := range []struct {
		name  string
		knobs func(*fake)
		ctx   func() (context.Context, context.CancelFunc)
		moved bool
	}{
		{"accepted then 500", func(f *fake) { f.lost = 500 }, nil, true},
		{"accepted then 502", func(f *fake) { f.lost = 502 }, nil, true},
		{"500 having done nothing", func(f *fake) { f.down = 500 }, nil, false},
		{"429 having done nothing", func(f *fake) { f.down = 429 }, nil, false},
		{"408 having done nothing", func(f *fake) { f.down = 408 }, nil, false},
		{"accepted then too slow", func(f *fake) { f.delay = 300 * time.Millisecond }, func() (context.Context, context.CancelFunc) {
			return context.WithTimeout(context.Background(), 50*time.Millisecond)
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newRig(t, circle.Mainnet, true)
			r.gw.set(tc.knobs)
			sub := r.pay(1500, nil)
			ctx := context.Background()
			if tc.ctx != nil {
				c, cancel := tc.ctx()
				defer cancel()
				ctx = c
			}
			v, err := r.s.Submit(ctx, r.org, r.payer, sub)
			if err != nil {
				t.Fatalf("an unknown outcome was answered as an error: %v", err)
			}
			if v.Status != remittance.Pending || v.Credited {
				t.Fatalf("view = %+v, want pending and uncredited", v)
			}
			if len(r.book.posted()) != 0 {
				t.Fatal("credited before Gateway showed the payment")
			}
			_, payments := r.gw.count()
			if (payments == 1) != tc.moved {
				t.Fatalf("%d payments at Gateway", payments)
			}

			// Gateway recovers; the reconciler finds (or sends) it.
			r.gw.set(func(f *fake) { f.lost, f.down, f.delay = 0, 0, 0 })
			r.s.Reconcile(context.Background())
			r.s.Reconcile(context.Background())
			rec := r.stored(sub)
			if rec.Status != remittance.Accepted || !rec.Credited {
				t.Fatalf("after reconcile: %+v", rec)
			}
			if c := r.book.posted(); len(c) != 1 || c[0].AmountCents != 1500 {
				t.Fatalf("credits = %+v, want one of 1500", c)
			}
			if _, payments := r.gw.count(); payments != 1 {
				t.Fatalf("Gateway holds %d payments, want 1", payments)
			}
		})
	}
}

func TestPresentingTheSamePaymentAgainIsOnePayment(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	sub := r.pay(700, nil)
	first, err := r.submit(sub)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		again, err := r.submit(sub)
		if err != nil || again.ID != first.ID || again.Status != remittance.Accepted {
			t.Fatalf("presentation %d: %+v %v", i+2, again, err)
		}
	}
	// Presented again two days later, when it would no longer pass the window
	// as a new payment, it is still answered with its record.
	r.now = r.now.Add(48 * time.Hour)
	if again, err := r.submit(sub); err != nil || again.ID != first.ID || again.Status != remittance.Accepted {
		t.Fatalf("two days later: %+v %v", again, err)
	}
	r.now = r.now.Add(-48 * time.Hour)
	// The same payment with v spelled 27/28 instead of 0/1 is still the same.
	sig, _ := circle.Canonical(sub.Signature)
	sub.Signature = sig
	if again, err := r.submit(sub); err != nil || again.ID != first.ID {
		t.Fatalf("canonical spelling: %+v %v", again, err)
	}
	if settles, _ := r.gw.count(); settles != 1 {
		t.Fatalf("Gateway asked %d times, want 1", settles)
	}
	if len(r.book.posted()) != 1 || r.receipts().Balance != 700 {
		t.Fatalf("credits %d, receipts %d", len(r.book.posted()), r.receipts().Balance)
	}
}

// The same nonce on a DIFFERENT payment is a conflict, never a replay of the
// first one's answer.
func TestReusingANonceForAnotherPaymentConflicts(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	tm := r.terms()
	if _, err := r.submit(r.sign(tm, 700, nil)); err != nil {
		t.Fatal(err)
	}
	_, err := r.submit(r.sign(tm, 900, nil))
	if ReasonOf(err) != ReasonConflict || StatusOf(err) != 409 {
		t.Fatalf("a second payment under one nonce: %v", err)
	}
	if settles, _ := r.gw.count(); settles != 1 || len(r.book.posted()) != 1 {
		t.Fatal("the conflicting payment reached Gateway or the ledger")
	}
}

// Many presentations of one payment at once, as from a retrying agent or a
// double-clicking human: Gateway accepts it once and the caller is credited once.
func TestConcurrentPresentationsOfOnePaymentCreditOnce(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	sub := r.pay(3300, nil)

	const n = 16
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// A second rail value is a second replica over the same store.
			s := r.s
			if i%2 == 1 {
				s = New(r.s.cfg, circle.New(r.server.URL))
				s.now = r.s.now
			}
			if _, err := s.Submit(context.Background(), r.org, r.payer, sub); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("a concurrent presentation failed: %v", err)
	}
	r.s.Reconcile(context.Background())

	if _, payments := r.gw.count(); payments != 1 {
		t.Fatalf("Gateway accepted %d payments, want 1", payments)
	}
	if c := r.book.posted(); len(c) != 1 || c[0].AmountCents != 3300 {
		t.Fatalf("credits = %+v, want one of 3300", c)
	}
	if got := int64(r.receipts().Balance); got != 3300 {
		t.Fatalf("receipts = %d cents, want 3300", got)
	}
	if rec := r.stored(sub); rec.Status != remittance.Accepted || !rec.Credited {
		t.Fatalf("record = %+v", rec)
	}
}

// The payer handed the same signed payment straight to Gateway first: the
// money is in the treasury all the same, so presenting it here credits it.
func TestAPaymentGatewayAlreadyHoldsIsAdopted(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	sub := r.pay(1200, nil)
	direct := New(r.s.cfg, circle.New(r.server.URL))
	sig, _ := circle.Canonical(sub.Signature)
	if st, err := direct.client.Settle(context.Background(), circle.Resource{URL: "x", Description: "x", MimeType: "x"},
		circle.Requirements{Scheme: "exact", Network: caip, Asset: usdc, Amount: sub.Authorization.Value, PayTo: treasury,
			Extra: circle.Signing{Name: circle.Name, Version: circle.Release, VerifyingContract: wallet}},
		circle.Payload{Authorization: sub.Authorization, Signature: sig}); err != nil || !st.Success {
		t.Fatalf("direct settle: %+v %v", st, err)
	}
	v, err := r.submit(sub)
	if err != nil || v.Status != remittance.Accepted || !v.Credited {
		t.Fatalf("submit: %+v %v", v, err)
	}
	if len(r.book.posted()) != 1 {
		t.Fatal("not credited exactly once")
	}
}

// Gateway accepted, the answer was lost, and Gateway's search lags: a re-send
// is refused (nonce used, or the balance it locked is gone) — and that refusal
// must NOT be believed, or a paid payment is never credited.
func TestARefusalOfAReSendIsNotBelieved(t *testing.T) {
	for _, reason := range []string{"nonce_already_used", "insufficient_balance", "authorization_validity_too_short"} {
		t.Run(reason, func(t *testing.T) {
			r := newRig(t, circle.Mainnet, true)
			r.gw.set(func(f *fake) { f.lost = 500 })
			sub := r.pay(800, nil)
			if v, err := r.submit(sub); err != nil || v.Status != remittance.Pending {
				t.Fatalf("submit: %+v %v", v, err)
			}
			// Search lags, and Gateway now refuses every send with reason.
			r.gw.set(func(f *fake) { f.lost, f.hide = 0, 4; f.reason = reason })
			r.s.Reconcile(context.Background())
			if rec := r.stored(sub); rec.Status != remittance.Pending || rec.Credited {
				t.Fatalf("after a refused re-send: %+v, want still pending", rec)
			}
			// Presenting it again says the same.
			if v, err := r.submit(sub); err != nil || v.Status != remittance.Pending {
				t.Fatalf("re-presented: %+v %v", v, err)
			}
			// Search catches up.
			r.gw.set(func(f *fake) { f.hide, f.reason = 0, "" })
			r.s.Reconcile(context.Background())
			if rec := r.stored(sub); rec.Status != remittance.Accepted || !rec.Credited {
				t.Fatalf("after search caught up: %+v", rec)
			}
			if len(r.book.posted()) != 1 {
				t.Fatal("not credited exactly once")
			}
		})
	}
}

// ── after acceptance ─────────────────────────────────────────────────────────

func TestTheReconcilerFollowsAPaymentToSettlement(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	v, err := r.submit(r.pay(1000, nil))
	if err != nil {
		t.Fatal(err)
	}
	r.gw.mark(v.Payment, circle.Batched)
	r.s.Reconcile(context.Background())
	if got, _ := r.s.Get(context.Background(), r.org, r.payer, v.ID); got.Status != remittance.Accepted {
		t.Fatalf("batched: %+v", got)
	}
	r.gw.mark(v.Payment, circle.Confirmed)
	r.s.Reconcile(context.Background())
	got, _ := r.s.Get(context.Background(), r.org, r.payer, v.ID)
	if got.Status != remittance.Settled || got.TxHash == "" || !got.Credited {
		t.Fatalf("confirmed: %+v", got)
	}
	// Settled is final, and nothing re-credits.
	r.s.Reconcile(context.Background())
	if len(r.book.posted()) != 1 {
		t.Fatal("credited twice")
	}
}

func TestAPaymentGatewayFailsAfterAcceptingIsReversedOnce(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	v, err := r.submit(r.pay(4000, nil))
	if err != nil || !v.Credited {
		t.Fatalf("submit: %+v %v", v, err)
	}
	r.gw.mark(v.Payment, circle.Failed)
	for i := 0; i < 3; i++ {
		r.s.Reconcile(context.Background())
	}
	got, _ := r.s.Get(context.Background(), r.org, r.payer, v.ID)
	if got.Status != remittance.Failed || got.Credited {
		t.Fatalf("view = %+v, want failed and not credited", got)
	}
	if r.book.taken() != 1 || r.credited() != 0 {
		t.Fatalf("clawbacks %d, ledger balance %d — want one clawback to zero", r.book.taken(), r.credited())
	}
	if got := int64(r.receipts().Balance); got != 0 {
		t.Fatalf("receipts = %d cents after the reversal, want 0", got)
	}
}

// A reversal is taken once however often it is asked for, by the flag on the
// remittance and not only by the ledger's own key.
func TestAReversalIsTakenOnce(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	v, err := r.submit(r.pay(4000, nil))
	if err != nil {
		t.Fatal(err)
	}
	rec, err := r.s.fail(context.Background(), r.byID(v.ID), "failed")
	if err != nil || !rec.Reversed {
		t.Fatalf("fail: %+v %v", rec, err)
	}
	for i := 0; i < 3; i++ {
		r.s.reverse(context.Background(), r.byID(v.ID))
	}
	if r.book.calls != 1 {
		t.Fatalf("the ledger was asked to claw back %d times, want 1", r.book.calls)
	}
}

func TestACreditTheLedgerMissedIsRetried(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	r.book.broken = true
	v, err := r.submit(r.pay(600, nil))
	if err != nil || v.Status != remittance.Accepted || v.Credited {
		t.Fatalf("with the ledger down: %+v %v — want accepted and NOT credited", v, err)
	}
	r.s.Reconcile(context.Background())
	if r.byID(v.ID).Credited {
		t.Fatal("credited while the ledger was down")
	}
	r.book.mu.Lock()
	r.book.broken = false
	r.book.mu.Unlock()
	r.s.Reconcile(context.Background())
	r.s.Reconcile(context.Background())
	if c := r.book.posted(); len(c) != 1 || c[0].AmountCents != 600 {
		t.Fatalf("credits = %+v", c)
	}
	if got := int64(r.receipts().Balance); got != 600 {
		t.Fatalf("receipts = %d", got)
	}
}

func (r *rig) byID(id string) *remittance.Remittance {
	r.t.Helper()
	rec, err := load(id)
	if err != nil {
		r.t.Fatal(err)
	}
	return rec
}

// A payment Gateway never showed, past its window: failed, never credited.
func TestAPaymentNeverSeenExpiresAsFailed(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	r.gw.set(func(f *fake) { f.down = 503 })
	sub := r.pay(900, nil)
	if v, err := r.submit(sub); err != nil || v.Status != remittance.Pending {
		t.Fatalf("submit: %+v %v", v, err)
	}
	r.s.Reconcile(context.Background()) // still down: stays pending
	before, _ := strconv.ParseInt(sub.Authorization.ValidBefore, 10, 64)
	sent, _ := r.gw.count()
	r.now = time.Unix(before, 0).Add(30 * time.Minute) // past, inside the grace
	r.s.Reconcile(context.Background())
	if rec := r.stored(sub); rec.Status != remittance.Pending {
		t.Fatalf("inside the grace: %+v", rec)
	}
	if again, _ := r.gw.count(); again != sent {
		t.Fatalf("a payment past its validBefore was sent to Gateway again (%d settles, was %d)", again, sent)
	}
	r.now = time.Unix(before, 0).Add(2 * time.Hour)
	r.s.Reconcile(context.Background())
	if rec := r.stored(sub); rec.Status != remittance.Failed || rec.Reason != "expired" || rec.Credited {
		t.Fatalf("after the grace: %+v", rec)
	}
	if len(r.book.posted()) != 0 {
		t.Fatal("credited")
	}
}

// ── reads ────────────────────────────────────────────────────────────────────

func TestAnotherCallersPaymentReadsAsMissing(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	v, err := r.submit(r.pay(500, nil))
	if err != nil {
		t.Fatal(err)
	}
	other := &organization.Organization{}
	other.Name, other.Live = "other"+r.org.Name, true
	for _, tc := range []struct {
		org   *organization.Organization
		payer string
		id    string
	}{
		{other, other.Name, v.ID},
		{r.org, r.payer + "/bob", v.ID},
		{r.org, r.payer, "rem_" + strings.Repeat("0", 32)},
		{r.org, r.payer, "not-an-id"},
	} {
		_, err := r.s.Get(context.Background(), tc.org, tc.payer, tc.id)
		if ReasonOf(err) != ReasonMissing || StatusOf(err) != 404 || err.Error() != errMissing.Error() {
			t.Fatalf("%s/%s %s: %v", tc.org.Name, tc.payer, tc.id, err)
		}
	}
}

func TestTreasuryReportsGatewaysFigures(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	if _, err := r.submit(r.pay(1000, nil)); err != nil {
		t.Fatal(err)
	}
	tr, err := r.s.Treasury(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if tr.Address != treasury || tr.Network != "mainnet" || tr.Total != "12.500001" || len(tr.Balances) != 2 ||
		tr.Balances[0].Chain != "Ethereum Sepolia" || tr.Balances[1].Chain != "Base Sepolia" {
		t.Fatalf("treasury = %+v", tr)
	}
	if len(tr.Arriving) != 1 || tr.Arriving[0].Amount != "2.500000" {
		t.Fatalf("arriving = %+v", tr.Arriving)
	}
	if tr.Credited < 1000 || tr.Open < 1 {
		t.Fatalf("credited %d open %d", tr.Credited, tr.Open)
	}
}

func TestDecimalIsExact(t *testing.T) {
	for in, want := range map[string]string{"12.5": "12500000", "0.000001": "1", "7": "7000000", "0.0": "0"} {
		got, err := decimal(in, 6)
		if err != nil || got.String() != want {
			t.Errorf("decimal(%q) = %v, %v; want %s", in, got, err, want)
		}
	}
	for _, bad := range []string{"", ".5", "1.0000001", "-1", "1e6", "1.2.3", "abc"} {
		if _, err := decimal(bad, 6); err == nil {
			t.Errorf("decimal(%q) parsed", bad)
		}
	}
	if format(big.NewInt(1), 6) != "0.000001" || format(big.NewInt(12500000), 6) != "12.500000" {
		t.Fatal("format")
	}
}

// ── Gateway's network table ──────────────────────────────────────────────────

func TestOnlyNetworksThisRailCanVerifyAreOffered(t *testing.T) {
	good := circle.Extra{Name: circle.Name, Version: "1", VerifyingContract: wallet, MinValidity: 604800,
		Assets: []circle.Asset{{Symbol: "USDC", Address: usdc, Decimals: 6, Versions: []string{"1", "2"}}}}
	kind := func(id string, edit func(*circle.Extra)) circle.Kind {
		e := good
		e.Assets = append([]circle.Asset(nil), good.Assets...)
		if edit != nil {
			edit(&e)
		}
		return circle.Kind{Version: 2, Scheme: "exact", Network: id, Extra: e}
	}
	nets := readNetworks([]circle.Kind{
		kind("eip155:8453", nil),
		kind("eip155:1", nil),
		{Version: 2, Scheme: "upto", Network: "eip155:10", Extra: good},
		kind("solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp", nil),
		kind("eip155:0", nil),
		kind("eip155:01", nil),
		kind("eip155:abc", nil),
		kind("eip155:137", func(e *circle.Extra) { e.Name = "GatewayWallet" }),
		kind("eip155:42161", func(e *circle.Extra) { e.Version = "2" }),
		kind("eip155:43114", func(e *circle.Extra) { e.VerifyingContract = "0x1234" }),
		kind("eip155:130", func(e *circle.Extra) { e.Assets[0].Decimals = 18 }),
		kind("eip155:146", func(e *circle.Extra) { e.Assets[0].Versions = []string{"2"} }),
		kind("eip155:480", func(e *circle.Extra) { e.Assets[0].Symbol = "EURC" }),
		kind("eip155:1329", func(e *circle.Extra) { e.Assets[0].Address = "nope" }),
		kind("eip155:999", func(e *circle.Extra) { e.MinValidity = 0 }),
	})
	var ids []string
	for _, n := range nets {
		ids = append(ids, n.id)
	}
	if strings.Join(ids, ",") != "eip155:1,eip155:999,eip155:8453" {
		t.Fatalf("offered %v", ids)
	}
	for _, n := range nets {
		if n.id == "eip155:999" && n.min != fallbackValidity {
			t.Fatalf("a network stating no floor got %v, want the SDK's %v", n.min, fallbackValidity)
		}
	}
}

func TestANetworkTableGatewayCannotServeShutsTheRail(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	r.gw.set(func(f *fake) { f.kinds = nil })
	if _, err := r.s.Terms(context.Background(), r.org, r.payer); ReasonOf(err) != ReasonUnavailable || StatusOf(err) != 503 {
		t.Fatalf("terms: %v", err)
	}
	if r.s.Networks(context.Background()) != nil {
		t.Fatal("offered networks Gateway does not list")
	}
}

func TestTheScheduleStartsOnlyWhenArmedAndStops(t *testing.T) {
	shutRail := New(circle.Config{}, nil)
	shutRail.Start()
	if shutRail.Running() {
		t.Fatal("an unarmed rail started its schedule")
	}
	r := newRig(t, circle.Mainnet, true)
	r.s.every = 10 * time.Millisecond
	r.s.Start()
	r.s.Start() // idempotent
	if !r.s.Running() {
		t.Fatal("not running")
	}
	r.s.Stop()
	r.s.Stop()
	if r.s.Running() {
		t.Fatal("still running")
	}
}

// Defense in depth: the requirement Gateway checks a payment against names the
// CONFIGURED treasury, so a record whose payee is anything else — however it
// came to exist — is refused by Gateway too, as address_mismatch.
func TestGatewayIsToldTheTreasuryNotThePaymentsPayee(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	sub := r.pay(500, nil)
	nets, err := r.s.networks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	c, err := verify(sub, nets, treasury, r.org.Name, r.payer)
	if err != nil {
		t.Fatal(err)
	}
	c.auth.To = "0x" + strings.Repeat("77", 20) // a record that skipped verify
	rec, err := r.s.claim(r.org, r.payer, c)
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.s.drive(context.Background(), rec)
	if ReasonOf(err) != "address_mismatch" {
		t.Fatalf("drive: %v — Gateway must be asked to pay the treasury, not the record's payee", err)
	}
	if len(r.book.posted()) != 0 {
		t.Fatal("credited")
	}
}
