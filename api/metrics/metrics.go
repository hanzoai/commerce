// Package metrics is the platform SaaS-operations god-view: the whole business's
// money, computed IN commerce (the system of record for subscriptions + the usage
// ledger) so admin.hanzo.ai only renders — never re-aggregates client-side and
// never calls Stripe directly. It is a cross-tenant aggregate (MRR/ARR, plan mix,
// usage tops, top customers), so every handler is gated on the STRICT
// middleware.RequirePlatformAdmin predicate (global admin or the trusted internal
// service token) — the same gate api/costs uses. Org-level admins are refused.
//
// Sources of truth (all local to commerce; nothing fabricated):
//   - Subscriptions: models/subscription — the internal recurring-billing ledger
//     (ProviderType "internal"/"bundle"), walked across every org namespace.
//   - Plan categories/prices: the embedded @hanzo/plans catalog (api/billing),
//     joined by plan slug.
//   - Usage / pay-as-you-go spend: models/transaction rows tagged "api-usage".
//
// Where a figure is genuinely not instrumented (e.g. plan-change events for
// upgrade/downgrade counts), it is reported as null with an honest note in Gaps —
// never a fabricated number.
package metrics

// SaaSMetrics is the single snapshot GET /v1/commerce/metrics/saas returns: the
// four operator panels (revenue, subscriptions, usage, customers) folded from ONE
// cross-org walk. Money is USD cents end to end; AsOf is when the walk ran.
type SaaSMetrics struct {
	AsOf     string              `json:"asOf"`
	Currency string              `json:"currency"`
	Window   string              `json:"window"` // the usage/new-churn window, e.g. "30d"
	Revenue  RevenueMetrics      `json:"revenue"`
	Subs     SubscriptionMetrics `json:"subscriptions"`
	Usage    UsageMetrics        `json:"usage"`
	// Customers are the top orgs by MRR + windowed usage, capped at the request limit.
	Customers []CustomerRow `json:"customers"`
	// Orgs is the total number of tenant organizations walked (denominator context).
	Orgs int `json:"orgs"`
	// Gaps are honest "not instrumented yet" notes for figures we cannot source.
	Gaps []string `json:"gaps"`
}

// RevenueMetrics is the recurring-revenue headline: run-rate (MRR/ARR) from ACTIVE
// subscriptions plus the new/churned movement inside the window.
type RevenueMetrics struct {
	MRRCents            int64         `json:"mrrCents"`
	ARRCents            int64         `json:"arrCents"`
	ActiveSubscriptions int           `json:"activeSubscriptions"`
	PayingCustomers     int           `json:"payingCustomers"`
	Trials              int           `json:"trials"`
	NewMRRCents         int64         `json:"newMrrCents"`
	ChurnedMRRCents     int64         `json:"churnedMrrCents"`
	NetNewMRRCents      int64         `json:"netNewMrrCents"`
	ByCategory          []CategoryMRR `json:"byCategory"`
}

// CategoryMRR is one plan-category bucket of run-rate MRR (e.g. world, personal,
// team, social) — the category comes from the embedded @hanzo/plans catalog.
type CategoryMRR struct {
	Category      string `json:"category"`
	MRRCents      int64  `json:"mrrCents"`
	Subscriptions int    `json:"subscriptions"`
}

// SubscriptionMetrics is the subscription operations panel: the per-plan mix, the
// active trial count, the new/canceled counts inside the window, and a recent
// events feed synthesized from real subscription create/cancel timestamps.
type SubscriptionMetrics struct {
	ByPlan       []PlanBreakdown `json:"byPlan"`
	TrialsActive int             `json:"trialsActive"`
	New          int             `json:"new"`
	Canceled     int             `json:"canceled"`
	// Upgrades/Downgrades are nil: plan-change events are NOT recorded (see Gaps).
	Upgrades   *int       `json:"upgrades"`
	Downgrades *int       `json:"downgrades"`
	Recent     []SubEvent `json:"recent"`
}

// PlanBreakdown is one plan's active/trialing counts, seats, and MRR contribution.
type PlanBreakdown struct {
	Plan     string `json:"plan"` // plan slug
	Name     string `json:"name"`
	Category string `json:"category"`
	Active   int    `json:"active"`
	Trialing int    `json:"trialing"`
	Seats    int    `json:"seats"`
	MRRCents int64  `json:"mrrCents"`
}

// SubEvent is one entry in the recent-subscription feed. Type is "created" or
// "canceled" (the two movements we can source from timestamps); MRRDeltaCents is
// the positive (new) or negative (churn) monthly contribution.
type SubEvent struct {
	At            string `json:"at"`
	Org           string `json:"org"`
	Type          string `json:"type"`
	Plan          string `json:"plan"`
	Category      string `json:"category"`
	MRRDeltaCents int64  `json:"mrrDeltaCents"`
}

// UsageMetrics is the metered / pay-as-you-go REVENUE headline: the billed usage
// (what customers actually paid on the wallet ledger — the api-usage withdrawals,
// NOT metered COGS) and the request count in the window. It is the money-domain
// complement to MRR: total revenue = recurring (MRR) + metered (this).
//
// PER-MODEL tokens, per-org AI-spend ranking, latency and error rate are
// deliberately NOT here. The canonical fleet LLM-observability aggregate is the
// cloud god-view GET /v1/admin/o11y (ClickHouse hanzo.cloud_usage + signoz_traces),
// already rendered by the console "Fleet Observability" board. The SaaS module
// consumes THAT for its AI panel rather than re-deriving per-model numbers from
// commerce's noindex ledger — one aggregate, no fork, no risk of two disagreeing
// per-model tables. What o11y cannot source (per-MODEL latency/error) is called out
// honestly in Gaps rather than fabricated here.
type UsageMetrics struct {
	Instrumented     bool  `json:"instrumented"`
	WindowUsageCents int64 `json:"windowUsageCents"`
	Requests         int64 `json:"requests"`
	// UntaggedRequests counts api-usage rows with no model tag — a data-quality
	// signal for the o11y attribution, reported honestly.
	UntaggedRequests int64 `json:"untaggedRequests"`
}

// CustomerRow is one top customer: its tier, status, run-rate MRR, windowed usage,
// seats, and the earliest active-subscription start. The console links Org to the
// existing fleet-customer drill-in rather than duplicating it.
type CustomerRow struct {
	Org        string `json:"org"`
	Plan       string `json:"plan"`
	Category   string `json:"category"`
	Status     string `json:"status"`
	MRRCents   int64  `json:"mrrCents"`
	UsageCents int64  `json:"usageCents"`
	Seats      int    `json:"seats"`
	Since      string `json:"since,omitempty"`
}
