// Package checkout: admin API scaffolding for /_/commerce/*.
//
// This file defines the minimum type/method surface the router expects
// (AdminStore interface, AdminAPI struct). The concrete handlers return
// HTTP 501 until the hanzo/base-backed store is wired — see STATUS.md.
//
// Security posture when endpoints go live:
//   - Every mutation derives the tenant from the IAM session claims
//     (never from the request body).
//   - Credentials never flow through commerce JSON; they stream directly
//     to KMS at commerce/{tenant}/{provider}/{field}.
//   - Every mutation appends to commerce_admin_audit with 7-year retention.
package checkout

import (
	"net/http"

	"github.com/zap-proto/zip"
)

// AdminStore is the persistence surface the admin handlers require. The
// production implementation wraps hanzo/base collections:
//   - tenants             → read/write Tenant records
//   - commerce_admin_audit → append-only audit log
//   - providers           → per-tenant provider enable/config
//
// The interface stays in this package so the admin code depends on the
// abstraction, not on base directly; this keeps the package unit-testable
// with in-memory stubs.
type AdminStore interface {
	// ListProviders returns the tenant's payment providers with credential
	// presence flags — never the actual credential values.
	ListProviders(tenantID string) ([]Provider, error)

	// SetProviderEnabled toggles the enabled flag for a single provider
	// on the given tenant. Writes an audit entry.
	SetProviderEnabled(tenantID, name string, enabled bool, actor string) error

	// AuditAppend records an admin action. Never stores secrets.
	AuditAppend(tenantID, actor, action string, meta map[string]any) error
}

// AdminAPI is the handler set bound to /_/commerce/*. Constructed by
// MountAdmin; holds the Resolver (for tenant config lookups) and the
// AdminStore (for persistence).
type AdminAPI struct {
	Resolver *StaticResolver
	Store    AdminStore
}

// notImplemented is the placeholder body until each endpoint is wired
// against the base-backed store. Returning 501 explicitly is safer than
// a silent stub that might look functional in a smoke test.
func notImplemented(c *zip.Ctx, op string) error {
	return c.JSON(http.StatusNotImplemented, map[string]any{
		"error": "not implemented",
		"op":    op,
	})
}

// ─── provider endpoints ─────────────────────────────────────────────────

func (a *AdminAPI) ListProviders(c *zip.Ctx) error   { return notImplemented(c, "list_providers") }
func (a *AdminAPI) EnableProvider(c *zip.Ctx) error  { return notImplemented(c, "enable_provider") }
func (a *AdminAPI) DisableProvider(c *zip.Ctx) error { return notImplemented(c, "disable_provider") }
func (a *AdminAPI) UploadCredentials(c *zip.Ctx) error {
	return notImplemented(c, "upload_credentials")
}
func (a *AdminAPI) RotateCredentials(c *zip.Ctx) error {
	return notImplemented(c, "rotate_credentials")
}
func (a *AdminAPI) TestProvider(c *zip.Ctx) error { return notImplemented(c, "test_provider") }

// ─── method endpoints ───────────────────────────────────────────────────

func (a *AdminAPI) ListMethods(c *zip.Ctx) error     { return notImplemented(c, "list_methods") }
func (a *AdminAPI) ConfigureMethod(c *zip.Ctx) error { return notImplemented(c, "configure_method") }

// ─── IDV endpoints ──────────────────────────────────────────────────────

func (a *AdminAPI) GetIDV(c *zip.Ctx) error { return notImplemented(c, "get_idv") }
func (a *AdminAPI) SetIDV(c *zip.Ctx) error { return notImplemented(c, "set_idv") }

// ─── IAM endpoints ──────────────────────────────────────────────────────

func (a *AdminAPI) GetIAM(c *zip.Ctx) error { return notImplemented(c, "get_iam") }
func (a *AdminAPI) SetIAM(c *zip.Ctx) error { return notImplemented(c, "set_iam") }

// ─── audit log ──────────────────────────────────────────────────────────

func (a *AdminAPI) AuditLog(c *zip.Ctx) error { return notImplemented(c, "audit_log") }

// Mount is the convenience entrypoint used by commerce.go setupRoutes.
// It wires the public checkout routes onto the /v1/commerce API group
// (on the caller's zip app) and registers the least-specific SPA fallback. Admin routes are attached separately by the superadmin
// router once the admin API is complete.
func Mount(app *zip.App, r Resolver, fwd Forwarder) {
	MountPublic(app.Group("/v1/commerce"), r, fwd)
	MountSPA(app)
}
