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
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/employee"
	"github.com/hanzoai/commerce/models/spendalert"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json/http"
)

// currentPeriod is the UTC calendar month, e.g. "2026-07". A cap resets at the
// first of each UTC month.
func currentPeriod() string { return time.Now().UTC().Format("2006-01") }

// periodStartUTC is midnight on the first of the current UTC month — the lower
// bound of the spend window a cap is measured over.
func periodStartUTC() time.Time {
	n := time.Now().UTC()
	return time.Date(n.Year(), n.Month(), 1, 0, 0, 0, 0, time.UTC)
}

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

// loadOrgScopes lists every spend-alert (budget/cap) row in the caller's org
// namespace, regardless of UserId — the org-wide policy set the cap verdict and
// the rate-limit config both read.
func loadOrgScopes(db *datastore.Datastore) ([]*spendalert.SpendAlert, error) {
	rootKey := db.NewKey("synckey", "", 1, nil)
	rows := make([]*spendalert.SpendAlert, 0)
	_, err := spendalert.Query(db).Ancestor(rootKey).GetAll(&rows)
	return rows, err
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
// covering row (most-restrictive-wins) and DENIES when any ENFORCE=true row is
// exceeded, reporting the tightest one. Soft (Enforce=false) rows never block —
// they only raise the warn utilization, as does an enforced row still under cap.
//
//	GET /v1/billing/spend-alerts/authorize?user=&project=&service=&amount=
func AuthorizeSpendCap(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))
	test := org.TestMode()

	reqProject := spendalert.NormalizeProject(c.Query("project"))
	reqService := strings.TrimSpace(c.Query("service"))
	amount := parseCents(c.Query("amount"))

	rows, err := loadOrgScopes(db)
	if err != nil {
		log.Error("spend-cap: load scopes failed: %v", err, c)
		http.Fail(c, 500, "failed to load spend caps", err)
		return
	}

	res := authorizeResult{Allow: true}
	var blockCap int64 // tightest violated enforced cap (most restrictive wins).
	for _, s := range rows {
		if s.Threshold <= 0 || !s.Covers(reqProject, reqService) {
			continue
		}
		spent, serr := scopeSpentCents(db, test, s.Project, s.Service)
		if serr != nil {
			log.Error("spend-cap: spend agg failed: %v", serr, c)
			http.Fail(c, 500, "failed to compute period spend", serr)
			return
		}
		// REUSE the existing spend primitive: within iff committed+requested <= cap.
		within := employee.WithinLimit(currency.Cents(spent), currency.Cents(amount), currency.Cents(s.Threshold))
		if s.Enforce && !within {
			// Most restrictive wins: keep the smallest violated cap.
			if res.Reason != "spend_cap" || s.Threshold < blockCap {
				blockCap = s.Threshold
				res.Reason = "spend_cap"
				res.Allow = false
				res.CapCents = s.Threshold
				res.SpentCents = spent
			}
			continue
		}
		if pct := int(spent * 100 / s.Threshold); pct >= s.EffectiveSoftPct() && pct > res.WarnPct {
			res.WarnPct = pct
		}
	}
	if !res.Allow {
		res.WarnPct = 0 // a deny carries no warn.
	}
	c.JSON(200, res)
}
