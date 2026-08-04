package billing

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/mintauth"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/transaction/util"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/thirdparty/kms"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
)

// topupTokenRequest is the wire body. There is deliberately NO userId field: the
// credited subject is resolved from the caller's own identity (topupDestination),
// never from the request, so a body value can not steer where the money lands.
type topupTokenRequest struct {
	SourceID    string `json:"sourceId"` // Square Web Payments SDK nonce
	AmountCents int64  `json:"amountCents"`
	Currency    string `json:"currency,omitempty"`
}

// topupBounds returns the inclusive [min,max] card top-up amount in cents. A
// server-side bound is the authoritative enforcement of the console's own
// MIN_TOPUP_CENTS/MAX_TOPUP_CENTS policy (a client cap is advisory — a scripted
// request bypasses it). Defaults: min 500 ($5) — the pay-as-you-go floor, owner
// directive 2026-08 — max 500000 ($5,000). Override per-deploy for risk tuning
// with COMMERCE_TOPUP_MIN_CENTS / COMMERCE_TOPUP_MAX_CENTS.
func topupBounds() (int64, int64) {
	return envCents("COMMERCE_TOPUP_MIN_CENTS", 500), envCents("COMMERCE_TOPUP_MAX_CENTS", 500000)
}

// envCents reads a positive-int-cents override from the environment, falling
// back to def when unset, non-numeric, or non-positive (fail-safe to the default
// bound rather than an unbounded/zero limit).
func envCents(key string, def int64) int64 {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return def
}

// topupDestination resolves the billing subject a top-up must credit: the SAME
// key the gateway reads (GetBalance ?user=) and debits (usage SourceId=), so a
// customer tops up exactly the account that gates their AI usage — the ONE
// canonical ledger. It defaults to the org slug (per-org billing: dedicated
// orgs, the paying model) and honors a finer in-org subject (e.g. the
// per-user `<org>/<name>` of the shared personal-billing catch-all) ONLY when
// it is provably within the caller's own org.
//
// The subject arrives as ?user=, which console2's server-side billing proxy
// sets from the validated session (overwriting any client value) and EdgeAuth
// locks to the caller's org. The in-org bound here is defense-in-depth: even if
// both were bypassed, a credit can never land outside the caller's own org —
// `s == org || s startsWith org+"/"`. Anything else falls back to the org slug,
// fail-secure. Returns "" only when no org is resolved (caller 401s).
func topupDestination(c *zip.Ctx) string {
	org := orgBillingKey(c)
	if org == "" {
		return ""
	}
	return inOrgSubject(org, c.Query("user"))
}

// TopupWithToken charges a Square Web Payments SDK nonce and credits the org's
// canonical balance. Use this for one-time card top-ups without saving a payment
// method first — the cold-customer "add credits" path.
//
//	POST /v1/billing/topup/token
//
// Body: { sourceId, amountCents, currency? }
// Header (optional): X-Idempotency-Key — a retry/double-submit with the same key
// (or, absent a key, the same (subject, amount) inside a 15-minute window) never
// double-charges or double-credits; it replays the first result. The same key is
// forwarded to Square, so the charge is exactly-once at the processor even if the
// local guard store is down.
// Returns: { transactionId, balanceCents, status }
func TopupWithToken(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)

	if v := c.Locals("kms"); v != nil {
		if kmsClient, ok := v.(*kms.CachedClient); ok {
			if err := kms.Hydrate(kmsClient, org); err != nil {
				log.Error("KMS hydration failed for org %q: %v", org.Name, err, c)
			}
		}
	}

	db := datastore.New(org.Namespaced(c.Context()))

	var req topupTokenRequest
	if err := c.Bind(&req); err != nil {
		return jsonhttp.Fail(c, 400, "invalid request body", err)
	}

	if req.SourceID == "" {
		return jsonhttp.Fail(c, 400, "sourceId is required", nil)
	}

	// Billing is per-org: credit the org's canonical balance, keyed by the
	// SAME subject the LLM gate reads and usage debits (see topupDestination).
	// A request-supplied userId/email must NOT become the destination key, or
	// the customer tops up one key and reads another. One identity, one key.
	billingKey := topupDestination(c)
	if billingKey == "" {
		return jsonhttp.Fail(c, 401, "missing identity headers", nil)
	}
	// Server-authoritative amount bounds (money-critical). The console UI caps
	// the amount, but this endpoint charges a REAL card and a scripted/forged
	// request bypasses the browser entirely — so the floor+ceiling MUST be
	// enforced here, not just client-side. Defaults mirror the console policy
	// (min $1, max $5,000: the $1 floor blocks card-testing micro-charges + the
	// Square per-txn fee floor; the $5,000 ceiling bounds a fat-finger / hostile
	// mega-charge on a cold, KYC-less card). Deployment-tunable for risk policy
	// without a rebuild (COMMERCE_TOPUP_{MIN,MAX}_CENTS). Enforced BEFORE the
	// idempotency guard and the charge — no money moves on an out-of-bounds amount.
	minCents, maxCents := topupBounds()
	if req.AmountCents < minCents || req.AmountCents > maxCents {
		return jsonhttp.Fail(c, 400, fmt.Sprintf("amountCents must be between %d and %d", minCents, maxCents), nil)
	}

	cur := currency.Type(strings.ToLower(req.Currency))
	if cur == "" {
		cur = currency.USD
	}

	// Idempotency guard (money-critical). A retry (client lost the response) or a
	// double-submit must charge + credit AT MOST ONCE. ONE derivation, shared with
	// every other card money move (idem.go): the caller's X-Idempotency-Key, else
	// the stable request facts — the AMOUNT, never the single-use nonce — bucketed
	// into a coarse window.
	idemKey := guardKey(c, "amount:"+strconv.FormatInt(req.AmountCents, 10)+":cur:"+string(cur))
	// The gateway key is derived from that SAME stable guard, so Square de-dups
	// the money move even if our guard store is unavailable or two submits race.
	// Without it the processor minted a fresh uuid per attempt and Square-side
	// dedup — the only backstop that survives a guard-store outage — was never
	// engaged at all.
	squareKey := gatewayKey("topup", billingKey, idemKey)
	rec, replay, gerr := idemBegin(db, "billing-topup:"+billingKey, idemKey)
	if gerr != nil {
		// The guard store is unavailable, so we cannot tell a first attempt from a
		// retry. Refuse. Proceeding used to be justified by "the single-use nonce
		// is still a backstop", but a re-tokenized retry carries a FRESH nonce, so
		// in exactly the case this branch fires the backstop does not exist. The
		// guard's whole purpose is the moment state is uncertain: refusing costs
		// the customer a retry, proceeding costs them a second real charge.
		log.Error("topup idempotency Begin failed (org=%s): %v", billingKey, gerr, c)
		return jsonhttp.Fail(c, 503, "billing is temporarily unavailable; please retry", gerr)
	} else if replay {
		if rec.Status == idempotencykey.StatusCompleted && rec.Response != "" {
			c.SetHeader("Content-Type", "application/json")
			return c.Bytes(200, []byte(rec.Response))
		}
		// A genuine concurrent in-flight top-up for this key — do not run a
		// second money move alongside it.
		return jsonhttp.Fail(c, 409, "top-up already in progress", nil)
	}
	// abandon releases the guard so a later attempt (with a fresh nonce) is not
	// wedged. Called ONLY on pre-credit failures where no balance moved; the
	// dead nonce prevents any double-charge on a same-nonce retry.
	abandon := func() {
		if rec != nil {
			_ = rec.Delete()
		}
	}

	ctx := c.Context()
	chargeReq := processor.PaymentRequest{
		Token:          req.SourceID,
		Amount:         currency.Cents(req.AmountCents),
		Currency:       cur,
		IdempotencyKey: squareKey, // Square de-dups the money move on this key
		Description:    fmt.Sprintf("Top-up %d %s for org %s", req.AmountCents, cur, billingKey),
	}

	// Build a per-org processor registry from the org's KMS-hydrated payment
	// credentials. The global registry holds empty singletons (Square is
	// registered unconfigured at init), so charges must go through the
	// org-scoped registry to reach the provider with real credentials.
	//
	// Via the shared `processorsForOrg` seam (production: payment.ProcessorsForOrg)
	// — the SAME one SubscribeWithCard uses. Calling payment.ProcessorsForOrg
	// directly is what made this handler untestable: no test could substitute a
	// processor, so the entire one-off top-up money path had zero coverage.
	reg := processorsForOrg(org)
	proc, err := reg.SelectProcessor(ctx, chargeReq)
	if err != nil {
		abandon()
		log.Error("No processor available for token topup: %v", err, c)
		return jsonhttp.Fail(c, 422, "no payment processor available", err)
	}

	result, err := proc.Charge(ctx, chargeReq)
	if err != nil {
		abandon()
		log.Error("Charge failed for token topup (org=%s): %v", billingKey, err, c)
		return jsonhttp.Fail(c, 402, "charge failed", err)
	}
	if !result.Success {
		abandon()
		msg := result.ErrorMessage
		if msg == "" {
			msg = "charge declined"
		}
		return jsonhttp.Fail(c, 402, msg, nil)
	}

	// Credit the org's canonical balance (per-org key — see topupDestination).
	trans := transaction.New(db)
	trans.Type = transaction.Deposit
	trans.DestinationId = billingKey
	trans.DestinationKind = "iam-user"
	trans.Currency = cur
	trans.Amount = currency.Cents(req.AmountCents)
	trans.Notes = fmt.Sprintf("Top-up via %s (ref: %s)", proc.Type(), result.ProcessorRef)
	trans.Tags = "topup"

	// Ledger test-ness MUST follow the charge environment (org.TestMode): a
	// Square sandbox charge credits the TEST bucket, never the live (spendable)
	// one. test==credit-bucket==read-bucket==charge-env.
	test := org.TestMode()
	trans.Test = test

	// The card nonce was charged (result.Success): a settled payment IS the mint
	// authority, so authorize THIS write at the ledger sink (money-in == credit).
	trans.SetContext(mintauth.WithAuthorized(trans.Context()))
	if err := trans.Create(); err != nil {
		// Charge succeeded but credit failed — leave the guard STARTED (a
		// same-key retry 409s rather than re-attempting) and log for manual
		// reconciliation. The money moved; the dead nonce blocks a re-charge.
		log.Error("RECONCILE: charge succeeded (ref=%s) but deposit failed for org %s: %v",
			result.ProcessorRef, billingKey, err, c)
		return jsonhttp.Fail(c, 500, "charge succeeded but balance credit failed; contact support", err)
	}

	// Read back the SAME key just credited so the returned balance matches
	// what me/balance will report (read == credit).
	var balanceCents currency.Cents
	if datas, err := util.GetTransactionsByCurrency(org.Namespaced(c.Context()), billingKey, "iam-user", cur, test); err == nil {
		if data, ok := datas.Data[cur]; ok {
			balanceCents = data.Balance
		}
	}

	resp := map[string]any{
		"transactionId": trans.Id(),
		"balanceCents":  balanceCents,
		"status":        "ok",
	}
	// Seal the idempotency guard with the exact success body so a retry replays
	// it verbatim (no second charge, identical response).
	if rec != nil {
		if body, mErr := json.Marshal(resp); mErr == nil {
			_ = idempotencykey.Complete(rec, string(body))
		}
	}
	return c.JSON(200, resp)
}
