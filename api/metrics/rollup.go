package metrics

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hanzoai/commerce/api/billing"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/transaction"
)

// options tunes one rollup: the usage/new-churn window, how many rows top-N lists
// keep, and whether to read the test ledger instead of live.
type options struct {
	window string // "7d" | "30d" | "90d" | "mtd" | "all"
	limit  int    // cap for the customers list
	test   bool   // read test ledger (default false → live business figures)
}

const (
	defaultWindow = "30d"
	defaultLimit  = 20
	eventFeedCap  = 30
	rollupTTL     = 45 * time.Second
	walkTimeout   = 60 * time.Second // detached walk bound; a fill outlives one client
)

type cachedRollup struct {
	at   time.Time
	data *SaaSMetrics
}

var (
	rollupMu    sync.Mutex
	rollupCache = map[string]cachedRollup{}
)

// rollup returns the SaaS snapshot for opts, served from a short-lived process
// cache so the operator panels (one endpoint) never re-walk every tenant on a
// refresh. The walk itself is detached + bounded so it completes even if the
// requester disconnects. This is the ONE cross-org aggregation of commerce's
// recurring money — the console/world admin tabs render it, they do not re-derive it.
func rollup(opts options) *SaaSMetrics {
	key := fmt.Sprintf("%s|%d|%v", opts.window, opts.limit, opts.test)

	rollupMu.Lock()
	if c, ok := rollupCache[key]; ok && time.Since(c.at) < rollupTTL {
		data := c.data
		rollupMu.Unlock()
		return data
	}
	rollupMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), walkTimeout)
	defer cancel()
	data := buildRollup(ctx, opts)

	rollupMu.Lock()
	rollupCache[key] = cachedRollup{at: time.Now(), data: data}
	rollupMu.Unlock()
	return data
}

// acc folds the per-tenant walk into the final snapshot. Maps key on the natural
// grouping (category/slug/org) and are projected to sorted slices at the end so the
// wire is deterministic (highest-value first).
type acc struct {
	now   time.Time
	start time.Time

	// revenue (from subscriptions — the recurring ledger)
	mrrCents   int64
	activeSubs int
	trials     int
	newMRR     int64
	churnedMRR int64
	catMRR     map[string]int64
	catSubs    map[string]int
	payingOrgs map[string]bool

	// subscriptions
	planActive   map[string]int
	planTrialing map[string]int
	planSeats    map[string]int
	planMRR      map[string]int64
	planName     map[string]string
	planCat      map[string]string
	newSubs      int
	canceledSubs int
	events       []SubEvent

	// usage (billed metered revenue — the api-usage wallet ledger)
	usageCents   int64
	requests     int64
	untagged     int64
	orgUsage     map[string]int64
	instrumented bool

	// customers (per-org money view)
	orgMRR    map[string]int64
	orgPlan   map[string]string // highest-MRR active plan slug per org
	orgPlanV  map[string]int64  // the MRR that won orgPlan (for tie-break)
	orgCat    map[string]string
	orgStatus map[string]string
	orgSeats  map[string]int
	orgSince  map[string]time.Time
	orgSeen   map[string]bool
}

func newAcc(now, start time.Time) *acc {
	return &acc{
		now: now, start: start,
		catMRR: map[string]int64{}, catSubs: map[string]int{}, payingOrgs: map[string]bool{},
		planActive: map[string]int{}, planTrialing: map[string]int{}, planSeats: map[string]int{},
		planMRR: map[string]int64{}, planName: map[string]string{}, planCat: map[string]string{},
		orgUsage: map[string]int64{},
		orgMRR:   map[string]int64{}, orgPlan: map[string]string{}, orgPlanV: map[string]int64{},
		orgCat: map[string]string{}, orgStatus: map[string]string{}, orgSeats: map[string]int{},
		orgSince: map[string]time.Time{}, orgSeen: map[string]bool{},
	}
}

// buildRollup walks every organization once, folding its subscriptions (the
// recurring-revenue ledger) and its api-usage transactions (the metered-revenue
// plane) into the SaaS snapshot. It mirrors the proven all-orgs pattern
// (api/billing.RunBillingCycleAllOrgs): list orgs from the root namespace, then
// read each org's own namespace. Empty/unreadable tenants contribute nothing (an
// honest zero, never a fabricated figure).
func buildRollup(ctx context.Context, opts options) *SaaSMetrics {
	now := time.Now().UTC()
	a := newAcc(now, windowStart(now, opts.window))

	rootDb := datastore.New(ctx)
	orgs := make([]*organization.Organization, 0)
	_, _ = organization.Query(rootDb).GetAll(&orgs)

	for _, org := range orgs {
		a.orgSeen[org.Name] = true
		db := datastore.New(org.Namespaced(ctx))
		foldSubscriptions(a, org.Name, db, opts.test)
		foldUsage(a, org.Name, db, opts.test)
	}

	return a.snapshot(opts)
}

// foldSubscriptions reads every subscription in an org's namespace (ancestor-less,
// the query shape allotment.go proves finds them all) and hands them to the pure
// fold.
func foldSubscriptions(a *acc, orgName string, db *datastore.Datastore, test bool) {
	subs := make([]*subscription.Subscription, 0)
	if _, err := subscription.Query(db).GetAll(&subs); err != nil {
		return
	}
	foldSubs(a, orgName, subs, test)
}

// foldSubs is the pure fold (no I/O) over one org's subscriptions: MRR, plan mix,
// trials, and new/churn movement. Bundle-child subscriptions (ProviderType
// "bundle", price 0 — the entitlement shadow of a paid parent) are skipped so
// counts and MRR are not double-inflated.
func foldSubs(a *acc, orgName string, subs []*subscription.Subscription, test bool) {
	for _, s := range subs {
		if s.Test != test || s.ProviderType == "bundle" {
			continue
		}
		slug := s.PlanId
		if slug == "" {
			slug = s.Plan.Slug
		}
		cat, name := planMeta(slug, s.Plan.Name)
		qty := s.Quantity
		if qty < 1 {
			qty = 1
		}
		mrr := monthlyNormalizedCents(int64(s.Plan.Price), string(s.Plan.Interval)) * int64(qty)

		switch s.Status {
		case subscription.Active:
			a.activeSubs++
			a.mrrCents += mrr
			a.catMRR[cat] += mrr
			a.catSubs[cat]++
			a.planActive[slug]++
			a.planSeats[slug] += qty
			a.planMRR[slug] += mrr
			a.planName[slug], a.planCat[slug] = name, cat
			if mrr > 0 {
				a.payingOrgs[orgName] = true
			}
			a.orgMRR[orgName] += mrr
			a.orgSeats[orgName] += qty
			a.orgCat[orgName] = cat
			a.orgStatus[orgName] = string(subscription.Active)
			if mrr >= a.orgPlanV[orgName] {
				a.orgPlanV[orgName], a.orgPlan[orgName] = mrr, slug
			}
			if cur, ok := a.orgSince[orgName]; !ok || s.CreatedAt.Before(cur) {
				a.orgSince[orgName] = s.CreatedAt
			}
		case subscription.Trialing:
			a.trials++
			a.planTrialing[slug]++
			a.planName[slug], a.planCat[slug] = name, cat
			if _, ok := a.orgStatus[orgName]; !ok {
				a.orgStatus[orgName] = string(subscription.Trialing)
				a.orgPlan[orgName], a.orgCat[orgName] = slug, cat
			}
		}

		// Window movement, independent of current status.
		if inWindow(s.CreatedAt, a.start, a.now) && (s.Status == subscription.Active || s.Status == subscription.Trialing) {
			a.newSubs++
			a.newMRR += mrr
			a.events = append(a.events, SubEvent{
				At: s.CreatedAt.UTC().Format(time.RFC3339), Org: orgName,
				Type: "created", Plan: slug, Category: cat, MRRDeltaCents: mrr,
			})
		}
		if (s.Canceled || s.Status == subscription.Canceled) && inWindow(s.CanceledAt, a.start, a.now) {
			a.canceledSubs++
			a.churnedMRR += mrr
			a.events = append(a.events, SubEvent{
				At: s.CanceledAt.UTC().Format(time.RFC3339), Org: orgName,
				Type: "canceled", Plan: slug, Category: cat, MRRDeltaCents: -mrr,
			})
		}
	}
}

// foldUsage reads the org's api-usage ledger (the metered-revenue plane) and hands
// it to the pure fold.
func foldUsage(a *acc, orgName string, db *datastore.Datastore, test bool) {
	transs := make([]*transaction.Transaction, 0)
	if _, err := transaction.Query(db).Filter("Tags=", "api-usage").GetAll(&transs); err != nil {
		return
	}
	foldTxns(a, orgName, transs, test)
}

// foldTxns is the pure fold (no I/O) over one org's api-usage ledger: windowed
// billed spend (what the customer paid), request count, and the per-org total used
// by the customers table. Per-model / latency / error attribution is out of scope
// here — that is the o11y god-view's job (see UsageMetrics doc).
func foldTxns(a *acc, orgName string, transs []*transaction.Transaction, test bool) {
	for _, t := range transs {
		if t.Test != test || !inWindow(t.CreatedAt, a.start, a.now) {
			continue
		}
		a.instrumented = true
		cents := int64(t.Amount)
		a.usageCents += cents
		a.requests++
		a.orgUsage[orgName] += cents
		if asString(t.Metadata["model"]) == "" {
			a.untagged++
		}
	}
}

// snapshot projects the accumulators into the wire snapshot, sorting every list by
// value (desc) and capping to the configured limits.
func (a *acc) snapshot(opts options) *SaaSMetrics {
	limit := opts.limit
	if limit <= 0 {
		limit = defaultLimit
	}

	byCategory := make([]CategoryMRR, 0, len(a.catMRR))
	for cat, mrr := range a.catMRR {
		byCategory = append(byCategory, CategoryMRR{Category: cat, MRRCents: mrr, Subscriptions: a.catSubs[cat]})
	}
	sort.Slice(byCategory, func(i, j int) bool { return byCategory[i].MRRCents > byCategory[j].MRRCents })

	byPlan := make([]PlanBreakdown, 0, len(a.planName))
	for slug, name := range a.planName {
		byPlan = append(byPlan, PlanBreakdown{
			Plan: slug, Name: name, Category: a.planCat[slug],
			Active: a.planActive[slug], Trialing: a.planTrialing[slug],
			Seats: a.planSeats[slug], MRRCents: a.planMRR[slug],
		})
	}
	sort.Slice(byPlan, func(i, j int) bool {
		if byPlan[i].MRRCents != byPlan[j].MRRCents {
			return byPlan[i].MRRCents > byPlan[j].MRRCents
		}
		return byPlan[i].Plan < byPlan[j].Plan
	})

	sort.Slice(a.events, func(i, j int) bool { return a.events[i].At > a.events[j].At })
	events := a.events
	if len(events) > eventFeedCap {
		events = events[:eventFeedCap]
	}

	gaps := []string{
		"subscriptions.upgrades / downgrades: plan-change events are not recorded (models/billingevent is not written on plan change), so upgrade/downgrade counts are unavailable — new/canceled are sourced from subscription create/cancel timestamps.",
		"usage: per-model tokens, per-org AI-spend ranking, latency and error rate are served by the cloud fleet LLM-obs god-view GET /v1/admin/o11y (the Datastore cloud_usage + signoz_traces), not re-derived here. Per-MODEL latency is captured NOWHERE in the stack; per-MODEL error rate is in cloud_usage.status but not yet exposed by /v1/admin/o11y topModels.",
	}
	if a.untagged > 0 {
		gaps = append(gaps, fmt.Sprintf("usage.untaggedRequests: %d api-usage request(s) carried no model tag (excluded from any model attribution; still counted in totals).", a.untagged))
	}

	return &SaaSMetrics{
		AsOf:     a.now.Format(time.RFC3339),
		Currency: "usd",
		Window:   normWindow(opts.window),
		Revenue: RevenueMetrics{
			MRRCents:            a.mrrCents,
			ARRCents:            a.mrrCents * 12,
			ActiveSubscriptions: a.activeSubs,
			PayingCustomers:     len(a.payingOrgs),
			Trials:              a.trials,
			NewMRRCents:         a.newMRR,
			ChurnedMRRCents:     a.churnedMRR,
			NetNewMRRCents:      a.newMRR - a.churnedMRR,
			ByCategory:          byCategory,
		},
		Subs: SubscriptionMetrics{
			ByPlan:       byPlan,
			TrialsActive: a.trials,
			New:          a.newSubs,
			Canceled:     a.canceledSubs,
			Upgrades:     nil,
			Downgrades:   nil,
			Recent:       events,
		},
		Usage: UsageMetrics{
			Instrumented:     a.instrumented,
			WindowUsageCents: a.usageCents,
			Requests:         a.requests,
			UntaggedRequests: a.untagged,
		},
		Customers: a.customers(limit),
		Orgs:      len(a.orgSeen),
		Gaps:      gaps,
	}
}

// customers builds the top-N customers by combined MRR + windowed billed usage. An
// org appears if it has recurring revenue OR metered usage; the row is honest about
// a pay-as-you-go customer (no plan) vs. a subscriber.
func (a *acc) customers(limit int) []CustomerRow {
	seen := map[string]bool{}
	rows := make([]CustomerRow, 0)
	add := func(org string) {
		if seen[org] {
			return
		}
		seen[org] = true
		status := a.orgStatus[org]
		if status == "" {
			status = "pay-as-you-go"
		}
		row := CustomerRow{
			Org: org, Plan: a.orgPlan[org], Category: a.orgCat[org], Status: status,
			MRRCents: a.orgMRR[org], UsageCents: a.orgUsage[org], Seats: a.orgSeats[org],
		}
		if since, ok := a.orgSince[org]; ok && !since.IsZero() {
			row.Since = since.UTC().Format(time.RFC3339)
		}
		rows = append(rows, row)
	}
	for org := range a.orgMRR {
		add(org)
	}
	for org := range a.orgUsage {
		add(org)
	}
	sort.Slice(rows, func(i, j int) bool {
		vi, vj := rows[i].MRRCents+rows[i].UsageCents, rows[j].MRRCents+rows[j].UsageCents
		if vi != vj {
			return vi > vj
		}
		return rows[i].Org < rows[j].Org
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}

// planMeta resolves a plan slug to its (category, display name) from the embedded
// @hanzo/plans catalog — the SAME catalog Stripe is seeded from — falling back to
// the subscription's embedded plan name and an honest "uncategorized" bucket.
func planMeta(slug, embeddedName string) (category, name string) {
	name = embeddedName
	category = "uncategorized"
	if p := billing.LookupStaticPlan(slug); p != nil {
		if p.Category != "" {
			category = p.Category
		}
		if p.Name != "" {
			name = p.Name
		}
	}
	if name == "" {
		name = slug
	}
	return category, name
}

// monthlyNormalizedCents normalizes a plan price to a monthly figure by its billing
// interval so annual and monthly plans are comparable in one MRR sum. Mirrors the
// cloud admin's normalizer exactly so MRR agrees across surfaces.
func monthlyNormalizedCents(priceCents int64, interval string) int64 {
	switch strings.ToLower(strings.TrimSpace(interval)) {
	case "year", "yearly", "annual", "annually":
		return priceCents / 12
	case "week", "weekly":
		return priceCents * 52 / 12
	case "day", "daily":
		return priceCents * 365 / 12
	default: // month/monthly and anything unrecognized → treat as monthly
		return priceCents
	}
}

func windowStart(now time.Time, window string) time.Time {
	switch strings.ToLower(strings.TrimSpace(window)) {
	case "7d":
		return now.AddDate(0, 0, -7)
	case "90d":
		return now.AddDate(0, 0, -90)
	case "mtd":
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	case "all":
		return time.Time{}
	default: // "30d"
		return now.AddDate(0, 0, -30)
	}
}

func normWindow(window string) string {
	switch w := strings.ToLower(strings.TrimSpace(window)); w {
	case "7d", "90d", "mtd", "all":
		return w
	default:
		return defaultWindow
	}
}

func inWindow(t, start, now time.Time) bool {
	if t.IsZero() {
		return false
	}
	if !start.IsZero() && t.Before(start) {
		return false
	}
	return !t.After(now.Add(time.Minute)) // small skew tolerance
}

func asString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprintf("%v", v))
}
