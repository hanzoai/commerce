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
// credited subject is resolved from the caller's own signed identity
// (userBillingKey → account.Payer), never from the request, so a body value can
// not steer where the money lands. This door takes a FRESH nonce only; a card the
// subject already saved goes through POST /v1/billing/topup (paymentMethodId), the
// one saved-card door.
type topupTokenRequest struct {
	SourceID    string `json:"sourceId"` // Square Web Payments SDK nonce
	AmountCents int64  `json:"amountCents"`
	Currency    string `json:"currency,omitempty"`
}

// The default top-up floor and ceiling, in cents. They are named constants and
// not literals because the test asserts them twice — once as the boundary a
// caller may charge, once as the value a bad override falls back to — and those
// two spellings drifted: the floor moved to $5 while the fallback assertion
// still read $1, so the suite contradicted itself and failed. One name, one
// value, and no way for the two readings to disagree again.
const (
	defaultTopupMinCents int64 = 500    // $5 — the pay-as-you-go floor, owner directive 2026-08
	defaultTopupMaxCents int64 = 500000 // $5,000
)

// topupBounds returns the inclusive [min,max] card top-up amount in cents. A
// server-side bound is the authoritative enforcement of the console's own
// MIN_TOPUP_CENTS/MAX_TOPUP_CENTS policy (a client cap is advisory — a scripted
// request bypasses it). Defaults: min 500 ($5) — the pay-as-you-go floor, owner
// directive 2026-08 — max 500000 ($5,000). Override per-deploy for risk tuning
// with COMMERCE_TOPUP_MIN_CENTS / COMMERCE_TOPUP_MAX_CENTS.
func topupBounds() (int64, int64) {
	return envCents("COMMERCE_TOPUP_MIN_CENTS", defaultTopupMinCents), envCents("COMMERCE_TOPUP_MAX_CENTS", defaultTopupMaxCents)
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

	// Credit the SAME account the LLM gate reads out of and debits — the payer
	// resolved from the caller's signed identity (userBillingKey → account.Payer),
	// the ONE rule shared with GetMyBalance and the ai gate. Never ?user= and never
	// a request-supplied userId: a client- or proxy-set selector is what let the
	// same customer top up `hanzo` on one charge and `hanzo/<user>` on the next,
	// stranding the credit off the key their usage draws from. One identity, one key.
	in := TakePaymentIn{
		SourceID:       req.SourceID,
		AmountCents:    req.AmountCents,
		Currency:       req.Currency,
		Subject:        userBillingKey(c),
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
