package billing

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/creditgrant"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json/http"
)

type createCreditGrantRequest struct {
	UserId      string   `json:"userId"`
	Name        string   `json:"name"`
	AmountCents int64    `json:"amountCents"`
	Currency    string   `json:"currency"`
	ExpiresIn   string   `json:"expiresIn"` // Go duration string, e.g. "720h"
	Priority    int      `json:"priority"`
	Eligibility []string `json:"eligibility"`
	Tags        string   `json:"tags"`
}

// CreateCreditGrant creates a new credit grant for a user.
//
//	POST /v1/billing/credit-grants
func CreateCreditGrant(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	var req createCreditGrantRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}

	if req.UserId == "" {
		return http.Fail(c, 400, "userId is required", nil)
	}

	if req.AmountCents <= 0 {
		return http.Fail(c, 400, "amountCents must be positive", nil)
	}

	cur := currency.Type(strings.ToLower(req.Currency))
	if cur == "" {
		cur = "usd"
	}

	// Idempotency (money-safe retries): a caller-supplied X-Idempotency-Key makes
	// a retry / double-submit grant AT MOST ONCE — a completed key replays the
	// stored body, an in-flight key 409s. Absent a key there is no guard: distinct
	// grants to the same subject are legitimately additive (an admin may comp the
	// same user twice on purpose), so we never dedupe by amount. Scoped to the
	// subject; the datastore is already org-namespaced. This is the SAME primitive
	// /deposit uses — one way to make a money mint exactly-once, so every program
	// composing this endpoint gets retry-safety for free.
	idemKey := strings.TrimSpace(c.Header("X-Idempotency-Key"))
	var idemRec *idempotencykey.IdempotencyKey
	if idemKey != "" {
		rec, replay, gerr := idempotencykey.Begin(db, "billing-credit-grant:"+req.UserId, idemKey)
		if gerr != nil {
			log.Error("credit-grant idempotency Begin failed (user=%s): %v", req.UserId, gerr, c)
		} else if replay {
			if rec.Status == idempotencykey.StatusCompleted && rec.Response != "" {
				c.SetHeader("Content-Type", "application/json")
				return c.Bytes(200, []byte(rec.Response))
			}
			return http.Fail(c, 409, "credit grant already in progress", nil)
		} else {
			idemRec = rec
		}
	}

	grant := creditgrant.New(db)
	grant.UserId = req.UserId
	grant.Name = req.Name
	grant.AmountCents = req.AmountCents
	grant.RemainingCents = req.AmountCents
	grant.Currency = cur
	grant.Priority = req.Priority
	grant.Eligibility = req.Eligibility
	grant.Tags = req.Tags

	if req.ExpiresIn != "" {
		dur, err := time.ParseDuration(req.ExpiresIn)
		if err != nil {
			// No grant created — release the guard so a later retry is not wedged.
			if idemRec != nil {
				_ = idemRec.Delete()
			}
			return http.Fail(c, 400, "invalid expiresIn duration", err)
		}
		grant.ExpiresAt = time.Now().Add(dur)
	}

	if err := grant.Create(); err != nil {
		// No balance moved — release the guard so a later retry is not wedged.
		if idemRec != nil {
			_ = idemRec.Delete()
		}
		log.Error("Failed to create credit grant: %v", err, c)
		return http.Fail(c, 500, "failed to create credit grant", err)
	}

	resp := map[string]any{
		"id":             grant.Id(),
		"userId":         grant.UserId,
		"name":           grant.Name,
		"amountCents":    grant.AmountCents,
		"remainingCents": grant.RemainingCents,
		"currency":       grant.Currency,
		"priority":       grant.Priority,
		"effectiveAt":    grant.EffectiveAt,
		"tags":           grant.Tags,
		"createdAt":      grant.CreatedAt,
	}
	if !grant.ExpiresAt.IsZero() {
		resp["expiresAt"] = grant.ExpiresAt
	}

	// Seal the guard with the exact success body so a same-key retry replays it
	// verbatim (no second grant).
	if idemRec != nil {
		if body, mErr := json.Marshal(resp); mErr == nil {
			_ = idempotencykey.Complete(idemRec, string(body))
		}
	}

	return c.JSON(201, resp)
}

// ListCreditGrants lists credit grants for a user.
//
//	GET /v1/billing/credit-grants?userId=...
func ListCreditGrants(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	userId := strings.TrimSpace(c.Query("userId"))
	if userId == "" {
		return http.Fail(c, 400, "userId query parameter is required", nil)
	}

	rootKey := db.NewKey("synckey", "", 1, nil)
	grants := make([]*creditgrant.CreditGrant, 0)
	q := creditgrant.Query(db).Ancestor(rootKey).
		Filter("UserId=", userId)

	if _, err := q.GetAll(&grants); err != nil {
		log.Error("Failed to list credit grants: %v", err, c)
		return http.Fail(c, 500, "failed to list credit grants", err)
	}

	items := make([]map[string]any, 0, len(grants))
	for _, g := range grants {
		item := map[string]any{
			"id":             g.Id(),
			"userId":         g.UserId,
			"name":           g.Name,
			"amountCents":    g.AmountCents,
			"remainingCents": g.RemainingCents,
			"currency":       g.Currency,
			"priority":       g.Priority,
			"effectiveAt":    g.EffectiveAt,
			"tags":           g.Tags,
			"voided":         g.Voided,
			"active":         g.IsActive(),
			"createdAt":      g.CreatedAt,
		}
		if !g.ExpiresAt.IsZero() {
			item["expiresAt"] = g.ExpiresAt
		}
		items = append(items, item)
	}

	return c.JSON(200, map[string]any{
		"grants": items,
		"count":  len(items),
	})
}

// GetCreditBalance returns the total available credit balance for a user.
//
//	GET /v1/billing/credit-balance?userId=...
func GetCreditBalance(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	userId := strings.TrimSpace(c.Query("userId"))
	if userId == "" {
		return http.Fail(c, 400, "userId query parameter is required", nil)
	}

	grants, err := getActiveGrants(db, userId)
	if err != nil {
		log.Error("Failed to query credit grants: %v", err, c)
		return http.Fail(c, 500, "failed to query credit balance", err)
	}

	// Sum by currency
	balances := make(map[currency.Type]int64)
	for _, g := range grants {
		balances[g.Currency] += g.RemainingCents
	}

	items := make([]map[string]any, 0, len(balances))
	for cur, amount := range balances {
		items = append(items, map[string]any{
			"currency":  cur,
			"available": amount,
		})
	}

	return c.JSON(200, map[string]any{
		"userId":   userId,
		"balances": items,
	})
}

// GetCreditBalanceBreakdown returns the credit balance grouped by tag.
// Used by Chat to distinguish trial vs paid credits.
//
//	GET /v1/billing/credit-balance/breakdown?userId=...
func GetCreditBalanceBreakdown(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	userId := strings.TrimSpace(c.Query("userId"))
	if userId == "" {
		return http.Fail(c, 400, "userId query parameter is required", nil)
	}

	grants, err := getActiveGrants(db, userId)
	if err != nil {
		log.Error("Failed to query credit grants for breakdown: %v", err, c)
		return http.Fail(c, 500, "failed to query credit balance", err)
	}

	type tagBalance struct {
		Cents     int64      `json:"cents"`
		ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	}

	breakdown := make(map[string]*tagBalance)
	var totalCents int64

	for _, g := range grants {
		tag := g.Tags
		if tag == "" {
			tag = "other"
		}

		tb, ok := breakdown[tag]
		if !ok {
			tb = &tagBalance{}
			breakdown[tag] = tb
		}
		tb.Cents += g.RemainingCents
		totalCents += g.RemainingCents

		// Track the earliest expiry for this tag group
		if !g.ExpiresAt.IsZero() {
			if tb.ExpiresAt == nil || g.ExpiresAt.Before(*tb.ExpiresAt) {
				exp := g.ExpiresAt
				tb.ExpiresAt = &exp
			}
		}
	}

	return c.JSON(200, map[string]any{
		"userId":    userId,
		"breakdown": breakdown,
		"total":     map[string]any{"cents": totalCents},
	})
}

// VoidCreditGrant voids a specific credit grant, making it unusable.
//
//	POST /v1/billing/credit-grants/:id/void
func VoidCreditGrant(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	id := c.Param("id")
	if id == "" {
		return http.Fail(c, 400, "grant id is required", nil)
	}

	grant := creditgrant.New(db)
	if err := grant.GetById(id); err != nil {
		return http.Fail(c, 404, "credit grant not found", err)
	}

	if grant.Voided {
		return http.Fail(c, 400, "grant is already voided", nil)
	}

	grant.Voided = true
	if err := grant.Update(); err != nil {
		log.Error("Failed to void credit grant: %v", err, c)
		return http.Fail(c, 500, "failed to void credit grant", err)
	}

	return c.JSON(200, map[string]any{
		"id":     grant.Id(),
		"voided": true,
	})
}

// getActiveGrants returns active, non-expired, non-voided grants for a user,
// sorted by priority ASC then ExpiresAt ASC.
func getActiveGrants(db *datastore.Datastore, userId string) ([]*creditgrant.CreditGrant, error) {
	rootKey := db.NewKey("synckey", "", 1, nil)
	grants := make([]*creditgrant.CreditGrant, 0)
	q := creditgrant.Query(db).Ancestor(rootKey).
		Filter("UserId=", userId).
		Filter("Voided=", false)

	keys, err := q.GetAll(&grants)
	if err != nil {
		return nil, err
	}

	// Reinitialize each loaded grant so it can be updated later.
	// Raw GetAll doesn't set b.ds / b.Model.db — without Init+SetKey,
	// calling Update() will panic (m.db == nil when rebuilding the key).
	for i, g := range grants {
		g.Init(db)
		if i < len(keys) {
			g.SetKey(keys[i])
		}
	}

	// Filter to active grants only
	active := make([]*creditgrant.CreditGrant, 0, len(grants))
	for _, g := range grants {
		if g.IsActive() {
			active = append(active, g)
		}
	}

	// Sort: priority ASC, then ExpiresAt ASC (zero = last)
	sort.Slice(active, func(i, j int) bool {
		if active[i].Priority != active[j].Priority {
			return active[i].Priority < active[j].Priority
		}
		// Within same priority, burn expiring grants first
		if active[i].ExpiresAt.IsZero() {
			return false
		}
		if active[j].ExpiresAt.IsZero() {
			return true
		}
		return active[i].ExpiresAt.Before(active[j].ExpiresAt)
	})

	return active, nil
}

// BurnCredits applies the credit burn-down algorithm: deducts amount from
// active grants in priority order. Returns the remaining amount (overage)
// and the grants that were modified.
func BurnCredits(db *datastore.Datastore, userId string, amount int64, meterId string) (int64, error) {
	grants, err := getActiveGrants(db, userId)
	if err != nil {
		return amount, err
	}

	remaining := amount

	for _, g := range grants {
		if remaining <= 0 {
			break
		}

		// Check meter eligibility
		if meterId != "" && !g.IsEligibleForMeter(meterId) {
			continue
		}

		deduct := g.RemainingCents
		if deduct > remaining {
			deduct = remaining
		}

		g.RemainingCents -= deduct
		remaining -= deduct

		if err := g.Update(); err != nil {
			return remaining, err
		}
	}

	return remaining, nil
}
