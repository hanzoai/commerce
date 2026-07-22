package billing

import (
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/spendalert"
	"github.com/hanzoai/commerce/util/json/http"
)

type createSpendAlertRequest struct {
	UserId    string `json:"userId"`
	Title     string `json:"title"`
	Threshold int64  `json:"threshold"`
	Currency  string `json:"currency"`
	// Scope + enforcement (issue #70).
	Project      string `json:"project"`
	Service      string `json:"service"`
	Enforce      bool   `json:"enforce"`
	SoftPct      int    `json:"softPct"`
	RateLimitRpm int    `json:"rateLimitRpm"`
}

// updateSpendAlertRequest uses pointers for every mutable field so an ABSENT
// field is preserved (partial update) rather than reset — a PATCH that only flips
// Enforce must not silently wipe the Threshold or rate limit.
type updateSpendAlertRequest struct {
	Title        *string `json:"title"`
	Threshold    *int64  `json:"threshold"`
	Project      *string `json:"project"`
	Service      *string `json:"service"`
	Enforce      *bool   `json:"enforce"`
	SoftPct      *int    `json:"softPct"`
	RateLimitRpm *int    `json:"rateLimitRpm"`
}

// maxScopeRowsPerOrg bounds how many spend-alert rows an org may hold. It caps
// both storage abuse and the cost of the per-request cap authorize scan (an
// attacker must not be able to inflate the row set to make the sum error / time
// out and thereby disable enforcement). loadOrgScopes reads at most this many.
const maxScopeRowsPerOrg = 200

// billingSubject is the ONE key spend caps are stored and looked up under, on BOTH
// the writer (this file) and the reader (the cloud gate / metering client). It is
// the ORG — the resolved organization name, which equals the validated owner claim
// (X-Org-Id). This is the EXACT key the gate uses (cloud middleware_billing.go
// identityFromCtx: `user := c.Org()`; ResourceMeter.Gate: `User: org`; the metering
// client sends `?user=<org>` + `X-Org-Id: <org>`; ScopeRules lists by `X-Org-Id`).
// Keying the WRITER on the org too makes a cap and its verdict/rate-rule resolve
// IDENTICALLY — the fix for the live bug where the writer stored under the IAM sub
// UUID while the reader looked up by org, so enforcement never bound.
//
// Caps are ORG-LEVEL policy (not per-user): any member of the org manages them,
// and the org is taken from the VALIDATED X-Org-Id (via TokenRequired), NEVER from
// a client ?user=/body field — so this is IDOR-safe and cannot cross tenants (a
// different org resolves to a different namespace AND a different subject). Empty
// only when no org is resolvable (unauthenticated).
func billingSubject(c *zip.Ctx) string {
	// Resolve the org SAFELY (#146). On the co-resident cloud embed path IAMTokenRequired
	// no-ops (no gateway-injected X-Org-Id), so a validated caller can reach a billing
	// handler with NO "organization" local. GetOrganization's unchecked type-assertion
	// panics on a nil interface there — surfacing as a 502 at the edge (cloud installs no
	// Recover, so fasthttp resets the conn) — so use the OK form and treat an absent org as
	// "no resolvable subject" ("").
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return ""
	}
	return strings.TrimSpace(org.Name)
}

// ownsAlert reports whether the caller may mutate this row: its owner (the org)
// must equal the caller's validated org. Cross-org is blocked twice over — the
// datastore is org-namespaced (a foreign row is unreachable by id) AND this check
// re-confirms the org. Org comes from the validated X-Org-Id, never a client field.
func ownsAlert(c *zip.Ctx, a *spendalert.SpendAlert) bool {
	sub := billingSubject(c)
	return sub != "" && a.UserId == sub
}

// spendAlertView is the wire shape of one row plus the DERIVED period spend for
// its scope (periodSpentCents/over/warn) — never stored, always computed from the
// deduped usage ledger for the current UTC month.
func spendAlertView(db *datastore.Datastore, test bool, a *spendalert.SpendAlert) map[string]any {
	soft := a.EffectiveSoftPct()
	view := map[string]any{
		"id":           a.Id(),
		"userId":       a.UserId,
		"title":        a.Title,
		"threshold":    a.Threshold,
		"currency":     a.Currency,
		"project":      a.Project,
		"service":      a.Service,
		"enforce":      a.Enforce,
		"softPct":      soft,
		"rateLimitRpm": a.RateLimitRpm,
		"triggeredAt":  a.TriggeredAt,
		// The UTC-month spend window this row is measured over (the Schedule the cap
		// resets on): `period` = "2006-01" and `resetsAt` = first of next UTC month.
		// Derived, never stored — so the self-service UI can show "resets on <date>"
		// straight from the canonical window without a second call.
		"period":    currentPeriod(),
		"resetsAt":  currentPeriodResetsAt(),
		"createdAt": a.CreatedAt,
		"updatedAt": a.UpdatedAt,
	}
	spent, err := scopeSpentCents(db, test, a.Project, a.Service)
	if err != nil {
		// Display must not fail on an aggregation hiccup — report the policy row
		// without derived spend rather than 500 the whole list.
		return view
	}
	view["periodSpentCents"] = spent
	// `over` derives from the SAME boundary as enforce (scopeExhausted) so the flag
	// can never drift from the verdict: over ⇔ a further billable request is refused.
	view["over"] = scopeExhausted(spent, a.Threshold)
	view["warn"] = a.Threshold > 0 && spent >= a.Threshold*int64(soft)/100
	return view
}

// saveAlert persists a modified spend-alert WITHOUT re-homing it off its synckey
// ancestor. GetById reconstructs a ROOT key (the lookup drops the ancestor), so a
// bare Update would Put the row at the root and the ancestor-scoped ListSpendAlerts
// (and loadOrgScopes, and ScopeRules) would LOSE it — the budget would vanish from
// the customer's list on any edit AND on every alert fire, silently disabling
// enforcement. Re-anchoring the key under the row's own Parent before the write
// keeps create/list/update/fire consistent. On a backend whose GetById already
// returns an ancestor-qualified key (k.Parent() != nil) this is a no-op, so it is
// safe regardless of datastore implementation.
func saveAlert(db *datastore.Datastore, a *spendalert.SpendAlert) error {
	if k := a.Key(); k != nil && a.Parent != nil && k.Parent() == nil {
		if err := a.SetKey(db.NewKey(k.Kind(), k.StringID(), k.IntID(), a.Parent)); err != nil {
			return err
		}
	}
	return a.Update()
}

// ListSpendAlerts returns the ORG's spend alerts (budgets/caps) plus derived
// period spend. It is the console Budgets read AND the source ScopeRules uses for
// the rate-limit config, so it MUST key on the same org the writer stored under.
//
//	GET /v1/billing/spend-alerts
func ListSpendAlerts(c *zip.Ctx) error {
	// #146: never panic on a missing org. The co-resident embed path can reach this read
	// with no "organization" local (IAMTokenRequired no-ops with no gateway X-Org-Id) —
	// GetOrganization would panic → 502. No resolvable org ⇒ nothing to read: an honest
	// empty list, the SAME result as an unresolvable subject below.
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return c.JSON(200, []map[string]any{})
	}
	db := datastore.New(org.Namespaced(c.Context()))
	test := org.TestMode()

	sub := billingSubject(c)
	if sub == "" {
		return c.JSON(200, []map[string]any{}) // no resolvable org — nothing to read.
	}
	rootKey := db.NewKey("synckey", "", 1, nil)
	q := spendalert.Query(db).Ancestor(rootKey).Limit(maxScopeRowsPerOrg).Filter("UserId=", sub)

	alerts := make([]*spendalert.SpendAlert, 0)
	if _, err := q.GetAll(&alerts); err != nil {
		log.Error("Failed to list spend alerts: %v", err, c)
		return http.Fail(c, 500, "failed to list spend alerts", err)
	}

	items := make([]map[string]any, 0, len(alerts))
	for _, a := range alerts {
		items = append(items, spendAlertView(db, test, a))
	}

	return c.JSON(200, items)
}

// CreateSpendAlert creates a spend alert / cap. UserId is optional (an org-wide
// or project/service scope cap has none). At least one limit must be meaningful:
// a Threshold>0 (spend cap) or a RateLimitRpm>0 (rate limit).
//
//	POST /v1/billing/spend-alerts
func CreateSpendAlert(c *zip.Ctx) error {
	// #146: resolve the org safely (see ListSpendAlerts). No validated org ⇒ a clean 401,
	// never a nil-deref panic (→ 502).
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return http.Fail(c, 401, "authentication required", nil)
	}
	db := datastore.New(org.Namespaced(c.Context()))

	var req createSpendAlertRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}

	// Key the cap on the ORG (billingSubject) — the SAME subject the gate reads by —
	// NEVER a client-supplied body userId or ?user=. This is the load-bearing fix:
	// a cap stored under the org is found by the gate's `?user=<org>` /
	// `X-Org-Id=<org>` lookup, so enforcement binds.
	subject := billingSubject(c)
	if subject == "" {
		return http.Fail(c, 401, "authentication required", nil)
	}
	req.UserId = subject

	// Bound the org's row set (storage + authorize-scan-cost abuse; keeps the cap
	// fail-closed path from being drowned by an attacker-inflated row list).
	rootKey := db.NewKey("synckey", "", 1, nil)
	if n, cerr := spendalert.Query(db).Ancestor(rootKey).Limit(maxScopeRowsPerOrg + 1).Count(); cerr == nil && n >= maxScopeRowsPerOrg {
		return http.Fail(c, 400, "spend-alert limit reached for this organization", nil)
	}

	if req.Threshold < 0 {
		return http.Fail(c, 400, "threshold must be >= 0", nil)
	}
	if req.RateLimitRpm < 0 {
		return http.Fail(c, 400, "rateLimitRpm must be >= 0", nil)
	}
	if req.Threshold <= 0 && req.RateLimitRpm <= 0 {
		return http.Fail(c, 400, "a spend cap (threshold) or a rate limit (rateLimitRpm) is required", nil)
	}
	if req.SoftPct < 0 || req.SoftPct > 100 {
		return http.Fail(c, 400, "softPct must be between 0 and 100", nil)
	}

	cur := strings.ToLower(strings.TrimSpace(req.Currency))
	if cur == "" {
		cur = "usd"
	}

	a := spendalert.New(db)
	a.UserId = req.UserId
	a.Title = req.Title
	a.Threshold = req.Threshold
	a.Currency = cur
	a.Project = spendalert.NormalizeProject(req.Project)
	a.Service = strings.TrimSpace(req.Service)
	a.Enforce = req.Enforce
	a.SoftPct = req.SoftPct
	a.RateLimitRpm = req.RateLimitRpm

	if err := a.Create(); err != nil {
		log.Error("Failed to create spend alert: %v", err, c)
		return http.Fail(c, 500, "failed to create spend alert", err)
	}

	return c.JSON(201, spendAlertView(db, org.TestMode(), a))
}

// UpdateSpendAlert patches an existing spend alert / cap. Only the fields present
// in the body change; the rest are preserved.
//
//	PATCH /v1/billing/spend-alerts/:id
func UpdateSpendAlert(c *zip.Ctx) error {
	// #146: resolve the org safely (see ListSpendAlerts). No validated org ⇒ a clean 401,
	// never a nil-deref panic (→ 502).
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return http.Fail(c, 401, "authentication required", nil)
	}
	db := datastore.New(org.Namespaced(c.Context()))

	id := c.Param("id")
	a := spendalert.New(db)
	if err := a.GetById(id); err != nil {
		return http.Fail(c, 404, "spend alert not found", err)
	}
	// Per-row ownership: refuse (as 404, not leaking existence) a row the caller
	// does not own — closes the guess-the-id IDOR on this mutating verb.
	if !ownsAlert(c, a) {
		return http.Fail(c, 404, "spend alert not found", nil)
	}

	var req updateSpendAlertRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}

	if req.Title != nil {
		a.Title = *req.Title
	}
	if req.Threshold != nil {
		if *req.Threshold < 0 {
			return http.Fail(c, 400, "threshold must be >= 0", nil)
		}
		a.Threshold = *req.Threshold
	}
	if req.Project != nil {
		a.Project = spendalert.NormalizeProject(*req.Project)
	}
	if req.Service != nil {
		a.Service = strings.TrimSpace(*req.Service)
	}
	if req.Enforce != nil {
		a.Enforce = *req.Enforce
	}
	if req.SoftPct != nil {
		if *req.SoftPct < 0 || *req.SoftPct > 100 {
			return http.Fail(c, 400, "softPct must be between 0 and 100", nil)
		}
		a.SoftPct = *req.SoftPct
	}
	if req.RateLimitRpm != nil {
		if *req.RateLimitRpm < 0 {
			return http.Fail(c, 400, "rateLimitRpm must be >= 0", nil)
		}
		a.RateLimitRpm = *req.RateLimitRpm
	}

	if err := saveAlert(db, a); err != nil {
		log.Error("Failed to update spend alert: %v", err, c)
		return http.Fail(c, 500, "failed to update spend alert", err)
	}

	return c.JSON(200, spendAlertView(db, org.TestMode(), a))
}

// DeleteSpendAlert deletes a spend alert by ID.
//
//	DELETE /v1/billing/spend-alerts/:id
func DeleteSpendAlert(c *zip.Ctx) error {
	// #146: resolve the org safely (see ListSpendAlerts). No validated org ⇒ a clean 401,
	// never a nil-deref panic (→ 502).
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return http.Fail(c, 401, "authentication required", nil)
	}
	db := datastore.New(org.Namespaced(c.Context()))

	id := c.Param("id")
	a := spendalert.New(db)
	if err := a.GetById(id); err != nil {
		return http.Fail(c, 404, "spend alert not found", err)
	}
	// Per-row ownership: refuse a row the caller does not own (guess-the-id IDOR).
	if !ownsAlert(c, a) {
		return http.Fail(c, 404, "spend alert not found", nil)
	}

	if err := a.Delete(); err != nil {
		log.Error("Failed to delete spend alert: %v", err, c)
		return http.Fail(c, 500, "failed to delete spend alert", err)
	}

	return c.JSON(204, nil)
}
