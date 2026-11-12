package billing

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/api/promo"
	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/organization"
	storemodel "github.com/hanzoai/commerce/models/store"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	types "github.com/hanzoai/commerce/types"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
)

// paypalOrderCreator is implemented by *paypal.Provider to open a server-side
// PayPal REST v2 order (intent CAPTURE or AUTHORIZE) without moving money yet.
type paypalOrderCreator interface {
	CreateOrder(ctx context.Context, req processor.PaymentRequest, intent string) (orderID string, status string, err error)
}

// PayPalOrderIn is the value input to CreatePayPalOrder. When PlanID is empty it
// opens a CAPTURE order for a bounded credit top-up; when PlanID is non-empty it
// opens an AUTHORIZE order at the plan's server-authoritative catalog price
// (less active promotion and validated coupon), matching walletSale's exact charge.
type PayPalOrderIn struct {
	// Top-up fields (used when PlanID == "").
	AmountCents int64 `json:"amountCents,omitempty"`

	// Subscription fields (used when PlanID != "").
	PlanID       string         `json:"planId,omitempty"`
	UserID       string         `json:"userId,omitempty"`
	StoreID      string         `json:"storeId,omitempty"`
	Quantity     int            `json:"quantity,omitempty"`
	Lines        []LineIn       `json:"lines,omitempty"`
	Level        int            `json:"level,omitempty"`
	Interval     types.Interval `json:"interval,omitempty"`
	Coupon       string         `json:"coupon,omitempty"`
	BalanceCents int64          `json:"balanceCents,omitempty"`

	// Shared fields.
	Currency       string       `json:"currency,omitempty"`
	Subject        string       `json:"-"`
	IdempotencyKey string       `json:"-"`
	Promo          *promo.Promo `json:"-"`
}

// PayPalOrderOut is the created PayPal REST v2 order returned to the PayPal JS SDK.
type PayPalOrderOut struct {
	OrderID     string `json:"orderId"`
	AmountCents int64  `json:"amountCents"`
	Currency    string `json:"currency"`
	Intent      string `json:"intent"`
	Status      string `json:"status"`
	Environment string `json:"environment"`
	Test        bool   `json:"test"`
}

// CreatePayPalOrder opens a server-priced PayPal REST v2 order for either a
// credit top-up (CAPTURE) or a plan subscription's first period (AUTHORIZE).
// No ledger, balance, or subscription record is written until the buyer approves
// the order and TakePayment / Subscribe captures it.
func CreatePayPalOrder(ctx context.Context, org *organization.Organization, in PayPalOrderIn) (*PayPalOrderOut, *PaymentFault) {
	if org == nil {
		return nil, fault(http.StatusUnauthorized, "missing identity headers", nil)
	}
	if strings.TrimSpace(in.Subject) == "" {
		return nil, fault(http.StatusUnauthorized, "missing identity headers", nil)
	}

	reg := processorsForOrg(org)
	p, err := reg.Get(processor.PayPal)
	if err != nil || p == nil || !p.IsAvailable(ctx) {
		return nil, fault(http.StatusUnprocessableEntity, "PayPal payment processor is not available for this organization", err)
	}
	creator, ok := p.(paypalOrderCreator)
	if !ok {
		return nil, fault(http.StatusUnprocessableEntity, "PayPal order creation is not supported", nil)
	}

	planID := strings.TrimSpace(in.PlanID)
	var (
		amountCents int64
		cur         currency.Type
		intent      string
		desc        string
		idemKey     string
	)

	if planID == "" {
		minCents, maxCents := topupBounds()
		if in.AmountCents < minCents || in.AmountCents > maxCents {
			return nil, fault(http.StatusBadRequest, fmt.Sprintf("amountCents must be between %d and %d", minCents, maxCents), nil)
		}
		amountCents = in.AmountCents
		cur = normalizeCurrency(in.Currency)
		intent = "CAPTURE"
		desc = fmt.Sprintf("Top-up %d %s for org %s", amountCents, cur, in.Subject)
		keySeed := in.IdempotencyKey
		if keySeed == "" {
			keySeed = windowKey("paypal-topup-order:amount:" + strconv.FormatInt(amountCents, 10) + ":cur:" + string(cur))
		}
		idemKey = gatewayKey("paypal-order-topup", in.Subject, keySeed)
	} else {
		db := datastore.New(org.Namespaced(ctx))
		if in.StoreID != "" {
			s := storemodel.New(db)
			if err := s.GetById(in.StoreID); err != nil {
				return nil, fault(http.StatusNotFound, "store not found", err)
			}
		}

		subIn := SubscribeIn{
			SourceID:     "paypal-order-quote",
			PlanID:       planID,
			Subject:      in.Subject,
			StoreID:      in.StoreID,
			Quantity:     in.Quantity,
			Lines:        in.Lines,
			Level:        in.Level,
			Interval:     in.Interval,
			Currency:     in.Currency,
			Coupon:       in.Coupon,
			Wallet:       true,
			Provider:     string(processor.PayPal),
			Promo:        in.Promo,
			BalanceCents: in.BalanceCents,
		}
		q, perr := price(db, org, subIn, planID, subIn.SourceID)
		if perr != nil {
			return nil, saleErrToFault(perr)
		}

		held := billingSubscription(db, in.Subject, org.TestMode())
		if held != nil {
			heldSlug := held.Plan.Slug
			if heldSlug == "" {
				heldSlug = held.PlanId
			}
			isYear := in.Interval == types.Yearly
			heldYear := held.Plan.Interval == types.Yearly || string(held.Plan.Interval) == "year"
			samePlan := (heldSlug == q.slug || held.PlanId == q.slug) && held.Plan.Price == q.plan.Price
			sameInterval := (isYear == heldYear)
			sameQty := in.Quantity <= 1 || held.Quantity == in.Quantity
			if samePlan && sameInterval && sameQty {
				return nil, fault(http.StatusConflict, fmt.Sprintf(
					"this account already pays for the %q plan (subscription %s); change that subscription instead of buying a second one",
					heldSlug, held.Id()), nil)
			}
		}

		chargeCents := q.charge
		couponCode := ""
		if raw := strings.TrimSpace(in.Coupon); raw != "" {
			couponCode = promo.CouponCode(raw)
			if couponCode == "" {
				return nil, fault(http.StatusBadRequest, "coupon is not valid", nil)
			}
			if !engine.BillsMonthly(q.plan) {
				return nil, fault(http.StatusBadRequest,
					fmt.Sprintf("coupon %s takes a percent off monthly periods, so it applies to monthly billing only", couponCode), nil)
			}
			offer, reason := promo.FindCoupon(ctx, couponCode, time.Now())
			if reason == "" && !everyLine(q.slug, q.lines, offer.AppliesTo) {
				reason = promo.CouponNotApplicable
			}
			if reason != "" {
				return nil, fault(http.StatusBadRequest, couponRefusal(couponCode, q.slug, reason), nil)
			}
			if promo.Redeemed(ctx, couponCode, org.Name) {
				return nil, fault(http.StatusConflict, fmt.Sprintf("%v (%s)", promo.ErrCouponRedeemedByOrg, couponCode), nil)
			}
			cpn := offer.Coupon()
			chargeCents = engine.Discounted(chargeCents, cpn.Percent)
			if chargeCents <= 0 {
				return nil, fault(http.StatusBadRequest,
					fmt.Sprintf("coupon %s leaves nothing to charge for plan %q, and a card sale must charge something", couponCode, q.slug), nil)
			}
		}

		amountCents = chargeCents
		cur = q.cur
		intent = "AUTHORIZE"
		desc = fmt.Sprintf("Subscription %s — first period", q.plan.Name)
		keySeed := in.IdempotencyKey
		if keySeed == "" {
			facts := "paypal-sub-order:store:" + in.StoreID + ":plan:" + q.slug +
				":level:" + strconv.Itoa(in.Level) + ":interval:" + string(q.plan.Interval) +
				":qty:" + strconv.Itoa(q.qty) + ":coupon:" + couponCode
			keySeed = windowKey(facts)
		}
		idemKey = gatewayKey("paypal-order-subscribe", in.Subject, keySeed)
	}

	orderID, status, cerr := creator.CreateOrder(ctx, processor.PaymentRequest{
		Amount:         currency.Cents(amountCents),
		Currency:       cur,
		CustomerID:     in.Subject,
		Description:    desc,
		IdempotencyKey: idemKey,
	}, intent)
	if cerr != nil {
		log.Error("PayPal CreateOrder failed (org=%s subject=%s): %v", org.Name, in.Subject, cerr)
		return nil, fault(http.StatusBadGateway, "failed to create PayPal order", cerr)
	}

	return &PayPalOrderOut{
		OrderID:     orderID,
		AmountCents: amountCents,
		Currency:    string(cur),
		Intent:      intent,
		Status:      status,
		Environment: org.PayPalEnvironment(),
		Test:        org.TestMode(),
	}, nil
}

func saleErrToFault(err error) *PaymentFault {
	switch {
	case IsSaleRefused(err):
		return fault(http.StatusBadRequest, err.Error(), nil)
	case IsSaleNotFound(err):
		return fault(http.StatusNotFound, err.Error(), nil)
	case IsSaleConflict(err):
		return fault(http.StatusConflict, err.Error(), nil)
	case IsSaleUnchargeable(err):
		return fault(http.StatusUnprocessableEntity, err.Error(), nil)
	case IsSaleUnavailable(err):
		return fault(http.StatusServiceUnavailable, err.Error(), nil)
	default:
		return fault(http.StatusBadRequest, err.Error(), err)
	}
}

// CreatePayPalBillingOrder is the HTTP handler for POST /v1/billing/paypal/order.
func CreatePayPalBillingOrder(c *zip.Ctx) error {
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return jsonhttp.Fail(c, http.StatusUnauthorized, "sign in to pay with PayPal", nil)
	}

	var req PayPalOrderIn
	if err := c.Bind(&req); err != nil {
		return jsonhttp.Fail(c, http.StatusBadRequest, "invalid request body", err)
	}

	var subject string
	if strings.TrimSpace(req.PlanID) != "" {
		if !sold(c) {
			return jsonhttp.Fail(c, http.StatusNotFound, "plan not found", nil)
		}
		subject = subscribeSubject(c, req.UserID)
		if subject == "" {
			return jsonhttp.Fail(c, http.StatusUnauthorized, "missing identity headers", nil)
		}
		if !mayActForOrg(c) {
			own := userBillingKey(c)
			if own == "" || own == orgBillingKey(c) {
				return jsonhttp.Fail(c, http.StatusForbidden, "only an admin of this organization may subscribe it", nil)
			}
			subject = own
		}
		req.Promo = promo.Active(c)
	} else {
		subject = userBillingKey(c)
	}

	req.Subject = subject
	req.IdempotencyKey = strings.TrimSpace(c.Header("X-Idempotency-Key"))

	out, f := CreatePayPalOrder(c.Context(), org, req)
	if f != nil {
		return jsonhttp.Fail(c, f.Status, f.Message, f.Err)
	}
	return c.JSON(http.StatusCreated, out)
}
