// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"
)

// Maximum of a quantity and of an amount: 2^53-1, the largest integer a JSON
// number carries exactly (schemas/shopping/types/line_item.json).
const maxInt = 9007199254740991

// invalid is a malformed request: HTTP 400.
func invalid(format string, args ...any) *Error {
	return status(http.StatusBadRequest, "invalid_request", format, args...)
}

func decode(b []byte, v any) *Error {
	if err := json.Unmarshal(b, v); err != nil {
		return invalid("the request body is not valid for this operation: %v", err)
	}
	return nil
}

// checkLines validates the line items of a request: an item id and a quantity
// of at least 1 and at most 2^53-1 on each line.
func checkLines(ls []LineItem) *Error {
	for i, l := range ls {
		if l.Item.ID == "" {
			return invalid("line_items[%d].item.id is required", i)
		}
		if l.Quantity < 1 || l.Quantity > maxInt {
			return invalid("line_items[%d].quantity must be an integer from 1 to %d", i, int64(maxInt))
		}
		if qu := l.Item.QuantityUnit; qu != nil && (qu.Unit == "" || qu.DisplayText == "") {
			return invalid("line_items[%d].item.quantity_unit needs unit and display_text", i)
		}
	}
	return nil
}

// checkSignals requires every signal key to be a reverse-domain name
// (shopping/checkout, "Signals").
func checkSignals(m map[string]json.RawMessage) *Error {
	for k := range m {
		if !nameRe.MatchString(k) {
			return invalid("signal key %q must be a reverse-domain name", k)
		}
	}
	return nil
}

func (s *Server) dispatch(q *request) reply {
	switch q.op {
	case opCreate:
		var in CreateRequest
		if e := decode(q.body, &in); e != nil {
			return errorReply(e)
		}
		if len(in.LineItems) == 0 && in.CartID == "" {
			return errorReply(invalid("a create request needs line_items or cart_id"))
		}
		if in.CartID != "" {
			return errorReply(invalid("cart_id requires the Cart capability, which is not negotiated"))
		}
		if e := checkLines(in.LineItems); e != nil {
			return errorReply(e)
		}
		if e := checkSignals(in.Signals); e != nil {
			return errorReply(e)
		}
		c, err := s.cfg.Backend.Create(q.r.Context(), q.who, &in)
		return s.checkout(q, c, err, http.StatusCreated)
	case opRead:
		c, err := s.cfg.Backend.Read(q.r.Context(), q.who, q.id)
		return s.checkout(q, c, err, http.StatusOK)
	case opUpdate:
		var in UpdateRequest
		if e := decode(q.body, &in); e != nil {
			return errorReply(e)
		}
		if len(in.LineItems) == 0 {
			return errorReply(invalid("an update request needs line_items"))
		}
		if e := checkLines(in.LineItems); e != nil {
			return errorReply(e)
		}
		if e := checkSignals(in.Signals); e != nil {
			return errorReply(e)
		}
		cur, err := s.cfg.Backend.Read(q.r.Context(), q.who, q.id)
		if err != nil {
			return s.checkout(q, cur, err, http.StatusOK)
		}
		switch cur.Status {
		case CompleteInProgress:
			return s.hold(q, cur, Err("in_progress", Recoverable, "", "the checkout is being completed and cannot be updated"))
		case Completed, Canceled:
			return s.hold(q, cur, Err("not_modifiable", Unrecoverable, "", "the checkout is %s and cannot be updated", cur.Status))
		}
		c, err := s.cfg.Backend.Update(q.r.Context(), q.who, q.id, &in)
		return s.checkout(q, c, err, http.StatusOK)
	case opComplete:
		var in CompleteRequest
		if e := decode(q.body, &in); e != nil {
			return errorReply(e)
		}
		if in.Payment == nil {
			return errorReply(invalid("a complete request needs payment"))
		}
		if e := checkSignals(in.Signals); e != nil {
			return errorReply(e)
		}
		cur, err := s.cfg.Backend.Read(q.r.Context(), q.who, q.id)
		if err != nil {
			return s.checkout(q, cur, err, http.StatusOK)
		}
		switch cur.Status {
		case Completed:
			return s.hold(q, cur, Err("not_modifiable", Unrecoverable, "", "the checkout is already completed"))
		case Canceled:
			return s.hold(q, cur, Err("not_modifiable", Unrecoverable, "", "the checkout is canceled"))
		case CompleteInProgress:
			return s.hold(q, cur, Err("in_progress", Recoverable, "", "the checkout is being completed"))
		case RequiresEscalation:
			return s.hold(q, cur)
		case Incomplete:
			return s.hold(q, cur, Err("not_ready", Recoverable, "", "the checkout is incomplete: resolve its messages before completing"))
		}
		if msg := s.checkPayment(in.Payment); msg != nil {
			cur.Status = Incomplete
			return s.hold(q, cur, *msg)
		}
		c, err := s.cfg.Backend.Complete(q.r.Context(), q.who, q.id, q.r.Header.Get(HeaderIdempotency), &in)
		if err == nil && c.Status == Completed && c.Order != nil {
			s.announce(q, c.Order.ID)
		}
		return s.checkout(q, c, err, http.StatusOK)
	case opCancel:
		cur, err := s.cfg.Backend.Read(q.r.Context(), q.who, q.id)
		if err != nil {
			return s.checkout(q, cur, err, http.StatusOK)
		}
		if cur.Status == Completed || cur.Status == Canceled {
			return s.hold(q, cur, Err("not_cancelable", Unrecoverable, "", "the checkout is %s and cannot be canceled", cur.Status))
		}
		c, err := s.cfg.Backend.Cancel(q.r.Context(), q.who, q.id)
		return s.checkout(q, c, err, http.StatusOK)
	case opOrder:
		o, err := s.orders.Order(q.r.Context(), q.who, q.id)
		switch {
		case err == nil:
			o.UCP = &Meta{Version: q.active.Version, Status: "success", Capabilities: q.active.Relevant(CapOrder)}
			if cerr := o.Check(); cerr != nil {
				s.fault(q.op.String(), cerr)
				return errorReply(status(http.StatusInternalServerError, "internal", "the order could not be produced"))
			}
			return jsonReply(http.StatusOK, o)
		case errors.Is(err, ErrNotFound):
			return jsonReply(http.StatusOK, s.errorResponse(q, CapOrder, "", Err("not_found", Unrecoverable, "", "Order not found.")))
		case errors.Is(err, ErrForbidden):
			return jsonReply(http.StatusOK, s.errorResponse(q, CapOrder, "", Err("unauthorized", Unrecoverable, "", "Not authorized to access this order.")))
		}
		return s.failure(q, err)
	}
	return errorReply(status(http.StatusNotFound, "not_found", "no such resource"))
}

// hold answers with the current checkout unchanged and the messages that say
// why the operation was not applied.
func (s *Server) hold(q *request, cur *Checkout, msgs ...Message) reply {
	cur.Messages = append(slices.Clone(cur.Messages), msgs...)
	return s.checkout(q, cur, nil, http.StatusOK)
}

// checkPayment validates a complete request's payment before it is handed to
// the backend: exactly one instrument (no split payments are negotiated), and
// one that names a handler the business advertised (overview, "Instrument
// Cardinality" and "Security Best Practices").
func (s *Server) checkPayment(p *Payment) *Message {
	if len(p.Instruments) != 1 {
		m := Err("payment_failed", Recoverable, "$.payment.instruments", "A checkout is paid with exactly one payment instrument")
		return &m
	}
	hid := p.Instruments[0].HandlerID
	for _, hs := range s.cfg.Profile.UCP.PaymentHandlers {
		for _, h := range hs {
			if h.ID == hid {
				return nil
			}
		}
	}
	m := Err("payment_failed", Recoverable, "$.payment.instruments[0].handler_id", "No payment handler %q is advertised for this checkout", hid)
	return &m
}

// checkout turns a backend outcome into the response: the checkout with its ucp
// envelope, a refusal as the business outcome it states, a missing resource as
// an error response, and anything else as a failure.
func (s *Server) checkout(q *request, c *Checkout, err error, ok int) reply {
	var ref *Refusal
	switch {
	case err == nil:
		return s.send(q, c, ok)
	case errors.As(err, &ref):
		if ref.Current != nil {
			cur := ref.Current
			cur.Messages = append(slices.Clone(cur.Messages), ref.Messages...)
			if ref.ContinueURL != "" {
				cur.ContinueURL = ref.ContinueURL
			}
			return s.send(q, cur, http.StatusOK)
		}
		return jsonReply(http.StatusOK, s.errorResponse(q, CapCheckout, ref.ContinueURL, ref.Messages...))
	case errors.Is(err, ErrNotFound):
		return jsonReply(http.StatusOK, s.errorResponse(q, CapCheckout, "", Err("not_found", Unrecoverable, "", "Checkout not found.")))
	}
	return s.failure(q, err)
}

func (s *Server) failure(q *request, err error) reply {
	switch {
	case errors.Is(err, ErrUnauthorized):
		return errorReply(status(http.StatusUnauthorized, "unauthorized", "authentication is required"))
	case errors.Is(err, ErrForbidden):
		return errorReply(status(http.StatusForbidden, "forbidden", "not permitted"))
	}
	var un *Unavailable
	if errors.As(err, &un) {
		e := status(http.StatusServiceUnavailable, "unavailable", "the service is temporarily unavailable")
		e.RetryAfter = un.RetryAfter
		return errorReply(e)
	}
	s.fault(q.op.String(), err)
	return errorReply(status(http.StatusInternalServerError, "internal", "unexpected error"))
}

// send adds the ucp envelope to a checkout and answers it. A checkout that does
// not satisfy the schema is a defect of the backend and is never sent.
func (s *Server) send(q *request, c *Checkout, ok int) reply {
	c.UCP = &Meta{
		Version:         q.active.Version,
		Status:          "success",
		Capabilities:    q.active.Relevant(CapCheckout),
		PaymentHandlers: s.handlers(q.peer),
	}
	if c.Links == nil {
		c.Links = slices.Clone(s.cfg.Links)
	}
	if err := c.Check(); err != nil {
		s.fault(q.op.String(), fmt.Errorf("backend returned an invalid checkout: %w", err))
		return errorReply(status(http.StatusInternalServerError, "internal", "unexpected error"))
	}
	return jsonReply(ok, c)
}

// errorResponse is the response when no resource can be established
// (schemas/common/types/error_response.json).
func (s *Server) errorResponse(q *request, root, continueURL string, msgs ...Message) ErrorResponse {
	if continueURL == "" {
		continueURL = s.cfg.Storefront
	}
	caps := q.active.Relevant(root)
	if !q.active.Has(root) {
		caps = map[string][]Capability{}
	}
	return ErrorResponse{
		UCP:         Meta{Version: q.active.Version, Status: "error", Capabilities: caps},
		Messages:    msgs,
		ContinueURL: continueURL,
	}
}

// announce sends the "order created" event to the platform when it negotiated
// the Order capability and declared where to send it. The delivery runs after
// the response is on its way; retries are the Webhooks' own.
func (s *Server) announce(q *request, orderID string) {
	w := s.cfg.Webhooks
	if w == nil || s.orders == nil || !q.active.Has(CapOrder) {
		return
	}
	url, err := WebhookURL(q.peer, q.active.Version)
	if err != nil {
		s.fault("webhook", err)
		return
	}
	who := q.who
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		o, err := s.orders.Order(ctx, who, orderID)
		if err != nil {
			s.fault("webhook", err)
			return
		}
		o.UCP = &Meta{Version: q.active.Version, Status: "success", Capabilities: q.active.Relevant(CapOrder)}
		if err := w.Send(ctx, url, o, EventID(orderID, "created"), w.now()); err != nil {
			s.fault("webhook", err)
		}
	}()
}
