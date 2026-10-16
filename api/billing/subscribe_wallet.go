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
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
)

// walletRail is a processor that can hold a payment, take it, or let it go.
type walletRail interface {
	Authorize(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error)
	Capture(ctx context.Context, transactionID string, amount money.Amount) (*processor.PaymentResult, error)
	CancelAuthorization(ctx context.Context, paymentID string) error
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
	charge       int64 // what the wallet pays today: price x seats, less promo and coupon
	list         int64 // price x seats
	cur          currency.Type
	coupon       engine.Coupon
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
		return nil, errors.New("the attempt could not be recorded, so the wallet was not charged; try again")
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

	// The Square key names this token as well as the attempt. The guard already
	// keeps one attempt from reaching Square twice; a new attempt after a voided
	// one carries a new token, which Square would refuse under the old key.
	auth, err := wr.Authorize(ctx, processor.PaymentRequest{
		Token:          t.sourceID,
		Amount:         currency.Cents(t.charge),
		Currency:       t.cur,
		IdempotencyKey: gatewayKey("subscribe-wallet", in.Subject, t.guard+":"+tokenTag(t.sourceID)),
		Description:    fmt.Sprintf("Subscription %s — first period", t.plan.Name),
	})
	if err != nil || auth == nil || !auth.Success {
		t.abandon()
		log.Error("wallet authorization failed for subscribe (subject=%s): %v", in.Subject, err)
		return nil, saleRefusal{saleDeclined, parseCardDeclineReason(auth, err)}
	}
	// void lets the authorization go. It must run even when the buyer has hung up.
	void := func() {
		if verr := wr.CancelAuthorization(context.WithoutCancel(ctx), auth.ProcessorRef); verr != nil {
			log.Error("subscribe: wallet authorization %s was not voided (subject=%s); Square lets it lapse: %v",
				auth.ProcessorRef, in.Subject, verr)
		}
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
		h, herr := promo.Reserve(ctx, t.coupon.Code, org.Name, card, time.Now())
		if herr != nil {
			void()
			t.abandon()
			return nil, holdRefusal(t.coupon.Code, herr)
		}
		hold = h
	}

	res, err := wr.Capture(ctx, auth.ProcessorRef, money.Amount{})
	switch {
	case err == nil && res != nil && res.Success:
	case err == nil:
		// Square answered, and the payment is not taken: nothing moved.
		hold.Release()
		void()
		t.abandon()
		return nil, saleRefusal{saleDeclined, parseCardDeclineReason(res, nil)}
	default:
		// No answer: the payment may have been taken. The guard stays held so a
		// retry pays nothing more, the coupon's use stays spent, and the payment
		// is named for reconciliation.
		uncredited(ctx, in.Events, org.Name, in.Subject, auth.ProcessorRef,
			"a wallet payment's capture went unanswered and no subscription was opened: "+err.Error(),
			t.charge, false)
		return nil, fmt.Errorf("the wallet payment %s could not be confirmed; nothing more will be charged for this attempt: %w",
			auth.ProcessorRef, err)
	}
	ref := res.ProcessorRef
	if ref == "" {
		ref = auth.ProcessorRef
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
	})
	if err != nil {
		uncredited(ctx, in.Events, org.Name, in.Subject, ref,
			"the first period was paid by a wallet and no subscription was opened: "+err.Error(),
			t.charge, false)
		return nil, err
	}
	sub.ProviderType = "credit"
	// Stamped before the invoice is built, which prices the promo off the row.
	sub.DiscountPercent, sub.DiscountName = t.promoPercent, t.promoName
	inv, err := engine.CreatePaidFirstInvoice(db, sub, "wallet", ref, t.coupon)
	if err != nil {
		log.Error("RECONCILE: subscription %s (subject=%s) was paid by wallet payment %s and its first invoice was not recorded: %v",
			sub.Id(), in.Subject, ref, err)
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
	}
	if body, mErr := json.Marshal(sale); mErr == nil {
		_ = idempotencykey.Complete(t.rec, string(body))
	}
	noticeSale(ctx, saleNote{
		Org: org.Name, Email: in.Email, Plan: t.plan, Seats: t.seats, Sale: sale,
		ListCents: t.list, PromoName: t.promoName, Coupon: t.coupon, Test: org.TestMode(),
	})
	return &saleOutcome{Sale: sale, Sub: sub, Inv: inv}, nil
}
