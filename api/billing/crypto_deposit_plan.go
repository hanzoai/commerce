package billing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hanzoai/commerce/api/promo"
	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/checkoutsession"
	"github.com/hanzoai/commerce/models/cryptopaymentintent"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentorg"
	"github.com/hanzoai/commerce/models/pendingintent"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	orgpkg "github.com/hanzoai/commerce/pkg/org"
	types "github.com/hanzoai/commerce/types"
)

// CreateCryptoDepositForPlan opens a crypto deposit address priced from the plan
// catalog so an on-chain confirmation opens the plan's first period automatically.
func CreateCryptoDepositForPlan(ctx context.Context, org *organization.Organization, payer, chain, token string, amountCents int64, planIn CryptoPlanIn) (CryptoDeposit, error) {
	if strings.TrimSpace(planIn.PlanID) == "" {
		return CreateCryptoDeposit(ctx, org, payer, chain, token, amountCents)
	}
	if org == nil {
		return CryptoDeposit{}, errors.New("crypto deposit: no organization")
	}
	if payer == "" {
		return CryptoDeposit{}, errors.New("crypto deposit: no payer")
	}
	db := datastore.New(org.Namespaced(ctx))
	subIn := SubscribeIn{
		SourceID: "crypto-plan-quote",
		PlanID:   strings.TrimSpace(planIn.PlanID),
		Subject:  payer,
		Quantity: planIn.Quantity,
		Level:    planIn.Level,
		Interval: planIn.Interval,
		Wallet:   true,
		Promo:    planIn.Promo,
	}
	q, perr := price(db, org, subIn, subIn.PlanID, subIn.SourceID)
	if perr != nil {
		return CryptoDeposit{}, depositValidationError{perr.Error()}
	}
	chargeCents := q.charge
	couponCode := ""
	if raw := strings.TrimSpace(planIn.Coupon); raw != "" {
		couponCode = promo.CouponCode(raw)
		if couponCode == "" || !engine.BillsMonthly(q.plan) {
			return CryptoDeposit{}, depositValidationError{"coupon is not valid for this plan"}
		}
		offer, reason := promo.FindCoupon(ctx, couponCode, time.Now())
		if reason == "" && !everyLine(q.slug, q.lines, offer.AppliesTo) {
			reason = promo.CouponNotApplicable
		}
		if reason != "" {
			return CryptoDeposit{}, depositValidationError{couponRefusal(couponCode, q.slug, reason)}
		}
		if promo.Redeemed(ctx, couponCode, org.Name) {
			return CryptoDeposit{}, depositValidationError{fmt.Sprintf("%v (%s)", promo.ErrCouponRedeemedByOrg, couponCode)}
		}
		chargeCents = engine.Discounted(chargeCents, offer.Coupon().Percent)
		if chargeCents <= 0 {
			return CryptoDeposit{}, depositValidationError{"coupon leaves nothing to charge"}
		}
	}
	normChain := strings.ToLower(strings.TrimSpace(chain))
	if normChain == "" {
		normChain = "base"
	}
	normToken := strings.ToLower(strings.TrimSpace(token))
	if normToken == "" {
		normToken = "usdc"
	}
	if normToken == "usdc" && (normChain == "base" || normChain == "solana" || normChain == "tempo") {
		if pol, _, perr := ReadRoutingVersioned(ctx, org.Name); perr == nil && pol.StripeCrypto() && stripeStablecoins(ctx, processorsForOrg(org)).On {
			dep, net, f := stripeDeposit(ctx, org, payer, chargeCents, normChain,
				fmt.Sprintf("crypto-plan:%s:%s:%d", q.slug, normChain, time.Now().Unix()/idemWindowSec),
				fmt.Sprintf("Subscription %s — first period for %s", q.slug, payer))
			if f == nil && dep != nil && len(dep.Addresses) > 0 {
				addr := dep.Addresses[0].Address
				for _, a := range dep.Addresses {
					if strings.EqualFold(a.Network, net) && a.Address != "" {
						addr = a.Address
						break
					}
				}
				sess := new(checkoutsession.Session)
				sess.SetId(newSessionID())
				sess.Org, sess.Subject, sess.Test, sess.Status = org.Name, strings.ToLower(payer), org.TestMode(), checkoutsession.Pending
				sess.Line, sess.Plan, sess.Interval, sess.TotalCents, sess.Intent = checkoutsession.Plan, q.slug, string(q.plan.Interval), chargeCents, dep.ID
				if serr := checkoutsession.Put(sess); serr != nil {
					cancelDeposit(ctx, org, dep.ID)
					return CryptoDeposit{}, serr
				}
				if rerr := pendingintent.PutRef(dep.ID, org.Name, strings.ToLower(payer), pendingintent.Plan, sess.Id(), chargeCents, "usd", org.TestMode()); rerr != nil {
					cancelDeposit(ctx, org, dep.ID)
					return CryptoDeposit{}, rerr
				}
				_ = pendingintent.Bind(dep.ID, pendingintent.Terms{Delayed: true, Network: net})
				return CryptoDeposit{
					ID:             dep.ID,
					Status:         string(cryptopaymentintent.Pending),
					Chain:          net,
					Token:          normToken,
					DepositAddress: addr,
					ExpiresAt:      time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
					AmountCents:    chargeCents,
					PlanID:         q.slug,
				}, nil
			}
		}
	}
	dep, err := CreateCryptoDeposit(ctx, org, payer, chain, token, chargeCents)
	if err != nil {
		return CryptoDeposit{}, err
	}
	intent := cryptopaymentintent.New(db)
	if err := intent.GetById(dep.ID); err == nil {
		if intent.Metadata == nil {
			intent.Metadata = types.Map{}
		}
		intent.Amount = chargeCents
		intent.Metadata["planId"] = q.slug
		intent.Metadata["quantity"] = q.qty
		intent.Metadata["level"] = planIn.Level
		intent.Metadata["interval"] = string(q.plan.Interval)
		intent.Metadata["coupon"] = couponCode
		intent.Metadata["chargeCents"] = chargeCents
		_ = intent.Update()
		dep = toCryptoDepositResponse(intent)
	}
	return dep, nil
}

// activateCryptoPlan opens a plan subscription once a plan-bound crypto deposit confirms.
func activateCryptoPlan(ctx context.Context, orgName, subject string, intent *cryptopaymentintent.CryptoPaymentIntent, depositedCents int64, txHash string) (string, int64, error) {
	if intent == nil || intent.Metadata == nil {
		return "", 0, nil
	}
	planID, _ := intent.Metadata["planId"].(string)
	planID = strings.TrimSpace(planID)
	if planID == "" {
		return "", 0, nil
	}
	chargeCents := intent.Amount
	switch v := intent.Metadata["chargeCents"].(type) {
	case float64:
		if int64(v) > 0 {
			chargeCents = int64(v)
		}
	case int64:
		if v > 0 {
			chargeCents = v
		}
	}
	if chargeCents <= 0 || depositedCents < chargeCents {
		return "", 0, nil
	}
	org, err := orgpkg.Resolve(ctx, orgName)
	if err != nil || org == nil {
		return "", 0, fmt.Errorf("resolve org %q: %w", orgName, err)
	}
	db := datastore.New(org.Namespaced(ctx))
	if held := billingSubscription(db, subject, org.TestMode()); held != nil && held.Status != subscription.Trialing {
		heldSlug := held.Plan.Slug
		if heldSlug == "" {
			heldSlug = held.PlanId
		}
		if strings.EqualFold(heldSlug, planID) {
			return held.Id(), 0, nil
		}
	}
	qty := 1
	switch v := intent.Metadata["quantity"].(type) {
	case float64:
		if int(v) > 0 {
			qty = int(v)
		}
	case int:
		if v > 0 {
			qty = v
		}
	}
	level := 0
	switch v := intent.Metadata["level"].(type) {
	case float64:
		level = int(v)
	case int:
		level = v
	}
	intervalStr, _ := intent.Metadata["interval"].(string)
	interval := types.Monthly
	if intervalStr == string(types.Yearly) || intervalStr == "year" {
		interval = types.Yearly
	}
	pl, err := resolveSubscriptionPlan(db, planID)
	if err != nil {
		return "", 0, err
	}
	if pl, err = planAt(pl, level, interval); err != nil {
		return "", 0, err
	}
	lines, err := linesAt(db, pl, nil, pl.Interval, currency.USD)
	if err != nil {
		return "", 0, err
	}
	meta := map[string]interface{}{
		"source":   "subscribe/crypto",
		"intentId": intent.Id(),
		"txHash":   txHash,
	}
	couponCode, _ := intent.Metadata["coupon"].(string)
	var cpn engine.Coupon
	var couponPlans []string
	if couponCode != "" {
		if offer, reason := promo.FindCoupon(ctx, couponCode, time.Now()); reason == "" {
			cpn, couponPlans = offer.Coupon(), offer.Plans
			meta["coupon"] = couponCode
		}
	}
	sub, err := createSubscription(db, pl, &createSubscriptionRequest{
		UserId:               subject,
		PlanId:               pl.Slug,
		DefaultPaymentMethod: "credits",
		Quantity:             qty,
		Metadata:             meta,
		Test:                 org.TestMode(),
		priced:               lines,
	})
	if err != nil {
		return "", 0, err
	}
	sub.ProviderType = "credit"
	engine.Grant(sub, cpn, couponPlans, sub.PeriodStart)
	ref := txHash
	if ref == "" {
		ref = intent.Id()
	}
	if inv, ierr := engine.CreatePaidFirstInvoice(db, sub, "crypto", ref); ierr == nil && inv != nil {
		_ = paymentorg.Put(ref, org.Name, subject, inv.Id(), 0, org.TestMode())
	} else {
		log.Warn("crypto plan %s: paid first invoice not recorded for sub %s: %v", intent.Id(), sub.Id(), ierr)
	}
	return sub.Id(), chargeCents, nil
}
