package affiliate

import (
	"github.com/zap-proto/zip"

	payoutcron "github.com/hanzoai/commerce/cron/payout/contributor"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/contributor"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/rest"
)

// registerContributor allows a user to register as an OSS contributor.
func registerContributor(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	contrib := contributor.New(db)
	if err := json.DecodeBytes(c.Body(), contrib); err != nil {
		return http.Fail(c, 400, "Failed to decode request body", err)
	}

	if contrib.GitLogin == "" || contrib.GitEmail == "" {
		return http.Fail(c, 400, "gitLogin and gitEmail are required", nil)
	}

	// Check if contributor already exists with this git login
	existing := contributor.New(db)
	if _, ok, _ := contributor.Query(db).Filter("GitLogin=", contrib.GitLogin).First(existing); ok {
		return http.Render(c, 200, existing)
	}

	contrib.Active = true
	if err := contrib.Create(); err != nil {
		return http.Fail(c, 500, "Failed to create contributor", err)
	}

	c.SetHeader("Location", c.Path()+"/"+contrib.Id())
	return http.Render(c, 201, contrib)
}

// contributorCreate returns the admin create override for contributor CRUD.
func contributorCreate(r *rest.Rest) func(*zip.Ctx) error {
	return func(c *zip.Ctx) error {
		if !r.CheckPermissions(c, "create") {
			return nil
		}

		org := middleware.GetOrganization(c)
		db := datastore.New(org.Namespaced(c.Context()))
		contrib := contributor.New(db)

		if err := json.DecodeBytes(c.Body(), contrib); err != nil {
			return r.Fail(c, 400, "Failed to decode request body", err)
		}

		if contrib.GitLogin == "" {
			return r.Fail(c, 400, "gitLogin is required", nil)
		}

		if err := contrib.Create(); err != nil {
			return r.Fail(c, 500, "Failed to create contributor", err)
		}

		c.SetHeader("Location", c.Path()+"/"+contrib.Id())
		return r.Render(c, 201, contrib)
	}
}

// contributorGetByLogin looks up a contributor by their git login.
func contributorGetByLogin(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	login := c.Param("login")

	contrib := contributor.New(db)
	if _, ok, _ := contributor.Query(db).Filter("GitLogin=", login).First(contrib); !ok {
		return http.Fail(c, 404, "No contributor found with login: "+login, nil)
	}

	return http.Render(c, 200, contrib)
}

// getEarnings returns a contributor's earnings summary.
func getEarnings(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	id := c.Param("contributorid")

	contrib := contributor.New(db)
	if err := contrib.GetById(id); err != nil {
		return http.Fail(c, 404, "Contributor not found: "+id, err)
	}

	return http.Render(c, 200, map[string]any{
		"contributorId": contrib.Id(),
		"gitLogin":      contrib.GitLogin,
		"totalEarned":   contrib.TotalEarned,
		"totalPending":  contrib.TotalPending,
		"linesAuthored": contrib.TotalLinesAuthored,
		"payoutMethod":  contrib.PayoutMethod,
		"currency":      contrib.Currency,
		"lastPaid":      contrib.LastPaid,
	})
}

// getAttributions returns a contributor's SBOM attributions.
func getAttributions(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	id := c.Param("contributorid")

	contrib := contributor.New(db)
	if err := contrib.GetById(id); err != nil {
		return http.Fail(c, 404, "Contributor not found: "+id, err)
	}

	return http.Render(c, 200, contrib.Attributions)
}

// createSBOMEntry creates or updates an SBOM entry for a component.
func createSBOMEntry(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	entry := contributor.NewSBOM(db)
	if err := json.DecodeBytes(c.Body(), entry); err != nil {
		return http.Fail(c, 400, "Failed to decode request body", err)
	}

	if entry.Component == "" {
		return http.Fail(c, 400, "component is required", nil)
	}

	if err := entry.Create(); err != nil {
		return http.Fail(c, 500, "Failed to create SBOM entry", err)
	}

	return http.Render(c, 201, entry)
}

// listSBOMEntries returns all SBOM entries.
func listSBOMEntries(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	entries := make([]contributor.SBOMEntry, 0)
	if _, err := contributor.QuerySBOM(db).GetAll(&entries); err != nil {
		return http.Fail(c, 500, "Failed to query SBOM entries", err)
	}

	return http.Render(c, 200, entries)
}

// getSBOMEntry returns a single SBOM entry by ID.
func getSBOMEntry(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	id := c.Param("sbomid")

	entry := contributor.NewSBOM(db)
	if err := entry.GetById(id); err != nil {
		return http.Fail(c, 404, "SBOM entry not found: "+id, err)
	}

	return http.Render(c, 200, entry)
}

// updateSBOMEntry updates an existing SBOM entry.
func updateSBOMEntry(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	id := c.Param("sbomid")

	entry := contributor.NewSBOM(db)
	if err := entry.GetById(id); err != nil {
		return http.Fail(c, 404, "SBOM entry not found: "+id, err)
	}

	if err := json.DecodeBytes(c.Body(), entry); err != nil {
		return http.Fail(c, 400, "Failed to decode request body", err)
	}

	if err := entry.Update(); err != nil {
		return http.Fail(c, 500, "Failed to update SBOM entry", err)
	}

	return http.Render(c, 200, entry)
}

// calculatePayouts runs the payout algorithm and returns results.
func calculatePayouts(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	var req struct {
		TotalRevenueCents int64                     `json:"totalRevenueCents"`
		ComponentRevenue  map[string]int64          `json:"componentRevenue"`
		Config            *contributor.PayoutConfig `json:"config,omitempty"`
	}

	if err := json.DecodeBytes(c.Body(), &req); err != nil {
		return http.Fail(c, 400, "Failed to decode request body", err)
	}

	if req.TotalRevenueCents <= 0 {
		return http.Fail(c, 400, "totalRevenueCents must be positive", nil)
	}

	// Load all active, verified contributors
	contributors := make([]contributor.Contributor, 0)
	if _, err := contributor.Query(db).Filter("Active=", true).Filter("Verified=", true).GetAll(&contributors); err != nil {
		return http.Fail(c, 500, "Failed to query contributors", err)
	}

	cfg := contributor.DefaultConfig()
	if req.Config != nil {
		cfg = *req.Config
	}

	summary := contributor.CalculatePayouts(
		req.TotalRevenueCents,
		contributors,
		req.ComponentRevenue,
		cfg,
	)

	return http.Render(c, 200, summary)
}

// previewPayouts returns a dry-run of what payouts would look like
// using the default config and current SBOM revenue data.
func previewPayouts(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	// Load SBOM entries to build component revenue map
	entries := make([]contributor.SBOMEntry, 0)
	if _, err := contributor.QuerySBOM(db).GetAll(&entries); err != nil {
		return http.Fail(c, 500, "Failed to query SBOM entries", err)
	}

	// Same real per-component attribution the payout cron uses: usage, then
	// lines, then equal weight across shipped components.
	componentRevenue := contributor.ComponentWeightsFromSBOM(entries)
	var totalRevenue int64
	for _, w := range componentRevenue {
		totalRevenue += w
	}

	// Load contributors
	contributors := make([]contributor.Contributor, 0)
	if _, err := contributor.Query(db).Filter("Active=", true).Filter("Verified=", true).GetAll(&contributors); err != nil {
		return http.Fail(c, 500, "Failed to query contributors", err)
	}

	summary := contributor.CalculatePayouts(
		totalRevenue,
		contributors,
		componentRevenue,
		contributor.DefaultConfig(),
	)

	return http.Render(c, 200, summary)
}

// executePayouts runs the OSS contributor payout EXECUTOR for the caller's org
// namespace: it computes the period's revenue, calculates the 25% split via
// SBOM attribution, and disburses each allocation — CreditGrants, queued
// Stripe transfers, and on-chain HUSD transfers (PayoutMethod="crypto").
//
// Dry-run by default (logs what would happen, pays nothing). Pass
// ?execute=true to actually disburse. Crypto payouts fail closed
// (ErrHUSDNotConfigured) unless HUSD_TOKEN_ADDRESS + HUSD_TREASURY_KEY are set
// from KMS, so an unconfigured deploy never silently mis-pays. Intended for
// admin / CronJob invocation (mirrors billing auto-recharge/run-all).
//
//	POST /payouts/execute              (dry-run)
//	POST /payouts/execute?execute=true (live disbursement)
func executePayouts(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	dryRun := c.Query("execute") != "true"

	if err := payoutcron.Payout(org.Namespaced(c.Context()), payoutcron.Config{
		Namespace: org.Namespace(),
		DryRun:    dryRun,
	}); err != nil {
		return http.Fail(c, 500, "contributor payout failed", err)
	}

	return http.Render(c, 200, map[string]any{"ok": true, "namespace": org.Namespace(), "dryRun": dryRun})
}
