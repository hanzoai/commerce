// Copyright © 2026 Hanzo AI. MIT License.

// The platform-admin CRUD over the rate authority — what one unit of anything
// costs, edited at admin.hanzo.ai instead of in a Go file.
//
// Rates used to live in an embedded 150KB JSON: 506 priced items across models,
// tools, infrastructure, cloud and datastore. Changing one number meant editing
// a module, cutting a tag, bumping the service and waiting for a build, so the
// published price and the intended price drifted for as long as that took.
// These rows are the price now; nothing is compiled in.
//
// PLATFORM ADMIN, NOT ORG ADMIN. A rate is cross-tenant money — one row decides
// what every customer pays for a model — so an org's own admin must not reach
// it. Each handler asks IsSuperAdmin() itself rather than trusting the bundle's
// gate: a token gate proves a caller is authenticated, never that they are the
// platform.
package rate

import (
	"errors"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/models/rate"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
)

func rateDB(c *zip.Ctx) *datastore.Datastore { return rate.AuthorityDB(c.Context()) }

func requireSuperAdmin(c *zip.Ctx) bool {
	claims := iammiddleware.GetIAMClaims(c)
	if claims == nil || !claims.IsSuperAdmin() {
		http.Fail(c, 403, "platform admin required to edit rates", errors.New("not a global admin"))
		return false
	}
	return true
}

// AdminRoute wires the rate CRUD and the bulk import on the /v1 bundle. args
// carry the bundle's own middleware; every handler also gates on IsSuperAdmin.
func AdminRoute(r zip.Router, args ...zip.Handler) {
	g := r.Group("/rates")
	g.Get("/entries", append(args, ListEntries)...)
	g.Post("/entries", append(args, CreateEntry)...)
	g.Put("/entries/:slug", append(args, UpdateEntry)...)
	g.Delete("/entries/:slug", append(args, DeleteEntry)...)
	g.Post("/import", append(args, ImportEntries)...)
}

// ListEntries returns the authority rows. Optionally narrowed by ?product= so
// an editor can show one surface at a time rather than every rate at once.
func ListEntries(c *zip.Ctx) error {
	if !requireSuperAdmin(c) {
		return nil
	}
	q := rate.Query(rateDB(c))
	if p := strings.TrimSpace(c.Query("product")); p != "" {
		q = q.Filter("Product=", p)
	}
	rows := make([]*rate.Rate, 0, 128)
	if _, err := q.GetAll(&rows); err != nil {
		return http.Fail(c, 500, "failed to list rates", err)
	}
	return http.Render(c, 200, rows)
}

// CreateEntry adds a rate. Product and Meter are both required: together they
// are the identity, and a rate keyed on the metered thing alone would let one
// product's price overwrite another's for the same name.
func CreateEntry(c *zip.Ctx) error {
	if !requireSuperAdmin(c) {
		return nil
	}
	in := new(rate.Rate)
	if err := json.DecodeBytes(c.Body(), in); err != nil {
		return http.Fail(c, 400, "invalid rate", err)
	}
	if strings.TrimSpace(in.Product) == "" || strings.TrimSpace(in.Meter) == "" {
		return http.Fail(c, 400, "product and meter are required", errors.New("no identity"))
	}

	db := rateDB(c)
	in.Bind()
	existing := rate.New(db)
	if ok, err := existing.Query().Filter("Slug=", in.Slug).Get(); err != nil {
		return http.Fail(c, 500, "failed to check for an existing rate", err)
	} else if ok {
		return http.Fail(c, 409, "a rate already exists for "+in.Slug, errors.New("duplicate"))
	}

	row := rate.New(db)
	assign(row, in)
	// Created HERE is created by a person, so it is theirs from the start and no
	// later import may quietly replace it.
	row.AdminEdited = true
	if err := row.Create(); err != nil {
		return http.Fail(c, 500, "failed to create rate", err)
	}
	return http.Render(c, 201, row)
}

// UpdateEntry edits a rate and MARKS IT as edited. That mark is the whole
// contract with the importer: an operator's price outranks the document it was
// imported from, so a later import leaves this row alone. Without it, a price
// set here would apply, work, and silently revert on the next import.
func UpdateEntry(c *zip.Ctx) error {
	if !requireSuperAdmin(c) {
		return nil
	}
	slug := strings.TrimSpace(c.Param("slug"))
	if slug == "" {
		return http.Fail(c, 400, "slug is required", errors.New("no slug"))
	}
	in := new(rate.Rate)
	if err := json.DecodeBytes(c.Body(), in); err != nil {
		return http.Fail(c, 400, "invalid rate", err)
	}

	row := rate.New(rateDB(c))
	ok, err := row.Query().Filter("Slug=", slug).Get()
	if err != nil {
		return http.Fail(c, 500, "failed to read rate", err)
	}
	if !ok {
		return http.Fail(c, 404, "no rate at "+slug, errors.New("not found"))
	}

	assign(row, in)
	row.Bind()
	row.AdminEdited = true
	if err := row.Update(); err != nil {
		return http.Fail(c, 500, "failed to update rate", err)
	}
	return http.Render(c, 200, row)
}

// DeleteEntry removes a rate outright. ARCHIVING (status=archived) is usually
// what is wanted instead: a deleted row cannot price a historical charge, and a
// past invoice that has to re-resolve its rate then has nothing to read.
func DeleteEntry(c *zip.Ctx) error {
	if !requireSuperAdmin(c) {
		return nil
	}
	slug := strings.TrimSpace(c.Param("slug"))
	row := rate.New(rateDB(c))
	ok, err := row.Query().Filter("Slug=", slug).Get()
	if err != nil {
		return http.Fail(c, 500, "failed to read rate", err)
	}
	if !ok {
		return http.Fail(c, 404, "no rate at "+slug, errors.New("not found"))
	}
	if err := row.Delete(); err != nil {
		return http.Fail(c, 500, "failed to delete rate", err)
	}
	return http.Render(c, 200, map[string]any{"deleted": slug})
}

// ImportEntries loads a whole rate document in one call — the way 506 published
// prices get into the authority without any of them being compiled in.
//
// It is the SEED, driven from admin rather than from an embed, and it reconciles
// rather than replaces: a row that matches is left alone, a row that has drifted
// is corrected, a row an operator has edited is skipped. So importing the same
// document twice is a no-op, and importing a corrected one moves exactly the
// rows that changed.
func ImportEntries(c *zip.Ctx) error {
	if !requireSuperAdmin(c) {
		return nil
	}
	var rows []*rate.Rate
	if err := json.DecodeBytes(c.Body(), &rows); err != nil {
		return http.Fail(c, 400, "expected an array of rates", err)
	}
	if len(rows) == 0 {
		return http.Fail(c, 400, "no rates in the document", errors.New("empty import"))
	}

	created, corrected, err := rate.Seed(rateDB(c), rows)
	if err != nil {
		return http.Fail(c, 500, "import failed partway; rerun it — the reconcile is idempotent", err)
	}
	return http.Render(c, 200, map[string]any{
		"received":  len(rows),
		"created":   created,
		"corrected": corrected,
		// What was neither created nor corrected: already correct, or protected
		// by an operator's edit. Reported so an import that changes nothing reads
		// as "nothing to do" rather than as a failure.
		"unchanged": len(rows) - created - corrected,
	})
}

// assign copies the editable fields of a rate, leaving identity and bookkeeping
// (key, timestamps, AdminEdited) to the caller. Listing them explicitly is what
// stops a request body from writing a field it has no business setting.
func assign(dst, src *rate.Rate) {
	dst.Product = src.Product
	dst.Meter = src.Meter
	dst.Label = src.Label
	dst.Unit = src.Unit
	dst.Rate = src.Rate
	dst.Currency = src.Currency
	dst.Source = src.Source
	dst.Included = src.Included
	if src.Status != "" {
		dst.Status = src.Status
	}
}
