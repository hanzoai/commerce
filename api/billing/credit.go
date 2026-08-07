package billing

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/creditledger"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	orgpkg "github.com/hanzoai/commerce/pkg/org"
	"github.com/hanzoai/commerce/util/json/http"

	. "github.com/hanzoai/commerce/types"
)

type creditRequest struct {
	Org            string `json:"org"`
	Currency       string `json:"currency"` // optional, default usd
	AmountCents    int64  `json:"amountCents"`
	Reason         string `json:"reason"`
	Tag            string `json:"tag"`
	ExpiresAt      string `json:"expiresAt"`      // RFC3339, optional
	IdempotencyKey string `json:"idempotencyKey"` // optional
}

// Credit is THE ONE way credit enters an org ledger. It appends a single
// org-keyed, non-cash credit grant to the org's balance — the same ledger
// GET /v1/billing/balance and the cloud AI spend-gate read, so a granted credit
// is immediately spendable.
//
//	POST /v1/billing/credit
//	{"org":"acme","amountCents":500,"reason":"welcome","tag":"starter-credit",
//	 "currency":"usd","expiresAt":"2027-01-01T00:00:00Z","idempotencyKey":"signup:acme"}
//
// MINT-GATED. The route is registered through middleware.Mint, which puts it
// behind middleware.PlatformOnly and records it in middleware.MintRoutes(),
// so ONLY the internal service token (cloud-api → commerce, COMMERCE_SERVICE_TOKEN)
// or a platform global admin (auth.IAMClaims.IsSuperAdmin, owner=="admin") reaches
// it; every self-service / org-owner / no-auth caller is refused (403/401) BEFORE
// the handler. A client-supplied mint amount is exactly what must never be
// self-service — that is why this is mint-gated and a user cannot credit itself.
//
// Backing: when the host injects a double-entry ledger (creditledger, set by
// commerce.EmbedConfig.Ledger — the cloud-embedded path), the credit is a balanced
// posting to the org account the AI gate reads (one ledger, no split). Standalone
// (no ledger injected), it appends a tagged Deposit to commerce's own datastore.
//
// "Starter credit" is not a special case — it is a parameterized call with
// tag=credit.StarterCreditTag, amountCents=credit.StarterCreditCents, and
// expiresAt=now+credit.StarterCreditDays; the billing/credit constants are the
// data a caller passes, not a second code path.
//
// Idempotent on idempotencyKey: the same key credits AT MOST ONCE.
func Credit(c *zip.Ctx) error {
	var req creditRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}

	org := strings.ToLower(strings.TrimSpace(req.Org))
	if org == "" {
		return http.Fail(c, 400, "org is required", nil)
	}
	if req.AmountCents <= 0 {
		return http.Fail(c, 400, "amountCents must be positive", nil)
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return http.Fail(c, 400, "reason is required", nil)
	}
	// Server-authoritative ceiling behind the gate (defense in depth, shared with
	// Deposit): even a leaked service token cannot mint an absurd balance at once.
	if maxCents := depositMaxCents(); req.AmountCents > maxCents {
		return http.Fail(c, 400, fmt.Sprintf("amountCents %d exceeds maximum of %d", req.AmountCents, maxCents), nil)
	}
	cur := strings.ToLower(strings.TrimSpace(req.Currency))
	if cur == "" {
		cur = "usd"
	}
	tag := strings.TrimSpace(req.Tag)

	var expiresAt time.Time
	if s := strings.TrimSpace(req.ExpiresAt); s != "" {
		t, perr := time.Parse(time.RFC3339, s)
		if perr != nil {
			return http.Fail(c, 400, "invalid expiresAt (want RFC3339)", perr)
		}
		expiresAt = t
	}

	// WHICH BOOKS, resolved BEFORE the two paths split, because both of them need it
	// and only one of them used to ask.
	//
	// The datastore fallback has always stamped its grant with the target org's
	// test-ness; the ledger path stated none, so its CreditInput carried the zero
	// value and a test-mode org's grant was posted into LIVE books — money the gate
	// happily spends on real inference, minted by a sandbox tenant. The two paths are
	// the same door and disagreed about the one fact that decides where the money
	// goes, so the fact is resolved once, here, off the org itself.
	targetOrg, oerr := creditTargetOrg(c, org)
	if oerr != nil {
		return http.Fail(c, 400, "invalid org", oerr)
	}
	test := targetOrg.TestMode()

	// Injected double-entry ledger (embedded-in-cloud path): the ONE ledger the AI
	// gate reads. Org-keyed; the impl handles idempotency + the balanced posting.
	if led := creditledger.Get(); led != nil {
		in := creditledger.CreditInput{
			Org: org, Currency: cur, Reason: reason, Tag: tag,
			IdempotencyKey: strings.TrimSpace(req.IdempotencyKey), AmountCents: req.AmountCents,
			Test: test,
		}
		if !expiresAt.IsZero() {
			in.ExpiresAt = &expiresAt
		}
		txID, balanceCents, err := led.Credit(c.Context(), in)
		if err != nil {
			log.Error("ledger credit failed (org=%s): %v", org, err, c)
			return http.Fail(c, 500, "failed to append credit", err)
		}
		resp := creditResponse(org, txID, cur, reason, tag, req.AmountCents, expiresAt)
		resp["balanceCents"] = balanceCents
		return c.JSON(201, resp)
	}

	return creditToDatastore(c, targetOrg, org, cur, reason, tag, req.AmountCents, expiresAt, strings.TrimSpace(req.IdempotencyKey))
}

// creditTargetOrg resolves the TARGET org named in the body — the org whose books
// this grant lands in, and therefore the authority on whether they are the sandbox
// ones.
//
// It prefers the request-scoped org (X-Org-Id) when it names the same tenant, because
// that value carries the per-request Live/Test view the balance read uses; otherwise
// it resolves the named org, which is how a platform admin credits any org without
// sending X-Org-Id.
//
// It is a function rather than two lines inside the datastore path because the LEDGER
// path needs the same answer, and asking it in only one of them is exactly how the two
// came to disagree.
func creditTargetOrg(c *zip.Ctx, org string) (*organization.Organization, error) {
	if scoped, ok := middleware.GetOrganizationOK(c); ok && scoped != nil &&
		strings.EqualFold(strings.TrimSpace(scoped.Name), org) {
		return scoped, nil
	}
	return orgpkg.Resolve(c.Context(), org)
}

// creditToDatastore is the standalone fallback: append the credit as a tagged
// Deposit to commerce's own org-namespaced ledger (the balance GET /billing/balance
// reads). Used only when no host ledger is injected. The write is authorized by the
// PlatformOnly gate alone (no handler self-authorization) — the mintauth sink fails
// closed if the gate is ever dropped.
func creditToDatastore(c *zip.Ctx, targetOrg *organization.Organization, org, cur, reason, tag string, amountCents int64, expiresAt time.Time, idemKey string) error {
	db := datastore.New(targetOrg.Namespaced(c.Context()))
	subject := strings.ToLower(strings.TrimSpace(targetOrg.Name)) // org-pooled billing key (== namespace slug)

	// Idempotency: same key ⇒ one grant; a completed key replays the stored grant
	// verbatim (200); an in-flight key 409s.
	var idemRec *idempotencykey.IdempotencyKey
	if idemKey != "" {
		rec, replay, gerr := idempotencykey.Begin(db, "billing-credit:"+subject, idemKey)
		if gerr != nil {
			log.Error("credit idempotency Begin failed (org=%s): %v", subject, gerr, c)
		} else if replay {
			if rec.Status == idempotencykey.StatusCompleted && rec.Response != "" {
				c.SetHeader("Content-Type", "application/json")
				return c.Bytes(200, []byte(rec.Response))
			}
			return http.Fail(c, 409, "credit already in progress", nil)
		} else {
			idemRec = rec
		}
	}

	trans := transaction.New(db)
	trans.Type = transaction.Deposit
	trans.DestinationId = subject
	trans.DestinationKind = "iam-user" // the gateway-spendable wallet (transaction.IAMUserKind)
	trans.Currency = currency.Type(cur)
	trans.Amount = currency.Cents(amountCents)
	trans.Notes = reason
	trans.Tags = tag
	if !expiresAt.IsZero() {
		trans.ExpiresAt = expiresAt
	}
	if targetOrg.TestMode() {
		trans.Test = true
	}
	trans.Metadata = Map{"reason": reason}

	if err := trans.Create(); err != nil {
		if idemRec != nil {
			_ = idemRec.Delete()
		}
		log.Error("failed to append credit grant (org=%s): %v", subject, err, c)
		return http.Fail(c, 500, "failed to append credit", err)
	}

	resp := creditResponse(org, trans.Id(), cur, reason, tag, amountCents, expiresAt)
	if idemRec != nil {
		if body, mErr := json.Marshal(resp); mErr == nil {
			_ = idempotencykey.Complete(idemRec, string(body))
		}
	}
	return c.JSON(201, resp)
}

// creditResponse is the ONE response shape for POST /credit, shared by the ledger
// and datastore paths (and stored verbatim for the datastore path's idempotent
// replay).
func creditResponse(org, id, cur, reason, tag string, amountCents int64, expiresAt time.Time) map[string]any {
	resp := map[string]any{
		"id":          id,
		"org":         org,
		"amountCents": amountCents,
		"currency":    cur,
		"reason":      reason,
	}
	if tag != "" {
		resp["tag"] = tag
	}
	if !expiresAt.IsZero() {
		resp["expiresAt"] = expiresAt
	}
	return resp
}
