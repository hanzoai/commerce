package billing

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/payout"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json/http"
)

type createPayoutRequest struct {
	Amount          int64                  `json:"amount"`
	Currency        string                 `json:"currency,omitempty"`
	DestinationType string                 `json:"destinationType"` // "bank_account" | "card"
	DestinationId   string                 `json:"destinationId"`
	Description     string                 `json:"description,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// Payout is one outbound transfer as this surface reports it. The fields a
// payout does not always have are pointers, because that is where the emission
// rule lives: an arrival date exists only once the money lands, and a failure
// message travels with its code even when the provider sent no words with it.
type Payout struct {
	Id              string        `json:"id"`
	Amount          int64         `json:"amount"` // cents
	Currency        currency.Type `json:"currency"`
	Status          payout.Status `json:"status"`
	DestinationType string        `json:"destinationType"`
	DestinationId   string        `json:"destinationId"`
	Created         time.Time     `json:"created"`
	Description     string        `json:"description,omitempty"`
	ArrivalDate     *time.Time    `json:"arrivalDate,omitempty"`
	ProviderRef     string        `json:"providerRef,omitempty"`
	FailureCode     string        `json:"failureCode,omitempty"`
	FailureMessage  *string       `json:"failureMessage,omitempty"`
	// Metadata is whatever the creator attached, carried as the bytes it
	// arrived as. Raw JSON is the one free-form shape that also crosses the
	// internal plane, where a map has no type.
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// CreatePayout creates a new outbound payout.
//
//	POST /v1/billing/payouts
func CreatePayout(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	var req createPayoutRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}

	if req.Amount <= 0 {
		return http.Fail(c, 400, "amount must be positive", nil)
	}
	if req.DestinationId == "" {
		return http.Fail(c, 400, "destinationId is required", nil)
	}

	p := payout.New(db)
	p.Amount = req.Amount
	if req.Currency != "" {
		p.Currency = currency.Type(req.Currency)
	}
	p.DestinationType = req.DestinationType
	p.DestinationId = req.DestinationId
	p.Description = req.Description
	if req.Metadata != nil {
		p.Metadata = req.Metadata
	}

	if err := p.Create(); err != nil {
		log.Error("Failed to create payout: %v", err, c)
		return http.Fail(c, 500, "failed to create payout", err)
	}

	return c.JSON(201, payoutResponse(p))
}

// GetPayout retrieves a payout by ID.
//
//	GET /v1/billing/payouts/:id
func GetPayout(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	p := payout.New(db)
	if err := p.GetById(c.Param("id")); err != nil {
		return http.Fail(c, 404, "payout not found", err)
	}

	return c.JSON(200, payoutResponse(p))
}

// ListPayouts is the org's payouts, newest first — the QUERY, with no HTTP in
// it.
//
// It takes values rather than a request so a caller that is not a request can
// ask: a peer process holding no datastore reads the same list over the
// internal plane, and re-deriving the query there would be a second
// implementation of one question. Two copies of "what has this org paid out" is
// how a treasury view and a payout page come to disagree about money that has
// already left.
func ListPayouts(ctx context.Context, org *organization.Organization) ([]Payout, error) {
	if org == nil {
		return nil, errors.New("payouts: no organization")
	}
	db := datastore.New(org.Namespaced(ctx))

	rootKey := db.NewKey("synckey", "", 1, nil)
	payouts := make([]Payout, 0)
	iter := payout.Query(db).Ancestor(rootKey).Order("-Created").Run()

	for {
		p := payout.New(db)
		if _, err := iter.Next(p); err != nil {
			break
		}
		payouts = append(payouts, payoutResponse(p))
	}
	return payouts, nil
}

// ListBillingPayouts lists payouts.
//
//	GET /v1/billing/payouts
func ListBillingPayouts(c *zip.Ctx) error {
	// #146 class: never panic on a missing org (co-resident embed path — see ListInvoices).
	// No org ⇒ honest empty list.
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return c.JSON(200, []Payout{})
	}

	payouts, err := ListPayouts(c.Context(), org)
	if err != nil {
		log.Error("Failed to list payouts: %v", err, c)
		return http.Fail(c, 500, "failed to list payouts", err)
	}

	return c.JSON(200, payouts)
}

// CancelPayout cancels a pending payout.
//
//	POST /v1/billing/payouts/:id/cancel
func CancelPayout(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	p := payout.New(db)
	if err := p.GetById(c.Param("id")); err != nil {
		return http.Fail(c, 404, "payout not found", err)
	}

	if err := p.Cancel(); err != nil {
		return http.Fail(c, 400, err.Error(), err)
	}

	if err := p.Update(); err != nil {
		log.Error("Failed to cancel payout: %v", err, c)
		return http.Fail(c, 500, "failed to cancel payout", err)
	}

	return c.JSON(200, payoutResponse(p))
}

func payoutResponse(p *payout.Payout) Payout {
	v := Payout{
		Id:              p.Id(),
		Amount:          p.Amount,
		Currency:        p.Currency,
		Status:          p.Status,
		DestinationType: p.DestinationType,
		DestinationId:   p.DestinationId,
		// The row's creation time. p.Created is the mixin's persisted-yet
		// predicate — a func, which no encoder can render — so the time comes
		// from the accessor.
		Created:     p.GetCreatedAt(),
		Description: p.Description,
		ProviderRef: p.ProviderRef,
		FailureCode: p.FailureCode,
	}
	if !p.ArrivalDate.IsZero() {
		arrived := p.ArrivalDate
		v.ArrivalDate = &arrived
	}
	if p.FailureCode != "" {
		msg := p.FailureMessage
		v.FailureMessage = &msg
	}
	if p.Metadata != nil {
		// Metadata reached the row as JSON, so it re-encodes; on the failure
		// that leaves, the key is absent rather than the whole payout.
		if b, err := json.Marshal(p.Metadata); err == nil {
			v.Metadata = b
		}
	}
	return v
}
