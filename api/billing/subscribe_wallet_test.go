package billing

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/hanzoai/commerce/api/promo"
	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A plan bought with Apple Pay or Google Pay: the first period is authorized on
// the wallet, a coupon's use is reserved against the card behind it, and only
// then is the payment taken. One use per org and one per card still hold, a use
// is never sold twice under concurrent presses, and nothing is credited.

// walletMock is a Square whose wallet payments name the card fp.
func walletMock(fp string) *mockSquareProcessor {
	m := squareMock("", "", "")
	m.walletCard = &processor.Card{Brand: "Visa", Last4: "4242", ExpMonth: 12, ExpYear: 2030, Fingerprint: fp}
	return m
}

func walletBody(token, plan, code, user string) string {
	b := fmt.Sprintf(`{"sourceId":%q,"wallet":true,"planId":%q`, token, plan)
	if code != "" {
		b += fmt.Sprintf(`,"coupon":%q`, code)
	}
	if user != "" {
		b += fmt.Sprintf(`,"userId":%q`, user)
	}
	return b + "}"
}

func TestWalletCoupon_FirstUseIsHalfPaidByTheWallet(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	warmCouponNS(t, ctx)
	fiftyOff(t, ctx)
	org := moneyOrg("wl-first")
	m := walletMock("fp_apple_pay")
	withFakeSquare(t, m)

	list := lookupPlan("dev").Price
	half := list - engine.DiscountCents(list, 50)

	r := invokeSubscribeCard(org, ctx, walletBody("wnon:apple", "dev", "50off", ""), nil)
	if r.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d %s, want 201", r.StatusCode, bodyOf(r))
	}
	out := jsonBody(t, r)
	if int64(out["amountCents"].(float64)) != half || out["coupon"] != "50OFF" || out["paymentMethodId"] != "credits" {
		t.Fatalf("receipt %+v, want %d with 50OFF, renewing from credits", out, half)
	}
	if m.authorizeCalls != 1 || m.lastAuthorizeAmount != half || m.captureCalls != 1 || m.cancelCalls != 0 {
		t.Fatalf("authorized %d for %d, captured %d, voided %d; want one hold of %d taken", m.authorizeCalls, m.lastAuthorizeAmount, m.captureCalls, m.cancelCalls, half)
	}
	if m.chargeCalls != 0 || m.createCustomerCalls != 0 {
		t.Fatalf("a wallet was charged as a card (charges %d, vaults %d)", m.chargeCalls, m.createCustomerCalls)
	}
	if n := couponUses(t, ctx, "50OFF"); n != 1 {
		t.Fatalf("coupon used %d, want 1", n)
	}
	if !promo.Redeemed(ctx, "50OFF", "wl-first") {
		t.Fatal("the org's use was not recorded")
	}

	db := datastore.New(org.Namespaced(ctx))
	sub := parentSub(t, db, "wl-first", "dev")
	if sub == nil {
		t.Fatal("no subscription")
	}
	if sub.DefaultPaymentMethod != "credits" || sub.ProviderType != "credit" || sub.DiscountPercent != 0 {
		t.Fatalf("subscription pm=%q provider=%q discount=%d, want credits/credit/0", sub.DefaultPaymentMethod, sub.ProviderType, sub.DiscountPercent)
	}
	invs := invoicesForSub(t, db, sub.Id())
	if len(invs) != 1 || invs[0].Status != billinginvoice.Paid || invs[0].AmountPaid != half || invs[0].PaymentMethod != "wallet" {
		t.Fatalf("first invoice %+v, want one paid by the wallet for %d", invs, half)
	}
	if invs[0].Metadata["coupon"] != "50OFF" {
		t.Fatalf("first invoice metadata %v, want the coupon recorded", invs[0].Metadata)
	}

	// Cash and credit stay where they were: the wallet paid the invoice, nothing
	// was credited to the balance and nothing was drawn from it.
	if avail, err := prepaidFor(ctx, org).Available(ctx, "wl-first", currency.USD); err != nil || avail != 0 {
		t.Fatalf("prepaid money %d (%v), want 0: the sale moved a ledger", avail, err)
	}

	// The renewal is the balance's to pay; with none, the invoice stays open and
	// no card is charged.
	s := dueNow(t, db, sub.Id())
	inv, _, err := engine.RenewSubscription(ctx, db, s, prepaidFor(ctx, org), chargeProviderForOrg(org))
	if err != nil || inv == nil {
		t.Fatalf("renew: inv=%v err=%v", inv, err)
	}
	if inv.AmountDue != list || inv.Status == billinginvoice.Paid || m.chargeCalls != 0 {
		t.Fatalf("renewal due=%d status=%s charges=%d, want %d left open with no card charged", inv.AmountDue, inv.Status, m.chargeCalls, list)
	}
}

// The second use: the same card in another org is refused with the hold voided,
// and another card in the same org is refused before any hold is taken.
func TestWalletCoupon_OncePerCardAndPerOrg(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	warmCouponNS(t, ctx)
	fiftyOff(t, ctx)
	m := walletMock("fp_same_phone")
	withFakeSquare(t, m)

	a := moneyOrg("wl-org-a")
	if r := invokeSubscribeCard(a, ctx, walletBody("wnon:a", "dev", "50OFF", ""), nil); r.StatusCode != http.StatusCreated {
		t.Fatalf("first sale status=%d %s", r.StatusCode, bodyOf(r))
	}

	// Another org, the same card behind the wallet: the card has redeemed.
	b := moneyOrg("wl-org-b")
	r := invokeSubscribeCard(b, ctx, walletBody("wnon:b", "dev", "50OFF", ""), nil)
	if body := bodyOf(r); r.StatusCode != http.StatusConflict || !strings.Contains(body, "card has already redeemed") {
		t.Fatalf("same card again: status=%d %s, want 409", r.StatusCode, body)
	}
	if m.captureCalls != 1 || m.cancelCalls != 1 {
		t.Fatalf("captured %d voided %d, want the second hold voided and never taken", m.captureCalls, m.cancelCalls)
	}

	// The same org A, another account and another card: the org has redeemed,
	// and that is known before the wallet is asked for anything.
	m.walletCard = &processor.Card{Brand: "Mastercard", Last4: "5100", ExpMonth: 1, ExpYear: 2031, Fingerprint: "fp_other_phone"}
	holds := m.authorizeCalls
	r = invokeSubscribeCard(a, ctx, walletBody("wnon:a2", "dev", "50OFF", "wl-org-a/member"), nil)
	if body := bodyOf(r); r.StatusCode != http.StatusConflict || !strings.Contains(body, "organization has already redeemed") {
		t.Fatalf("same org again: status=%d %s, want 409", r.StatusCode, body)
	}
	if m.authorizeCalls != holds {
		t.Fatalf("the org refusal held the wallet: %d holds, want %d", m.authorizeCalls, holds)
	}
	if n := couponUses(t, ctx, "50OFF"); n != 1 {
		t.Fatalf("coupon used %d, want 1", n)
	}

	// Org B with its own card and no coupon: an ordinary wallet sale at full price.
	m.walletCard = &processor.Card{Brand: "Visa", Last4: "1881", ExpMonth: 3, ExpYear: 2032, Fingerprint: "fp_b_phone"}
	r = invokeSubscribeCard(b, ctx, walletBody("wnon:b2", "dev", "", ""), nil)
	if r.StatusCode != http.StatusCreated || m.lastAuthorizeAmount != lookupPlan("dev").Price {
		t.Fatalf("plain wallet sale status=%d held %d, want 201 at the list price", r.StatusCode, m.lastAuthorizeAmount)
	}
}

// Two presses at once, each with its own retry key, for the last org-and-card
// use: one sale, one refusal, one payment taken, one use spent. Driven through
// the core, as the coupon limit race is: the HTTP harness shares an app.
func TestWalletCoupon_ConcurrentDoubleSubmitSellsOnce(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	warmCouponNS(t, ctx)
	fiftyOff(t, ctx)
	m := walletMock("fp_double")
	withFakeSquare(t, m)
	org := moneyOrg("wl-double")

	const presses = 6
	var wg sync.WaitGroup
	var mu sync.Mutex
	sold, held := 0, 0
	start := make(chan struct{})
	for i := 0; i < presses; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			// Each press is its own account in the org, so only the coupon can
			// stop the second sale.
			_, err := Subscribe(ctx, org, SubscribeIn{
				SourceID: fmt.Sprintf("wnon:%d", i), Wallet: true, PlanID: "dev", Coupon: "50OFF",
				Subject: fmt.Sprintf("wl-double/u%d", i), IdempotencyKey: fmt.Sprintf("press-%d", i),
			})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				sold++
			case IsSaleConflict(err):
				held++
			default:
				t.Errorf("press %d: %v", i, err)
			}
		}(i)
	}
	close(start)
	wg.Wait()

	if sold != 1 || held != presses-1 {
		t.Fatalf("sold %d refused %d, want 1 and %d", sold, held, presses-1)
	}
	if m.captureCalls != 1 {
		t.Fatalf("captured %d payments, want 1", m.captureCalls)
	}
	if m.cancelCalls != m.authorizeCalls-1 {
		t.Fatalf("held %d and voided %d: every hold but the sold one must be let go", m.authorizeCalls, m.cancelCalls)
	}
	if n := couponUses(t, ctx, "50OFF"); n != 1 {
		t.Fatalf("coupon used %d, want 1", n)
	}

	// The same press twice with one key: the second replays or waits, and takes nothing.
	m2 := walletMock("fp_same_key")
	withFakeSquare(t, m2)
	org2 := moneyOrg("wl-samekey")
	var wg2 sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg2.Add(1)
		go func(i int) {
			defer wg2.Done()
			_, errs[i] = Subscribe(ctx, org2, SubscribeIn{
				SourceID: fmt.Sprintf("wnon:k%d", i), Wallet: true, PlanID: "dev", Coupon: "50OFF",
				Subject: "wl-samekey", IdempotencyKey: "one-press",
			})
		}(i)
	}
	wg2.Wait()
	for i, err := range errs {
		if err != nil && !IsSaleConflict(err) {
			t.Fatalf("one key twice, press %d: %v", i, err)
		}
	}
	if m2.captureCalls != 1 || couponUses(t, ctx, "50OFF") != 2 {
		t.Fatalf("one key twice: %v, captured %d, coupon used %d; want one payment and one more use", errs, m2.captureCalls, couponUses(t, ctx, "50OFF"))
	}
}

// Cash App Pay names no card: a coupon on it is refused with the hold voided,
// and without one it buys the plan at full price.
func TestWalletCoupon_CashAppTakesNoCode(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	warmCouponNS(t, ctx)
	fiftyOff(t, ctx)
	m := squareMock("", "", "")
	m.walletBrand = "CASH_APP"
	withFakeSquare(t, m)
	org := moneyOrg("wl-cashapp")

	r := invokeSubscribeCard(org, ctx, walletBody("wnon:cash", "dev", "50OFF", ""), nil)
	if body := bodyOf(r); r.StatusCode != http.StatusBadRequest || !strings.Contains(body, "card, Apple Pay or Google Pay") {
		t.Fatalf("cash app with a code: status=%d %s, want 400", r.StatusCode, body)
	}
	if m.captureCalls != 0 || m.cancelCalls != 1 || couponUses(t, ctx, "50OFF") != 0 {
		t.Fatalf("captured %d voided %d used %d, want the hold voided and nothing spent", m.captureCalls, m.cancelCalls, couponUses(t, ctx, "50OFF"))
	}
	r = invokeSubscribeCard(org, ctx, walletBody("wnon:cash2", "dev", "", ""), nil)
	if r.StatusCode != http.StatusCreated || m.lastAuthorizeAmount != lookupPlan("dev").Price || m.captureCalls != 1 {
		t.Fatalf("cash app without a code: status=%d %s held %d", r.StatusCode, bodyOf(r), m.lastAuthorizeAmount)
	}
}

// A payment Square will not take gives the coupon's use back and voids the hold;
// a capture that goes unanswered keeps the attempt held, so a retry pays nothing.
func TestWalletCoupon_CaptureOutcomes(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	warmCouponNS(t, ctx)
	fiftyOff(t, ctx)

	m := walletMock("fp_refused")
	m.captureStatus = "FAILED"
	withFakeSquare(t, m)
	org := moneyOrg("wl-capfail")
	r := invokeSubscribeCard(org, ctx, walletBody("wnon:x", "dev", "50OFF", ""), nil)
	if r.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("refused capture: status=%d %s, want 402", r.StatusCode, bodyOf(r))
	}
	if m.cancelCalls != 1 || couponUses(t, ctx, "50OFF") != 0 || promo.Redeemed(ctx, "50OFF", "wl-capfail") {
		t.Fatalf("refused capture: voided %d, used %d; want the hold voided and the use given back", m.cancelCalls, couponUses(t, ctx, "50OFF"))
	}

	lost := walletMock("fp_lost")
	lost.captureErr = errors.New("connection reset")
	withFakeSquare(t, lost)
	org2 := moneyOrg("wl-caplost")
	hdr := map[string]string{"X-Idempotency-Key": "lost-1"}
	if r := invokeSubscribeCard(org2, ctx, walletBody("wnon:y", "dev", "50OFF", ""), hdr); r.StatusCode < 500 {
		t.Fatalf("unanswered capture: status=%d %s, want a 5xx", r.StatusCode, bodyOf(r))
	}
	lost.captureErr = nil
	if r := invokeSubscribeCard(org2, ctx, walletBody("wnon:y2", "dev", "50OFF", ""), hdr); r.StatusCode != http.StatusConflict {
		t.Fatalf("retry after an unanswered capture: status=%d %s, want 409 in progress", r.StatusCode, bodyOf(r))
	}
	if lost.authorizeCalls != 1 || lost.cancelCalls != 0 {
		t.Fatalf("held %d voided %d after an unanswered capture, want one hold and no void", lost.authorizeCalls, lost.cancelCalls)
	}
	if !promo.Redeemed(ctx, "50OFF", "wl-caplost") {
		t.Fatal("the use of a payment that may have been taken was given back")
	}
}

// What a wallet sends with sourceId: a token. A balance is no wallet.
func TestWalletSale_Refusals(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	warmCouponNS(t, ctx)
	m := walletMock("fp_r")
	withFakeSquare(t, m)
	org := moneyOrg("wl-refuse")
	for _, body := range []string{
		`{"sourceId":"balance","wallet":true,"planId":"dev"}`,
		`{"paymentMethodId":"pm_1","wallet":true,"planId":"dev"}`,
	} {
		if r := invokeSubscribeCard(org, ctx, body, nil); r.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s: status=%d %s, want 400", body, r.StatusCode, bodyOf(r))
		}
	}
	if m.authorizeCalls != 0 {
		t.Fatalf("a refused request held a wallet %d time(s)", m.authorizeCalls)
	}
}
