package billing

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/types/currency"
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
	// SubscriptionId and Metadata are carried by this endpoint only; the typed op has
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

// ListInvoices is the org's billing invoices, filtered — the QUERY, with no HTTP
// in it.
//
// It takes values rather than a request so a caller that is not a request can
// ask: the same list is read over the internal plane by a peer process that
// keeps no invoice store, and re-deriving this query there would be a second
// implementation of one question. Two invoice filters is how a billing page and
// a dunning run come to disagree about what is owed.
//
// Empty userID, status or subscriptionID means "do not filter on it", which is
// what an absent query parameter has always meant here.
//
// The list is paged: newest first, bounded by (limit, offset). A page that
// states no limit gets [PageParams] default, and no page — from this query or
// any other caller of it — ever returns more than the max, so one answer
// cannot grow with the org's whole billing history.
//
// The rows come back as the same view the HTTP endpoints render, so a peer reads the
// fields it already knows. The ENVELOPE around them is each endpoint's own; putting
// it here would make every future caller inherit one endpoint's shape.
func ListInvoices(ctx context.Context, org *organization.Organization, userID, status, subscriptionID string, limit, offset int) ([]Invoice, error) {
	if org == nil {
		return nil, fmt.Errorf("invoices: %w", errNoOrg)
	}
	limit, offset = PageParams(limit, offset)
	db := datastore.New(org.Namespaced(ctx))
	// Under no ancestor: an invoice saved again after it was loaded by id — every
	// paid one — is stored without the synckey parent it was created under.
	q := billinginvoice.Query(db)
	if userID != "" {
		q = q.Filter("UserId=", userID)
	}
	if status != "" {
		q = q.Filter("Status=", status)
	}
	if subscriptionID != "" {
		q = q.Filter("SubscriptionId=", subscriptionID)
	}
	q = q.Order("-CreatedAt").Limit(limit).Offset(offset)

	invoices := make([]*billinginvoice.BillingInvoice, 0)
	if _, err := q.GetAll(&invoices); err != nil {
		return nil, err
	}

	items := make([]Invoice, 0, len(invoices))
	for _, inv := range invoices {
		items = append(items, invoiceResponse(inv))
	}
	return items, nil
}

// Invoice page shape: one answer holds at most InvoicePageMax rows, and a
// missing limit is the default, not "everything".
const (
	InvoicePageDefault = 50
	InvoicePageMax     = 200
)

// PageParams normalizes a page request the way this listing answers it:
// an absent limit is the default, a limit past the max is the max, and a
// negative offset is zero. One rule, so the HTTP surface and every peer
// reader of ListInvoices cannot agree on two.
func PageParams(limit, offset int) (int, int) {
	switch {
	case limit <= 0:
		limit = InvoicePageDefault
	case limit > InvoicePageMax:
		limit = InvoicePageMax
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// PageCursor encodes the page a reader resumes from: the next row's offset,
// base64 (URL-safe, no padding) so it round-trips through a query string
// untouched. The cursor is opaque to the reader — a position, not a promise
// about how the rows are ordered.
func PageCursor(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}

// PageOffset decodes a [PageCursor]; an absent cursor is offset zero, and a
// malformed one is a refused page, not a guess.
func PageOffset(cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, fmt.Errorf("invoices: cursor does not decode: %w", err)
	}
	offset, err := strconv.Atoi(string(raw))
	if err != nil || offset < 0 {
		return 0, fmt.Errorf("invoices: cursor names no row position")
	}
	return offset, nil
}

// ListBillingInvoices lists billing invoices, filtered by userId/status/
// subscriptionId and paged: ?limit= (default 50, max 200) and ?cursor=
// (the page a previous answer returned — absent starts at the newest row).
//
//	GET /v1/billing/invoices?userId=...&status=...&limit=...&cursor=...
func ListBillingInvoices(c *zip.Ctx) error {
	// #146 class: never panic on a missing org. On the co-resident cloud embed path
	// this read can run with no "organization" local (IAMTokenRequired no-ops with no
	// gateway X-Org-Id) — GetOrganization would panic → 502. No org ⇒ honest empty.
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return c.JSON(200, map[string]any{"invoices": []map[string]any{}, "count": 0})
	}

	limit := 0
	if s := c.Query("limit"); s != "" {
		limit, _ = strconv.Atoi(s) // a malformed limit is the default, not a 500
	}
	offset, err := PageOffset(c.Query("cursor"))
	if err != nil {
		return http.Fail(c, 400, "failed to list invoices", err)
	}

	items, err := ListInvoices(c.Context(), org,
		strings.TrimSpace(c.Query("userId")),
		strings.TrimSpace(c.Query("status")),
		strings.TrimSpace(c.Query("subscriptionId")),
		limit, offset)
	if err != nil {
		log.Error("Failed to list invoices: %v", err, c)
		return http.Fail(c, 500, "failed to list invoices", err)
	}

	resp := map[string]any{"invoices": items, "count": len(items)}
	// A full page is the only shape that may hold a next page.
	if eff, _ := PageParams(limit, offset); len(items) == eff {
		resp["cursor"] = PageCursor(offset + eff)
	}
	return c.JSON(200, resp)
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

// Invoice is a billing invoice as every invoice endpoint has answered with it —
// InvoiceView's facts plus the ones only the endpoints carry (the period, the tax
// and discount lines, the dunning attempt count).
//
// The three lifecycle timestamps are pointers: each key has always been emitted
// only once its moment has happened, and a due date of the zero time is a
// different claim from an invoice that is not yet due.
type Invoice struct {
	ID             string                    `json:"id"`
	UserID         string                    `json:"userId"`
	CustomerEmail  string                    `json:"customerEmail"`
	SubscriptionID string                    `json:"subscriptionId"`
	PeriodStart    time.Time                 `json:"periodStart"`
	PeriodEnd      time.Time                 `json:"periodEnd"`
	Subtotal       int64                     `json:"subtotal"`
	Tax            int64                     `json:"tax"`
	Discount       int64                     `json:"discount"`
	CreditApplied  int64                     `json:"creditApplied"`
	AmountDue      int64                     `json:"amountDue"`
	AmountPaid     int64                     `json:"amountPaid"`
	Currency       currency.Type             `json:"currency"`
	Status         billinginvoice.Status     `json:"status"`
	PaymentMethod  string                    `json:"paymentMethod"`
	PaymentRef     string                    `json:"paymentRef"`
	Number         int                       `json:"number"`
	NumberStr      string                    `json:"numberStr"`
	AttemptCount   int                       `json:"attemptCount"`
	LineItems      []billinginvoice.LineItem `json:"lineItems"`
	CreatedAt      time.Time                 `json:"createdAt"`
	UpdatedAt      time.Time                 `json:"updatedAt"`
	DueDate        *time.Time                `json:"dueDate,omitempty"`
	PaidAt         *time.Time                `json:"paidAt,omitempty"`
	VoidedAt       *time.Time                `json:"voidedAt,omitempty"`
}

// invoiceResponse projects the model onto that view. It stays the ONE projection
// the endpoints share, now typed, so the list a peer reads and the body a browser
// reads cannot drift into describing one invoice two ways.
func invoiceResponse(inv *billinginvoice.BillingInvoice) Invoice {
	resp := Invoice{
		ID:             inv.Id(),
		UserID:         inv.UserId,
		CustomerEmail:  inv.CustomerEmail,
		SubscriptionID: inv.SubscriptionId,
		PeriodStart:    inv.PeriodStart,
		PeriodEnd:      inv.PeriodEnd,
		Subtotal:       inv.Subtotal,
		Tax:            inv.Tax,
		Discount:       inv.Discount,
		CreditApplied:  inv.CreditApplied,
		AmountDue:      inv.AmountDue,
		AmountPaid:     inv.AmountPaid,
		Currency:       inv.Currency,
		Status:         inv.Status,
		PaymentMethod:  inv.PaymentMethod,
		PaymentRef:     inv.PaymentRef,
		Number:         inv.Number,
		NumberStr:      inv.NumberStr,
		AttemptCount:   inv.AttemptCount,
		LineItems:      inv.LineItems,
		CreatedAt:      inv.CreatedAt,
		UpdatedAt:      inv.UpdatedAt,
	}

	if !inv.DueDate.IsZero() {
		due := inv.DueDate
		resp.DueDate = &due
	}
	if !inv.PaidAt.IsZero() {
		paid := inv.PaidAt
		resp.PaidAt = &paid
	}
	if !inv.VoidedAt.IsZero() {
		voided := inv.VoidedAt
		resp.VoidedAt = &voided
	}

	return resp
}
