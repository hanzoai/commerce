// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

// Package acp is the merchant (seller) side of the Agentic Commerce Protocol, API version
// 2026-04-17: the Agentic Checkout REST API (create, update, retrieve, complete and cancel a
// checkout session), the order webhook the seller sends to the agent, the discovery document
// at /.well-known/acp.json, and the payment handler declaration, all behind a Backend that
// the seller's commerce system implements.
//
// The package follows the released schema, not the prose of the RFCs where they differ
// (the RFCs still describe an items[] request with quantity; the schema's request item is
// {id, name, unit_amount}). What the seller cannot honour it refuses with the spec's error
// rather than accept in silence. The payment service provider's delegate_payment endpoint is
// not part of the seller's API and is not implemented.
package acp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Config is what a Service needs from its host. Nothing environmental has a default except
// the idempotency ledger, the clock, the clock skew and the error report.
type Config struct {
	// Backend is the seller's commerce system.
	Backend Backend
	// Identify establishes the caller from the request: it validates the bearer token and
	// answers the merchant and buyer it stands for. An error answers 401.
	Identify func(*http.Request) (Caller, error)
	// Handlers are the payment handlers a session declares for the caller. The host owns
	// them because they carry the seller's merchant account at its PSP.
	Handlers func(context.Context, Caller) ([]PaymentHandler, error)
	// Order is the permanent URL where the buyer views an order.
	Order func(id string) string
	// Links are the seller's policy and support URLs a session lists.
	Links []Link
	// Ledger keeps idempotency keys; nil is an in-process Memory.
	Ledger Ledger
	// Events receives the order webhook; nil sends none.
	Events Sender
	// Verify checks the Signature of a request; nil does not verify requests, which the
	// spec only recommends. When set, Signature and Timestamp are required.
	Verify Verifier
	// Skew bounds how far a request Timestamp may be from the clock; zero is 300 seconds,
	// the tolerance the webhook specification recommends.
	Skew time.Duration
	// Report receives what the service could not answer to the agent: an unexpected backend
	// error, a webhook that was not delivered. Nil writes it to slog.
	Report func(error)
	// Now is the clock; nil is time.Now.
	Now func() time.Time
}

// Service is the seller side of the Agentic Checkout API.
type Service struct {
	cfg Config
}

// New returns a Service. It refuses a Config that lacks what a session cannot be stated
// without.
func New(cfg Config) (*Service, error) {
	switch {
	case cfg.Backend == nil:
		return nil, errors.New("acp: Config.Backend is required")
	case cfg.Identify == nil:
		return nil, errors.New("acp: Config.Identify is required")
	case cfg.Handlers == nil:
		return nil, errors.New("acp: Config.Handlers is required")
	case cfg.Order == nil:
		return nil, errors.New("acp: Config.Order is required")
	}
	if cfg.Ledger == nil {
		cfg.Ledger = NewMemory()
	}
	if cfg.Skew <= 0 {
		cfg.Skew = 300 * time.Second
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Report == nil {
		cfg.Report = func(err error) { slog.Error("acp", "error", err) }
	}
	if cfg.Links == nil {
		cfg.Links = []Link{}
	}
	return &Service{cfg: cfg}, nil
}

// Digital goods are fulfilled instantly: the one fulfillment option a session offers.
const (
	digitalID    = "digital"
	digitalTitle = "Digital delivery"
)

// Create opens a checkout session (POST /checkout_sessions).
func (s *Service) Create(ctx context.Context, who Caller, req *CreateRequest) (*CheckoutSession, error) {
	if f := s.validateCreate(req); f != nil {
		return nil, f
	}
	hs, err := s.handlers(ctx, who)
	if err != nil {
		return nil, err
	}
	c, err := s.cfg.Backend.Open(ctx, who, req.LineItems[0].ID)
	if err != nil {
		return nil, s.refusal(err, "$.line_items[0].item.id", req.LineItems[0].ID)
	}
	return s.session(c, hs, advise(req.LineItems, req.Coupons, req.Discounts, c)...), nil
}

// Update changes a checkout session (POST /checkout_sessions/{id}).
func (s *Service) Update(ctx context.Context, who Caller, id string, req *UpdateRequest) (*CheckoutSession, error) {
	if f := validateUpdate(req); f != nil {
		return nil, f
	}
	hs, err := s.handlers(ctx, who)
	if err != nil {
		return nil, err
	}
	c, err := s.cfg.Backend.Read(ctx, who, id)
	if err != nil {
		return nil, s.refusal(err, "", "")
	}
	if c.State != Open && c.State != Failed {
		return nil, Conflict("The checkout session is " + string(c.State) + " and can no longer be changed.")
	}
	item := c.Item.ID
	if len(req.LineItems) > 0 {
		item = req.LineItems[0].ID
	}
	if f := selected(req.SelectedFulfillmentOptions, item); f != nil {
		return nil, f
	}
	if len(req.LineItems) > 0 {
		if c, err = s.cfg.Backend.Change(ctx, who, id, item); err != nil {
			return nil, s.refusal(err, "$.line_items[0].item.id", item)
		}
	}
	return s.session(c, hs, advise(req.LineItems, req.Coupons, req.Discounts, c)...), nil
}

// Retrieve answers the latest state of a session (GET /checkout_sessions/{id}).
func (s *Service) Retrieve(ctx context.Context, who Caller, id string) (*CheckoutSession, error) {
	hs, err := s.handlers(ctx, who)
	if err != nil {
		return nil, err
	}
	c, err := s.cfg.Backend.Read(ctx, who, id)
	if err != nil {
		return nil, s.refusal(err, "", "")
	}
	return s.session(c, hs), nil
}

// Complete pays a session (POST /checkout_sessions/{id}/complete). key is the
// Idempotency-Key of the request. A payment the bank or token declined is not an error: the
// session comes back unchanged and ready for payment, with the reason in messages.
func (s *Service) Complete(ctx context.Context, who Caller, id, key string, req *CompleteRequest) (*CheckoutSession, error) {
	if f := validateComplete(req); f != nil {
		return nil, f
	}
	hs, err := s.handlers(ctx, who)
	if err != nil {
		return nil, err
	}
	pd := req.PaymentData
	var pay Charge
	for _, h := range hs {
		if h.ID == pd.HandlerID {
			pay.Handler = h
		}
	}
	if pay.Handler.ID == "" {
		return nil, Invalid("$.payment_data.handler_id", "The payment handler '"+pd.HandlerID+"' is not offered by this session.")
	}
	if pd.Instrument.HandlerID != "" && pd.Instrument.HandlerID != pd.HandlerID {
		return nil, Invalid("$.payment_data.instrument.handler_id", "The instrument names another handler than payment_data.handler_id.")
	}
	pay.Instrument, pay.Credential, pay.Token = pd.Instrument.Type, pd.Instrument.Credential.Type, pd.Instrument.Credential.Token
	c, err := s.cfg.Backend.Pay(ctx, who, id, key, pay)
	if err != nil {
		return nil, s.refusal(err, "", "")
	}
	if c.State == Paid {
		s.emit(ctx, EventOrderCreate, c)
	}
	return s.session(c, hs), nil
}

// Cancel abandons a session (POST /checkout_sessions/{id}/cancel).
func (s *Service) Cancel(ctx context.Context, who Caller, id string, req *CancelRequest) (*CheckoutSession, error) {
	if f := validateCancel(req); f != nil {
		return nil, f
	}
	hs, err := s.handlers(ctx, who)
	if err != nil {
		return nil, err
	}
	c, err := s.cfg.Backend.Cancel(ctx, who, id)
	if err != nil {
		return nil, s.refusal(err, "", "")
	}
	return s.session(c, hs), nil
}

// refusal turns a backend error into the Fault the agent is answered with. A *Fault passes
// through; ErrNoItem is the item the agent named; anything else is reported and answered
// without its text.
func (s *Service) refusal(err error, param, item string) *Fault {
	var f *Fault
	switch {
	case errors.As(err, &f):
		return f
	case errors.Is(err, ErrNoItem):
		return NoItem(param, item)
	}
	s.cfg.Report(err)
	return Internal()
}

// handlers are the payment handlers the caller's sessions declare. They are read before a
// request changes anything, so a session is never opened that cannot be stated.
func (s *Service) handlers(ctx context.Context, who Caller) ([]PaymentHandler, error) {
	hs, err := s.cfg.Handlers(ctx, who)
	if err != nil {
		s.cfg.Report(fmt.Errorf("acp: the payment handlers could not be read: %w", err))
		return nil, Unavailable("Payment handlers are unavailable.")
	}
	return hs, nil
}

// session states a checkout as the session the agent reads. The extra messages are the ones
// the request itself earned.
func (s *Service) session(c *Checkout, handlers []PaymentHandler, extra ...Message) *CheckoutSession {
	line := "line_item_" + c.Item.ID
	unit := c.Total
	out := &CheckoutSession{
		ID:       c.ID,
		Protocol: &ProtocolVersion{Version: Version},
		Capabilities: Capabilities{
			Payment: &Payment{Handlers: handlers},
			// The seller runs none of the interventions in-band, so the intersection with
			// whatever the agent supports is empty (rfc.capability_negotiation.md §4.5.1).
			Interventions: &Interventions{Supported: []string{}, Required: []string{}, Enforcement: "conditional"},
		},
		Status:   status(c.State),
		Currency: s.cfg.Backend.Currency(),
		LineItems: []LineItem{{
			ID:         line,
			Item:       Item{ID: c.Item.ID},
			Quantity:   1,
			Name:       c.Item.Name,
			UnitAmount: &unit,
			Totals:     breakdown(c.Total, "Base Amount"),
		}},
		FulfillmentOptions: []FulfillmentOption{{
			Type: "digital", ID: digitalID, Title: digitalTitle,
			Totals: []Total{{Type: "total", DisplayText: digitalTitle, Amount: 0}},
		}},
		SelectedFulfillmentOptions: []SelectedFulfillmentOption{{Type: "digital", OptionID: digitalID, ItemIDs: []string{c.Item.ID}}},
		Totals:                     breakdown(c.Total, "Item(s) total"),
		Messages:                   append([]Message{}, extra...),
		Links:                      s.cfg.Links,
	}
	switch {
	case c.State == Canceled:
		out.Messages = append(out.Messages, Message{Type: "info", ContentType: "plain", Content: "Checkout session has been canceled."})
	case c.Refusal != nil:
		r := c.Refusal
		out.Messages = append(out.Messages, Message{Type: "error", Code: r.Code, Param: r.Param, Resolution: r.Resolution, ContentType: "plain", Content: r.Message})
	}
	if c.State == Paid && c.Receipt != nil {
		o := s.order(c, "")
		out.Order = &o
	}
	return out
}

// status maps a checkout state to the session status the spec names.
func status(st State) string {
	switch st {
	case Paid:
		return StatusCompleted
	case Paying:
		return StatusCompleteInProgress
	case Canceled:
		return StatusCanceled
	}
	return StatusReadyForPayment
}

// breakdown is the totals of one unit with no tax, discount or fulfillment charge: the
// seller computes none.
func breakdown(total int64, base string) []Total {
	return []Total{
		{Type: "items_base_amount", DisplayText: base, Amount: total},
		{Type: "subtotal", DisplayText: "Subtotal", Amount: total},
		{Type: "total", DisplayText: "Total", Amount: total},
	}
}

// order states a paid checkout as an order. typ is "order" for a webhook payload.
func (s *Service) order(c *Checkout, typ string) Order {
	total := c.Total
	return Order{
		Type:              typ,
		ID:                c.Receipt.ID,
		CheckoutSessionID: c.ID,
		PermalinkURL:      s.cfg.Order(c.Receipt.ID),
		Status:            "confirmed",
		LineItems: []OrderLineItem{{
			ID: "line_item_" + c.Item.ID, Title: c.Item.Name, ProductID: c.Item.ID,
			Quantity:  OrderLineItemQuantity{Ordered: 1, Current: 1},
			UnitPrice: &total, Subtotal: &total,
		}},
		Totals: []Total{{Type: "total", DisplayText: "Total", Amount: total}},
	}
}

// emit sends an order event. A failure is reported, never returned: the order exists.
func (s *Service) emit(ctx context.Context, typ string, c *Checkout) {
	if s.cfg.Events == nil || c.Receipt == nil {
		return
	}
	if err := s.cfg.Events.Send(ctx, Event{Type: typ, Data: s.order(c, "order")}); err != nil {
		s.cfg.Report(fmt.Errorf("acp: %s for order %s was not delivered: %w", typ, c.Receipt.ID, err))
	}
}

// advise is the warnings a request earned: a price that is not the one the agent expected,
// and discount codes the seller does not take.
func advise(items []Item, coupons []string, discounts *DiscountsRequest, c *Checkout) []Message {
	var out []Message
	if len(items) > 0 && items[0].UnitAmount != nil && *items[0].UnitAmount != c.Total {
		out = append(out, Message{
			Type: "warning", Code: "price_change", Resolution: "requires_buyer_review",
			Param: "$.line_items[0].unit_amount", ContentType: "plain",
			Content: fmt.Sprintf("The price of '%s' is %d, not %d.", c.Item.Name, c.Total, *items[0].UnitAmount),
		})
	}
	for i := range coupons {
		out = append(out, noDiscount(fmt.Sprintf("$.coupons[%d]", i)))
	}
	if discounts != nil {
		for i := range discounts.Codes {
			out = append(out, noDiscount(fmt.Sprintf("$.discounts.codes[%d]", i)))
		}
	}
	return out
}

func noDiscount(param string) Message {
	return Message{Type: "warning", Code: "discount_code_invalid", Resolution: "recoverable", Param: param,
		ContentType: "plain", Content: "This seller does not accept discount codes."}
}

// selected checks the fulfillment options an agent selects against the one the session
// offers (rfc.agentic_checkout.md §8: option_id MUST match a fulfillment option).
func selected(opts []SelectedFulfillmentOption, item string) *Fault {
	for i, o := range opts {
		at := fmt.Sprintf("$.selected_fulfillment_options[%d]", i)
		switch {
		case o.Type != "digital":
			return Invalid(at+".type", "Only digital fulfillment is offered.")
		case o.OptionID != digitalID:
			return Invalid(at+".option_id", "The fulfillment option '"+o.OptionID+"' is not offered by this session.")
		}
		for j, id := range o.ItemIDs {
			if id != item {
				return Invalid(fmt.Sprintf("%s.item_ids[%d]", at, j), "The item '"+id+"' is not in this session.")
			}
		}
	}
	return nil
}

// ---- validation: the boundary ----

// maxNotes is the schema's maxLength of order_notes.
const maxNotes = 5000

func (s *Service) validateCreate(r *CreateRequest) *Fault {
	switch {
	case len(r.LineItems) == 0:
		return Missing("$.line_items", "line_items must hold at least one item.")
	case r.Currency == "":
		return Missing("$.currency", "currency is required.")
	case r.Capabilities == nil:
		return Missing("$.capabilities", "capabilities is required.")
	case len(r.LineItems) > 1:
		return Unsupported("$.line_items", "This seller sells one item per checkout session.")
	case !strings.EqualFold(r.Currency, s.cfg.Backend.Currency()):
		return Unsupported("$.currency", "This seller settles in "+strings.ToUpper(s.cfg.Backend.Currency())+".")
	}
	if f := items(r.LineItems); f != nil {
		return f
	}
	if f := parties(r.Buyer, r.FulfillmentDetails, r.OrderNotes); f != nil {
		return f
	}
	return nil
}

func validateUpdate(r *UpdateRequest) *Fault {
	if len(r.LineItems) > 1 {
		return Unsupported("$.line_items", "This seller sells one item per checkout session.")
	}
	if f := items(r.LineItems); f != nil {
		return f
	}
	return parties(r.Buyer, r.FulfillmentDetails, r.OrderNotes)
}

func validateComplete(r *CompleteRequest) *Fault {
	pd := r.PaymentData
	switch {
	case pd == nil:
		return Missing("$.payment_data", "payment_data is required.")
	case pd.PurchaseOrderNumber != "":
		return Unsupported("$.payment_data.purchase_order_number", "Purchase orders are not accepted.")
	case pd.PaymentTerms != "":
		return Unsupported("$.payment_data.payment_terms", "Payment terms are not accepted.")
	case pd.DueDate != "":
		return Unsupported("$.payment_data.due_date", "Payment terms are not accepted.")
	case pd.ApprovalRequired != nil:
		return Unsupported("$.payment_data.approval_required", "Payment approval is not supported.")
	case pd.HandlerID == "":
		return Missing("$.payment_data.handler_id", "handler_id is required.")
	case pd.Instrument == nil:
		return Missing("$.payment_data.instrument", "instrument is required.")
	case pd.Instrument.Type == "":
		return Missing("$.payment_data.instrument.type", "instrument.type is required.")
	case pd.Instrument.Credential.Type == "":
		return Missing("$.payment_data.instrument.credential.type", "credential.type is required.")
	case pd.Instrument.Credential.Token == "":
		return Missing("$.payment_data.instrument.credential.token", "credential.token is required.")
	case r.AuthenticationResult != nil:
		return Unsupported("$.authentication_result", "This seller does not take an authentication result: it runs no in-band authentication.")
	}
	if a := r.AffiliateAttribution; a != nil && (a.Provider == "" || (a.Token == "" && a.PublisherID == "")) {
		return Invalid("$.affiliate_attribution", "An attribution needs a provider and a token or a publisher_id.")
	}
	return parties(r.Buyer, nil, r.OrderNotes)
}

func validateCancel(r *CancelRequest) *Fault {
	t := r.IntentTrace
	if t == nil {
		return nil
	}
	switch t.ReasonCode {
	case "price_sensitivity", "shipping_cost", "shipping_speed", "product_fit", "trust_security",
		"returns_policy", "payment_options", "comparison", "timing_deferred", "other":
	case "":
		return Missing("$.intent_trace.reason_code", "reason_code is required.")
	default:
		return Invalid("$.intent_trace.reason_code", "reason_code is not one of the defined reasons.")
	}
	if len(t.TraceSummary) > 500 {
		return Invalid("$.intent_trace.trace_summary", "trace_summary is at most 500 characters.")
	}
	for k, v := range t.Metadata {
		switch v.(type) {
		case string, float64, bool:
		default:
			return Invalid("$.intent_trace.metadata."+k, "metadata values are strings, numbers or booleans.")
		}
	}
	return nil
}

func items(its []Item) *Fault {
	for i, it := range its {
		if it.ID == "" {
			return Missing(fmt.Sprintf("$.line_items[%d].id", i), "id is required.")
		}
	}
	return nil
}

// parties checks the buyer, the fulfillment details and the order notes of a request.
func parties(b *Buyer, fd *FulfillmentDetails, notes *string) *Fault {
	if b != nil && b.Email == "" {
		return Missing("$.buyer.email", "buyer.email is required.")
	}
	if fd != nil && fd.Address != nil {
		a := fd.Address
		for _, f := range []struct{ name, val string }{
			{"name", a.Name}, {"line_one", a.LineOne}, {"city", a.City},
			{"state", a.State}, {"country", a.Country}, {"postal_code", a.PostalCode},
		} {
			if f.val == "" {
				return Missing("$.fulfillment_details.address."+f.name, "address."+f.name+" is required.")
			}
		}
	}
	if notes != nil && len(*notes) > maxNotes {
		return Invalid("$.order_notes", "order_notes is at most 5000 characters.")
	}
	return nil
}
