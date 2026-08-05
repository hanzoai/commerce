// Package draftorder wires the admin order-builder HTTP surface: CRUD on draft
// orders and their line items, plus the complete action that converts a draft
// into a REAL order (models/order) with the same items and total.
//
// Every handler is tenant-scoped via middleware.GetOrganization(c) +
// org.Namespaced(c.Context()) — the same isolation the rest of the /v1 surface
// uses — so a caller only ever touches its own org's drafts. Custom sub-routes
// pass the Namespace middleware explicitly since the REST layer only
// auto-namespaces default CRUD.
package draftorder

import (
	"errors"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	draftorderModel "github.com/hanzoai/commerce/models/draftorder"
	"github.com/hanzoai/commerce/models/draftorderitem"
	"github.com/hanzoai/commerce/models/lineitem"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/rest"
)

func Route(router zip.Router, args ...zip.Handler) {
	namespaced := middleware.Namespace()

	api := rest.New(draftorderModel.DraftOrder{})
	api.POST("/:draftorderid/complete", namespaced, Complete)
	api.GET("/:draftorderid/items", namespaced, ListItems)
	api.Route(router, args...)

	// Line items are a first-class sub-resource: the builder adds/removes them
	// with base CRUD (POST/DELETE /v1/draft-order-item), and reads them back
	// scoped to a draft via GET /v1/draft-order/:id/items above.
	rest.New(draftorderitem.DraftOrderItem{}).Route(router, args...)
}

// orgDB returns the caller's org-scoped datastore. Drafts and their items are
// created via the namespaced store, so every read/write here must bind the same
// per-org store (datastore.New binds the shared systemDB regardless of the
// namespace in context, which would miss the per-org rows — Red MED-1).
func orgDB(c *zip.Ctx) *datastore.Datastore {
	org := middleware.GetOrganization(c)
	return datastore.NewNamespaced(org.Namespaced(c.Context()))
}

// ListItems returns a draft's line items plus the projected running total. This
// is the read the line-item builder polls as the admin adds/removes lines.
func ListItems(c *zip.Ctx) error {
	db := orgDB(c)
	id := c.Param("draftorderid")

	d := draftorderModel.New(db)
	if err := d.GetById(id); err != nil {
		return http.Fail(c, 404, "No draft order found with id: "+id, err)
	}

	items, err := draftorderModel.Items(db, d.Id())
	if err != nil {
		return http.Fail(c, 500, "Failed to list draft items", err)
	}

	return http.Render(c, 200, map[string]any{
		"draftOrderId": d.Id(),
		"currency":     d.Currency,
		"items":        items,
		"totalCents":   draftorderModel.TotalCents(items),
	})
}

// Complete converts a draft order into a real order: it builds one order line
// per draft line (server-authoritative unit price), sets the order total to the
// projected draft total, creates the order, and marks the draft complete with a
// back-reference to the order it produced.
//
// Money action, so it is admin-gated INSIDE the handler (the route middleware
// no-ops on the IAM path). Idempotent: completing an already-completed draft
// returns its existing order rather than creating a second one.
func Complete(c *zip.Ctx) error {
	if !middleware.RequireAdmin(c) {
		return nil
	}
	db := orgDB(c)
	id := c.Param("draftorderid")

	d := draftorderModel.New(db)
	if err := d.GetById(id); err != nil {
		return http.Fail(c, 404, "No draft order found with id: "+id, err)
	}

	// Idempotent replay: a completed draft returns its already-created order.
	if !d.IsDraft() {
		if d.OrderId != "" {
			o := order.New(db)
			if err := o.GetById(d.OrderId); err == nil {
				return http.Render(c, 200, o)
			}
		}
		return http.Fail(c, 409, "Draft order is not in draft status: "+d.Status, errors.New("draft not in draft status"))
	}

	items, err := draftorderModel.Items(db, d.Id())
	if err != nil {
		return http.Fail(c, 500, "Failed to load draft items", err)
	}
	if len(items) == 0 {
		return http.Fail(c, 400, "Cannot complete a draft order with no line items", errors.New("draft has no items"))
	}

	o := order.New(db)
	o.UserId = d.CustomerId
	o.Email = d.Email
	o.Currency = draftCurrency(d.Currency)
	o.Status = order.Open
	for _, it := range items {
		li := lineitem.LineItem{
			ProductId:   it.ProductId,
			ProductName: it.ProductName,
			VariantId:   it.VariantId,
			VariantName: it.VariantName,
			Quantity:    it.Quantity,
		}
		// Price lives on the embedded ProductCachedValues; it is the unit price
		// the admin set on the draft line (server-authoritative).
		li.Price = it.UnitPriceCents
		o.Items = append(o.Items, li)
	}

	// Total is the projection over the draft lines — the order total MUST match
	// the draft total exactly.
	total := draftorderModel.TotalCents(items)
	o.LineTotal = total
	o.TaxableLineTotal = total
	o.Subtotal = total
	o.Total = total

	if err := o.Create(); err != nil {
		return http.Fail(c, 500, "Failed to create order from draft", err)
	}

	d.Status = draftorderModel.StatusComplete
	d.OrderId = o.Id()
	if err := d.Update(); err != nil {
		return http.Fail(c, 500, "Failed to mark draft order complete", err)
	}

	c.SetHeader("Location", "/v1/order/"+o.Id())
	return http.Render(c, 201, o)
}

// draftCurrency defaults a blank draft currency to USD so the produced order
// always carries a concrete currency.
func draftCurrency(cur currency.Type) currency.Type {
	if cur == "" {
		return currency.USD
	}
	return cur
}
