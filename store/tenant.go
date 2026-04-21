package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/hanzoai/base/core"
	"github.com/hanzoai/base/tools/types"
)

// ErrTenantNotFound is returned by every lookup that fails. Handlers MUST
// translate this to HTTP 404 (never 500) and MUST NOT echo the lookup key
// in the response body — that would be a free fingerprinting oracle.
var ErrTenantNotFound = errors.New("store: tenant not found")

// ErrDuplicateTenant is returned when a Create would violate the unique-by-
// name index. Handlers translate to 409 Conflict.
var ErrDuplicateTenant = errors.New("store: tenant with that name already exists")

// ErrInvalidHostname is returned when a hostname fails normalization or
// contains characters that are illegal in a Host header value. Handlers
// translate to 400.
var ErrInvalidHostname = errors.New("store: invalid hostname")

// ErrHostnameClaimed is returned when a Create or UpdateHostnames would
// cross-assign a hostname already owned by a different tenant. Handlers
// translate to 409 Conflict with a body that does NOT reveal the colliding
// tenant's identity — that would be a fingerprinting oracle.
var ErrHostnameClaimed = errors.New("store: hostname already claimed by another tenant")

// TenantRepo is the typed persistence API over the commerce_tenants
// collection. It intentionally does not do tenant scoping — that is the
// handler layer's responsibility. Repo methods trust their caller.
type TenantRepo struct {
	app core.App
}

// NewTenantRepo wraps a base app. The collection must already exist (the
// migration under store/migrations creates it on Bootstrap).
func NewTenantRepo(app core.App) *TenantRepo {
	return &TenantRepo{app: app}
}

// Create persists a new tenant row atomically with its hostname claims.
//
// Atomicity guarantee (P8-C1 fix): tenant row save + hostname inserts run
// inside a single transaction. A collision on hostname (unique index on
// commerce_tenant_hostnames.hostname) rolls back the tenant row as well, so
// a failed create leaves no dangling state and no "dormant" row.
//
// Uniqueness:
//   - `name` — enforced by idx_commerce_tenants_name (unique). Duplicate
//     returns ErrDuplicateTenant.
//   - each hostname — enforced by idx_commerce_tenant_hostnames_unique
//     (unique). Cross-tenant collision returns ErrHostnameClaimed.
//
// The caller MUST have already checked superadmin privilege; Create does
// not verify identity.
func (r *TenantRepo) Create(t *Tenant) error {
	if t == nil {
		return errors.New("store: nil tenant")
	}
	if strings.TrimSpace(t.Name) == "" {
		return errors.New("store: tenant name required")
	}

	hosts, err := normalizeHostnames(t.Hostnames)
	if err != nil {
		return err
	}
	t.Hostnames = hosts

	return r.app.RunInTransaction(func(txApp core.App) error {
		collection, err := txApp.FindCollectionByNameOrId("commerce_tenants")
		if err != nil {
			return fmt.Errorf("store: find collection: %w", err)
		}

		rec := core.NewRecord(collection)
		if err := applyTenantToRecord(rec, t); err != nil {
			return err
		}

		if err := txApp.Save(rec); err != nil {
			if isUniqueViolation(err) {
				return ErrDuplicateTenant
			}
			return fmt.Errorf("store: save tenant: %w", err)
		}

		if err := writeTenantHostnames(txApp, rec.Id, hosts); err != nil {
			return err
		}

		t.ID = rec.Id
		t.Created = rec.GetDateTime("created").Time()
		t.Updated = rec.GetDateTime("updated").Time()
		return nil
	})
}

// UpdateHostnames replaces the tenant's hostname claim set atomically. All of
// the tenant's existing commerce_tenant_hostnames rows are deleted and the
// canonicalized new set is inserted under the same transaction; the unique
// index on commerce_tenant_hostnames.hostname rejects any incoming hostname
// already owned by a different tenant (ErrHostnameClaimed).
//
// Concurrency semantics: serializable via the SQL engine's unique-index
// contention. Two transactions racing to claim "pay.shared.test" — at most
// one commits; the other returns ErrHostnameClaimed. No application-layer
// mutex is required or sufficient across replicas.
func (r *TenantRepo) UpdateHostnames(id string, hostnames []string) error {
	if strings.TrimSpace(id) == "" {
		return ErrTenantNotFound
	}
	hosts, err := normalizeHostnames(hostnames)
	if err != nil {
		return err
	}

	return r.app.RunInTransaction(func(txApp core.App) error {
		rec, err := txApp.FindRecordById("commerce_tenants", id)
		if err != nil || rec == nil {
			return ErrTenantNotFound
		}

		if err := writeTenantHostnames(txApp, id, hosts); err != nil {
			return err
		}

		// Keep the denormalized commerce_tenants.hostnames JSON column in
		// sync with the authoritative join table so tenant admin views
		// return a consistent projection in a single SELECT.
		hostsJSON, err := json.Marshal(hosts)
		if err != nil {
			return err
		}
		rec.Set("hostnames", string(hostsJSON))
		if err := txApp.Save(rec); err != nil {
			return fmt.Errorf("store: save tenant hostnames mirror: %w", err)
		}
		return nil
	})
}

// writeTenantHostnames is the tx-scoped inner routine shared by Create and
// UpdateHostnames. Deletes all current rows for this tenant, then inserts the
// new set; a unique-index violation returns ErrHostnameClaimed and the tx
// aborts so no partial state survives. Callers MUST invoke this from within
// RunInTransaction — it does not open its own tx.
func writeTenantHostnames(txApp core.App, tenantID string, hosts []string) error {
	existing, err := txApp.FindRecordsByFilter(
		"commerce_tenant_hostnames",
		"tenant = {:tid}",
		"created",
		-1, 0,
		map[string]any{"tid": tenantID},
	)
	if err != nil {
		return fmt.Errorf("store: list existing hostnames: %w", err)
	}
	for _, row := range existing {
		if err := txApp.Delete(row); err != nil {
			return fmt.Errorf("store: delete old hostname: %w", err)
		}
	}

	if len(hosts) == 0 {
		return nil
	}

	hostnamesCollection, err := txApp.FindCollectionByNameOrId("commerce_tenant_hostnames")
	if err != nil {
		return fmt.Errorf("store: find hostnames collection: %w", err)
	}

	for _, h := range hosts {
		rec := core.NewRecord(hostnamesCollection)
		rec.Set("hostname", h)
		rec.Set("tenant", tenantID)
		if err := txApp.Save(rec); err != nil {
			if isUniqueViolation(err) {
				return ErrHostnameClaimed
			}
			return fmt.Errorf("store: save hostname: %w", err)
		}
	}
	return nil
}

// FindByID returns the tenant with the given id, or ErrTenantNotFound.
func (r *TenantRepo) FindByID(id string) (*Tenant, error) {
	if strings.TrimSpace(id) == "" {
		return nil, ErrTenantNotFound
	}
	rec, err := r.app.FindRecordById("commerce_tenants", id)
	if err != nil {
		// Collapse every lookup failure to ErrTenantNotFound so callers
		// cannot distinguish "missing row" from "missing column" through
		// the error type. Error strings still carry detail via %w for
		// debug logs.
		return nil, ErrTenantNotFound
	}
	return recordToTenant(rec)
}

// FindByHostname resolves a hostname to its owning tenant via the
// commerce_tenant_hostnames join table. The input is normalized (lowercase,
// trailing-dot stripped, port stripped) before lookup; malformed inputs
// return ErrInvalidHostname. Exact-match only — suffix spoofing
// ("pay.satschel.com.evil.com") is rejected because the row-per-hostname
// store does a point lookup on the unique index, never a prefix/LIKE scan.
func (r *TenantRepo) FindByHostname(host string) (*Tenant, error) {
	h, err := normalizeHostname(host)
	if err != nil {
		return nil, err
	}

	row, err := r.app.FindFirstRecordByFilter(
		"commerce_tenant_hostnames",
		"hostname = {:h}",
		map[string]any{"h": h},
	)
	if err != nil || row == nil {
		return nil, ErrTenantNotFound
	}

	tenantID := row.GetString("tenant")
	if tenantID == "" {
		return nil, ErrTenantNotFound
	}

	tenantRec, err := r.app.FindRecordById("commerce_tenants", tenantID)
	if err != nil || tenantRec == nil {
		// Cascade delete should have cleaned orphaned hostname rows — if we
		// see one, treat it as not-found rather than 500, to avoid leaking
		// store-drift state to the public endpoint.
		return nil, ErrTenantNotFound
	}
	return recordToTenant(tenantRec)
}

// UpdateProviders replaces the tenant's providers list atomically. Concurrency
// model: last-write-wins. If two admins PUT at the same time, the later save
// overwrites the earlier one. The audit log (logged at the handler layer)
// records both attempts so operators can reconcile. A future slice may add
// optimistic-locking via a row version column — that is out of scope here.
func (r *TenantRepo) UpdateProviders(id string, providers []Provider) error {
	if strings.TrimSpace(id) == "" {
		return ErrTenantNotFound
	}
	rec, err := r.app.FindRecordById("commerce_tenants", id)
	if err != nil {
		return ErrTenantNotFound
	}

	payload, err := json.Marshal(providers)
	if err != nil {
		return fmt.Errorf("store: marshal providers: %w", err)
	}
	rec.Set("providers", string(payload))

	if err := r.app.Save(rec); err != nil {
		return fmt.Errorf("store: save providers: %w", err)
	}
	return nil
}

// List returns tenants ordered by name ascending for admin dashboards.
// limit is clamped to [1, 500]; offset is clamped to [0, ∞). A zero limit
// is treated as 50 to avoid accidental full-table scans.
func (r *TenantRepo) List(limit, offset int) ([]*Tenant, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}

	records, err := r.app.FindRecordsByFilter(
		"commerce_tenants",
		"",
		"name",
		limit,
		offset,
	)
	if err != nil {
		return nil, fmt.Errorf("store: list tenants: %w", err)
	}

	out := make([]*Tenant, 0, len(records))
	for _, rec := range records {
		t, err := recordToTenant(rec)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

// ─── helpers ────────────────────────────────────────────────────────────

// applyTenantToRecord maps a Tenant value onto a record. All JSON fields are
// marshaled to a canonical byte form; empty slices are stored as "[]" rather
// than "null" to keep the hostname-LIKE lookup from matching a JSON null.
func applyTenantToRecord(rec *core.Record, t *Tenant) error {
	rec.Set("name", t.Name)

	hosts := t.Hostnames
	if hosts == nil {
		hosts = []string{}
	}
	hostsJSON, err := json.Marshal(hosts)
	if err != nil {
		return err
	}
	rec.Set("hostnames", string(hostsJSON))

	brandJSON, err := json.Marshal(t.Brand)
	if err != nil {
		return err
	}
	rec.Set("brand", string(brandJSON))

	iamJSON, err := json.Marshal(t.IAM)
	if err != nil {
		return err
	}
	rec.Set("iam", string(iamJSON))

	idvJSON, err := json.Marshal(t.IDV)
	if err != nil {
		return err
	}
	rec.Set("idv", string(idvJSON))

	providers := t.Providers
	if providers == nil {
		providers = []Provider{}
	}
	providersJSON, err := json.Marshal(providers)
	if err != nil {
		return err
	}
	rec.Set("providers", string(providersJSON))

	rec.Set("bd_endpoint", t.BDEndpoint)

	allowlist := t.ReturnURLAllowlist
	if allowlist == nil {
		allowlist = []string{}
	}
	allowlistJSON, err := json.Marshal(allowlist)
	if err != nil {
		return err
	}
	rec.Set("return_url_allowlist", string(allowlistJSON))

	return nil
}

// recordToTenant inverts applyTenantToRecord.
func recordToTenant(rec *core.Record) (*Tenant, error) {
	t := &Tenant{
		ID:         rec.Id,
		Name:       rec.GetString("name"),
		BDEndpoint: rec.GetString("bd_endpoint"),
		Created:    rec.GetDateTime("created").Time(),
		Updated:    rec.GetDateTime("updated").Time(),
	}

	if err := unmarshalJSONField(rec, "hostnames", &t.Hostnames); err != nil {
		return nil, err
	}
	if err := unmarshalJSONField(rec, "brand", &t.Brand); err != nil {
		return nil, err
	}
	if err := unmarshalJSONField(rec, "iam", &t.IAM); err != nil {
		return nil, err
	}
	if err := unmarshalJSONField(rec, "idv", &t.IDV); err != nil {
		return nil, err
	}
	if err := unmarshalJSONField(rec, "providers", &t.Providers); err != nil {
		return nil, err
	}
	if err := unmarshalJSONField(rec, "return_url_allowlist", &t.ReturnURLAllowlist); err != nil {
		return nil, err
	}
	return t, nil
}

// unmarshalJSONField reads a JSON-typed column off a record into dst. Empty
// or null columns produce a zero-value dst (not an error) — base persists
// unset JSON fields as "null" or empty bytes.
func unmarshalJSONField(rec *core.Record, name string, dst any) error {
	raw, _ := rec.GetRaw(name).(types.JSONRaw)
	if len(raw) == 0 {
		return nil
	}
	s := strings.TrimSpace(raw.String())
	if s == "" || s == "null" {
		return nil
	}
	if err := json.Unmarshal([]byte(s), dst); err != nil {
		return fmt.Errorf("store: decode %s: %w", name, err)
	}
	return nil
}

// normalizeHostname applies the canonical checkout tenant-resolution rule:
//   - lowercase
//   - trailing "." stripped (absolute DNS form — "pay.satschel.com." == "pay.satschel.com")
//   - :port stripped
//   - reject embedded whitespace / control bytes / quote characters
//   - reject empty / pure-port input
//
// The rule deliberately does NOT trim leading/trailing whitespace. A well-
// formed Host header has none, and silently repairing input turns a bug
// (or an attack probe) into a silent success. Match checkout/tenant.go's
// stance: reject on any non-printable byte rather than normalize it away.
func normalizeHostname(host string) (string, error) {
	if host == "" {
		return "", ErrInvalidHostname
	}
	for i := 0; i < len(host); i++ {
		b := host[i]
		if b <= 0x20 || b == 0x7f || b == '"' || b == '\\' {
			return "", ErrInvalidHostname
		}
	}
	if strings.HasPrefix(host, "[") {
		return "", ErrInvalidHostname // no IPv6 literals as tenant keys
	}
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	host = strings.TrimSuffix(host, ".")
	if host == "" {
		return "", ErrInvalidHostname
	}
	host = strings.ToLower(host)
	if ip := net.ParseIP(host); ip != nil {
		return "", ErrInvalidHostname // raw IPs are not valid tenant keys
	}
	if !validHostname(host) {
		return "", ErrInvalidHostname
	}
	return host, nil
}

// normalizeHostnames normalizes each entry and dedupes. Empty input is valid
// (tenant starts with no hostnames and gets them added later).
func normalizeHostnames(hosts []string) ([]string, error) {
	if len(hosts) == 0 {
		return []string{}, nil
	}
	seen := make(map[string]struct{}, len(hosts))
	out := make([]string, 0, len(hosts))
	for _, h := range hosts {
		n, err := normalizeHostname(h)
		if err != nil {
			return nil, err
		}
		if _, dup := seen[n]; dup {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out, nil
}

// validHostname enforces RFC 1123 hostname syntax: labels 1–63 chars,
// LDH-only (letters/digits/hyphen), no leading/trailing hyphen per label,
// total length ≤253. Zero-allocation hand-rolled scanner.
func validHostname(host string) bool {
	if len(host) == 0 || len(host) > 253 {
		return false
	}
	labelLen := 0
	prev := byte('.')
	for i := 0; i < len(host); i++ {
		c := host[i]
		switch {
		case c == '.':
			if labelLen == 0 || prev == '-' {
				return false
			}
			labelLen = 0
		case c == '-':
			if labelLen == 0 {
				return false
			}
			labelLen++
		case (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9'):
			labelLen++
		default:
			return false
		}
		if labelLen > 63 {
			return false
		}
		prev = c
	}
	return labelLen > 0 && prev != '-'
}

// isUniqueViolation detects the base-surfaced unique-index error. Base has
// two layers of uniqueness enforcement, both of which must be translated to
// ErrDuplicateTenant so the handler layer sees a stable error:
//
//  1. Field-level validator on a TextField with a unique index — base
//     returns `"<fieldName>: Value must be unique."` (ozzo-validation).
//  2. DB-level constraint (SQLite: "UNIQUE constraint failed"; Postgres:
//     "duplicate key value violates unique constraint") when validation is
//     bypassed (e.g. SaveNoValidate) or the race closes between validate
//     and save.
//
// Both strings are checked. A future base release that changes the wording
// will surface as a test regression, never as a silent 500.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Value must be unique") ||
		strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "duplicate key value")
}

// _ unused-import guards.
var _ = time.Time{}
