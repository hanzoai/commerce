package checkout

import (
	"net/http"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/store"
)

// Turning a payment rail on and off.
//
// The provider list was READABLE and not writable: ListProviders projects
// {name, enabled} for an admin UI, and nothing could change it. So which rails a
// deployment offers — card, Apple Pay, Google Pay, Cash App, ACH, wire, crypto —
// was decided by whatever happened to be in the tenant row, and moving one meant
// editing the database. That is the gap this closes.
//
// ONE RAIL PER CALL, and that is a correctness requirement rather than a taste:
// a Provider carries a KMSPath naming where its credentials live, and
// ListProviders deliberately strips it (an admin UI has no business knowing
// where a secret is). A PUT that replaced the whole list from what the UI can
// see would therefore write back every provider with an EMPTY KMSPath and
// silently disconnect every rail from its credentials — the read projection and
// the write shape are not the same object, and a round-trip through the browser
// is exactly where that becomes a data-loss bug. Naming one rail keeps every
// other record untouched, byte for byte.
//
// Scope is the caller's own tenant, resolved from the IAM owner claim. No tenant
// id is accepted from the client, so there is no cross-tenant write to defend
// against — the same rule ListProviders already follows.

// knownRails is the set of provider names that MEAN something downstream. The
// pay SPA maps a provider name to the methods it surfaces (square →
// card/apple_pay/google_pay/cash_app/ach; plaid → ach; wire, crypto and the
// other processors to themselves), so a name outside this set enables nothing
// and would be config that looks applied and does nothing at all. Refusing it
// is the difference between a typo that reports success and a typo that says
// what the valid names are.
var knownRails = map[string]bool{
	"square":    true,
	"stripe":    true,
	"braintree": true,
	"plaid":     true,
	"wire":      true,
	"crypto":    true,
}

// setProviderRequest is the body: one boolean, because the name is in the path.
// A pointer distinguishes "enabled: false" from an absent field — the zero value
// of a bool is a legitimate instruction here, and a missing one is a malformed
// request rather than a disable.
type setProviderRequest struct {
	Enabled *bool `json:"enabled"`
}

// SetProviderEnabled turns one payment rail on or off for the caller's tenant.
//
//	PUT /_/commerce/providers/{name}   {"enabled": true}
//
// It is idempotent: setting a rail to the state it already holds succeeds and
// changes nothing. A rail the tenant has never carried is APPENDED (disabled
// rails are absent, not present-and-false, in rows written before this existed),
// which is what makes turning a rail on for the first time possible at all.
func (a *TenantAdminAPI) SetProviderEnabled(c *zip.Ctx) error {
	// Anonymous is 401 — not signed in, never the authorization 403. Same
	// distinction ListProviders draws, and a browser only re-authenticates on 401.
	if !iammiddleware.IsIAMAuthenticated(c) {
		return c.JSON(http.StatusUnauthorized, map[string]any{"error": "authentication required"})
	}
	claims := iammiddleware.GetIAMClaims(c)
	if !isSuperadmin(claims) && !isTenantAdmin(claims) {
		return c.JSON(http.StatusForbidden, map[string]any{"error": "admin role required"})
	}

	name := strings.ToLower(strings.TrimSpace(c.Param("name")))
	if name == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "a provider name is required"})
	}
	if !knownRails[name] {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error": "unknown provider " + name + "; nothing downstream reads that name",
			"known": knownRailNames(),
		})
	}

	var req setProviderRequest
	if err := c.Bind(&req); err != nil || req.Enabled == nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "body must be {\"enabled\": true|false}"})
	}

	owner := claims.Owner
	if owner == "" {
		return c.JSON(http.StatusNotFound, map[string]any{"error": "not found"})
	}
	tenant, err := a.findTenantByOwner(owner)
	if err != nil {
		// Byte-identical 404 whether the row is absent or belongs to someone
		// else: the owner claim pins scope, and a status that separates the two
		// turns a tenant name into something worth guessing.
		return c.JSON(http.StatusNotFound, map[string]any{"error": "not found"})
	}

	// Copy the stored records forward WHOLE — KMSPath included — and change the
	// single field this call is about.
	next := make([]store.Provider, len(tenant.Providers))
	copy(next, tenant.Providers)
	found := false
	for i := range next {
		if strings.EqualFold(strings.TrimSpace(next[i].Name), name) {
			next[i].Enabled = *req.Enabled
			found = true
			break
		}
	}
	if !found {
		next = append(next, store.Provider{Name: name, Enabled: *req.Enabled})
	}

	if err := a.Store.Tenants.UpdateProviders(tenant.ID, next); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": "could not save the provider list"})
	}

	// Answer with the same projection ListProviders serves, so a UI can render
	// the result of the write without a second round trip — and so the KMS paths
	// stay where they belong, which is out of the response.
	projected := make([]providerListItem, 0, len(next))
	for _, p := range next {
		projected = append(projected, providerListItem{Name: p.Name, Enabled: p.Enabled})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"tenant":    tenant.Name,
		"providers": projected,
	})
}

// knownRailNames lists the accepted provider names for the 400 body, sorted so
// the message is stable rather than map-ordered.
func knownRailNames() []string {
	out := make([]string, 0, len(knownRails))
	for n := range knownRails {
		out = append(out, n)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
