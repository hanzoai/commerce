package billing

import (
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json/http"
)

// orgBillingKey returns the canonical per-org billing key: the org slug.
//
// Billing is per-org — one balance covers the whole org (see the LLM gate
// in hanzoai/ai routers/filter_balance.go, which reads
// GET /v1/billing/balance?user=<orgSlug> with X-Org-Id=<orgSlug>, and
// debits usage against SourceId=<orgSlug>). The deposit, usage, and read
// paths MUST all resolve the same key or a customer tops up one key and
// reads another. The key is the resolved org's own slug (org.Name), which
// equals the namespace we read/write — guaranteeing destination-key ==
// namespace-slug, the exact invariant the proven gate relies on.
//
// Returns "" when no org is resolved (or the privileged "platform" org,
// which has no namespace) — callers should 401.
//
// It reads the org with GetOrganizationOK, which reports absence, rather than
// GetOrganization, which MustGet-PANICS on it. The nil check below only ever meant
// anything with the OK form: reached without a resolved org — a handler mounted
// outside the auth-token group, the case GetOrganizationOK exists for — the panic
// form 500s where this must fail closed and let the caller 401. Money never bills
// a guess, and a missing org is the emptiest guess there is.
func orgBillingKey(c *zip.Ctx) string {
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(org.Name))
}

// userBillingKey is the wallet the caller pays from — the account the credential
// names, resolved by the one shared rule (payer, resolve.go). "" when no org is
// resolved, or when the credential names nobody: unattributable is refused by the
// caller, never billed free.
func userBillingKey(c *zip.Ctx) string {
	return payer(c).Subject()
}

// GetMyBalance returns the calling user's balance for a given currency.
// Identity comes from the gateway-injected X-Org-Id / X-User-Id headers;
// no admin token required.
//
//	GET /v1/billing/me/balance?currency=usd
func GetMyBalance(c *zip.Ctx) error {
	// The wallet the gate reads: the person on a personal-billing org, the org's pool
	// otherwise. Reading the org here while the gate read the person is how a customer
	// tops up one key and sees another.
	user := userBillingKey(c)
	if user == "" {
		return http.Fail(c, 401, "missing identity headers", nil)
	}

	org := middleware.GetOrganization(c)
	ctx := org.Namespaced(c.Context())

	curQ := c.Query("currency")
	if curQ == "" {
		curQ = "usd"
	}
	cur := currency.Type(strings.ToLower(curQ))

	split, err := bucketedSplit(ctx, user, cur, org.TestMode())
	if err != nil {
		return http.Fail(c, 500, "failed to query balance", err)
	}
	card := getCardOnFile(datastore.New(ctx), user)

	resp := map[string]any{
		"user":      user,
		"currency":  cur,
		"balance":   int64(split.Balance),
		"holds":     int64(split.Holds),
		"available": int64(split.Available),
	}
	for k, v := range bucketFields(split, card) {
		resp[k] = v
	}
	return c.JSON(200, resp)
}
