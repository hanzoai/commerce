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
	"github.com/hanzoai/commerce/models/idempotencykey"
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
	if v := viewSubscription(sub); v.Plan.Slug != "dev" {
		t.Fatalf("the row reads as plan %+v, want slug dev so a reader can match the catalog", v.Plan)
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

// A payment Square will not take gives the coupon's use back and voids the hold.
// A capture with no answer is settled by reading the payment back: taken opens
// the plan, let go gives the use back, and unknown seals the attempt for good.
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

	// No answer, and the payment reads back COMPLETED: the plan is opened once.
	taken := walletMock("fp_taken")
	taken.captureLost = 1
	taken.readStatus = "COMPLETED"
	withFakeSquare(t, taken)
	if r := invokeSubscribeCard(moneyOrg("wl-taken"), ctx, walletBody("wnon:t", "dev", "50OFF", ""), nil); r.StatusCode != http.StatusCreated {
		t.Fatalf("capture read back as taken: status=%d %s, want 201", r.StatusCode, bodyOf(r))
	}
	if taken.authorizeCalls != 1 || taken.captureCalls != 1 || taken.readCalls != 1 || taken.cancelCalls != 0 {
		t.Fatalf("held %d captured %d read %d voided %d, want one of each read and no void",
			taken.authorizeCalls, taken.captureCalls, taken.readCalls, taken.cancelCalls)
	}

	// No answer, and the payment is still held: it is taken now.
	held := walletMock("fp_held")
	held.captureLost = 1
	held.readStatus = "APPROVED"
	withFakeSquare(t, held)
	if r := invokeSubscribeCard(moneyOrg("wl-held"), ctx, walletBody("wnon:h", "dev", "", ""), nil); r.StatusCode != http.StatusCreated || held.captureCalls != 2 {
		t.Fatalf("held after a lost capture: status=%d %s captures=%d, want 201 after a second capture", r.StatusCode, bodyOf(r), held.captureCalls)
	}

	// No answer, and the payment reads back CANCELED: nothing was taken.
	gone := walletMock("fp_gone")
	gone.captureLost = 1
	gone.readStatus = "CANCELED"
	withFakeSquare(t, gone)
	r = invokeSubscribeCard(moneyOrg("wl-gone"), ctx, walletBody("wnon:g", "dev", "50OFF", ""), map[string]string{"X-Idempotency-Key": "gone-1"})
	if r.StatusCode != http.StatusPaymentRequired || promo.Redeemed(ctx, "50OFF", "wl-gone") {
		t.Fatalf("capture read back as canceled: status=%d %s redeemed=%v, want 402 and the use given back",
			r.StatusCode, bodyOf(r), promo.Redeemed(ctx, "50OFF", "wl-gone"))
	}
	gone.readStatus = ""
	if r := invokeSubscribeCard(moneyOrg("wl-gone"), ctx, walletBody("wnon:g2", "dev", "50OFF", ""), map[string]string{"X-Idempotency-Key": "gone-1"}); r.StatusCode != http.StatusCreated {
		t.Fatalf("retry after nothing was taken: status=%d %s, want 201", r.StatusCode, bodyOf(r))
	}

	// No answer either way: the attempt is sealed, so its key never pays again.
	lost := walletMock("fp_lost")
	lost.captureErr = errors.New("connection reset")
	withFakeSquare(t, lost)
	org2 := moneyOrg("wl-caplost")
	hdr := map[string]string{"X-Idempotency-Key": "lost-1"}
	if r := invokeSubscribeCard(org2, ctx, walletBody("wnon:y", "dev", "50OFF", ""), hdr); r.StatusCode != http.StatusBadGateway {
		t.Fatalf("unconfirmed capture: status=%d %s, want 502", r.StatusCode, bodyOf(r))
	}
	db := datastore.New(org2.Namespaced(ctx))
	if rec, replay, err := idempotencykey.Begin(db, "billing-subscribe:wl-caplost:", "lost-1"); err != nil || !replay || rec.Status != idempotencykey.StatusCompleted {
		t.Fatalf("the attempt was not sealed: replay=%v err=%v", replay, err)
	}
	lost.captureErr = nil
	if r := invokeSubscribeCard(org2, ctx, walletBody("wnon:y2", "dev", "50OFF", ""), hdr); r.StatusCode != http.StatusConflict {
		t.Fatalf("retry after an unconfirmed capture: status=%d %s, want 409", r.StatusCode, bodyOf(r))
	}
	if lost.authorizeCalls != 1 || lost.cancelCalls != 0 {
		t.Fatalf("held %d voided %d after an unconfirmed capture, want one hold and no void", lost.authorizeCalls, lost.cancelCalls)
	}
	if !promo.Redeemed(ctx, "50OFF", "wl-caplost") {
		t.Fatal("the use of a payment that may have been taken was given back")
	}
}

// An authorization with no answer is asked again under the same key, which
// Square answers with the first call's payment. A decline is Square's answer
// and is not asked again. Two silences hold the attempt and take nothing.
func TestWalletSale_AuthorizeOutcomes(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	warmCouponNS(t, ctx)

	once := walletMock("fp_once")
	once.authorizeLost = 1
	withFakeSquare(t, once)
	if r := invokeSubscribeCard(moneyOrg("wl-auth1"), ctx, walletBody("wnon:a", "dev", "", ""), nil); r.StatusCode != http.StatusCreated {
		t.Fatalf("one lost authorization: status=%d %s, want 201", r.StatusCode, bodyOf(r))
	}
	if once.authorizeCalls != 2 || once.authorizeKeys[0] != once.authorizeKeys[1] || once.captureCalls != 1 {
		t.Fatalf("authorized %d under %v, captured %d; want the same key twice and one capture", once.authorizeCalls, once.authorizeKeys, once.captureCalls)
	}

	no := walletMock("fp_no")
	no.authorizeErr = &processor.Answer{Err: errors.New("402: CARD_DECLINED")}
	withFakeSquare(t, no)
	if r := invokeSubscribeCard(moneyOrg("wl-auth2"), ctx, walletBody("wnon:b", "dev", "", ""), nil); r.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("declined: status=%d %s, want 402", r.StatusCode, bodyOf(r))
	}
	if no.authorizeCalls != 1 {
		t.Fatalf("a decline was asked %d times, want once", no.authorizeCalls)
	}

	silent := walletMock("fp_silent")
	silent.authorizeLost = 2
	withFakeSquare(t, silent)
	hdr := map[string]string{"X-Idempotency-Key": "silent-1"}
	org := moneyOrg("wl-auth3")
	if r := invokeSubscribeCard(org, ctx, walletBody("wnon:c", "dev", "", ""), hdr); r.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("two lost authorizations: status=%d %s, want 503", r.StatusCode, bodyOf(r))
	}
	if r := invokeSubscribeCard(org, ctx, walletBody("wnon:c2", "dev", "", ""), hdr); r.StatusCode != http.StatusConflict || silent.authorizeCalls != 2 {
		t.Fatalf("retry inside the attempt: status=%d authorized %d, want 409 and no new hold", r.StatusCode, silent.authorizeCalls)
	}
}

// Seats are bounded on every path that opens or resizes a plan, so price times
// seats never wraps: a team of seven quadrillion is refused, not sold for $8.84.
func TestSale_SeatsAreBounded(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	warmCouponNS(t, ctx)
	m := walletMock("fp_seats")
	withFakeSquare(t, m)
	org := moneyOrg("wl-seats")
	for _, q := range []int{7378697629483821, maxSeats("team") + 1} {
		body := fmt.Sprintf(`{"sourceId":"wnon:s","wallet":true,"planId":"team","quantity":%d}`, q)
		if r := invokeSubscribeCard(org, ctx, body, nil); r.StatusCode != http.StatusBadRequest || !strings.Contains(bodyOf(r), "at most") {
			t.Fatalf("%d seats: status=%d, want 400", q, r.StatusCode)
		}
	}
	if m.authorizeCalls != 0 {
		t.Fatalf("an unbounded seat count reached the wallet %d time(s)", m.authorizeCalls)
	}
	if maxSeats("team") != 100 || maxSeats("dev") != seatCeiling {
		t.Fatalf("maxSeats team=%d dev=%d, want the catalog's 100 and the ceiling", maxSeats("team"), maxSeats("dev"))
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
