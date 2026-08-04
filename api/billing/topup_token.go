package billing

import (
	"os"
	"strconv"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
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
//
// This is the BROWSER projection of TakePayment (payment_core.go), which holds the
// whole money move. Everything below the Bind is reading values off the request —
// the subject from ?user= bounded to the caller's org, the retry key from the
// header, the KMS client from the request locals — and handing them to the one
// core. The typed op an agent calls is the other projection of the same core, so
// the bounds, the idempotency derivation and the ledger write are shared rather
// than reimplemented.
func TopupWithToken(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)

	var req topupTokenRequest
	if err := c.Bind(&req); err != nil {
		return jsonhttp.Fail(c, 400, "invalid request body", err)
	}

	// Billing is per-org: credit the org's canonical balance, keyed by the SAME
	// subject the LLM gate reads and usage debits (see topupDestination). A
	// request-supplied userId/email must NOT become the destination key, or the
	// customer tops up one key and reads another. One identity, one key.
	in := TakePaymentIn{
		SourceID:       req.SourceID,
		AmountCents:    req.AmountCents,
		Currency:       req.Currency,
		Subject:        topupDestination(c),
		IdempotencyKey: strings.TrimSpace(c.Header("X-Idempotency-Key")),
	}
	if v := c.Locals("kms"); v != nil {
		if kmsClient, ok := v.(*kms.CachedClient); ok {
			in.KMS = kmsClient
		}
	}

	out, f := TakePayment(c.Context(), org, in)
	if f != nil {
		return jsonhttp.Fail(c, f.Status, f.Message, f.Err)
	}
	return c.JSON(200, out)
}
