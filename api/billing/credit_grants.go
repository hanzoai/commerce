package billing

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/creditgrant"
	"github.com/hanzoai/commerce/models/organization"
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
//	POST /v1/billing/credits
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
			return http.Fail(c, 400, "invalid expiresIn duration", err)
		}
		grant.ExpiresAt = time.Now().Add(dur)
	}

	if err := grant.Create(); err != nil {
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

	return c.JSON(201, resp)
}

// The two refusals a billing read makes about its own scope, as values rather
// than prose. A caller outside this package maps each to the status the endpoints
// below map it to; matching on a message would make the wording a contract, and
// the wording is not one.
var (
	errNoOrg  = errors.New("no organization")
	errNoUser = errors.New("no user")
)

// IsNoOrg reports whether err is a core declining to answer because nothing
// named the tenant. The list endpoints answer this with an empty page rather than a
// status: an unnamed tenant has no rows, which is not something the caller can
// fix by asking differently.
func IsNoOrg(err error) bool { return errors.Is(err, errNoOrg) }

// IsNoUser reports whether err is a core declining to answer because nothing
// named the subject. The endpoints answer this 400, because dropping the subject
// does not narrow a credit read — it widens it to everyone in the org.
func IsNoUser(err error) bool { return errors.Is(err, errNoUser) }

// CreditGrant is one grant as the credit list has always described it: the row
// itself, spent or unspent, plus Active — the one fact a reader cannot work out
// from the others, since usability turns on the clock as well as the balance.
//
// ExpiresAt is a pointer because the wire has always distinguished a grant that
// lapses from one that does not by the PRESENCE of the key, and "expires at the
// zero time" is a different statement from "never expires".
type CreditGrant struct {
	ID             string        `json:"id"`
	UserID         string        `json:"userId"`
	Name           string        `json:"name"`
	AmountCents    int64         `json:"amountCents"`
	RemainingCents int64         `json:"remainingCents"`
	Currency       currency.Type `json:"currency"`
	Priority       int           `json:"priority"`
	EffectiveAt    time.Time     `json:"effectiveAt"`
	Tags           string        `json:"tags"`
	Voided         bool          `json:"voided"`
	Active         bool          `json:"active"`
	CreatedAt      time.Time     `json:"createdAt"`
	ExpiresAt      *time.Time    `json:"expiresAt,omitempty"`
}

// CreditBalance is what a user has left to spend, one entry per currency.
type CreditBalance struct {
	UserID   string        `json:"userId"`
	Balances []CreditEntry `json:"balances"`
}

// CreditEntry is one currency's share of a balance.
type CreditEntry struct {
	Currency  currency.Type `json:"currency"`
	Available int64         `json:"available"`
}

// CreditBreakdown is the same balance split by grant tag — trial credit told
// apart from purchased, which is the question Chat asks before it spends any.
type CreditBreakdown struct {
	UserID    string                `json:"userId"`
	Breakdown map[string]*CreditTag `json:"breakdown"`
	Total     CreditTotal           `json:"total"`
}

// CreditTag is one tag's cents and the earliest moment any of them lapse. The
// expiry is a pointer for the same reason it is on a grant: absent means nothing
// under this tag expires.
type CreditTag struct {
	Cents     int64      `json:"cents"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

// CreditTotal is the sum across every tag.
type CreditTotal struct {
	Cents int64 `json:"cents"`
}

// ListCreditGrants is a user's credit grants — all of them, spent, expired and
// voided included, because a grant list is a LEDGER and a ledger that hides its
// spent rows cannot be reconciled against a burn-down.
//
// It takes values rather than a request so a caller that is not a request can
// ask: the same list is read over the internal plane by a peer process that
// holds no grant store, and re-deriving the query there would be a second
// implementation of one question.
//
// The user is required rather than optional-like-a-filter. Dropping it does not
// narrow the answer, it WIDENS it to every user in the org — one tenant's
// customers reading each other's grants — so an empty user is refused here as
// firmly as a missing org.
func ListCreditGrants(ctx context.Context, org *organization.Organization, userID string) ([]CreditGrant, error) {
	if org == nil {
		return nil, fmt.Errorf("credit grants: %w", errNoOrg)
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("credit grants: %w", errNoUser)
	}
	db := datastore.New(org.Namespaced(ctx))
	rootKey := db.NewKey("synckey", "", 1, nil)
	grants := make([]*creditgrant.CreditGrant, 0)
	q := creditgrant.Query(db).Ancestor(rootKey).
		Filter("UserId=", userID)

	if _, err := q.GetAll(&grants); err != nil {
		return nil, err
	}

	items := make([]CreditGrant, 0, len(grants))
	for _, g := range grants {
		item := CreditGrant{
			ID:             g.Id(),
			UserID:         g.UserId,
			Name:           g.Name,
			AmountCents:    g.AmountCents,
			RemainingCents: g.RemainingCents,
			Currency:       g.Currency,
			Priority:       g.Priority,
			EffectiveAt:    g.EffectiveAt,
			Tags:           g.Tags,
			Voided:         g.Voided,
			Active:         g.IsActive(),
			CreatedAt:      g.CreatedAt,
		}
		if !g.ExpiresAt.IsZero() {
			exp := g.ExpiresAt
			item.ExpiresAt = &exp
		}
		items = append(items, item)
	}
	return items, nil
}

// ListBillingCreditGrants lists credit grants for a user.
//
//	GET /v1/billing/credits?userId=...
func ListBillingCreditGrants(c *zip.Ctx) error {
	// Same nil-org panic as GetTier, and it 500'd live beside it. Here the honest
	// answer differs: this is a LIST, and its five siblings on the same chain
	// (invoices, subscriptions, alerts, payouts) already answer an empty list when
	// no org resolves. An empty grant list matches them and cannot be mistaken for
	// money — unlike a tier, which is an authorization the router acts on.
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return c.JSON(200, map[string]any{"grants": []map[string]any{}, "count": 0})
	}

	userId := strings.TrimSpace(c.Query("userId"))
	if userId == "" {
		return http.Fail(c, 400, "userId query parameter is required", nil)
	}

	items, err := ListCreditGrants(c.Context(), org, userId)
	if err != nil {
		log.Error("Failed to list credit grants: %v", err, c)
		return http.Fail(c, 500, "failed to list credit grants", err)
	}

	return c.JSON(200, map[string]any{
		"grants": items,
		"count":  len(items),
	})
}

// ReadCreditBalance sums a user's ACTIVE grants per currency — what is spendable
// right now, which is why voided, exhausted and lapsed grants contribute nothing.
//
// It takes values for the same reason the grant list does, and it answers with
// the totals rather than the grants: a caller that only needs to know what is
// spendable should never have to reproduce the burn order to find out.
func ReadCreditBalance(ctx context.Context, org *organization.Organization, userID string) (*CreditBalance, error) {
	if org == nil {
		return nil, fmt.Errorf("credit balance: %w", errNoOrg)
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("credit balance: %w", errNoUser)
	}
	grants, err := getActiveGrants(datastore.New(org.Namespaced(ctx)), userID)
	if err != nil {
		return nil, err
	}

	// Sum by currency
	balances := make(map[currency.Type]int64)
	for _, g := range grants {
		balances[g.Currency] += g.RemainingCents
	}

	out := &CreditBalance{UserID: userID, Balances: make([]CreditEntry, 0, len(balances))}
	for cur, amount := range balances {
		out.Balances = append(out.Balances, CreditEntry{Currency: cur, Available: amount})
	}
	return out, nil
}

// ReadCreditBreakdown is the same balance grouped by the grants' tags, with each
// group's earliest expiry, so a caller can say "$5 of trial credit, gone Friday"
// instead of one undifferentiated number. Untagged grants group under "other".
func ReadCreditBreakdown(ctx context.Context, org *organization.Organization, userID string) (*CreditBreakdown, error) {
	if org == nil {
		return nil, fmt.Errorf("credit breakdown: %w", errNoOrg)
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("credit breakdown: %w", errNoUser)
	}
	grants, err := getActiveGrants(datastore.New(org.Namespaced(ctx)), userID)
	if err != nil {
		return nil, err
	}

	out := &CreditBreakdown{UserID: userID, Breakdown: make(map[string]*CreditTag)}
	for _, g := range grants {
		tag := g.Tags
		if tag == "" {
			tag = "other"
		}

		tb, ok := out.Breakdown[tag]
		if !ok {
			tb = &CreditTag{}
			out.Breakdown[tag] = tb
		}
		tb.Cents += g.RemainingCents
		out.Total.Cents += g.RemainingCents

		// Track the earliest expiry for this tag group
		if !g.ExpiresAt.IsZero() {
			if tb.ExpiresAt == nil || g.ExpiresAt.Before(*tb.ExpiresAt) {
				exp := g.ExpiresAt
				tb.ExpiresAt = &exp
			}
		}
	}
	return out, nil
}

// GetCreditBalance returns the total available credit balance for a user.
//
//	GET /v1/billing/credit-balance?userId=...
func GetCreditBalance(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)

	userId := strings.TrimSpace(c.Query("userId"))
	if userId == "" {
		return http.Fail(c, 400, "userId query parameter is required", nil)
	}

	balance, err := ReadCreditBalance(c.Context(), org, userId)
	if err != nil {
		log.Error("Failed to query credit grants: %v", err, c)
		return http.Fail(c, 500, "failed to query credit balance", err)
	}

	return c.JSON(200, balance)
}

// GetCreditBalanceBreakdown returns the credit balance grouped by tag.
// Used by Chat to distinguish trial vs paid credits.
//
//	GET /v1/billing/credit-balance/breakdown?userId=...
func GetCreditBalanceBreakdown(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)

	userId := strings.TrimSpace(c.Query("userId"))
	if userId == "" {
		return http.Fail(c, 400, "userId query parameter is required", nil)
	}

	breakdown, err := ReadCreditBreakdown(c.Context(), org, userId)
	if err != nil {
		log.Error("Failed to query credit grants for breakdown: %v", err, c)
		return http.Fail(c, 500, "failed to query credit balance", err)
	}

	return c.JSON(200, breakdown)
}

// VoidCreditGrant voids a specific credit grant, making it unusable.
//
//	POST /v1/billing/credits/:id/void
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
