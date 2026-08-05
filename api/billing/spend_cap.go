package billing

// Per-scope spend-cap ENFORCEMENT (issue #70) — the money-path verdict the cloud
// metering gate consumes, computed over the SAME spend-alert rows the console
// Budgets page manages (no parallel model) and the SAME append-only usage ledger
// the balance gate reads (no parallel counter → no drift).
//
// Composition when several rows cover one request: MOST RESTRICTIVE WINS. The
// request is DENIED if ANY covering ENFORCE=true row is over its cap; the tightest
// (smallest-cap) violated row is reported. When none block, the highest soft-
// threshold utilization across covering rows drives the X-Spend-Warn header.
//
// Concurrency note: the commerce ledger is append-only with NO atomic
// compare-and-swap (RunInTransaction is a stub), so the cap is a SOFT boundary —
// a burst of concurrent requests can each read the same pre-debit spend and
// slightly overshoot. This is inherent to a lock-free money ledger and accepted;
// the cap still bounds sustained spend to the ceiling.

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/employee"
	"github.com/hanzoai/commerce/models/spendalert"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/json/http"
)

// PeriodSpendFunc reports a scope's cumulative spend (cents) in the CURRENT UTC
// period for an org — the value scopeExhausted/warn compare a cap against. The HOST
// injects it (SetPeriodSpendReader) so the cap reads the SAME ledger the host
// records usage in. In the co-resident cloud binary usage is recorded on the FINANCE
// ledger (fin.RecordUsage), NOT commerce's own transaction store — which the unified
// binary leaves empty — so without this the cap would sum 0 and never enforce. nil
// (standalone commerce) → the append-only transaction-ledger query below, unchanged.
type PeriodSpendFunc func(ctx context.Context, org string, test bool, project, service string) (int64, error)

var periodSpendReaderVal atomic.Pointer[PeriodSpendFunc]

// SetPeriodSpendReader installs the host's period-spend source. Pass nil to clear
// (standalone commerce). Set once at boot, read per request; safe for concurrent use.
func SetPeriodSpendReader(f PeriodSpendFunc) {
	if f == nil {
		periodSpendReaderVal.Store(nil)
		return
	}
	periodSpendReaderVal.Store(&f)
}

func periodSpendReader() PeriodSpendFunc {
	if p := periodSpendReaderVal.Load(); p != nil {
		return *p
	}
	return nil
}

// currentPeriod is the UTC calendar month, e.g. "2026-07". A cap resets at the
// first of each UTC month.
func currentPeriod() string { return time.Now().UTC().Format("2006-01") }

// periodStartUTC is midnight on the first of the current UTC month — the lower
// bound of the spend window a cap is measured over.
func periodStartUTC() time.Time {
	n := time.Now().UTC()
	return time.Date(n.Year(), n.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// periodResetsAtUTC is midnight on the first of the NEXT UTC month — the instant
// the current spend window rolls over and every cap's period spend resets to zero.
func periodResetsAtUTC() time.Time { return periodStartUTC().AddDate(0, 1, 0) }

// currentPeriodResetsAt renders periodResetsAtUTC as an RFC3339 UTC string — the
// canonical SpendAlert.resetsAt the self-service UI shows ("resets on <date>").
func currentPeriodResetsAt() string { return periodResetsAtUTC().Format(time.RFC3339) }

// parseCents parses a non-negative cents amount; any invalid/negative value is 0
// (a pure "am I already over the cap" check).
func parseCents(s string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// scopeSpentCents sums this period's api-usage debits for the (project,service)
// scope in the caller's org namespace. Empty project/service is a WILDCARD on
// that axis (no filter), so the org-wide scope ("","") sums ALL usage, a project
// scope (P,"") sums every service in P, etc. — the covering relation.
//
// Reuses the exact ledger the balance gate reads (Withdraw, SourceKind
// "iam-user", Tags "api-usage") so the cap counts precisely the debited spend.
// The window is a single inequality (CreatedAt >= period start), datastore-legal
// alongside the equality filters. Idempotent recording (usage.go dedups on
// requestId) guarantees a retried debit is counted at most once.
func scopeSpentCents(db *datastore.Datastore, test bool, project, service string) (int64, error) {
	// Host-injected source (co-resident cloud binary): the org's period spend from
	// the finance ledger, where the unified binary actually records usage. The org
	// is the datastore namespace; the window (current UTC month) is the reader's own
	// concern, matching periodStartUTC. Standalone commerce (nil reader) falls
	// through to the transaction query below.
	if r := periodSpendReader(); r != nil {
		return r(db.Context, nscontext.GetNamespace(db.Context), test, project, service)
	}
	rootKey := db.NewKey("synckey", "", 1, nil)
	q := transaction.Query(db).Ancestor(rootKey).
		Filter("Test=", test).
		Filter("SourceKind=", "iam-user").
		Filter("Tags=", "api-usage").
		Filter("CreatedAt>=", periodStartUTC())
	if project != "" {
		q = q.Filter("Project=", project)
	}
	if service != "" {
		q = q.Filter("Service=", service)
	}

	transs := make([]*transaction.Transaction, 0)
	if _, err := q.GetAll(&transs); err != nil {
		return 0, err
	}
	var sum int64
	for _, t := range transs {
		sum += int64(t.Amount)
	}
	return sum, nil
}

// loadOrgScopes lists the org's spend-alert (budget/cap) rows — the org-wide
// policy set the cap verdict and the rate-limit config both read. BOUNDED to
// maxScopeRowsPerOrg so an attacker cannot inflate the row set to slow the scan
// (CreateSpendAlert enforces the same cap on writes).
func loadOrgScopes(db *datastore.Datastore) ([]*spendalert.SpendAlert, error) {
	rootKey := db.NewKey("synckey", "", 1, nil)
	rows := make([]*spendalert.SpendAlert, 0)
	_, err := spendalert.Query(db).Ancestor(rootKey).Limit(maxScopeRowsPerOrg).GetAll(&rows)
	return rows, err
}

// scopeExhausted reports whether a scope has reached/exceeded its cap such that
// ENFORCE refuses any further billable (>=1c) request — the ONE boundary both the
// verdict and the `over` display derive from, so they can never drift. It is
// exactly !WithinLimit(spent, 1, cap): committing one more cent breaks the cap.
func scopeExhausted(spent, cap int64) bool {
	return cap > 0 && !employee.WithinLimit(currency.Cents(spent), 1, currency.Cents(cap))
}

// hardAxesValidated reports whether a row may HARD-enforce (402) given whether the
// caller's PROJECT is bound to a validated claim. The ORG axis is always validated
// (owner claim) and the SERVICE axis is server-derived (route/provider, not a
// client field) — both always trustworthy. Only a row that constrains PROJECT
// (row.Project != "") needs projectValidated; without it that row DEGRADES to a
// soft warn (records + warns, never 402) so a forgeable X-Project-Id can never be
// used to hard-stop, nor evaded to bypass. (IAM does not yet mint a project claim;
// when it does, cloud sets pv=1 and project caps auto-harden — see
// principal.ValidatedProject.)
func hardAxesValidated(s *spendalert.SpendAlert, projectValidated bool) bool {
	return s.Project == "" || projectValidated
}

// authorizeResult is the gate verdict the metering client consumes. reason is
// "" (allow) or "spend_cap" (deny). warnPct is the actual utilization percent of
// the most-utilized covering cap when at/over its soft threshold (else 0), so the
// gate emits X-Spend-Warn without a second round trip.
type authorizeResult struct {
	Allow      bool   `json:"allow"`
	Reason     string `json:"reason"`
	CapCents   int64  `json:"capCents"`
	SpentCents int64  `json:"spentCents"`
	WarnPct    int    `json:"warnPct"`
}

// AuthorizeSpendCap is the per-request cap verdict for a (project,service) scope
// and a proposed amount, over the org's spend-alert rows. It evaluates EVERY
// covering row (most-restrictive-wins) and DENIES when any HARD-enforceable
// ENFORCE=true row is exceeded, reporting the tightest one. Soft rows — and a
// project-scoped enforce row whose project axis is NOT validated (pv=0) — never
// block; they only raise the warn utilization.
//
// FAIL OPEN on UNKNOWN spend: if an enforce row's spend sum cannot be read (a transient
// finance-ledger error), the verdict does NOT block — a backend blip must not 402 an
// under-cap customer, nor storm every capped org at once. A row DENIES only when spend is
// KNOWN and over the cap, so this never fails open on a real overage. Per-scope sums are
// memoized so covering rows sharing a scope cost one query. The row scan is bounded
// (loadOrgScopes).
//
//	GET /v1/billing/alerts/authorize?user=&project=&service=&amount=&pv=
func AuthorizeSpendCap(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	test := org.TestMode()

	reqProject := spendalert.NormalizeProject(c.Query("project"))
	reqService := strings.TrimSpace(c.Query("service"))
	amount := parseCents(c.Query("amount"))
	projectValidated := c.Query("pv") == "1"

	rows, err := loadOrgScopes(db)
	if err != nil {
		log.Error("spend-cap: load scopes failed: %v", err, c)
		return http.Fail(c, 500, "failed to load spend caps", err)
	}

	spentBy := map[string]int64{} // memoize per (project,service) scope.
	res := authorizeResult{Allow: true}
	var blockCap int64 // tightest violated enforced cap (most restrictive wins).
	for _, s := range rows {
		if s.Threshold <= 0 || !s.Covers(reqProject, reqService) {
			continue
		}
		hard := s.Enforce && hardAxesValidated(s, projectValidated)

		key := s.Project + "\x00" + s.Service
		spent, ok := spentBy[key]
		if !ok {
			v, serr := scopeSpentCents(db, test, s.Project, s.Service)
			if serr != nil {
				// FAIL OPEN on UNKNOWN spend. A transient finance-ledger read error must NOT
				// 402 an under-cap customer — and a co-resident SQLite blip under load would
				// otherwise storm EVERY capped org at once (self-amplifying: load → store
				// contention → read errors → 402s → retries → more load). We cannot PROVE a
				// scope is over its cap without its spend, so we do NOT block: the balance
				// gate still bounds spend and the next request re-reads. A genuine over-cap
				// still denies below, where spend IS known — so this fails open ONLY on the
				// unknown, never on a real overage. Logged for observability.
				log.Error("spend-cap: spend read failed for scope (cap=%d) — failing OPEN, not blocking: %v", s.Threshold, serr, c)
				continue
			}
			spent = v
			spentBy[key] = v
		}

		// REUSE the spend primitive: within iff committed+requested <= cap.
		within := employee.WithinLimit(currency.Cents(spent), currency.Cents(amount), currency.Cents(s.Threshold))
		if hard && !within {
			if res.Reason != "spend_cap" || s.Threshold < blockCap {
				blockCap = s.Threshold
				res.Reason = "spend_cap"
				res.Allow = false
				res.CapCents = s.Threshold
				res.SpentCents = spent
			}
			continue
		}
		// Warn utilization — for soft rows, degraded project rows, AND enforce rows
		// still under cap (approaching the ceiling).
		if pct := int(spent * 100 / s.Threshold); pct >= s.EffectiveSoftPct() && pct > res.WarnPct {
			res.WarnPct = pct
		}
	}
	// ENFORCEMENT GATE (SPEND_CAP_ENFORCE, default OFF = fail-open). Because a push to
	// main AUTO-DEPLOYS to prod, the finance-cap read + the S2S auth fix can go live
	// while the cap only OBSERVES — it must NOT start blocking real customers until an
	// operator has PROVEN correct enforcement on a canary and deliberately flips the
	// flag ON. When OFF, a spend_cap deny is downgraded to an allow (logged as a shadow
	// so the would-block is visible in prod BEFORE the flip). When ON, it denies.
	if !res.Allow && res.Reason == "spend_cap" && !spendCapEnforce() {
		log.Info("spend-cap SHADOW: scope over cap (cap=%d spent=%d) but SPEND_CAP_ENFORCE=off — allowing", res.CapCents, res.SpentCents, c)
		res.Allow = true
		res.Reason = ""
		res.CapCents = 0
		res.SpentCents = 0
	}

	if !res.Allow {
		res.WarnPct = 0 // a deny carries no warn.
	}
	return c.JSON(200, res)
}

// spendCapEnforce reports whether a spend_cap verdict actually DENIES. Default OFF
// (fail-open): the cap can be live+dormant on an auto-deployed binary while it only
// observes, until an operator flips SPEND_CAP_ENFORCE=true after the canary proof.
func spendCapEnforce() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("SPEND_CAP_ENFORCE")), "true")
}
