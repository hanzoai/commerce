package billing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/spendalert"
	"github.com/hanzoai/commerce/util/json/http"
)

// AlertSpec is what a new spend alert / cap is opened from. It takes Enforce as a
// POINTER for the same reason AlertPatch does: a plain bool collapses "the caller said
// nothing" into "the caller said false", and here those must mean opposite things. A
// budget a customer sets is a budget they want held, so an ABSENT enforce creates a
// HARD cap; only an explicit "enforce": false buys the warn-only row. Every other
// field is fine as a plain value because its zero IS the honest default (no rate
// limit, DefaultSoftPct, usd).
//
// The owner is NOT a field here. A cap is keyed on the caller's validated org, passed
// to CreateAlert beside the spec, so there is no body field a caller could aim at
// another tenant.
type AlertSpec struct {
	Title     string `json:"title"`
	Threshold int64  `json:"threshold"`
	Currency  string `json:"currency"`
	// Scope + enforcement (issue #70).
	Project      string `json:"project"`
	Service      string `json:"service"`
	Enforce      *bool  `json:"enforce"`
	SoftPct      int    `json:"softPct"`
	RateLimitRpm int    `json:"rateLimitRpm"`
}

// AlertPatch is a partial change to a spend alert: every mutable field is a pointer so
// an ABSENT field is preserved rather than reset — a change that only flips Enforce
// must not silently wipe the Threshold or the rate limit.
type AlertPatch struct {
	Title        *string `json:"title"`
	Threshold    *int64  `json:"threshold"`
	Project      *string `json:"project"`
	Service      *string `json:"service"`
	Enforce      *bool   `json:"enforce"`
	SoftPct      *int    `json:"softPct"`
	RateLimitRpm *int    `json:"rateLimitRpm"`
}

// Alert is one spend alert / cap as its readers see it: the stored policy row plus the
// DERIVED period spend for its scope — never stored, always computed from the deduped
// usage ledger for the current UTC month.
//
// PeriodSpentCents, Over and Warn are POINTERS because their ABSENCE carries meaning
// of its own. When the aggregation cannot be read the policy row is still reported,
// without derived spend, rather than failing the whole read; a zero cannot say that,
// since "nothing spent" and "spend unknown" are different answers.
type Alert struct {
	Id           string `json:"id"`
	UserId       string `json:"userId"`
	Title        string `json:"title"`
	Threshold    int64  `json:"threshold"`
	Currency     string `json:"currency"`
	Project      string `json:"project"`
	Service      string `json:"service"`
	Enforce      bool   `json:"enforce"`
	SoftPct      int    `json:"softPct"`
	RateLimitRpm int    `json:"rateLimitRpm"`
	TriggeredAt  string `json:"triggeredAt"`

	// The UTC-month spend window this row is measured over (the Schedule the cap
	// resets on): Period = "2006-01" and ResetsAt = first of next UTC month. Derived,
	// never stored — so the self-service UI can show "resets on <date>" straight from
	// the canonical window without a second call.
	Period    string    `json:"period"`
	ResetsAt  string    `json:"resetsAt"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	PeriodSpentCents *int64 `json:"periodSpentCents,omitempty"`
	// Over derives from the SAME boundary as enforce (scopeExhausted) so the flag can
	// never drift from the verdict: over ⇔ a further billable request is refused.
	Over *bool `json:"over,omitempty"`
	Warn *bool `json:"warn,omitempty"`
}

// alertValidationError marks a client-side (HTTP 400) spend-alert failure — a
// threshold, softPct or row-count refusal — as distinct from an internal (500) one, so
// a core can refuse the caller's values without knowing what a status code is.
type alertValidationError struct{ msg string }

func (e alertValidationError) Error() string { return e.msg }

// errAlertNotFound is the miss: no such row in the caller's org, OR a row the caller
// does not own. ONE error for both, because callers get one answer — saying which of
// the two it was turns a guessed id into an oracle for what exists.
var errAlertNotFound = errors.New("spend alert: no such alert for this org")

// IsAlertNotFound reports whether an alert core refused because the row is not the
// caller's to see. A row that does not exist and a row owned by another tenant answer
// the SAME way on purpose: distinguishing them would let anyone walk ids and learn
// which ones are real, so every caller — this module's endpoint and the cloud side alike —
// renders both as one miss.
func IsAlertNotFound(err error) bool { return errors.Is(err, errAlertNotFound) }

// IsAlertRefusal reports whether an alert core refused the caller's own values — a
// negative threshold, a softPct out of range, an org already at its row limit. It is
// the line between a mistake the caller can fix and a store that failed us: the first
// is answered by handing the message back for correction, the second by admitting the
// failure is ours. Every caller draws that line in the same place by asking here.
func IsAlertRefusal(err error) bool {
	var ve alertValidationError
	return errors.As(err, &ve)
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

// spendAlertView renders one row plus the DERIVED period spend for its scope
// (PeriodSpentCents/Over/Warn) — never stored, always computed from the deduped usage
// ledger for the current UTC month.
func spendAlertView(db *datastore.Datastore, test bool, a *spendalert.SpendAlert) Alert {
	soft := a.EffectiveSoftPct()
	view := Alert{
		Id:           a.Id(),
		UserId:       a.UserId,
		Title:        a.Title,
		Threshold:    a.Threshold,
		Currency:     a.Currency,
		Project:      a.Project,
		Service:      a.Service,
		Enforce:      a.Enforce,
		SoftPct:      soft,
		RateLimitRpm: a.RateLimitRpm,
		TriggeredAt:  a.TriggeredAt,
		Period:       currentPeriod(),
		ResetsAt:     currentPeriodResetsAt(),
		CreatedAt:    a.CreatedAt,
		UpdatedAt:    a.UpdatedAt,
	}
	spent, err := scopeSpentCents(db, test, a.Project, a.Service)
	if err != nil {
		// Display must not fail on an aggregation hiccup — report the policy row
		// without derived spend rather than 500 the whole list.
		return view
	}
	over := scopeExhausted(spent, a.Threshold)
	warn := a.Threshold > 0 && spent >= a.Threshold*int64(soft)/100
	view.PeriodSpentCents, view.Over, view.Warn = &spent, &over, &warn
	return view
}

// saveAlert persists a modified spend-alert WITHOUT re-homing it off its synckey
// ancestor. GetById reconstructs a ROOT key (the lookup drops the ancestor), so a
// bare Update would Put the row at the root and the ancestor-scoped ListAlerts
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

// ownedAlert reads one of subject's alerts by id, or answers errAlertNotFound. The
// caller's org is enforced twice over: the datastore is org-namespaced, so a foreign
// row is unreachable by id at all, and the row's own owner must still equal subject.
// An empty subject owns nothing — it must never match a row that carries no owner.
func ownedAlert(db *datastore.Datastore, subject, id string) (*spendalert.SpendAlert, error) {
	a := spendalert.New(db)
	if err := a.GetById(id); err != nil {
		return nil, fmt.Errorf("%w: %v", errAlertNotFound, err)
	}
	if subject == "" || a.UserId != subject {
		return nil, errAlertNotFound
	}
	return a, nil
}

// ListAlerts is the ORG's spend alerts (budgets/caps) with the period spend each is
// measured against — the QUERY, with no HTTP in it.
//
// It takes values rather than a request so a caller that is not a request can ask. The
// same rows drive the console Budgets page, the gate's rate-limit rules and the cap
// verdict, and a peer that holds no datastore reads them over the internal plane;
// re-deriving the query there would be a second implementation of one question, and
// two copies of a spend cap is how a cap and a rate limit come to disagree about which
// requests they bind.
//
// subject is the key the rows are stored under — the caller's validated org, resolved
// at the endpoint (billingSubject) and never taken from a client field, so the writer and
// every reader resolve one cap identically. Nobody owns nothing: an empty subject reads
// empty rather than matching rows that carry no owner.
//
// It returns the ROWS as their readers see them, not an envelope: the HTTP handler
// wraps these its way and a peer wraps them its own.
func ListAlerts(ctx context.Context, org *organization.Organization, subject string) ([]Alert, error) {
	if org == nil {
		return nil, errors.New("alerts: no organization")
	}
	if subject == "" {
		return make([]Alert, 0), nil
	}
	db := datastore.New(org.Namespaced(ctx))
	rootKey := db.NewKey("synckey", "", 1, nil)
	q := spendalert.Query(db).Ancestor(rootKey).Limit(maxScopeRowsPerOrg).Filter("UserId=", subject)

	alerts := make([]*spendalert.SpendAlert, 0)
	if _, err := q.GetAll(&alerts); err != nil {
		return nil, err
	}

	test := org.TestMode()
	items := make([]Alert, 0, len(alerts))
	for _, a := range alerts {
		items = append(items, spendAlertView(db, test, a))
	}
	return items, nil
}

// ListSpendAlerts returns the ORG's spend alerts (budgets/caps) plus derived
// period spend. It is the console Budgets read AND the source ScopeRules uses for
// the rate-limit config, so it MUST key on the same org the writer stored under.
//
//	GET /v1/billing/alerts
func ListSpendAlerts(c *zip.Ctx) error {
	// #146: never panic on a missing org. The co-resident embed path can reach this read
	// with no "organization" local (IAMTokenRequired no-ops with no gateway X-Org-Id) —
	// GetOrganization would panic → 502. No resolvable org ⇒ nothing to read: an honest
	// empty list, the SAME answer ListAlerts gives for an unresolvable subject.
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return c.JSON(200, []Alert{})
	}

	items, err := ListAlerts(c.Context(), org, billingSubject(c))
	if err != nil {
		log.Error("Failed to list spend alerts: %v", err, c)
		return http.Fail(c, 500, "failed to list spend alerts", err)
	}

	return c.JSON(200, items)
}

// CreateAlert opens a spend alert / cap for subject — the WRITE, with no HTTP in it.
// At least one limit must be meaningful: a Threshold>0 (spend cap) or a
// RateLimitRpm>0 (rate limit).
//
// It takes values rather than a request because the endpoint is not the only writer: a
// peer that provisions a tenant's budget over the internal plane must open the row the
// same way, and a second create path is how two rows come to describe one budget.
//
// subject is the key the row is stored under — the caller's validated org, resolved by
// the caller, NEVER a client-supplied userId. That is the load-bearing part: a cap
// stored under the org is found by the gate's `?user=<org>` / `X-Org-Id=<org>` lookup,
// so enforcement binds.
//
// A refusal of the caller's values answers IsAlertRefusal; anything else is the store
// failing. Which status each becomes is the endpoint's business, not this one's.
func CreateAlert(ctx context.Context, org *organization.Organization, subject string, spec AlertSpec) (*Alert, error) {
	if org == nil {
		return nil, errors.New("alerts: no organization")
	}
	if subject == "" {
		return nil, errors.New("alerts: no subject")
	}
	db := datastore.New(org.Namespaced(ctx))

	// Bound the org's row set (storage + authorize-scan-cost abuse; keeps the cap
	// fail-closed path from being drowned by an attacker-inflated row list).
	rootKey := db.NewKey("synckey", "", 1, nil)
	if n, cerr := spendalert.Query(db).Ancestor(rootKey).Limit(maxScopeRowsPerOrg + 1).Count(); cerr == nil && n >= maxScopeRowsPerOrg {
		return nil, alertValidationError{"spend-alert limit reached for this organization"}
	}

	if spec.Threshold < 0 {
		return nil, alertValidationError{"threshold must be >= 0"}
	}
	if spec.RateLimitRpm < 0 {
		return nil, alertValidationError{"rateLimitRpm must be >= 0"}
	}
	if spec.Threshold <= 0 && spec.RateLimitRpm <= 0 {
		return nil, alertValidationError{"a spend cap (threshold) or a rate limit (rateLimitRpm) is required"}
	}
	if spec.SoftPct < 0 || spec.SoftPct > 100 {
		return nil, alertValidationError{"softPct must be between 0 and 100"}
	}

	cur := strings.ToLower(strings.TrimSpace(spec.Currency))
	if cur == "" {
		cur = "usd"
	}

	a := spendalert.New(db)
	a.UserId = subject
	a.Title = spec.Title
	a.Threshold = spec.Threshold
	a.Currency = cur
	a.Project = spendalert.NormalizeProject(spec.Project)
	a.Service = strings.TrimSpace(spec.Service)
	// A new cap enforces unless its creator opted out. The customer who set a ceiling
	// meant the ceiling; a row that only warns while spend runs past it is a budget in
	// name only. Existing rows are untouched by this — the default lives HERE, on the
	// create path, so nothing rewrites a cap someone already chose to keep soft.
	a.Enforce = spec.Enforce == nil || *spec.Enforce
	a.SoftPct = spec.SoftPct
	a.RateLimitRpm = spec.RateLimitRpm

	if err := a.Create(); err != nil {
		return nil, err
	}

	view := spendAlertView(db, org.TestMode(), a)
	return &view, nil
}

// CreateSpendAlert creates a spend alert / cap for the calling org. At least one limit
// must be meaningful: a Threshold>0 (spend cap) or a RateLimitRpm>0 (rate limit).
//
//	POST /v1/billing/alerts
func CreateSpendAlert(c *zip.Ctx) error {
	// #146: resolve the org safely (see ListSpendAlerts). No validated org ⇒ a clean 401,
	// never a nil-deref panic (→ 502).
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return http.Fail(c, 401, "authentication required", nil)
	}

	var spec AlertSpec
	if err := c.Bind(&spec); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}

	// Key the cap on the ORG (billingSubject) — the SAME subject the gate reads by —
	// NEVER a client-supplied body userId or ?user=.
	subject := billingSubject(c)
	if subject == "" {
		return http.Fail(c, 401, "authentication required", nil)
	}

	a, err := CreateAlert(c.Context(), org, subject, spec)
	if err != nil {
		if IsAlertRefusal(err) {
			return http.Fail(c, 400, err.Error(), nil)
		}
		log.Error("Failed to create spend alert: %v", err, c)
		return http.Fail(c, 500, "failed to create spend alert", err)
	}

	return c.JSON(201, a)
}

// UpdateAlert applies a partial change to one of subject's alerts — the WRITE, with no
// HTTP in it. Only the fields the patch carries change; the rest are preserved.
//
// It takes values rather than a request for the reason CreateAlert does: the budget a
// peer adjusts over the internal plane and the budget the console edits are one row,
// and one row deserves one write path.
//
// Ownership is part of the question rather than a layer above it — a row belonging to
// anyone but subject answers errAlertNotFound, exactly as a row that does not exist,
// so a guessed id never becomes an oracle for what the org holds.
func UpdateAlert(ctx context.Context, org *organization.Organization, subject, id string, patch AlertPatch) (*Alert, error) {
	if org == nil {
		return nil, errors.New("alerts: no organization")
	}
	db := datastore.New(org.Namespaced(ctx))

	a, err := ownedAlert(db, subject, id)
	if err != nil {
		return nil, err
	}

	if patch.Title != nil {
		a.Title = *patch.Title
	}
	if patch.Threshold != nil {
		if *patch.Threshold < 0 {
			return nil, alertValidationError{"threshold must be >= 0"}
		}
		a.Threshold = *patch.Threshold
	}
	if patch.Project != nil {
		a.Project = spendalert.NormalizeProject(*patch.Project)
	}
	if patch.Service != nil {
		a.Service = strings.TrimSpace(*patch.Service)
	}
	if patch.Enforce != nil {
		a.Enforce = *patch.Enforce
	}
	if patch.SoftPct != nil {
		if *patch.SoftPct < 0 || *patch.SoftPct > 100 {
			return nil, alertValidationError{"softPct must be between 0 and 100"}
		}
		a.SoftPct = *patch.SoftPct
	}
	if patch.RateLimitRpm != nil {
		if *patch.RateLimitRpm < 0 {
			return nil, alertValidationError{"rateLimitRpm must be >= 0"}
		}
		a.RateLimitRpm = *patch.RateLimitRpm
	}

	if err := saveAlert(db, a); err != nil {
		return nil, err
	}

	view := spendAlertView(db, org.TestMode(), a)
	return &view, nil
}

// UpdateSpendAlert patches an existing spend alert / cap. Only the fields present
// in the body change; the rest are preserved.
//
//	PATCH /v1/billing/alerts/:id
func UpdateSpendAlert(c *zip.Ctx) error {
	// #146: resolve the org safely (see ListSpendAlerts). No validated org ⇒ a clean 401,
	// never a nil-deref panic (→ 502).
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return http.Fail(c, 401, "authentication required", nil)
	}

	var patch AlertPatch
	if err := c.Bind(&patch); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}

	a, err := UpdateAlert(c.Context(), org, billingSubject(c), c.Param("id"), patch)
	if err != nil {
		// A row the caller does not own is refused AS a miss — no existence leak.
		if IsAlertNotFound(err) {
			return http.Fail(c, 404, "spend alert not found", err)
		}
		if IsAlertRefusal(err) {
			return http.Fail(c, 400, err.Error(), nil)
		}
		log.Error("Failed to update spend alert: %v", err, c)
		return http.Fail(c, 500, "failed to update spend alert", err)
	}

	return c.JSON(200, a)
}

// DeleteAlert removes one of subject's alerts — the WRITE, with no HTTP in it. A row
// belonging to anyone but subject answers errAlertNotFound, exactly as a row that does
// not exist, so deleting by guessed id tells a caller nothing.
//
// It returns only an error: a deleted budget has no value left to report, and inventing
// one would give every caller a shape to misread.
func DeleteAlert(ctx context.Context, org *organization.Organization, subject, id string) error {
	if org == nil {
		return errors.New("alerts: no organization")
	}
	db := datastore.New(org.Namespaced(ctx))

	a, err := ownedAlert(db, subject, id)
	if err != nil {
		return err
	}
	return a.Delete()
}

// DeleteSpendAlert deletes a spend alert by ID.
//
//	DELETE /v1/billing/alerts/:id
func DeleteSpendAlert(c *zip.Ctx) error {
	// #146: resolve the org safely (see ListSpendAlerts). No validated org ⇒ a clean 401,
	// never a nil-deref panic (→ 502).
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return http.Fail(c, 401, "authentication required", nil)
	}

	if err := DeleteAlert(c.Context(), org, billingSubject(c), c.Param("id")); err != nil {
		// A row the caller does not own is refused AS a miss — no existence leak.
		if IsAlertNotFound(err) {
			return http.Fail(c, 404, "spend alert not found", err)
		}
		log.Error("Failed to delete spend alert: %v", err, c)
		return http.Fail(c, 500, "failed to delete spend alert", err)
	}

	return c.JSON(204, nil)
}
