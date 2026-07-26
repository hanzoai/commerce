// Package claim wires the order-claim HTTP surface: CRUD on claims (with their
// claimed lines) plus the accept / reject decisions. A claim is a customer's
// report that delivered items were damaged / wrong / missing; the merchant
// accepts it — settling with a refund or a replacement order — or rejects it.
//
// Every handler is tenant-scoped via the caller's organization namespace: the
// custom sub-routes pass the Namespace middleware explicitly and read/write the
// per-org store (datastore.NewNamespaced), the same isolation the rest of /v1
// uses. Accept moves money, so it is admin-gated INSIDE the handler (the route
// middleware no-ops on the IAM path) and is idempotent: an already-accepted
// claim returns its prior outcome without refunding twice.
package claim

import (
	"errors"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	claimModel "github.com/hanzoai/commerce/models/claim"
	"github.com/hanzoai/commerce/models/claimitem"
	"github.com/hanzoai/commerce/models/lineitem"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/refund"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/rest"
)

func Route(router zip.Router, args ...zip.Handler) {
	namespaced := middleware.Namespace()

	api := rest.New(claimModel.Claim{})
	api.Create = Create
	api.POST("/:claimid/accept", namespaced, Accept)
	api.POST("/:claimid/reject", namespaced, Reject)
	api.GET("/:claimid/items", namespaced, Items)
	api.Route(router, args...)
}

// itemRequest is one claimed line in a create request.
type itemRequest struct {
	ItemId   string `json:"itemId"`
	Quantity int    `json:"quantity"`
	Reason   string `json:"reason"`
}

// createRequest is the body of POST /claim: the claim fields plus its lines.
type createRequest struct {
	OrderId      string        `json:"orderId"`
	Resolution   string        `json:"resolution"`
	Reason       string        `json:"reason"`
	CurrencyCode currency.Type `json:"currencyCode"`
	Items        []itemRequest `json:"items"`
}

// Create files a claim against an order and persists its claimed lines as
// claimitem rows. It validates the order exists (in the caller's org), the
// resolution type, and that every line names a valid reason and a positive
// quantity — but defers the "can't claim more than ordered" money check to
// accept, where it is enforced against the live order.
func Create(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.NewNamespaced(org.Namespaced(c.Context()))

	var req createRequest
	if err := json.DecodeBytes(c.Body(), &req); err != nil {
		return http.Fail(c, 400, "Failed to decode request body", err)
	}

	req.OrderId = strings.TrimSpace(req.OrderId)
	if req.OrderId == "" {
		return http.Fail(c, 400, "orderId is required", errors.New("missing orderId"))
	}
	if req.Resolution == "" {
		req.Resolution = claimModel.ResolutionRefund
	}
	if !claimModel.ValidResolution(req.Resolution) {
		return http.Fail(c, 400, "resolution must be refund or replace", errors.New("invalid resolution"))
	}
	if len(req.Items) == 0 {
		return http.Fail(c, 400, "at least one claim item is required", errors.New("no items"))
	}
	for _, it := range req.Items {
		if strings.TrimSpace(it.ItemId) == "" {
			return http.Fail(c, 400, "each item requires an itemId", errors.New("missing itemId"))
		}
		if it.Quantity < 1 {
			return http.Fail(c, 400, "each item quantity must be positive", errors.New("non-positive quantity"))
		}
		if !claimitem.ValidReason(it.Reason) {
			return http.Fail(c, 400, "each item reason must be damaged|wrong_item|missing|other", errors.New("invalid reason"))
		}
	}

	// The order must exist in the caller's org (tenant isolation + integrity).
	ord := order.New(db)
	if err := ord.GetById(req.OrderId); err != nil {
		return http.Fail(c, 404, "No order found with id: "+req.OrderId, err)
	}

	cur := req.CurrencyCode
	if cur == "" {
		cur = ord.Currency
	}
	if cur == "" {
		cur = "usd"
	}

	cl := claimModel.New(db)
	cl.OrderId = req.OrderId
	cl.Resolution = req.Resolution
	cl.Status = claimModel.StatusPending
	cl.Reason = req.Reason
	cl.CurrencyCode = cur
	if err := cl.Create(); err != nil {
		return http.Fail(c, 500, "Failed to create claim", err)
	}

	for _, it := range req.Items {
		item := claimitem.New(db)
		item.ClaimId = cl.Id()
		item.ItemId = it.ItemId
		item.Quantity = it.Quantity
		item.Reason = it.Reason
		if err := item.Create(); err != nil {
			return http.Fail(c, 500, "Failed to create claim item", err)
		}
	}

	return http.Render(c, 201, cl)
}

// Items returns the claimed lines for a claim.
func Items(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.NewNamespaced(org.Namespaced(c.Context()))

	id := c.Param("claimid")
	cl := claimModel.New(db)
	if err := cl.GetById(id); err != nil {
		return http.Fail(c, 404, "No claim found with id: "+id, err)
	}

	items, err := loadItems(db, cl.Id())
	if err != nil {
		return http.Fail(c, 500, "Failed to load claim items", err)
	}
	return http.Render(c, 200, items)
}

// loadItems returns the claimitem rows for a claim.
func loadItems(db *datastore.Datastore, claimId string) ([]*claimitem.ClaimItem, error) {
	items := make([]*claimitem.ClaimItem, 0, 8)
	if _, err := claimitem.Query(db).Filter("ClaimId=", claimId).GetAll(&items); err != nil {
		return nil, err
	}
	return items, nil
}

// acceptResponse is the settled claim plus the settlement it produced.
type acceptResponse struct {
	Claim              *claimModel.Claim `json:"claim"`
	AmountCents        currency.Cents    `json:"amountCents"`
	RefundId           string            `json:"refundId,omitempty"`
	ReplacementOrderId string            `json:"replacementOrderId,omitempty"`
}

// Accept settles a pending claim. Money move ⇒ admin-gated inside the handler.
// Idempotent: an already-accepted claim returns its prior outcome without
// refunding or building a replacement again. The settled amount is computed from
// the claimed quantities × the order's line prices; claiming more units than
// were ordered is rejected (422) before any money moves.
func Accept(c *zip.Ctx) error {
	if !middleware.RequireAdmin(c) {
		return nil // RequireAdmin already wrote the 403
	}

	org := middleware.GetOrganization(c)
	db := datastore.NewNamespaced(org.Namespaced(c.Context()))

	id := c.Param("claimid")
	cl := claimModel.New(db)
	if err := cl.GetById(id); err != nil {
		return http.Fail(c, 404, "No claim found with id: "+id, err)
	}

	// Idempotent replay: an accepted claim returns its recorded outcome. No
	// second refund, no second replacement order.
	if cl.Status == claimModel.StatusAccepted {
		return http.Render(c, 200, acceptResponse{
			Claim: cl, AmountCents: cl.AmountCents,
			RefundId: cl.RefundId, ReplacementOrderId: cl.ReplacementOrderId,
		})
	}
	if cl.Status != claimModel.StatusPending {
		return http.Fail(c, 409, "Cannot accept claim in status: "+cl.Status, errors.New("claim not pending"))
	}

	ord := order.New(db)
	if err := ord.GetById(cl.OrderId); err != nil {
		return http.Fail(c, 404, "No order found with id: "+cl.OrderId, err)
	}

	items, err := loadItems(db, cl.Id())
	if err != nil {
		return http.Fail(c, 500, "Failed to load claim items", err)
	}
	if len(items) == 0 {
		return http.Fail(c, 400, "claim has no items", errors.New("empty claim"))
	}

	// Compute the settled amount from the order's line prices. A claimed line
	// must reference a real order line and cannot claim more units than ordered.
	amount, err := settledAmount(ord, items)
	if err != nil {
		return http.Fail(c, 422, err.Error(), err)
	}

	switch cl.Resolution {
	case claimModel.ResolutionRefund:
		// Ledger-level over-refund guard: never refund past the order total.
		if ord.Refunded+amount > ord.Total {
			return http.Fail(c, 422, "refund would exceed the order total", errors.New("over-refund"))
		}
		r := refund.New(db)
		r.Amount = int64(amount)
		r.Currency = cl.CurrencyCode
		r.Status = refund.Succeeded
		r.Reason = "claim:" + cl.Id()
		if err := r.Create(); err != nil {
			return http.Fail(c, 500, "Failed to create refund", err)
		}
		ord.Refunded += amount
		if err := ord.Update(); err != nil {
			return http.Fail(c, 500, "Failed to update order", err)
		}
		cl.RefundId = r.Id()

	case claimModel.ResolutionReplace:
		repl, err := buildReplacement(db, ord, items)
		if err != nil {
			return http.Fail(c, 500, "Failed to create replacement order", err)
		}
		cl.ReplacementOrderId = repl.Id()
	}

	cl.AmountCents = amount
	cl.Status = claimModel.StatusAccepted
	if err := cl.Update(); err != nil {
		return http.Fail(c, 500, "Failed to update claim", err)
	}

	return http.Render(c, 200, acceptResponse{
		Claim: cl, AmountCents: cl.AmountCents,
		RefundId: cl.RefundId, ReplacementOrderId: cl.ReplacementOrderId,
	})
}

// settledAmount sums (order line price × claimed quantity) over the claim's
// lines. It errors if a line references no order line, or claims more units than
// were ordered.
func settledAmount(ord *order.Order, items []*claimitem.ClaimItem) (currency.Cents, error) {
	byId := make(map[string]lineitem.LineItem, len(ord.Items))
	for _, li := range ord.Items {
		byId[li.Id()] = li
	}
	var total currency.Cents
	for _, it := range items {
		li, ok := byId[it.ItemId]
		if !ok {
			return 0, errors.New("claimed item is not on the order: " + it.ItemId)
		}
		if it.Quantity > li.Quantity {
			return 0, errors.New("cannot claim more than ordered for item: " + it.ItemId)
		}
		total += li.Price * currency.Cents(it.Quantity)
	}
	return total, nil
}

// buildReplacement creates a new open order carrying the claimed lines at their
// claimed quantities, linked back to the original order and claim.
func buildReplacement(db *datastore.Datastore, ord *order.Order, items []*claimitem.ClaimItem) (*order.Order, error) {
	byId := make(map[string]lineitem.LineItem, len(ord.Items))
	for _, li := range ord.Items {
		byId[li.Id()] = li
	}
	repl := order.New(db)
	repl.Currency = ord.Currency
	repl.UserId = ord.UserId
	repl.Email = ord.Email
	repl.StoreId = ord.StoreId
	repl.Status = order.Open
	repl.Metadata["replacesOrderId"] = ord.Id()
	for _, it := range items {
		li := byId[it.ItemId]
		li.Quantity = it.Quantity
		repl.Items = append(repl.Items, li)
	}
	if err := repl.Create(); err != nil {
		return nil, err
	}
	return repl, nil
}

// Reject declines a pending claim. It does not move money.
func Reject(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.NewNamespaced(org.Namespaced(c.Context()))

	id := c.Param("claimid")
	cl := claimModel.New(db)
	if err := cl.GetById(id); err != nil {
		return http.Fail(c, 404, "No claim found with id: "+id, err)
	}

	if cl.Status == claimModel.StatusRejected {
		return http.Render(c, 200, cl) // idempotent
	}
	if !cl.IsOpen() {
		return http.Fail(c, 409, "Cannot reject claim in status: "+cl.Status, errors.New("claim not pending"))
	}

	cl.Status = claimModel.StatusRejected
	if err := cl.Update(); err != nil {
		return http.Fail(c, 500, "Failed to reject claim", err)
	}
	return http.Render(c, 200, cl)
}
