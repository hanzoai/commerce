// Copyright © 2026 Hanzo AI. MIT License.

// The platform-admin CRUD over the rate authority — what one unit of anything
// costs, edited at admin.hanzo.ai instead of in a Go file.
//
// WHAT THESE ROWS PRICE, and what they do not. The authority covers the
// platform's own metered work — see api/rate/catalog.go for the seeded set. Two
// products are deliberately outside it because each already has an authority
// that is read first:
//
//	a MODEL is priced by its ModelRoute row (InputPrice/OutputPrice), resolved
//	org-first and read ahead of config and the compiled table;
//	a TOOL is priced by its marketplace listing, set by the publisher who is paid.
//
// An earlier version of this comment claimed these rows were already the price
// of 506 items and that nothing was compiled in. Neither was true — nothing read
// them at all — and a file that describes an authority it does not yet have is
// how the next reader comes to trust a number that is not being charged.
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

// SuperAdmin refuses anyone who is not the platform.
//
// It is bound ON THE GROUP rather than repeated at the top of each handler. Five
// copies of one sentence is five chances to add a sixth handler and forget it,
// and forgetting it here opens every customer's price to any org's own admin. On
// the group it is a property of the ADDRESS: a route registered under /rates
// carries it whether or not its author thought about it.
//
// It is checked SEPARATELY from the bundle's own middleware, which proves a
// caller is authenticated and never that they are the platform.
func SuperAdmin(c *zip.Ctx) error {
	claims := iammiddleware.GetIAMClaims(c)
	if claims == nil || !claims.IsSuperAdmin() {
		return http.Fail(c, 403, "platform admin required to edit rates", errors.New("not a platform admin"))
	}
	return c.Next()
}

// AdminRoute wires the rate CRUD and the bulk import on the /v1 bundle. args
// carry the bundle's own middleware; SuperAdmin gates the whole group.
func AdminRoute(r zip.Router, args ...zip.Handler) {
	g := r.Group("/rates", append(args, SuperAdmin)...)
	g.Get("/entries", ListEntries)
	g.Post("/entries", CreateEntry)
	// ADDRESSED BY THE PARTS, because the identity IS the parts: a slug is
	// product + "/" + meter, so it carries a slash and can never be one path
	// segment. Mounted at ":slug" these two matched nothing that exists — every
	// real rate 404'd, on both verbs, for every caller.
	g.Put("/entries/:product/:meter", UpdateEntry)
	g.Delete("/entries/:product/:meter", DeleteEntry)
	g.Post("/import", ImportEntries)
}

// ListEntries returns the authority rows. Optionally narrowed by ?product= so
// an editor can show one surface at a time rather than every rate at once.
func ListEntries(c *zip.Ctx) error {
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
	row.Take(in)
	// Created HERE is created by a person, so it is theirs from the start and no
	// later import may quietly replace it.
	row.AdminEdited = true
	if err := row.Create(); err != nil {
		return http.Fail(c, 500, "failed to create rate", err)
	}
	return http.Render(c, 201, row)
}

// addressed is the rate this request names, read from the two path parts and
// bound the same way a write binds. ONE reader, so update and delete cannot
// disagree about what "the rate at this address" means.
func addressed(c *zip.Ctx) (string, bool) {
	product := strings.TrimSpace(c.Param("product"))
	meter := strings.TrimSpace(c.Param("meter"))
	if product == "" || meter == "" {
		return "", false
	}
	r := &rate.Rate{Product: product, Meter: meter}
	r.Bind()
	return r.Slug, true
}

// UpdateEntry edits a rate and MARKS IT as edited. That mark is the whole
// contract with the importer: an operator's price outranks the document it was
// imported from, so a later import leaves this row alone. Without it, a price
// set here would apply, work, and silently revert on the next import.
func UpdateEntry(c *zip.Ctx) error {
	slug, ok := addressed(c)
	if !ok {
		return http.Fail(c, 400, "product and meter are required", errors.New("no identity"))
	}
	in := new(rate.Rate)
	if err := json.DecodeBytes(c.Body(), in); err != nil {
		return http.Fail(c, 400, "invalid rate", err)
	}

	row := rate.New(rateDB(c))
	found, err := row.Query().Filter("Slug=", slug).Get()
	if err != nil {
		return http.Fail(c, 500, "failed to read rate", err)
	}
	if !found {
		return http.Fail(c, 404, "no rate at "+slug, errors.New("not found"))
	}

	row.Take(in)
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
	slug, ok := addressed(c)
	if !ok {
		return http.Fail(c, 400, "product and meter are required", errors.New("no identity"))
	}
	row := rate.New(rateDB(c))
	found, err := row.Query().Filter("Slug=", slug).Get()
	if err != nil {
		return http.Fail(c, 500, "failed to read rate", err)
	}
	if !found {
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
