// Package b2b wires the B2B commerce HTTP surface: companies, employees (with
// their spending headroom), quotes (accept/reject + message thread), and
// spend-approvals. Every handler is tenant-scoped via the caller's
// organization namespace; the REST layer auto-namespaces the default CRUD
// routes and each custom sub-route passes the Namespace middleware explicitly.
package b2b

import (
	"errors"
	"strconv"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/approval"
	"github.com/hanzoai/commerce/models/company"
	"github.com/hanzoai/commerce/models/employee"
	"github.com/hanzoai/commerce/models/quote"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/rest"
)

func Route(router zip.Router, args ...zip.Handler) {
	namespaced := middleware.Namespace()

	rest.New(company.Company{}).Route(router, args...)

	employees := rest.New(employee.Employee{})
	employees.GET("/:employeeid/spending", namespaced, EmployeeSpending)
	employees.Route(router, args...)

	quotes := rest.New(quote.Quote{})
	quotes.POST("/:quoteid/accept", namespaced, AcceptQuote)
	quotes.POST("/:quoteid/reject", namespaced, RejectQuote)
	quotes.POST("/:quoteid/messages", namespaced, AddQuoteMessage)
	quotes.GET("/:quoteid/messages", namespaced, ListQuoteMessages)
	quotes.Route(router, args...)

	rest.New(quote.QuoteMessage{}).Route(router, args...)

	approvals := rest.New(approval.Approval{})
	approvals.POST("/:approvalid/approve", namespaced, ApproveApproval)
	approvals.POST("/:approvalid/reject", namespaced, RejectApproval)
	approvals.Route(router, args...)
}

// EmployeeSpending returns an employee's remaining spend headroom. The amount
// already committed is supplied by the caller (the cart/order in flight) via
// the `committed` query param in cents, defaulting to 0.
func EmployeeSpending(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.NewNamespaced(org.Namespaced(c.Context()))

	emp := employee.New(db)
	if err := emp.GetById(c.Param("employeeid")); err != nil {
		return http.Fail(c, 404, "No employee found with id: "+c.Param("employeeid"), err)
	}

	var committed currency.Cents
	if raw := c.Query("committed"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return http.Fail(c, 400, "committed must be an integer number of cents", err)
		}
		committed = currency.Cents(n)
	}

	return http.Render(c, 200, map[string]any{
		"employeeId":     emp.Id(),
		"limitCents":     emp.SpendingLimitCents,
		"committedCents": committed,
		"remainingCents": employee.RemainingSpendCents(committed, emp.SpendingLimitCents),
		"withinLimit":    employee.WithinLimit(committed, 0, emp.SpendingLimitCents),
	})
}

// AcceptQuote transitions a pending quote to accepted. Accepting a quote commits
// the org to a negotiated price, so it is admin-only — enforced inside the
// handler because the route middleware no-ops on the IAM path (Red HIGH-4).
func AcceptQuote(c *zip.Ctx) error {
	if !middleware.RequireAdmin(c) {
		return nil
	}
	org := middleware.GetOrganization(c)
	db := datastore.NewNamespaced(org.Namespaced(c.Context()))

	q := quote.New(db)
	if err := q.GetById(c.Param("quoteid")); err != nil {
		return http.Fail(c, 404, "No quote found with id: "+c.Param("quoteid"), err)
	}

	if !q.IsActionable() {
		return http.Fail(c, 409, "Quote is not actionable in status: "+q.Status, errors.New("quote not actionable"))
	}

	q.Status = quote.StatusAccepted
	if err := q.Update(); err != nil {
		return http.Fail(c, 500, "Failed to accept quote", err)
	}

	return http.Render(c, 200, q)
}

// rejectQuoteRequest optionally names who rejected: customer (default) or
// merchant, selecting the corresponding rejected status.
type rejectQuoteRequest struct {
	By string `json:"by"`
}

// RejectQuote transitions a pending quote to a rejected state. Admin-only (a
// quote state decision) — gated inside the handler like AcceptQuote.
func RejectQuote(c *zip.Ctx) error {
	if !middleware.RequireAdmin(c) {
		return nil
	}
	org := middleware.GetOrganization(c)
	db := datastore.NewNamespaced(org.Namespaced(c.Context()))

	q := quote.New(db)
	if err := q.GetById(c.Param("quoteid")); err != nil {
		return http.Fail(c, 404, "No quote found with id: "+c.Param("quoteid"), err)
	}

	if !q.IsActionable() {
		return http.Fail(c, 409, "Quote is not actionable in status: "+q.Status, errors.New("quote not actionable"))
	}

	var req rejectQuoteRequest
	// Body is optional; ignore decode errors on an empty body.
	_ = json.DecodeBytes(c.Body(), &req)

	status := quote.StatusCustomerRejected
	switch req.By {
	case "", quote.AuthorCustomer:
		status = quote.StatusCustomerRejected
	case quote.AuthorMerchant:
		status = quote.StatusMerchantRejected
	default:
		return http.Fail(c, 400, "by must be customer or merchant", errors.New("invalid rejecter"))
	}

	q.Status = status
	if err := q.Update(); err != nil {
		return http.Fail(c, 500, "Failed to reject quote", err)
	}

	return http.Render(c, 200, q)
}

// addQuoteMessageRequest is the body for appending a message to a quote thread.
type addQuoteMessageRequest struct {
	AuthorId   string `json:"authorId"`
	AuthorType string `json:"authorType"`
	Text       string `json:"text"`
	ItemId     string `json:"itemId"`
}

// AddQuoteMessage appends a message to a quote's negotiation thread.
func AddQuoteMessage(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.NewNamespaced(org.Namespaced(c.Context()))

	quoteId := c.Param("quoteid")

	q := quote.New(db)
	if err := q.GetById(quoteId); err != nil {
		return http.Fail(c, 404, "No quote found with id: "+quoteId, err)
	}

	var req addQuoteMessageRequest
	if err := json.DecodeBytes(c.Body(), &req); err != nil {
		return http.Fail(c, 400, "Failed decode request body", err)
	}

	if req.Text == "" {
		return http.Fail(c, 400, "text is required", errors.New("missing text"))
	}
	switch req.AuthorType {
	case quote.AuthorCustomer, quote.AuthorMerchant:
	default:
		return http.Fail(c, 400, "authorType must be customer or merchant", errors.New("invalid authorType"))
	}

	msg := quote.NewMessage(db)
	msg.QuoteId = q.Id()
	msg.AuthorId = req.AuthorId
	msg.AuthorType = req.AuthorType
	msg.Text = req.Text
	msg.ItemId = req.ItemId

	if err := msg.Create(); err != nil {
		return http.Fail(c, 500, "Failed to create quote message", err)
	}

	return http.Render(c, 201, msg)
}

// ListQuoteMessages lists the messages on a quote's thread.
func ListQuoteMessages(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.NewNamespaced(org.Namespaced(c.Context()))

	quoteId := c.Param("quoteid")

	q := quote.New(db)
	if err := q.GetById(quoteId); err != nil {
		return http.Fail(c, 404, "No quote found with id: "+quoteId, err)
	}

	var messages []*quote.QuoteMessage
	if _, err := quote.QueryMessages(db).Filter("QuoteId=", q.Id()).GetAll(&messages); err != nil {
		return http.Fail(c, 500, "Failed to list quote messages", err)
	}

	return http.Render(c, 200, messages)
}

// resolveApprovalRequest optionally names the resolver, recorded on HandledBy.
type resolveApprovalRequest struct {
	HandledBy string `json:"handledBy"`
}

// ApproveApproval approves a pending approval.
func ApproveApproval(c *zip.Ctx) error {
	return resolveApproval(c, approval.ActionApprove)
}

// RejectApproval rejects a pending approval.
func RejectApproval(c *zip.Ctx) error {
	return resolveApproval(c, approval.ActionReject)
}

// resolveApproval applies an approve/reject action to an approval, enforcing
// the pending-only transition via approval.NextStatus. Admin-only: an approval
// authorizes spend, so both ApproveApproval and RejectApproval gate here inside
// the handler (the route middleware no-ops on the IAM path — Red HIGH-4).
func resolveApproval(c *zip.Ctx, action string) error {
	if !middleware.RequireAdmin(c) {
		return nil
	}
	org := middleware.GetOrganization(c)
	db := datastore.NewNamespaced(org.Namespaced(c.Context()))

	id := c.Param("approvalid")

	a := approval.New(db)
	if err := a.GetById(id); err != nil {
		return http.Fail(c, 404, "No approval found with id: "+id, err)
	}

	next, err := approval.NextStatus(a.Status, action)
	if err != nil {
		return http.Fail(c, 409, err.Error(), err)
	}

	var req resolveApprovalRequest
	_ = json.DecodeBytes(c.Body(), &req)

	a.Status = next
	a.HandledBy = req.HandledBy
	if err := a.Update(); err != nil {
		return http.Fail(c, 500, "Failed to update approval", err)
	}

	return http.Render(c, 200, a)
}
