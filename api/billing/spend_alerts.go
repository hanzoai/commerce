package billing

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
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

// resolveSpendAlertUserId returns the userId to scope spend-alert queries, or ""
// to list the org-wide policy set (all rows). An explicit ?user= wins; otherwise
// the caller's own subject is used. Empty means "no user filter" (scope-cap view).
//
// Behind the /billing bridge the ?user= param is PINNED to the caller's own
// validated subject (clients/console scopedBillingSearch), so this is the caller's
// own identity — the trust boundary that makes the id-ownership check below sound.
func resolveSpendAlertUserId(c *gin.Context) string {
	user := strings.TrimSpace(c.Query("user"))
	if user != "" {
		return user
	}
	return iammiddleware.GetIAMClaims(c).Subject
}

// ownsAlert reports whether the caller may mutate this row. Tenant isolation is
// already structural (the datastore is org-namespaced), so this closes the
// WITHIN-org IDOR: a caller may PATCH/DELETE only a row whose UserId is their OWN
// resolved subject (the /billing bridge pins ?user= to that subject). A row owned
// by another subject — reachable only by GUESSING its id — is refused. A caller
// with no resolved subject can mutate nothing. Ownership never comes from the row
// id alone.
func ownsAlert(c *gin.Context, a *spendalert.SpendAlert) bool {
	caller := resolveSpendAlertUserId(c)
	return caller != "" && a.UserId == caller
}

// spendAlertView is the wire shape of one row plus the DERIVED period spend for
// its scope (periodSpentCents/over/warn) — never stored, always computed from the
// deduped usage ledger for the current UTC month.
func spendAlertView(db *datastore.Datastore, test bool, a *spendalert.SpendAlert) gin.H {
	soft := a.EffectiveSoftPct()
	view := gin.H{
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
		"createdAt":    a.CreatedAt,
		"updatedAt":    a.UpdatedAt,
	}
	spent, err := scopeSpentCents(db, test, a.Project, a.Service)
	if err != nil {
		// Display must not fail on an aggregation hiccup — report the policy row
		// without derived spend rather than 500 the whole list.
		return view
	}
	view["periodSpentCents"] = spent
	if a.Threshold > 0 {
		view["over"] = spent >= a.Threshold
		view["warn"] = spent >= a.Threshold*int64(soft)/100
	} else {
		view["over"] = false
		view["warn"] = false
	}
	return view
}

// ListSpendAlerts returns spend alerts (budgets/caps) plus derived period spend.
// With ?user= it returns that user's rows (back-compat); without a resolvable
// user it returns the ORG-WIDE policy set (every scope cap), so the console
// Budgets page can manage org/project/service caps.
//
//	GET /v1/billing/spend-alerts[?user=:userId]
func ListSpendAlerts(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))
	test := org.TestMode()

	rootKey := db.NewKey("synckey", "", 1, nil)
	q := spendalert.Query(db).Ancestor(rootKey)
	if userId := resolveSpendAlertUserId(c); userId != "" {
		q = q.Filter("UserId=", userId)
	}

	alerts := make([]*spendalert.SpendAlert, 0)
	if _, err := q.GetAll(&alerts); err != nil {
		log.Error("Failed to list spend alerts: %v", err, c)
		http.Fail(c, 500, "failed to list spend alerts", err)
		return
	}

	items := make([]gin.H, 0, len(alerts))
	for _, a := range alerts {
		items = append(items, spendAlertView(db, test, a))
	}

	c.JSON(200, items)
}

// CreateSpendAlert creates a spend alert / cap. UserId is optional (an org-wide
// or project/service scope cap has none). At least one limit must be meaningful:
// a Threshold>0 (spend cap) or a RateLimitRpm>0 (rate limit).
//
//	POST /v1/billing/spend-alerts
func CreateSpendAlert(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	var req createSpendAlertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		http.Fail(c, 400, "invalid request body", err)
		return
	}

	// Default UserId to the caller's subject only when the client passed one
	// implicitly; a truly org-wide scope cap may carry no user.
	if req.UserId == "" {
		req.UserId = iammiddleware.GetIAMClaims(c).Subject
	}

	if req.Threshold < 0 {
		http.Fail(c, 400, "threshold must be >= 0", nil)
		return
	}
	if req.RateLimitRpm < 0 {
		http.Fail(c, 400, "rateLimitRpm must be >= 0", nil)
		return
	}
	if req.Threshold <= 0 && req.RateLimitRpm <= 0 {
		http.Fail(c, 400, "a spend cap (threshold) or a rate limit (rateLimitRpm) is required", nil)
		return
	}
	if req.SoftPct < 0 || req.SoftPct > 100 {
		http.Fail(c, 400, "softPct must be between 0 and 100", nil)
		return
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
		http.Fail(c, 500, "failed to create spend alert", err)
		return
	}

	c.JSON(201, spendAlertView(db, org.TestMode(), a))
}

// UpdateSpendAlert patches an existing spend alert / cap. Only the fields present
// in the body change; the rest are preserved.
//
//	PATCH /v1/billing/spend-alerts/:id
func UpdateSpendAlert(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	id := c.Param("id")
	a := spendalert.New(db)
	if err := a.GetById(id); err != nil {
		http.Fail(c, 404, "spend alert not found", err)
		return
	}
	// Per-row ownership: refuse (as 404, not leaking existence) a row the caller
	// does not own — closes the guess-the-id IDOR on this mutating verb.
	if !ownsAlert(c, a) {
		http.Fail(c, 404, "spend alert not found", nil)
		return
	}

	var req updateSpendAlertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		http.Fail(c, 400, "invalid request body", err)
		return
	}

	if req.Title != nil {
		a.Title = *req.Title
	}
	if req.Threshold != nil {
		if *req.Threshold < 0 {
			http.Fail(c, 400, "threshold must be >= 0", nil)
			return
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
			http.Fail(c, 400, "softPct must be between 0 and 100", nil)
			return
		}
		a.SoftPct = *req.SoftPct
	}
	if req.RateLimitRpm != nil {
		if *req.RateLimitRpm < 0 {
			http.Fail(c, 400, "rateLimitRpm must be >= 0", nil)
			return
		}
		a.RateLimitRpm = *req.RateLimitRpm
	}

	if err := a.Update(); err != nil {
		log.Error("Failed to update spend alert: %v", err, c)
		http.Fail(c, 500, "failed to update spend alert", err)
		return
	}

	c.JSON(200, spendAlertView(db, org.TestMode(), a))
}

// DeleteSpendAlert deletes a spend alert by ID.
//
//	DELETE /v1/billing/spend-alerts/:id
func DeleteSpendAlert(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	id := c.Param("id")
	a := spendalert.New(db)
	if err := a.GetById(id); err != nil {
		http.Fail(c, 404, "spend alert not found", err)
		return
	}
	// Per-row ownership: refuse a row the caller does not own (guess-the-id IDOR).
	if !ownsAlert(c, a) {
		http.Fail(c, 404, "spend alert not found", nil)
		return
	}

	if err := a.Delete(); err != nil {
		log.Error("Failed to delete spend alert: %v", err, c)
		http.Fail(c, 500, "failed to delete spend alert", err)
		return
	}

	c.JSON(204, nil)
}
