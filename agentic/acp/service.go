// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

// Package acp is the seller side of the Agentic Commerce Protocol, API 2026-04-17: the
// Agentic Checkout REST API, the order webhook, the discovery document and the payment handler
// declaration, over a Backend the seller's commerce system implements. It follows the released
// JSON Schema where the RFC prose differs, and refuses what the seller cannot honour with the
// spec's error. delegate_payment is the PSP's endpoint and is not implemented.
package acp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Config is what a Service needs from its host.
type Config struct {
	Backend Backend
	// Identify validates the bearer token and answers the merchant and buyer it stands for.
	Identify func(*http.Request) (Caller, error)
	// Handlers are the payment handlers a session declares for the caller.
	Handlers func(context.Context, Caller) ([]PaymentHandler, error)
	// Order is the URL where the buyer views an order.
	Order func(id string) string
	// Links are the policy and support URLs a session lists.
	Links  []Link
	Ledger Ledger
	// RetryAfter is the Retry-After, in seconds, of an idempotency_in_flight answer (§6.4).
	RetryAfter int
	// Report receives an unexpected backend error or an undelivered webhook.
	Report func(error)
	// Events receives the order webhook; nil sends none.
	Events Sender
	// Verify checks request signatures; nil does not (the spec recommends it). When set, Skew is
	// how far the Timestamp may be from the clock.
	Verify Verifier
	Skew   time.Duration
	// Now is the clock; nil is time.Now.
	Now func() time.Time
}

// Service is the seller side of the Agentic Checkout API.
type Service struct {
	cfg Config
}

// New returns a Service.
func New(cfg Config) (*Service, error) {
	switch {
	case cfg.Backend == nil || cfg.Identify == nil || cfg.Handlers == nil || cfg.Order == nil || cfg.Ledger == nil || cfg.Report == nil:
		return nil, errors.New("acp: Backend, Identify, Handlers, Order, Ledger and Report are required")
	case cfg.RetryAfter <= 0:
		return nil, errors.New("acp: RetryAfter is required")
	case cfg.Verify != nil && cfg.Skew <= 0:
		return nil, errors.New("acp: Skew is required with Verify")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Links == nil {
		cfg.Links = []Link{}
	}
	return &Service{cfg: cfg}, nil
}

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
	if len(req.LineItems) > 0 {
		if c, err = s.cfg.Backend.Change(ctx, who, id, req.LineItems[0].ID); err != nil {
			return nil, s.refusal(err, "$.line_items[0].item.id", req.LineItems[0].ID)
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

// Complete pays a session (POST /checkout_sessions/{id}/complete). key is the request's
// Idempotency-Key. A declined payment is not an error: the session comes back ready for payment
// with the reason in messages.
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
func (s *Service) Cancel(ctx context.Context, who Caller, id string) (*CheckoutSession, error) {
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

// refusal is the Fault a backend error is answered with; an unexpected error is reported and
// answered without its text.
func (s *Service) refusal(err error, param, item string) *Fault {
	var f *Fault
	switch {
	case errors.As(err, &f):
		return f
	case errors.Is(err, ErrNoItem):
		return noItem(param, item)
	}
	s.cfg.Report(err)
	return internal()
}

// handlers are read before a request changes anything, so no session is opened that cannot be stated.
func (s *Service) handlers(ctx context.Context, who Caller) ([]PaymentHandler, error) {
	hs, err := s.cfg.Handlers(ctx, who)
	if err != nil {
		s.cfg.Report(fmt.Errorf("acp: the payment handlers could not be read: %w", err))
		return nil, Unavailable("Payment handlers are unavailable.")
	}
	return hs, nil
}

// session states a checkout as a CheckoutSession; extra are the messages the request earned.
func (s *Service) session(c *Checkout, handlers []PaymentHandler, extra ...Message) *CheckoutSession {
	line := "line_item_" + c.Item.ID
	unit := c.Total
	out := &CheckoutSession{
		ID:       c.ID,
		Protocol: &ProtocolVersion{Version: Version},
		Capabilities: Capabilities{
			Payment: &Payment{Handlers: handlers},
			// No intervention runs in-band: the intersection is empty (rfc.capability_negotiation.md §4.5.1).
			Interventions: &Interventions{Supported: []string{}},
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
		FulfillmentOptions: []FulfillmentOption{},
		Totals:             breakdown(c.Total, "Item(s) total"),
		Messages:           append([]Message{}, extra...),
		Links:              s.cfg.Links,
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

// status maps a checkout state to a session status.
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

// breakdown is the totals of one unit; the seller computes no tax, discount or fulfillment charge.
func breakdown(total int64, base string) []Total {
	return []Total{
		{Type: "items_base_amount", DisplayText: base, Amount: total},
		{Type: "subtotal", DisplayText: "Subtotal", Amount: total},
		{Type: "total", DisplayText: "Total", Amount: total},
	}
}

// order states a paid checkout as an order, "confirmed" (rfc.orders.md §4.1); typ is "order" in a webhook.
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

// emit sends an order event; a failure is reported, not returned, since the order exists.
func (s *Service) emit(ctx context.Context, typ string, c *Checkout) {
	if s.cfg.Events == nil || c.Receipt == nil {
		return
	}
	if err := s.cfg.Events.Send(ctx, Event{Type: typ, Data: s.order(c, "order")}); err != nil {
		s.cfg.Report(fmt.Errorf("acp: %s for order %s was not delivered: %w", typ, c.Receipt.ID, err))
	}
}

// advise is the warnings a request earned: a price other than the unit_amount the agent sent
// (price_change), and discount codes (discount_code_invalid) since no discount extension runs.
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

func (s *Service) validateCreate(r *CreateRequest) *Fault {
	switch {
	case len(r.LineItems) == 0:
		return missing("$.line_items", "line_items must hold at least one item.")
	case r.Currency == "":
		return missing("$.currency", "currency is required.")
	case r.Capabilities == nil:
		return missing("$.capabilities", "capabilities is required.")
	case len(r.LineItems) > 1:
		return Unsupported("$.line_items", "This seller sells one item per checkout session.")
	case !strings.EqualFold(r.Currency, s.cfg.Backend.Currency()):
		return Unsupported("$.currency", "This seller settles in "+strings.ToUpper(s.cfg.Backend.Currency())+".")
	}
	return items(r.LineItems)
}

// validateUpdate refuses any selected fulfillment option: the session offers none
// (rfc.agentic_checkout.md §8: option_id MUST match a fulfillment option).
func validateUpdate(r *UpdateRequest) *Fault {
	switch {
	case len(r.LineItems) > 1:
		return Unsupported("$.line_items", "This seller sells one item per checkout session.")
	case len(r.SelectedFulfillmentOptions) > 0:
		return Invalid("$.selected_fulfillment_options[0].option_id", "This session offers no fulfillment options.")
	}
	return items(r.LineItems)
}

func validateComplete(r *CompleteRequest) *Fault {
	pd := r.PaymentData
	if pd == nil {
		return missing("$.payment_data", "payment_data is required.")
	}
	for _, f := range []struct {
		name string
		set  bool
	}{
		{"purchase_order_number", pd.PurchaseOrderNumber != ""}, {"payment_terms", pd.PaymentTerms != ""},
		{"due_date", pd.DueDate != ""}, {"approval_required", pd.ApprovalRequired != nil},
	} {
		if f.set {
			return Unsupported("$.payment_data."+f.name, "Purchase orders and payment terms are not accepted.")
		}
	}
	switch {
	case pd.HandlerID == "":
		return missing("$.payment_data.handler_id", "handler_id is required.")
	case pd.Instrument == nil:
		return missing("$.payment_data.instrument", "instrument is required.")
	case pd.Instrument.Type == "":
		return missing("$.payment_data.instrument.type", "instrument.type is required.")
	case pd.Instrument.Credential.Type == "":
		return missing("$.payment_data.instrument.credential.type", "credential.type is required.")
	case pd.Instrument.Credential.Token == "":
		return missing("$.payment_data.instrument.credential.token", "credential.token is required.")
	case r.AuthenticationResult != nil:
		return Unsupported("$.authentication_result", "This seller runs no in-band authentication.")
	}
	return nil
}

func items(its []Item) *Fault {
	for i, it := range its {
		if it.ID == "" {
			return missing(fmt.Sprintf("$.line_items[%d].id", i), "id is required.")
		}
	}
	return nil
}
