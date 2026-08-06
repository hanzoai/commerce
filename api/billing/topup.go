package billing

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/mintauth"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/transaction/util"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/thirdparty/kms"
	"github.com/hanzoai/commerce/treasury"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
)

// Sentinel errors so callers (Topup HTTP handler, auto-recharge cron) can map a
// charge outcome to the right HTTP status / log without re-deriving it.
var (
	errNoProcessor            = errors.New("no payment processor available")
	errChargedButCreditFailed = errors.New("charge succeeded but balance credit failed")
	errAmountOutOfBounds      = errors.New("amount outside the permitted range")
	errGuardUnavailable       = errors.New("idempotency guard unavailable")
	errChargeInFlight         = errors.New("charge already in progress")
)

// receipt is what a completed charge seals into its idempotency guard, so a
// replay returns the SAME answer instead of moving money a second time.
type receipt struct {
	TransactionId string `json:"transactionId"`
	BalanceCents  int64  `json:"balanceCents"`
}

type topupRequest struct {
	UserID          string `json:"userId"`
	PaymentMethodID string `json:"paymentMethodId"`
	AmountCents     int64  `json:"amountCents"`
	Currency        string `json:"currency,omitempty"`
}

// chargeAndCredit charges a saved payment method via the org's KMS-hydrated
// processor registry and, on success, credits the user's prepaid balance with a
// Deposit transaction. The org MUST already be KMS-hydrated by the caller (so
// payment.ProcessorsForOrg sees real Square credentials). Returns the deposit
// transaction id and the new balance.
//
// This is the single charge primitive reused by both the on-session top-up
// (Topup) and the off-session auto-recharge cron. For a Square card-on-file the
// SourceID is the saved card id (pm.ProviderRef) and CustomerID must be the
// Square customer id — a card-on-file is only chargeable in its customer's
// context (fall back to the org slug for legacy methods saved before card-on-file).
func chargeAndCredit(c *zip.Ctx, org *organization.Organization, db *datastore.Datastore, pm *paymentmethod.PaymentMethod, amountCents int64, cur currency.Type, userId, idemKey, description string) (string, currency.Cents, error) {
	// Server-authoritative amount bounds, enforced HERE so both callers inherit
	// them: the HTTP top-up (where a scripted request bypasses the browser cap)
	// and the auto-recharge cron (where a mis-set config would otherwise charge a
	// real card an arbitrary amount, off-session, with nobody watching). Same
	// [min,max] as the token top-up — one bound, one definition.
	minCents, maxCents := topupBounds()
	if amountCents < minCents || amountCents > maxCents {
		return "", 0, fmt.Errorf("%w: %d not in [%d,%d]", errAmountOutOfBounds, amountCents, minCents, maxCents)
	}

	// Idempotency guard (money-critical). This path charges a SAVED card, which —
	// unlike a single-use nonce — is reusable, so there is no natural backstop: a
	// retried request or a re-fired cron would simply charge the card again. The
	// guard is the whole protection, so it is taken BEFORE any money moves and
	// fails CLOSED: if the store is unavailable we cannot tell a first attempt
	// from a retry, and refusing costs a retry while proceeding costs the
	// customer a duplicate charge on their statement.
	rec, replay, gerr := idemBegin(db, "billing-charge:"+userId, idemKey)
	if gerr != nil {
		return "", 0, fmt.Errorf("%w: %v", errGuardUnavailable, gerr)
	}
	if replay {
		// Already ran to completion: replay the stored answer, do not re-charge.
		if rec.Status == idempotencykey.StatusCompleted && rec.Response != "" {
			var prev receipt
			if json.Unmarshal([]byte(rec.Response), &prev) == nil {
				return prev.TransactionId, currency.Cents(prev.BalanceCents), nil
			}
		}
		// A genuine concurrent attempt for this key is still in flight — do not
		// run a second money move alongside it.
		return "", 0, errChargeInFlight
	}
	// abandon releases the guard so a later attempt is not wedged. Called ONLY on
	// failures where no money moved.
	abandon := func() {
		if rec != nil {
			_ = rec.Delete()
		}
	}

	squareCustomerID := pm.CustomerId
	if pm.Metadata != nil {
		if v, ok := pm.Metadata["squareCustomerId"].(string); ok && v != "" {
			squareCustomerID = v
		}
	}

	ctx := c.Context()
	chargeReq := processor.PaymentRequest{
		Token:      pm.ProviderRef,
		Amount:     currency.Cents(amountCents),
		Currency:   cur,
		CustomerID: squareCustomerID,
		// The gateway de-dups on this key, derived from the SAME stable guard key.
		// It is the backstop that survives a guard-store outage or two racing
		// submits, because it is enforced where the money actually moves.
		IdempotencyKey: gatewayKey("charge", userId, idemKey),
		Description:    description,
	}

	// The global registry holds empty singletons (Square is registered
	// unconfigured at init), so charges must go through the org-scoped registry.
	//
	// Via the shared processorsForOrg seam (production: payment.ProcessorsForOrg)
	// — the SAME one SubscribeWithCard and TopupWithToken use. Reaching for
	// payment.ProcessorsForOrg directly is what made this primitive untestable:
	// no test could substitute a processor, so the saved-card top-up AND the
	// off-session auto-recharge cron both ran with zero coverage.
	reg := processorsForOrg(org)
	proc, err := reg.SelectProcessor(ctx, chargeReq)
	if err != nil {
		abandon()
		return "", 0, fmt.Errorf("%w: %v", errNoProcessor, err)
	}

	result, err := proc.Charge(ctx, chargeReq)
	if err != nil {
		abandon()
		return "", 0, fmt.Errorf("charge failed: %w", err)
	}
	if !result.Success {
		abandon()
		msg := result.ErrorMessage
		if msg == "" {
			msg = "charge declined"
		}
		return "", 0, errors.New(msg)
	}

	// Step 4: the card was charged (money in), so mint the credit ON CHAIN as
	// prepaid HUSD to the org's derived address instead of a DB deposit row —
	// idempotent by the processor reference so a retried charge credits once.
	if chainCreditEnabled() {
		rc, mErr := chainMintCredit(c, org, userId, amountCents, treasury.BucketPrepaid,
			fmt.Sprintf("topup via %s (ref: %s)", proc.Type(), result.ProcessorRef),
			"topup:"+result.ProcessorRef)
		if mErr != nil {
			log.Error("RECONCILE: charge succeeded (ref=%s) but chain mint failed for user %s: %v",
				result.ProcessorRef, userId, mErr, c)
			return "", 0, fmt.Errorf("%w: ref=%s: %v", errChargedButCreditFailed, result.ProcessorRef, mErr)
		}
		var balanceCents currency.Cents
		if datas, bErr := util.GetTransactionsByCurrency(org.Namespaced(c.Context()), userId, "iam-user", cur, org.TestMode()); bErr == nil {
			if data, ok := datas.Data[cur]; ok {
				balanceCents = data.Balance
			}
		}
		seal(rec, rc.TxHash, balanceCents)
		return rc.TxHash, balanceCents, nil
	}

	trans := transaction.New(db)
	trans.Type = transaction.Deposit
	trans.DestinationId = userId
	trans.DestinationKind = "iam-user"
	trans.Currency = cur
	trans.Amount = currency.Cents(amountCents)
	trans.Notes = fmt.Sprintf("Top-up via %s (ref: %s)", proc.Type(), result.ProcessorRef)
	trans.Tags = "topup"
	// Ledger test-ness MUST follow the charge environment (org.TestMode), not
	// org.Live alone — otherwise a Square sandbox charge could credit the live
	// (spendable) balance. test==credit-bucket==read-bucket==charge-env.
	test := org.TestMode()
	trans.Test = test
	// The card was charged (result.Success): a settled payment IS the mint
	// authority, so authorize THIS write at the ledger sink (money-in == credit).
	trans.SetContext(mintauth.WithAuthorized(trans.Context()))
	if err := trans.Create(); err != nil {
		// Charge succeeded but credit failed — log with full context for manual reconciliation.
		log.Error("RECONCILE: charge succeeded (ref=%s) but deposit failed for user %s: %v",
			result.ProcessorRef, userId, err, c)
		return "", 0, fmt.Errorf("%w: ref=%s: %v", errChargedButCreditFailed, result.ProcessorRef, err)
	}

	// Read back the new balance so the caller doesn't need a separate request.
	var balanceCents currency.Cents
	if datas, err := util.GetTransactionsByCurrency(org.Namespaced(c.Context()), userId, "iam-user", cur, test); err == nil {
		if data, ok := datas.Data[cur]; ok {
			balanceCents = data.Balance
		}
	}
	seal(rec, trans.Id(), balanceCents)
	return trans.Id(), balanceCents, nil
}

// seal records the successful charge's answer on its idempotency guard so a
// retry with the same key replays it verbatim instead of charging again.
//
// Note what does NOT call this: the charge-succeeded-but-credit-failed paths
// deliberately leave the guard STARTED, so a same-key retry is refused as
// in-flight rather than re-charging a card whose money already moved. That
// wedges the key until it ages out, which is the correct trade — the failure
// needs reconciliation, not another charge.
func seal(rec *idempotencykey.IdempotencyKey, txID string, balanceCents currency.Cents) {
	if rec == nil {
		return
	}
	if body, err := json.Marshal(receipt{TransactionId: txID, BalanceCents: int64(balanceCents)}); err == nil {
		_ = idempotencykey.Complete(rec, string(body))
	}
}

// Topup charges a saved payment method and credits the user's balance.
//
//	POST /v1/billing/topup
//
// Body: { userId, paymentMethodId, amountCents, currency? }
// Returns: { transactionId, balanceCents, status }
func Topup(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)

	// Hydrate payment credentials from KMS (same pattern as checkout/subscription).
	if v := c.Locals("kms"); v != nil {
		if kmsClient, ok := v.(*kms.CachedClient); ok {
			if err := kms.Hydrate(kmsClient, org); err != nil {
				log.Error("KMS hydration failed for org %q: %v", org.Name, err, c)
			}
		}
	}

	db := datastore.New(org.Namespaced(c.Context()))

	var req topupRequest
	if err := c.Bind(&req); err != nil {
		return jsonhttp.Fail(c, 400, "invalid request body", err)
	}

	if req.PaymentMethodID == "" {
		return jsonhttp.Fail(c, 400, "paymentMethodId is required", nil)
	}

	// The credited subject is bounded to the caller's OWN org. The request names
	// a userId, but an unbounded body field decides where money lands, so it is
	// narrowed by the SAME in-org rule the token top-up and subscribe paths use:
	// the org slug, or an <org>/<user> child of it, else the org slug. Fail-secure.
	subject := inOrgSubject(orgBillingKey(c), req.UserID)
	if subject == "" {
		return jsonhttp.Fail(c, 401, "missing identity headers", nil)
	}

	cur := currency.Type(strings.ToLower(req.Currency))
	if cur == "" {
		cur = "usd"
	}

	// Load the payment method. It must be the PAYING SUBJECT's own instrument:
	// paymentMethodId is an unpinned body field that can name another subject's
	// card inside the org, and the credit lands on the caller — charging someone
	// else's card to fund your own balance is the exact cross-subject move this
	// refuses. 404, so method ids can't be probed. Stricter on purpose than the
	// read-side callerMayReachBillingSubject: a charge binds to the RESOLVED
	// paying subject, so an org member's fine <org>/<user> subject can never
	// spend the org owner's shared card.
	pm := paymentmethod.New(db)
	if err := pm.GetById(req.PaymentMethodID); err != nil {
		return jsonhttp.Fail(c, 404, "payment method not found", err)
	}
	if pm.CustomerId != subject && pm.UserId != subject {
		return jsonhttp.Fail(c, 404, "payment method not found", nil)
	}
	if strings.TrimSpace(pm.ProviderRef) == "" {
		return jsonhttp.Fail(c, 422, "saved payment method has no chargeable card — add the card again", nil)
	}

	// Stable across a retry: the saved card, the amount and the currency, in a
	// coarse window. Never the payment-method row's mutable state, never a nonce.
	guard := guardKey(c, "pm:"+req.PaymentMethodID+":amount:"+strconv.FormatInt(req.AmountCents, 10)+":cur:"+string(cur))

	desc := fmt.Sprintf("Top-up %d %s for %s", req.AmountCents, cur, subject)
	txID, balanceCents, err := chargeAndCredit(c, org, db, pm, req.AmountCents, cur, subject, guard, desc)
	if err != nil {
		switch {
		case errors.Is(err, errAmountOutOfBounds):
			minCents, maxCents := topupBounds()
			return jsonhttp.Fail(c, 400, fmt.Sprintf("amountCents must be between %d and %d", minCents, maxCents), nil)
		case errors.Is(err, errGuardUnavailable):
			log.Error("topup idempotency guard unavailable (subject=%s): %v", subject, err, c)
			return jsonhttp.Fail(c, 503, "billing is temporarily unavailable; please retry", err)
		case errors.Is(err, errChargeInFlight):
			return jsonhttp.Fail(c, 409, "top-up already in progress", nil)
		case errors.Is(err, errNoProcessor):
			log.Error("No processor available for topup: %v", err, c)
			return jsonhttp.Fail(c, 422, "no payment processor available", err)
		case errors.Is(err, errChargedButCreditFailed):
			return jsonhttp.Fail(c, 500, "charge succeeded but balance credit failed; contact support", err)
		default:
			log.Error("Charge failed for topup (subject=%s pm=%s): %v", subject, req.PaymentMethodID, err, c)
			return jsonhttp.Fail(c, 402, err.Error(), nil)
		}
	}

	return c.JSON(200, map[string]any{
		"transactionId": txID,
		"balanceCents":  balanceCents,
		"status":        "ok",
	})
}
