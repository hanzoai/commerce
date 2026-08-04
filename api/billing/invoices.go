package billing

import (
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/util/json/http"
)

type createInvoiceRequest struct {
	UserId         string                    `json:"userId"`
	CustomerEmail  string                    `json:"customerEmail"`
	SubscriptionId string                    `json:"subscriptionId"`
	Currency       string                    `json:"currency"`
	LineItems      []billinginvoice.LineItem `json:"lineItems"`
	Metadata       map[string]interface{}    `json:"metadata"`
}

// CreateInvoice creates a new draft billing invoice.
//
//	POST /v1/billing/invoices
func CreateInvoice(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)

	var req createInvoiceRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}

	in := InvoiceIn{
		UserID:        req.UserId,
		CustomerEmail: req.CustomerEmail,
		Currency:      req.Currency,
	}
	for _, li := range req.LineItems {
		in.Lines = append(in.Lines, InvoiceLine{
			Description: li.Description,
			Amount:      li.Amount,
			Quantity:    li.Quantity,
			UnitPrice:   li.UnitPrice,
		})
	}
	inv, f := raiseInvoice(c.Context(), org, in)
	if f != nil {
		return http.Fail(c, f.Status, f.Message, f.Err)
	}
	// SubscriptionId and Metadata are carried by this door only; the typed op has
	// no field for either, so they are set after the shared core has built the row.
	if req.SubscriptionId != "" || req.Metadata != nil {
		inv.SubscriptionId = req.SubscriptionId
		if req.Metadata != nil {
			inv.Metadata = req.Metadata
		}
		if err := inv.Update(); err != nil {
			log.Error("Failed to attach subscription/metadata to invoice: %v", err, c)
		}
	}
	return c.JSON(201, invoiceResponse(inv))
}

// ListInvoices lists billing invoices, optionally filtered by userId and status.
//
//	GET /v1/billing/invoices?userId=...&status=...
func ListInvoices(c *zip.Ctx) error {
	// #146 class: never panic on a missing org. On the co-resident cloud embed path
	// this read can run with no "organization" local (IAMTokenRequired no-ops with no
	// gateway X-Org-Id) — GetOrganization would panic → 502. No org ⇒ honest empty.
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return c.JSON(200, map[string]any{"invoices": []map[string]any{}, "count": 0})
	}
	db := datastore.New(org.Namespaced(c.Context()))

	rootKey := db.NewKey("synckey", "", 1, nil)
	invoices := make([]*billinginvoice.BillingInvoice, 0)
	q := billinginvoice.Query(db).Ancestor(rootKey)

	userId := strings.TrimSpace(c.Query("userId"))
	if userId != "" {
		q = q.Filter("UserId=", userId)
	}

	status := strings.TrimSpace(c.Query("status"))
	if status != "" {
		q = q.Filter("Status=", status)
	}

	subId := strings.TrimSpace(c.Query("subscriptionId"))
	if subId != "" {
		q = q.Filter("SubscriptionId=", subId)
	}

	if _, err := q.GetAll(&invoices); err != nil {
		log.Error("Failed to list invoices: %v", err, c)
		return http.Fail(c, 500, "failed to list invoices", err)
	}

	items := make([]map[string]any, 0, len(invoices))
	for _, inv := range invoices {
		items = append(items, invoiceResponse(inv))
	}

	return c.JSON(200, map[string]any{
		"invoices": items,
		"count":    len(items),
	})
}

// GetInvoice returns a single billing invoice by ID.
//
//	GET /v1/billing/invoices/:id
func GetInvoice(c *zip.Ctx) error {
	inv, _, f := loadInvoice(c.Context(), middleware.GetOrganization(c), c.Param("id"))
	if f != nil {
		return http.Fail(c, f.Status, f.Message, f.Err)
	}
	return c.JSON(200, invoiceResponse(inv))
}

// FinalizeInvoice transitions an invoice from draft to open.
//
//	POST /v1/billing/invoices/:id/finalize
func FinalizeInvoice(c *zip.Ctx) error {
	inv, f := issueInvoice(c.Context(), middleware.GetOrganization(c), c.Param("id"), eventsOf(c))
	if f != nil {
		return http.Fail(c, f.Status, f.Message, f.Err)
	}
	return c.JSON(200, invoiceResponse(inv))
}

// PayInvoice attempts to collect payment on an open invoice.
//
//	POST /v1/billing/invoices/:id/pay
func PayInvoice(c *zip.Ctx) error {
	oc, f := collectInvoice(c.Context(), middleware.GetOrganization(c), c.Param("id"), kmsOf(c), eventsOf(c))
	if f != nil {
		return http.Fail(c, f.Status, f.Message, f.Err)
	}
	if oc.Replayed != nil {
		return c.JSON(200, map[string]any{
			"invoice":    invoiceResponse(oc.Inv),
			"collection": oc.Replayed,
		})
	}
	return c.JSON(200, map[string]any{
		"invoice":    invoiceResponse(oc.Inv),
		"collection": oc.Result,
	})
}

// VoidInvoice voids a draft or open invoice.
//
//	POST /v1/billing/invoices/:id/void
func VoidInvoice(c *zip.Ctx) error {
	inv, f := voidInvoice(c.Context(), middleware.GetOrganization(c), c.Param("id"), eventsOf(c))
	if f != nil {
		return http.Fail(c, f.Status, f.Message, f.Err)
	}
	return c.JSON(200, invoiceResponse(inv))
}

// UpcomingInvoice generates a preview of the next invoice for a subscription.
//
//	GET /v1/billing/invoices/upcoming?userId=...&subscriptionId=...
func UpcomingInvoice(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	userId := strings.TrimSpace(c.Query("userId"))
	if userId == "" {
		return http.Fail(c, 400, "userId query parameter is required", nil)
	}

	// Aggregate current usage for this user
	lineItems, subtotal, err := engine.AggregateUsage(db, userId, time.Time{}, time.Time{})
	if err != nil {
		log.Error("Failed to aggregate usage: %v", err, c)
		return http.Fail(c, 500, "failed to aggregate usage", err)
	}

	// Apply credit burn-down for preview
	creditApplied := int64(0)
	amountDue := subtotal

	if subtotal > 0 {
		remaining, err := BurnCreditsPreview(db, userId, subtotal)
		if err == nil {
			creditApplied = subtotal - remaining
			amountDue = remaining
		}
	}

	return c.JSON(200, map[string]any{
		"userId":        userId,
		"lineItems":     lineItems,
		"subtotal":      subtotal,
		"creditApplied": creditApplied,
		"amountDue":     amountDue,
		"currency":      "usd",
	})
}

// BurnCreditsPreview calculates credit burn without actually deducting.
// Returns the remaining amount after credits would be applied.
func BurnCreditsPreview(db *datastore.Datastore, userId string, amount int64) (int64, error) {
	grants, err := getActiveGrants(db, userId)
	if err != nil {
		return amount, err
	}

	remaining := amount
	for _, g := range grants {
		if remaining <= 0 {
			break
		}
		deduct := g.RemainingCents
		if deduct > remaining {
			deduct = remaining
		}
		remaining -= deduct
	}

	return remaining, nil
}

func invoiceResponse(inv *billinginvoice.BillingInvoice) map[string]any {
	resp := map[string]any{
		"id":             inv.Id(),
		"userId":         inv.UserId,
		"customerEmail":  inv.CustomerEmail,
		"subscriptionId": inv.SubscriptionId,
		"periodStart":    inv.PeriodStart,
		"periodEnd":      inv.PeriodEnd,
		"subtotal":       inv.Subtotal,
		"tax":            inv.Tax,
		"discount":       inv.Discount,
		"creditApplied":  inv.CreditApplied,
		"amountDue":      inv.AmountDue,
		"amountPaid":     inv.AmountPaid,
		"currency":       inv.Currency,
		"status":         inv.Status,
		"paymentMethod":  inv.PaymentMethod,
		"paymentRef":     inv.PaymentRef,
		"number":         inv.Number,
		"numberStr":      inv.NumberStr,
		"attemptCount":   inv.AttemptCount,
		"lineItems":      inv.LineItems,
		"createdAt":      inv.CreatedAt,
		"updatedAt":      inv.UpdatedAt,
	}

	if !inv.DueDate.IsZero() {
		resp["dueDate"] = inv.DueDate
	}
	if !inv.PaidAt.IsZero() {
		resp["paidAt"] = inv.PaidAt
	}
	if !inv.VoidedAt.IsZero() {
		resp["voidedAt"] = inv.VoidedAt
	}

	return resp
}
