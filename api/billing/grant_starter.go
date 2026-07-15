package billing

import (
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/credit"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/treasury"
	"github.com/hanzoai/commerce/util/json/http"
)

type grantStarterRequest struct {
	User    string `json:"user"`
	Trigger string `json:"trigger"`
}

// GrantStarter ensures an explicit subject has received the one-time $100
// starter credit, idempotently. It is the on-signup welcome grant invoked by
// trusted services (chat, cloud-api) with the commerce service token — the
// service-to-service twin of POST /billing/credit (GrantStarterCredit, the
// admin/UI path). Neither requires a payment method, so a brand-new user "just
// works" on first chat without a card (a card gates top-up BEYOND the credit).
//
// The subject is explicit and caller-supplied: "owner/name" for per-user
// (personal-org) billing, or the org slug for org-pooled billing — it MUST match
// the subject the cloud gateway debits and reads (object.BillingSubject in
// hanzoai/ai). The grant is scoped to the X-Org-Id namespace.
//
// Idempotent + race-safe: credit.GrantIfEligibleNow dedupes on the starter-credit
// tag inside a datastore transaction, so duplicate/concurrent calls (e.g. the
// same user opening several chats at once) never double-grant — no bleed.
//
//	POST /v1/billing/grant-starter   {"user":"hanzo/alice","trigger":"chat_first_use"}
func GrantStarter(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	if org == nil {
		return http.Fail(c, 401, "missing organization", nil)
	}
	db := datastore.New(org.Namespaced(c.Context()))

	var req grantStarterRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}

	user := strings.ToLower(strings.TrimSpace(req.User))
	if user == "" {
		return http.Fail(c, 400, "user is required", nil)
	}

	trigger := strings.TrimSpace(req.Trigger)
	if trigger == "" {
		trigger = "service-grant"
	}

	// Step 4: chain-backed path mints the starter credit on-chain (idempotent by
	// the shared welcome key), the service twin of GrantStarterCredit's chain path.
	if chainCreditEnabled() {
		rc, mErr := chainMintCredit(c, org, user, int64(credit.StarterCreditCents), treasury.BucketCredit,
			"starter-credit:"+trigger, welcomeMintKey(org, user))
		if mErr != nil {
			return http.Fail(c, 502, "on-chain starter credit mint failed", mErr)
		}
		return c.JSON(200, map[string]any{
			"user":     user,
			"amount":   credit.StarterCreditCents,
			"currency": "usd",
			"granted":  !rc.Replayed,
			"onChain":  true,
			"txHash":   rc.TxHash,
		})
	}

	granted, err := credit.GrantIfEligibleNow(db, user, trigger)
	if err != nil {
		return http.Fail(c, 500, "failed to grant starter credit", err)
	}

	return c.JSON(200, map[string]any{
		"user":     user,
		"amount":   credit.StarterCreditCents,
		"currency": "usd",
		"granted":  granted,
	})
}
