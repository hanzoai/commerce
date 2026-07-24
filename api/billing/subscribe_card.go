package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	storemodel "github.com/hanzoai/commerce/models/store"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/thirdparty/kms"
	"github.com/hanzoai/commerce/util/json/http"
)

// processorsForOrg resolves the per-org payment processor registry — the ONE seam
// through which saved-card subscription code reaches the payment provider. It is a
// package var so tests can inject a registry with a fake Square processor;
// production is payment.ProcessorsForOrg (built from the org's KMS-hydrated
// credentials). Callers MUST hydrate the org's creds first (hydratePaymentCreds).
var processorsForOrg = payment.ProcessorsForOrg

// subscribeIdemWindowSec is the fallback idempotency WINDOW for a header-less
// subscribe: within this window a repeated (subject, planId) subscribe de-dups (a
// lost-response retry), but after it a genuine later re-subscribe to the same plan
// proceeds — the guard is not a permanent per-plan lock. Clients that need exact
// idempotency send an X-Idempotency-Key (the SPA does), which is used verbatim and
// is never windowed.
const subscribeIdemWindowSec = 900 // 15 minutes

// hydratePaymentCreds loads the org's payment-provider credentials from KMS (via
// the request-scoped cached client in c.Locals) so processorsForOrg sees real
// Square credentials. Best-effort + non-fatal — mirrors TopupWithToken /
// CreatePaymentMethod. A no-op when KMS is not wired (dev/tests / env-var creds).
func hydratePaymentCreds(c *zip.Ctx, org *organization.Organization) {
	if v := c.Locals("kms"); v != nil {
		if kmsClient, ok := v.(*kms.CachedClient); ok {
			if err := kms.Hydrate(kmsClient, org); err != nil {
				log.Error("KMS hydration failed for org %q: %v", org.Name, err, c)
			}
		}
	}
}

// inOrgSubject narrows a requested billing subject to the caller's OWN org: it
// returns the requested subject only when it is the org slug or a <org>/<user>
// child of it, else the org slug — fail-secure, so a subject (and the money keyed
// to it) can never be steered outside the caller's org. The ONE in-org bound,
// shared by topupDestination (query) and subscribeSubject (body).
func inOrgSubject(org, requested string) string {
	s := strings.ToLower(strings.TrimSpace(requested))
	if s == org || strings.HasPrefix(s, org+"/") {
		return s
	}
	return org
}

// subscribeSubject resolves the billing subject a card subscription belongs to:
// the org slug, or a finer in-org <org>/<user> subject when the requested userId
// is provably within the caller's own org (same bound as topupDestination, but the
// subject arrives in the request BODY, not ?user=). Returns "" when no org
// resolves (caller 401s).
func subscribeSubject(c *zip.Ctx, requested string) string {
	org := orgBillingKey(c)
	if org == "" {
		return ""
	}
	return inOrgSubject(org, requested)
}

type subscribeCardRequest struct {
	SourceID string `json:"sourceId"` // Square Web Payments SDK nonce (single-use)
	PlanID   string `json:"planId"`
	UserID   string `json:"userId,omitempty"`
	StoreID  string `json:"storeId,omitempty"`
	Quantity int    `json:"quantity,omitempty"`
	Currency string `json:"currency,omitempty"`
}

// SubscribeWithCard vaults a Square card nonce as a reusable card-on-file, charges
// it for the plan's FIRST period at the SERVER-AUTHORITATIVE catalog price, and
// creates the subscription — all server-side, in one transaction of intent. The
// settled charge IS the mint authority (mirrors topup_token's mintauth.WithAuthorized
// rationale), so it creates the paid-tier subscription WITHOUT the
// CreateBillingSubscription C1-a mint gate (which forbids a self-serve paid tier
// with zero payment). The plan's INCLUDED monthly credits flow through the existing
// allotment path unchanged; the plan FEE is NOT credited to the spendable AI-credit
// wallet (a subscription is not a top-up).
//
//	POST /v1/billing/subscribe/card
//
// Body: { sourceId, planId, userId?, quantity?, currency? } — NO client amount:
// the price is the plan's catalog price, always.
// Header (optional): X-Idempotency-Key — a retry/double-submit with the same key
// (or, absent a key, the same single-use nonce) never double-charges: it replays
// the first result.
// Returns: { subscriptionId, invoiceId, planId, amountCents, currency, status }
func SubscribeWithCard(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	// Hydrate the org's payment credentials so the per-org Square processor used to
	// vault + charge the card carries real credentials.
	hydratePaymentCreds(c, org)
	db := datastore.New(org.Namespaced(c.Context()))

	var req subscribeCardRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}
	req.SourceID = strings.TrimSpace(req.SourceID)
	req.PlanID = strings.TrimSpace(req.PlanID)
	if req.SourceID == "" {
		return http.Fail(c, 400, "sourceId is required", nil)
	}
	if req.PlanID == "" {
		return http.Fail(c, 400, "planId is required", nil)
	}
	if req.StoreID != "" {
		s := storemodel.New(db)
		if err := s.GetById(req.StoreID); err != nil {
			return http.Fail(c, 404, "store not found", err)
		}
	}

	// Billing subject: the org slug, or an in-org <org>/<user> ONLY when provably
	// within the caller's own org (userId is honored only inside that bound).
	subject := subscribeSubject(c, req.UserID)
	if subject == "" {
		return http.Fail(c, 401, "missing identity headers", nil)
	}

	// Server-authoritative plan + price. A client amount is NEVER consulted — the
	// charge is the plan's catalog price × billable seats, so a scripted request
	// cannot underpay.
	p, err := resolveSubscriptionPlan(db, req.PlanID)
	if err != nil {
		return http.Fail(c, 404, "plan not found", err)
	}
	if int64(p.Price) <= 0 {
		// A $0 plan needs no card — the free tier is self-serve via POST
		// /v1/billing/subscriptions. This endpoint's contract is a PAID card sub.
		return http.Fail(c, 400,
			fmt.Sprintf("plan %q is free — no card charge required; use POST /v1/billing/subscriptions", req.PlanID), nil)
	}
	// Billable seats: a per-seat plan charges Price × quantity (floored at the
	// catalog's minSeats); a flat plan is always ×1. This MUST match
	// buildPeriodInvoice's seat math so the first charge equals the invoice's
	// AmountDue — never under-collect a per-seat plan yet record it paid in full.
	qty := req.Quantity
	if qty < 1 {
		qty = 1
	}
	seatMult := int64(1)
	if perSeat(req.PlanID) {
		if min := minSeats(req.PlanID); qty < min {
			return http.Fail(c, 400,
				fmt.Sprintf("plan %q requires at least %d seats (got %d)", req.PlanID, min, qty), nil)
		}
		seatMult = int64(qty)
	}
	chargeCents := int64(p.Price) * seatMult
	cur := currency.Type(strings.ToLower(strings.TrimSpace(req.Currency)))
	if cur == "" {
		cur = currency.USD
	}

	// Idempotency (money-critical). The client SHOULD send a STABLE X-Idempotency-Key
	// per checkout attempt (the SPA does), so a lost-response retry — even with a
	// FRESH single-use nonce — replays instead of vaulting + charging again. Absent a
	// header, fall back to the STABLE (subject, planId): the single-use nonce is NOT a
	// stable key (a re-tokenized retry mints a new nonce), which is exactly the
	// double-charge to avoid. Scoped to the subject so keys never collide across tenants.
	guardKey := strings.TrimSpace(c.Header("X-Idempotency-Key"))
	if guardKey == "" {
		// No client key: fall back to (subject, planId) bucketed by a coarse time
		// WINDOW so a lost-response retry (seconds apart) de-dups but a genuine later
		// re-subscribe is not blocked forever (see subscribeIdemWindowSec).
		guardKey = "store:" + req.StoreID + ":plan:" + req.PlanID + ":w" +
			strconv.FormatInt(time.Now().Unix()/subscribeIdemWindowSec, 10)
	}
	// The Square idempotency key is derived from the SAME stable guard key (never the
	// single-use nonce), so Square itself de-dups the money move even if the local
	// guard store is unavailable OR two submits race — the definitive backstop.
	squareKey := "subscribe:" + subject + ":" + guardKey

	rec, replay, gerr := idempotencykey.Begin(db, "billing-subscribe:"+subject+":"+req.StoreID, guardKey)
	if gerr != nil {
		// Guard store unavailable. Proceed WITHOUT the local guard: the stable Square
		// idempotency key above still makes the CHARGE exactly-once at the processor,
		// so a retry can't double-charge (at worst a duplicate sub row, far less
		// severe than a double charge) rather than block a legitimate first subscribe.
		log.Error("subscribe idempotency Begin failed (subject=%s): %v", subject, gerr, c)
		rec = nil
	} else if replay {
		if rec.Status == idempotencykey.StatusCompleted && rec.Response != "" {
			c.SetHeader("Content-Type", "application/json")
			return c.Bytes(200, []byte(rec.Response))
		}
		return http.Fail(c, 409, "subscription already in progress", nil)
	}
	// abandon releases the guard on pre-charge failures where no money moved, so a
	// later attempt is not wedged.
	abandon := func() {
		if rec != nil {
			_ = rec.Delete()
		}
	}

	// Vault the single-use nonce as a reusable card-on-file (one Square customer per
	// org). Creating the card on file ALSO validates the card, so this verifies AND
	// saves in the single nonce use — we then charge the card-on-file id, never the
	// spent nonce (never both with one nonce).
	reg := processorsForOrg(org)
	cp, ok := squareCustomerProcessorFrom(reg)
	if !ok {
		abandon()
		return http.Fail(c, 422, "no card-on-file payment processor available for this organization", nil)
	}
	iamEmail, _ := c.Locals("iam_email").(string)
	cof, err := attachSquareCardOnFile(c.Context(), cp, existingSquareCustomerID(db, subject), strings.TrimSpace(iamEmail), subject, req.SourceID)
	if err != nil {
		abandon()
		return http.Fail(c, 402, parseCardDeclineReason(&processor.PaymentResult{ErrorMessage: err.Error()}, err), nil)
	}

	// Charge the SAVED card (card-on-file id + Square customer id) for the first
	// period at the server-authoritative price × seats, with the stable Square
	// idempotency key. All-or-nothing: on a decline no subscription is created.
	res, err := chargeSavedCard(c.Context(), reg, cof.CardID, cof.CustomerID, chargeCents, cur, squareKey,
		fmt.Sprintf("Subscription %s — first period", p.Name))
	if err != nil || res == nil || !res.Success {
		// The charge failed AFTER vaulting — delete the just-vaulted card-on-file so
		// a declined attempt leaves NO orphaned Square card (best-effort).
		_ = cp.RemovePaymentMethod(c.Context(), cof.CustomerID, cof.CardID)
		abandon()
		log.Error("saved-card charge failed for subscribe (subject=%s): %v", subject, err, c)
		return http.Fail(c, 402, parseCardDeclineReason(res, err), nil)
	}

	// Persist the vaulted card as a payment method (ProviderRef = card-on-file id,
	// Metadata.squareCustomerId = Square customer) so renewals can re-charge it —
	// the SAME convention as CreatePaymentMethod.
	pm := paymentmethod.New(db)
	pm.CustomerId = subject
	pm.UserId = subject
	pm.Type = "card"
	pm.ProviderRef = cof.CardID
	pm.ProviderType = string(processor.Square)
	pm.IsDefault = true
	pm.Metadata = map[string]interface{}{
		"squareCustomerId": cof.CustomerID,
		"squareCardId":     cof.CardID,
		"squareVerified":   true,
	}
	if err := pm.Create(); err != nil {
		// The card was charged but persisting the reusable method failed — the first
		// period is paid, yet renewals can't re-charge. Leave the guard STARTED (a
		// same-key retry replays / 409s, never re-charges) and log for reconciliation.
		log.Error("RECONCILE: subscribe charge succeeded (ref=%s) but payment-method persist failed for subject %s: %v",
			res.ProcessorRef, subject, err, c)
		return http.Fail(c, 500, "charge succeeded but saving the card failed; contact support", err)
	}

	// Create the subscription server-side, authorized because the payment settled
	// (mint authority = the settled charge — the CreateBillingSubscription C1-a gate
	// is deliberately not on this path). No trial: the customer is paying now, so
	// the first period is Active immediately.
	p.TrialPeriodDays = 0
	sub, err := createSubscription(c, db, org, p, &createSubscriptionRequest{
		UserId:               subject,
		PlanId:               req.PlanID,
		StoreId:              req.StoreID,
		DefaultPaymentMethod: pm.Id(),
		Quantity:             qty,
		Metadata:             map[string]interface{}{"source": "subscribe/card"},
	})
	if err != nil {
		// Money moved + card saved; surface the failure. Leave the guard STARTED so a
		// retry replays rather than re-charging.
		log.Error("RECONCILE: subscribe charge succeeded (ref=%s) but subscription create failed for subject %s: %v",
			res.ProcessorRef, subject, err, c)
		return subscriptionCreateError(c, err)
	}

	// Mark the FIRST period PAID by the card charge — a paid BillingInvoice
	// referencing the processor ref — and advance to the next period. This also makes
	// the subscription payment-backed (Square provider + a linked invoice), so its
	// INCLUDED monthly allotment flows (subscriptionPaymentBacked).
	sub.ProviderType = string(processor.Square)
	inv, err := engine.CreatePaidFirstInvoice(db, sub, "card", res.ProcessorRef)
	if err != nil {
		// The subscription exists and the money is collected; a first-invoice record
		// failure is non-fatal to the entitlement. Log for reconciliation.
		log.Error("subscribe: failed to record paid first invoice (subject=%s, ref=%s): %v", subject, res.ProcessorRef, err, c)
	}
	if err := sub.Update(); err != nil {
		log.Error("subscribe: failed to update subscription after first invoice (subject=%s): %v", subject, err, c)
	}

	if inv != nil {
		emitInvoicePaid(c, org.Name, inv)
	}

	invoiceID := ""
	if inv != nil {
		invoiceID = inv.Id()
	}
	resp := map[string]any{
		"subscriptionId": sub.Id(),
		"invoiceId":      invoiceID,
		"planId":         req.PlanID,
		"amountCents":    chargeCents,
		"currency":       cur,
		"status":         "ok",
	}
	// Seal the idempotency guard with the exact success body so a retry replays it
	// verbatim (no second vault/charge/subscribe, identical response).
	if rec != nil {
		if body, mErr := json.Marshal(resp); mErr == nil {
			_ = idempotencykey.Complete(rec, string(body))
		}
	}
	return c.JSON(201, resp)
}

// chargeSavedCard charges an already-vaulted card-on-file (cardID owned by the
// Square customerID) for amountCents via the org's processor registry. It is the
// ONE saved-card charge path — shared by SubscribeWithCard's first charge and the
// renewal ProviderCharger. reg MUST be a per-org registry (processorsForOrg(org))
// whose org has hydrated Square credentials.
func chargeSavedCard(ctx context.Context, reg *processor.Registry, cardID, customerID string, amountCents int64, cur currency.Type, idempotencyKey, desc string) (*processor.PaymentResult, error) {
	req := processor.PaymentRequest{
		Token:          cardID,     // Square card-on-file id as the SourceID
		CustomerID:     customerID, // the Square customer that owns the card (required for card-on-file)
		Amount:         currency.Cents(amountCents),
		Currency:       cur,
		IdempotencyKey: idempotencyKey, // Square de-dups the money move on this key
		Description:    desc,
	}
	proc, err := reg.SelectProcessor(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("no payment processor for saved-card charge: %w", err)
	}
	return proc.Charge(ctx, req)
}

// chargeProviderForOrg returns an engine.ProviderCharger bound to org: given an
// open invoice, it resolves the invoice's subscription → DefaultPaymentMethod →
// the vaulted card (paymentmethod.ProviderRef = card-on-file id, Metadata
// squareCustomerId) and charges the remaining amount on it via the org's Square
// processor, returning the processor ref. org MUST already be KMS-hydrated by the
// caller (as TopupWithToken / auto-recharge do) so processorsForOrg sees real
// credentials. A resolution/charge failure returns an error → CollectInvoice
// leaves the invoice OPEN (no double-charge on retry).
func chargeProviderForOrg(org *organization.Organization) engine.ProviderCharger {
	return func(ctx context.Context, db *datastore.Datastore, inv *billinginvoice.BillingInvoice, amountCents int64) (string, error) {
		if amountCents <= 0 {
			return "", fmt.Errorf("nothing to charge")
		}
		if inv.SubscriptionId == "" {
			return "", fmt.Errorf("invoice %s has no subscription; no card on file to charge", inv.Id())
		}
		sub := subscription.New(db)
		if err := sub.GetById(inv.SubscriptionId); err != nil {
			return "", fmt.Errorf("subscription %s not found: %w", inv.SubscriptionId, err)
		}
		if strings.TrimSpace(sub.DefaultPaymentMethod) == "" {
			return "", fmt.Errorf("subscription %s has no default payment method", sub.Id())
		}
		pm := paymentmethod.New(db)
		if err := pm.GetById(sub.DefaultPaymentMethod); err != nil {
			return "", fmt.Errorf("payment method %s not found: %w", sub.DefaultPaymentMethod, err)
		}
		cardID := strings.TrimSpace(pm.ProviderRef)
		if cardID == "" {
			return "", fmt.Errorf("payment method %s has no card-on-file token", pm.Id())
		}
		cur := inv.Currency
		if cur == "" {
			cur = currency.USD
		}
		// Square idempotency key scoped to (subscription, period, attempt). The PERIOD
		// (not the invoice id) is the shared axis: concurrent renewals each build their
		// OWN invoice row for the same period, so an invoice-id key would NOT de-dup them
		// — the period does. AttemptCount makes each dunning RETRY a FRESH key so the
		// processor lets the re-collection through (a period-only key would make the
		// gateway replay the first decline forever and wedge dunning).
		squareKey := "collect:sub:" + inv.SubscriptionId +
			":period:" + strconv.FormatInt(inv.PeriodStart.Unix(), 10) +
			":attempt:" + strconv.Itoa(inv.AttemptCount)
		reg := processorsForOrg(org)
		res, err := chargeSavedCard(ctx, reg, cardID, squareCustomerIDOf(pm), amountCents, cur, squareKey,
			fmt.Sprintf("Subscription renewal invoice %s", inv.NumberStr))
		if err != nil || res == nil || !res.Success {
			return "", fmt.Errorf("%s", parseCardDeclineReason(res, err))
		}
		return res.ProcessorRef, nil
	}
}

// squareCustomerIDOf reads the Square customer id stored on a vaulted payment
// method (the CreatePaymentMethod / subscribe-card convention:
// Metadata["squareCustomerId"]). A card-on-file is only chargeable in its Square
// customer's context, so this is required to charge a saved card.
func squareCustomerIDOf(pm *paymentmethod.PaymentMethod) string {
	if pm.Metadata != nil {
		if v, ok := pm.Metadata["squareCustomerId"].(string); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
