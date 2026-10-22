package billing

// A plan bought with a wallet: Apple Pay, Google Pay or Cash App Pay.
//
// A wallet's token is single-use and Square keeps none on file, so the wallet
// pays the FIRST period itself and every renewal is drawn from the prepaid
// balance — the subscription is opened exactly as one paid from the balance is
// (ProviderType "credit", DefaultPaymentMethod "credits"), with its first invoice
// paid by the wallet's Square payment rather than by a draw.
//
// THE PAYMENT IS AUTHORIZED BEFORE IT IS TAKEN. An authorization is what names
// the card behind an Apple Pay or Google Pay token (its fingerprint, brand, last
// four and expiry), and a coupon is one use per org AND one use per card, so the
// use is reserved against that card between the authorization and the capture.
// A coupon this card or this org has already used voids the authorization:
// nothing is charged. Cash App Pay names no card and no stable payer either (a
// $Cashtag can be changed by its holder), so a coupon is refused on it, and the
// authorization voided.
//
// No ledger moves for the first period: the money is the Square payment, the
// invoice it pays says so, and the coupon discounts that invoice alone. Cash and
// credit are untouched; nothing is credited to make a discount work.
//
// AN UNANSWERED CALL IS ASKED AGAIN, NEVER GUESSED. Everything from the
// authorization on runs whether or not the buyer is still waiting. An
// authorization with no answer is sent again under the same key, which Square
// answers with the first one's result; a capture with no answer is settled by
// reading the payment back. Only a payment Square says it did not take lets the
// attempt go. One that stays unknown seals the attempt, so a retry under its key
// is refused rather than paying again, and is named for reconciliation.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hanzoai/money"

	"github.com/hanzoai/commerce/api/promo"
	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentorg"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
)

// walletRail is a processor that can hold a payment, take it, or let it go.
type walletRail interface {
	Authorize(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error)
	Capture(ctx context.Context, transactionID string, amount money.Amount) (*processor.PaymentResult, error)
	CancelAuthorization(ctx context.Context, paymentID string) error
	GetTransaction(ctx context.Context, txID string) (*processor.Transaction, error)
}

// walletWait bounds one call to the processor once the buyer may have gone.
const walletWait = 20 * time.Second

// settlement is where a held payment ended up.
type settlement int

const (
	settledTaken    settlement = iota // the payment is COMPLETED
	settledReleased                   // Square holds nothing: canceled, failed or voided
	settledUnknown                    // Square did not say
)

// settle reads a held payment back and finishes it: COMPLETED is taken;
// CANCELED or FAILED took nothing; one still APPROVED is captured, since the
// sale already decided to take it. A read is never a second charge.
func settle(ctx context.Context, wr walletRail, id string) settlement {
	for range 2 {
		c, cancel := context.WithTimeout(ctx, walletWait)
		tx, err := wr.GetTransaction(c, id)
		cancel()
		if err != nil || tx == nil {
			continue
		}
		switch status := strings.ToUpper(strings.TrimSpace(tx.Status)); {
		case processor.Settled(status):
			return settledTaken
		case status == "CANCELED" || status == "FAILED":
			return settledReleased
		case status == "APPROVED":
			c, cancel := context.WithTimeout(ctx, walletWait)
			res, err := wr.Capture(c, id, money.Amount{})
			cancel()
			if err == nil && res != nil && res.Success {
				return settledTaken
			}
		}
	}
	return settledUnknown
}

// answered reports whether a processor error is the processor's own answer
// (it said no, so nothing is held) rather than a call that never got one.
func answered(err error) bool {
	var a interface{ Answered() bool }
	return errors.As(err, &a) && a.Answered()
}

// walletRailFrom is the org's Square processor, when it can hold and take a
// payment.
func walletRailFrom(ctx context.Context, reg *processor.Registry) (walletRail, bool) {
	p, err := reg.Get(processor.Square)
	if err != nil || p == nil || !p.IsAvailable(ctx) {
		return nil, false
	}
	w, ok := p.(walletRail)
	return w, ok
}

// walletTerms is what subscribe resolved before the wallet is asked for money:
// the plan as bought, its price after the promo and the coupon, the coupon, and
// the attempt's guard.
type walletTerms struct {
	plan         *plan.Plan
	planID       string
	slug         string
	qty          int
	seats        int64
	lines        []subscription.Line // the team's other seat types, priced
	charge       int64               // what the wallet pays today: price x seats, less promo and coupon
	list         int64               // price x seats
	cur          currency.Type
	coupon       engine.Coupon
	couponPlans  []string // the plan slugs the coupon covers, granted with it
	promoPercent int
	promoName    string
	sourceID     string
	guard        string
	rec          *idempotencykey.IdempotencyKey
	abandon      func()
}

// walletCard is the card behind a wallet payment, in the form the one-use-per-
// card rule keys on (instrumentOf): Square's fingerprint of the card, else its
// brand, last four and expiry. "" when the payment names no card.
func walletCard(res *processor.PaymentResult) string {
	if res == nil || res.Card == nil {
		return ""
	}
	c := res.Card
	if fp := strings.TrimSpace(c.Fingerprint); fp != "" {
		return "fp:" + fp
	}
	if c.Brand != "" && c.Last4 != "" {
		return fmt.Sprintf("card:%s:%s:%d/%d", strings.ToLower(c.Brand), c.Last4, c.ExpMonth, c.ExpYear)
	}
	return ""
}

// tokenTag is a short, stable name for a single-use token, for a processor key.
func tokenTag(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])[:12]
}

// walletSale authorizes the first period on the wallet, reserves the coupon's
// use against the card behind it, captures, and opens the subscription.
func walletSale(ctx context.Context, db *datastore.Datastore, org *organization.Organization, in SubscribeIn, t walletTerms) (*saleOutcome, error) {
	// A wallet payment is made only under a recorded attempt. Its processor key
	// names the token as well as the attempt (below), so the guard is what keeps a
	// retry from authorizing a second time; with no guard there is nothing that
	// would.
	if t.rec == nil {
		t.abandon()
		return nil, saleRefusal{saleUnavailable, "the attempt could not be recorded, so the wallet was not charged; try again"}
	}
	// A free trial starts on a card kept on file, which a wallet is not.
	if t.plan.TrialPeriodDays > 0 {
		t.abandon()
		return nil, saleRefusal{saleRefused, fmt.Sprintf(
			"plan %q starts with a %d-day free trial, which begins on a card kept on file; pay by card to start it",
			t.slug, t.plan.TrialPeriodDays)}
	}
	wr, ok := walletRailFrom(ctx, processorsForOrg(org))
	if !ok {
		t.abandon()
		return nil, saleRefusal{saleUnchargeable, "no wallet payment processor available for this organization"}
	}

	// From here the sale finishes whether or not the buyer is still waiting: a
	// hang-up mid-call is not an answer from Square.
	work := context.WithoutCancel(ctx)
	// seal keeps this attempt's guard for good when Square may have taken the
	// money: a retry under its key is refused instead of paying again.
	seal := func() {
		if err := idempotencykey.Complete(t.rec, ""); err != nil {
			log.Error("subscribe: the wallet attempt %s was not sealed (subject=%s): %v", t.guard, in.Subject, err)
		}
	}

	// The Square key names this token as well as the attempt. The guard keeps one
	// attempt from reaching Square twice; a new attempt after a voided one carries
	// a new token, which Square would refuse under the old key.
	req := processor.PaymentRequest{
		Token:          t.sourceID,
		Amount:         currency.Cents(t.charge),
		Currency:       t.cur,
		IdempotencyKey: gatewayKey("subscribe-wallet", in.Subject, t.guard+":"+tokenTag(t.sourceID)),
		Description:    fmt.Sprintf("Subscription %s — first period", t.plan.Name),
	}
	authorize := func() (*processor.PaymentResult, error) {
		c, cancel := context.WithTimeout(work, walletWait)
		defer cancel()
		return wr.Authorize(c, req)
	}
	auth, err := authorize()
	if err != nil && !answered(err) {
		// No answer. The same key again is answered with the first call's result,
		// so this either finds the hold the first call made or makes the only one.
		auth, err = authorize()
	}
	switch {
	case err == nil && auth != nil && auth.Success:
	case err == nil || answered(err):
		// Square said no: nothing is held.
		t.abandon()
		log.Error("wallet authorization declined for subscribe (subject=%s): %v", in.Subject, err)
		return nil, saleRefusal{saleDeclined, parseCardDeclineReason(auth, err)}
	default:
		// Still no answer: a hold may exist that this attempt cannot name. The guard
		// stays held so a retry waits out the attempt rather than holding twice.
		log.Error("RECONCILE: a wallet authorization for subscribe went unanswered twice (subject=%s attempt=%s): %v",
			in.Subject, t.guard, err)
		return nil, saleRefusal{saleUnavailable, "the wallet could not be reached, and nothing was taken; try again in a few minutes"}
	}
	// void lets the authorization go, and says whether Square confirmed it.
	void := func() bool {
		c, cancel := context.WithTimeout(work, walletWait)
		defer cancel()
		if verr := wr.CancelAuthorization(c, auth.ProcessorRef); verr != nil {
			log.Error("subscribe: wallet authorization %s was not voided (subject=%s): %v",
				auth.ProcessorRef, in.Subject, verr)
			return false
		}
		return true
	}
	// unsettled is the end of an attempt Square may have taken money for: sealed,
	// named for reconciliation, and told plainly.
	unsettled := func(ref, why string) error {
		seal()
		uncredited(work, in.Events, org.Name, in.Subject, ref, why, t.charge, false)
		return saleRefusal{saleUnsettled, fmt.Sprintf(
			"the wallet payment %s could not be confirmed and nothing more will be charged for it; it is being reconciled", ref)}
	}

	// The coupon's use, reserved against the card behind the wallet under the
	// coupon's lock, exactly as a typed card's is: an org or a card that has used
	// the code is refused here, before anything is taken.
	var hold *promo.Hold
	if t.coupon.Code != "" {
		card := walletCard(auth)
		if card == "" {
			void()
			t.abandon()
			return nil, saleRefusal{saleRefused, fmt.Sprintf(
				"coupon %s is redeemed with a card, Apple Pay or Google Pay, and this wallet names no card", t.coupon.Code)}
		}
		h, herr := promo.Reserve(work, t.coupon.Code, org.Name, card, time.Now())
		if herr != nil {
			void()
			t.abandon()
			return nil, holdRefusal(t.coupon.Code, herr)
		}
		hold = h
	}

	c, cancel := context.WithTimeout(work, walletWait)
	res, err := wr.Capture(c, auth.ProcessorRef, money.Amount{})
	cancel()
	state := settledTaken
	switch {
	case err == nil && res != nil && res.Success:
	case err == nil && res != nil && void():
		// Square answered and did not take it, and the hold is let go.
		state = settledReleased
	default:
		// No answer, or a hold that would not let go: read it back.
		state = settle(work, wr, auth.ProcessorRef)
	}
	switch state {
	case settledReleased:
		hold.Release()
		t.abandon()
		return nil, saleRefusal{saleDeclined, parseCardDeclineReason(res, nil)}
	case settledUnknown:
		// The coupon's use stays spent with the payment it may have paid for.
		return nil, unsettled(auth.ProcessorRef, fmt.Sprintf(
			"a wallet payment's capture was not confirmed and no subscription was opened: %v", err))
	}
	ref := auth.ProcessorRef
	if res != nil && res.ProcessorRef != "" {
		ref = res.ProcessorRef
	}

	// The subscription, opened as one paid from the balance is, because that is
	// how it renews. The money moved, so a failure from here on leaves the guard
	// held: a retry replays nothing and charges nothing.
	t.plan.TrialPeriodDays = 0
	meta := map[string]interface{}{"source": "subscribe/wallet"}
	if t.coupon.Code != "" {
		meta["coupon"] = t.coupon.Code
	}
	if auth.Wallet != "" {
		meta["wallet"] = auth.Wallet
	}
	sub, err := createSubscription(db, t.plan, &createSubscriptionRequest{
		UserId:               in.Subject,
		PlanId:               t.planID,
		StoreId:              in.StoreID,
		DefaultPaymentMethod: "credits",
		Quantity:             t.qty,
		Metadata:             meta,
		Test:                 org.TestMode(),
		priced:               t.lines,
	})
	if err != nil {
		return nil, unsettled(ref, "the first period was paid by a wallet and no subscription was opened: "+err.Error())
	}
	sub.ProviderType = "credit"
	// Stamped before the invoice is built, which prices the promo and the coupon
	// off the row.
	sub.DiscountPercent, sub.DiscountName = t.promoPercent, t.promoName
	engine.Grant(sub, t.coupon, t.couponPlans, sub.PeriodStart)
	// The paid first invoice is what makes a balance-renewed plan payment-backed,
	// so it is written twice before it is given up on, and then reconciled.
	inv, err := engine.CreatePaidFirstInvoice(db, sub, "wallet", ref)
	if err != nil {
		inv, err = engine.CreatePaidFirstInvoice(db, sub, "wallet", ref)
	}
	if err != nil {
		uncredited(work, in.Events, org.Name, in.Subject, ref, fmt.Sprintf(
			"subscription %s was paid by a wallet and its first invoice was not recorded, so it confers nothing: %v", sub.Id(), err),
			t.charge, false)
	} else if err := paymentorg.Put(ref, org.Name, in.Subject, inv.Id(), org.TestMode()); err != nil {
		log.Warn("subscribe: payment %s: the index to its invoice was not written, so its refund or dispute will not find the period it paid: %v", ref, err)
	}
	if err := sub.Update(); err != nil {
		log.Error("subscribe: failed to update subscription after first invoice (subject=%s): %v", in.Subject, err)
	}

	invoiceID := ""
	if inv != nil {
		invoiceID = inv.Id()
	}
	sale := &Sale{
		SubscriptionID:  sub.Id(),
		InvoiceID:       invoiceID,
		PlanID:          t.planID,
		Level:           in.Level,
		Interval:        t.plan.Interval,
		PaymentMethodID: "credits",
		AmountCents:     t.charge,
		Currency:        string(t.cur),
		Status:          "ok",
		Coupon:          t.coupon.Code,
		CouponPercent:   t.coupon.Percent,
		CouponMonths:    t.coupon.Months,
	}
	if body, mErr := json.Marshal(sale); mErr == nil {
		_ = idempotencykey.Complete(t.rec, string(body))
	}
	noticeSale(work, saleNote{
		Org: org.Name, Email: in.Email, Plan: t.plan, Seats: t.seats + int64(lineSeats(t.lines)), Sale: sale,
		ListCents: t.list, PromoName: t.promoName, Coupon: t.coupon, Test: org.TestMode(),
	})
	return &saleOutcome{Sale: sale, Sub: sub, Inv: inv}, nil
}
