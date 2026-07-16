package billing

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json/http"
)

// depositMaxCents is the server-authoritative ceiling on a SINGLE Deposit/Refund,
// defense-in-depth BEHIND the PlatformOnly gate: even a leaked service token or a
// fat-fingered internal call cannot mint an absurd balance in one shot. The
// default ($1,000,000) is generous so legitimate settlement/promo deposits are
// untouched; tune per-deploy with COMMERCE_DEPOSIT_MAX_CENTS (envCents fails safe
// to the default on unset/non-positive). Reuses the topup env helper — one way to
// read a cents bound.
func depositMaxCents() int64 {
	return envCents("COMMERCE_DEPOSIT_MAX_CENTS", 100_000_000)
}

type depositRequest struct {
	User      string `json:"user"`
	Currency  string `json:"currency"`
	Amount    int64  `json:"amount"` // cents
	Notes     string `json:"notes"`
	Tags      string `json:"tags"`
	ExpiresIn int    `json:"expiresIn"` // days until expiry (0 = no expiry)
}

// Deposit creates a deposit (credit) transaction for an IAM user.
//
//	POST /v1/billing/deposit
//
// Used by internal services to add funds to a user's account (payment
// processor settlement, manual credit, promotional grants, etc.).
func Deposit(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	// ONE central commerce ledger, org-scoped by the namespace column (New binds
	// the namespace from org.Namespaced(c.Context()) — the same store+scoping the LLM
	// gateway prepaid gate and every balance read use: tier.go/zap.go/usage.go and
	// models/transaction/util). NOT NewNamespaced: that split deposits into
	// per-org SQLite FILES (/app/data/orgs/<org>/data.db) while the gate read the
	// central store → every balance read $0 → 402 on all AI. Tenancy is a value
	// (the row's org namespace + destinationId), not a place (a physical file).
	db := datastore.New(org.Namespaced(c.Context()))

	var req depositRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}

	req.User = strings.ToLower(strings.TrimSpace(req.User))
	if req.User == "" {
		return http.Fail(c, 400, "user is required", nil)
	}

	if req.Amount <= 0 {
		return http.Fail(c, 400, "amount must be positive", nil)
	}

	// Server-authoritative ceiling (H1). Bounds a single mint even for a trusted
	// caller — no unbounded credit in one request.
	if maxCents := depositMaxCents(); req.Amount > maxCents {
		return http.Fail(c, 400, fmt.Sprintf("amount %d exceeds maximum deposit of %d cents", req.Amount, maxCents), nil)
	}

	cur := currency.Type(strings.ToLower(req.Currency))
	if cur == "" {
		cur = "usd"
	}

	// Step 4: when the chain-backed ledger is enabled, a deposit MINTS on-chain
	// HUSD to the org's derived address (bucketed by its tags) instead of writing
	// a DB credit row — the value is created by the treasury key, not this process.
	// The X-Idempotency-Key (if any) makes the mint exactly-once; without one,
	// distinct deposits are legitimately additive (a fresh key each call).
	if chainCreditEnabled() {
		mintKey := randomIdemKey("deposit:" + req.User + ":")
		if hdr := strings.TrimSpace(c.Header("X-Idempotency-Key")); hdr != "" {
			mintKey = "deposit:" + req.User + ":" + hdr
		}
		reason := req.Notes
		if reason == "" {
			reason = "deposit"
		}
		rc, err := chainMintCredit(c, org, req.User, req.Amount, bucketForTags(req.Tags), reason, mintKey)
		if err != nil {
			log.Error("chain deposit mint failed for %s: %v", req.User, err, c)
			return http.Fail(c, 502, "on-chain credit mint failed", err)
		}
		return c.JSON(201, map[string]any{
			"transactionId": rc.TxHash,
			"user":          req.User,
			"amount":        req.Amount,
			"currency":      cur,
			"type":          "deposit",
			"tags":          req.Tags,
			"onChain":       true,
			"txHash":        rc.TxHash,
			"replayed":      rc.Replayed,
		})
	}

	// Optional idempotency (H1). A caller-supplied X-Idempotency-Key makes a
	// retry / double-submit credit AT MOST ONCE: a completed key replays the
	// stored body, an in-flight key 409s. Absent a key there is no guard —
	// distinct deposits to the same user (repeated settlements) are legitimately
	// additive, so we never dedupe by amount. Scoped to the subject so keys never
	// collide across users; the datastore is already org-namespaced.
	idemKey := strings.TrimSpace(c.Header("X-Idempotency-Key"))
	var idemRec *idempotencykey.IdempotencyKey
	if idemKey != "" {
		rec, replay, gerr := idempotencykey.Begin(db, "billing-deposit:"+req.User, idemKey)
		if gerr != nil {
			log.Error("deposit idempotency Begin failed (user=%s): %v", req.User, gerr, c)
		} else if replay {
			if rec.Status == idempotencykey.StatusCompleted && rec.Response != "" {
				c.SetHeader("Content-Type", "application/json")
				return c.Bytes(200, []byte(rec.Response))
			}
			return http.Fail(c, 409, "deposit already in progress", nil)
		} else {
			idemRec = rec
		}
	}

	notes := req.Notes
	if notes == "" {
		notes = fmt.Sprintf("Deposit: %d cents %s", req.Amount, cur)
	}

	trans := transaction.New(db)
	trans.Type = transaction.Deposit
	trans.DestinationId = req.User
	trans.DestinationKind = "iam-user"
	trans.Currency = cur
	trans.Amount = currency.Cents(req.Amount)
	trans.Notes = notes
	trans.Tags = req.Tags

	if req.ExpiresIn > 0 {
		trans.ExpiresAt = time.Now().AddDate(0, 0, req.ExpiresIn)
	}

	if org.TestMode() {
		trans.Test = true
	}

	if err := trans.Create(); err != nil {
		// No balance moved — release the guard so a later retry is not wedged.
		if idemRec != nil {
			_ = idemRec.Delete()
		}
		log.Error("Failed to create deposit transaction: %v", err, c)
		return http.Fail(c, 500, "failed to create deposit", err)
	}

	resp := map[string]any{
		"transactionId": trans.Id(),
		"user":          req.User,
		"amount":        req.Amount,
		"currency":      cur,
		"type":          "deposit",
		"tags":          req.Tags,
	}
	if !trans.ExpiresAt.IsZero() {
		resp["expiresAt"] = trans.ExpiresAt
	}

	// Seal the guard with the exact success body so a same-key retry replays it
	// verbatim (no second credit).
	if idemRec != nil {
		if body, mErr := json.Marshal(resp); mErr == nil {
			_ = idempotencykey.Complete(idemRec, string(body))
		}
	}

	return c.JSON(201, resp)
}
