// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/commerce/api/billing"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/checkoutsession"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/test/ae"
)

// The adapter is driven through the server, against commerce's own checkout
// store (api/billing creates, reads, updates and reads orders for real); only
// the card charge is replaced, because it needs a live processor and is
// covered by api/billing's own tests.
type commerceHarness struct {
	*harness
	c    *Commerce
	org  *organization.Organization
	ctx  context.Context
	pays []billing.PaymentIn
	out  func(id string) billing.CompleteOut
}

const sptHandler = "spt_handler"

func newCommerceHarness(t *testing.T, mutate ...func(*Commerce)) *commerceHarness {
	t.Helper()
	ctx := ae.NewContext()
	t.Cleanup(ctx.Close)
	org := organization.New(datastore.New(ctx))
	org.Name = "ucp-" + strings.ToLower(strings.ReplaceAll(t.Name(), "/", "-"))
	if err := org.Create(); err != nil {
		t.Fatal(err)
	}
	c, err := NewCommerce(Commerce{
		Handler:   sptHandler,
		Permalink: func(id string) string { return "https://merchant.example/orders/" + id },
		Orgs:      func(context.Context, string) (*organization.Organization, error) { return org, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range mutate {
		m(c)
	}
	ch := &commerceHarness{c: c, org: org, ctx: ctx}
	// A completion that pays: the checkout becomes complete.
	c.complete = func(ctx context.Context, o *organization.Organization, subject, host, id, key string, pay billing.PaymentIn) billing.CompleteOut {
		ch.pays = append(ch.pays, pay)
		if ch.out != nil {
			return ch.out(id)
		}
		checkoutsession.Change(id, func(x *checkoutsession.Session) bool {
			x.Status, x.Key, x.PaymentID, x.CompletedAt = checkoutsession.Complete, key, "pay_1", time.Now()
			return true
		})
		v, _ := billing.ReadCheckout(ctx, o, subject, id)
		return billing.CompleteOut{Status: 200, View: v}
	}
	ch.harness = newHarness(t, map[string][]Handler{"com.example.spt": {{
		Entity:               Entity{ID: sptHandler, Version: Version},
		AvailableInstruments: []AvailableInstrument{{Type: "card"}},
	}}})
	ch.srv.cfg.Backend = c
	ch.srv.orders = c
	ch.identify = func(*http.Request) (Caller, error) { return Caller{Org: org.Name, Subject: "buyer-1"}, nil }
	ch.peers.m[platformURL] = platform(Version, CapCheckout, CapOrder)
	return ch
}

func (h *commerceHarness) req(method, path string, body any, key string) *checkoutResult {
	h.t.Helper()
	var b []byte
	if body != nil {
		b, _ = json.Marshal(body)
	}
	c := call{method: method, path: path, body: b}
	if key != "" {
		c.header = map[string]string{"Idempotency-Key": key}
	}
	w := h.do(c)
	return &checkoutResult{code: w.Code, raw: w.Body.Bytes(), t: h.t}
}

type checkoutResult struct {
	code int
	raw  []byte
	t    *testing.T
}

func (r *checkoutResult) checkout() *Checkout {
	r.t.Helper()
	var c Checkout
	if err := json.Unmarshal(r.raw, &c); err != nil {
		r.t.Fatalf("%v\n%s", err, r.raw)
	}
	return &c
}

func (r *checkoutResult) errorResponse() *ErrorResponse {
	r.t.Helper()
	var e ErrorResponse
	if err := json.Unmarshal(r.raw, &e); err != nil {
		r.t.Fatalf("%v\n%s", err, r.raw)
	}
	return &e
}

// nthKey is the nth distinct idempotency key.
func nthKey(n int) string { return fmt.Sprintf("00000000-0000-4000-8000-%012d", n) }

func line(id string, qty int64) []LineItem { return []LineItem{{Item: Item{ID: id}, Quantity: qty}} }

func payment(handler, token string) *CompleteRequest {
	in := Instrument{ID: "pi_1", HandlerID: handler, Type: "card"}
	if token != "" {
		in.Credential = &Credential{Type: "stripe_token", Token: token}
	}
	return &CompleteRequest{Payment: &Payment{Instruments: []Instrument{in}}}
}

const (
	key1 = "11111111-1111-4111-8111-111111111111"
	key2 = "22222222-2222-4222-8222-222222222222"
	key3 = "33333333-3333-4333-8333-333333333333"
	key4 = "44444444-4444-4444-8444-444444444444"
)

func TestCreditIsSoldCompletedAndReadBack(t *testing.T) {
	h := newCommerceHarness(t)
	r := h.req("POST", "/checkout-sessions", CreateRequest{LineItems: line("credit-1000", 1)}, key1)
	if r.code != 201 {
		t.Fatalf("create: %d %s", r.code, r.raw)
	}
	c := r.checkout()
	if c.Status != ReadyForComplete || c.Currency != "USD" || len(c.LineItems) != 1 {
		t.Fatalf("checkout = %+v", c)
	}
	li := c.LineItems[0]
	if li.Item.ID != "credit-1000" || *li.Item.Price != 1000 || li.Quantity != 1 || li.Item.Title == "" {
		t.Fatalf("line = %+v", li)
	}
	if err := CheckTotals(c.Totals); err != nil || c.Totals[0].Amount != 1000 || c.Totals[1].Amount != 1000 {
		t.Fatalf("totals = %+v (%v): priced by commerce, from the catalog", c.Totals, err)
	}
	if err := Sum(c.Totals); err != nil {
		t.Fatal(err)
	}
	if len(c.UCP.PaymentHandlers["com.example.spt"]) != 1 || c.UCP.PaymentHandlers["com.example.spt"][0].ID != sptHandler {
		t.Fatalf("handlers = %+v", c.UCP.PaymentHandlers)
	}
	id := c.ID

	// Update replaces the item; the price follows the catalog.
	r = h.req("PUT", "/checkout-sessions/"+id, UpdateRequest{LineItems: line("credit-2500", 1)}, key2)
	if r.code != 200 || r.checkout().Totals[1].Amount != 2500 || r.checkout().LineItems[0].Item.ID != "credit-2500" {
		t.Fatalf("update: %d %s", r.code, r.raw)
	}

	// Complete with a shared payment token.
	r = h.req("POST", "/checkout-sessions/"+id+"/complete", payment(sptHandler, "spt_test_123"), key3)
	if r.code != 200 {
		t.Fatalf("complete: %d %s", r.code, r.raw)
	}
	done := r.checkout()
	if done.Status != Completed || done.Order == nil || done.Order.PermalinkURL != "https://merchant.example/orders/"+done.Order.ID {
		t.Fatalf("completed = %+v", done)
	}
	if len(h.pays) != 1 || h.pays[0].Type != "shared_payment_token" || h.pays[0].Token != "spt_test_123" {
		t.Fatalf("payment handed to commerce = %+v", h.pays)
	}
	// The same completion again is the same purchase: one charge, the same answer.
	again := h.req("POST", "/checkout-sessions/"+id+"/complete", payment(sptHandler, "spt_test_123"), key3)
	if again.code != 200 || string(again.raw) != string(r.raw) || len(h.pays) != 1 {
		t.Fatalf("a duplicate completion: %d, %d charges", again.code, len(h.pays))
	}
	// A completed checkout is immutable.
	r = h.req("PUT", "/checkout-sessions/"+id, UpdateRequest{LineItems: line("credit-500", 1)}, key4)
	if m := r.checkout().Messages; r.code != 200 || len(m) == 0 || m[len(m)-1].Code != "not_modifiable" || r.checkout().Status != Completed {
		t.Fatalf("update after completion: %d %s", r.code, r.raw)
	}
	// Reading it back gives the same checkout.
	r = h.req("GET", "/checkout-sessions/"+id, nil, "")
	if g := r.checkout(); r.code != 200 || g.Status != Completed || g.Order.ID != done.Order.ID {
		t.Fatalf("get: %d %s", r.code, r.raw)
	}
	// The order.
	r = h.req("GET", "/orders/"+done.Order.ID, nil, "")
	var o Order
	if err := json.Unmarshal(r.raw, &o); err != nil || r.code != 200 {
		t.Fatalf("order: %d %v %s", r.code, err, r.raw)
	}
	if err := o.Check(); err != nil {
		t.Fatal(err)
	}
	if o.CheckoutID != id || o.PermalinkURL != done.Order.PermalinkURL || o.LineItems[0].Item.ID != "credit-2500" || o.LineItems[0].Status != LineFulfilled ||
		o.Totals[1].Amount != 2500 || o.Currency != "USD" {
		t.Fatalf("order = %+v", o)
	}
}

func TestAPlanIsSoldByItsSlug(t *testing.T) {
	h := newCommerceHarness(t)
	plans, err := billing.ReadPlans(h.ctx, "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	var slug, name string
	var price int64
	for _, p := range plans {
		if p.Price > 0 && !p.ContactSales {
			slug, name, price = p.Slug, p.Name, p.Price
			break
		}
	}
	if slug == "" {
		t.Skip("the catalog sells no paid plan on the default host")
	}
	r := h.req("POST", "/checkout-sessions", CreateRequest{LineItems: line("plan-"+slug, 1)}, key1)
	if r.code != 201 {
		t.Fatalf("create: %d %s", r.code, r.raw)
	}
	c := r.checkout()
	if c.LineItems[0].Item.ID != "plan-"+slug || c.LineItems[0].Item.Title != name || c.Totals[1].Amount != price {
		t.Fatalf("plan checkout = %+v; the catalog's name and price", c.LineItems[0])
	}
}

func TestItemsTheCatalogDoesNotSellAreRefusedAsUnavailable(t *testing.T) {
	h := newCommerceHarness(t)
	n := 100
	for name, tc := range map[string]struct {
		lines []LineItem
		code  string
	}{
		"not an item":        {line("shoes", 1), "item_unavailable"},
		"bad credit amount":  {line("credit-abc", 1), "item_unavailable"},
		"credit padded":      {line("credit-0100", 1), "item_unavailable"},
		"credit below floor": {line("credit-1", 1), "item_unavailable"},
		"unknown plan":       {line("plan-no-such-plan", 1), "item_unavailable"},
		"empty plan":         {line("plan-", 1), "item_unavailable"},
		"two of a kind":      {line("credit-1000", 2), "quantity_unsupported"},
		"two lines":          {append(line("credit-1000", 1), line("credit-500", 1)...), "too_many_items"},
		"by the pound": {[]LineItem{{Item: Item{ID: "credit-1000", QuantityUnit: &QuantityUnit{Descriptor: Descriptor{Unit: "LBR", DisplayText: "lb"}}}, Quantity: 1}},
			"quantity_unit_mismatch"},
	} {
		t.Run(name, func(t *testing.T) {
			n++
			r := h.req("POST", "/checkout-sessions", CreateRequest{LineItems: tc.lines}, nthKey(n))
			if r.code != 200 {
				t.Fatalf("status %d: a refusal is a business outcome\n%s", r.code, r.raw)
			}
			e := r.errorResponse()
			if e.UCP.Status != "error" || e.Messages[0].Code != tc.code || e.Messages[0].Severity != Unrecoverable {
				t.Fatalf("response = %+v", e)
			}
		})
	}
	// The same asserted default unit is not a mismatch.
	zero := 0
	r := h.req("POST", "/checkout-sessions", CreateRequest{LineItems: []LineItem{{Item: Item{ID: "credit-1000", QuantityUnit: &QuantityUnit{Descriptor: Descriptor{Unit: "C62", Scale: &zero, DisplayText: "each"}}}, Quantity: 1}}}, key2)
	if r.code != 201 {
		t.Fatalf("an explicit each: %d %s", r.code, r.raw)
	}
}

func TestAnotherBuyersCheckoutIsNotFound(t *testing.T) {
	h := newCommerceHarness(t)
	r := h.req("POST", "/checkout-sessions", CreateRequest{LineItems: line("credit-1000", 1)}, key1)
	id := r.checkout().ID
	h.identify = func(*http.Request) (Caller, error) { return Caller{Org: h.org.Name, Subject: "buyer-2"}, nil }
	for _, c := range []struct{ method, path string }{{"GET", "/checkout-sessions/" + id}, {"POST", "/checkout-sessions/" + id + "/cancel"}} {
		r = h.req(c.method, c.path, nil, key2)
		if r.code != 200 || r.errorResponse().Messages[0].Code != "not_found" {
			t.Fatalf("%s: %d %s", c.method, r.code, r.raw)
		}
	}
	r = h.req("POST", "/checkout-sessions/"+id+"/complete", payment(sptHandler, "spt_x"), key3)
	if r.errorResponse().Messages[0].Code != "not_found" || len(h.pays) != 0 {
		t.Fatalf("complete by another buyer: %s, %d charges", r.raw, len(h.pays))
	}
}

func TestPaymentThatCommerceCannotHonourIsRefusedWithPaymentFailed(t *testing.T) {
	h := newCommerceHarness(t)
	// A second handler the host advertises that commerce cannot charge.
	h.srv.cfg.Profile.UCP.PaymentHandlers["com.google.pay"] = []Handler{{Entity: Entity{ID: "gpay_1234", Version: Version}}}
	id := h.req("POST", "/checkout-sessions", CreateRequest{LineItems: line("credit-1000", 1)}, key1).checkout().ID
	n := 200
	for name, tc := range map[string]struct {
		req  *CompleteRequest
		path string
	}{
		"another handler": {payment("gpay_1234", "tok"), "$.payment.instruments[0].handler_id"},
		"no credential":   {payment(sptHandler, ""), "$.payment.instruments[0].credential"},
		"unadvertised":    {payment("made_up", "spt_1"), "$.payment.instruments[0].handler_id"},
	} {
		n++
		r := h.req("POST", "/checkout-sessions/"+id+"/complete", tc.req, nthKey(n))
		c := r.checkout()
		m := c.Messages[len(c.Messages)-1]
		if r.code != 200 || c.Status != Incomplete || m.Code != "payment_failed" || m.Severity != Recoverable || m.Path != tc.path {
			t.Fatalf("%s: %d %s", name, r.code, r.raw)
		}
	}
	if len(h.pays) != 0 {
		t.Fatalf("%d charges were made for payments that cannot be honoured", len(h.pays))
	}
	// The checkout is still usable afterwards.
	r := h.req("POST", "/checkout-sessions/"+id+"/complete", payment(sptHandler, "spt_ok"), key2)
	if r.checkout().Status != Completed {
		t.Fatalf("complete after refusals: %s", r.raw)
	}
}

func TestPaymentRefusalsFromCommerceAreRecoverable(t *testing.T) {
	h := newCommerceHarness(t)
	id := h.req("POST", "/checkout-sessions", CreateRequest{LineItems: line("credit-1000", 1)}, key1).checkout().ID
	h.out = func(id string) billing.CompleteOut {
		checkoutsession.Change(id, func(x *checkoutsession.Session) bool {
			x.Status, x.Reason = checkoutsession.Failed, "token_expired: this token has expired"
			return true
		})
		v, _ := billing.ReadCheckout(h.ctx, h.org, "buyer-1", id)
		return billing.CompleteOut{Status: 402, View: v}
	}
	r := h.req("POST", "/checkout-sessions/"+id+"/complete", payment(sptHandler, "spt_old"), key2)
	c := r.checkout()
	m := c.Messages[len(c.Messages)-1]
	// The refusal is recoverable: the checkout is incomplete until it is updated,
	// and completing it as it stands is not offered.
	if again := h.req("POST", "/checkout-sessions/"+id+"/complete", payment(sptHandler, "spt_new"), nthKey(300)).checkout(); again.Status != Incomplete || again.Messages[len(again.Messages)-1].Code != "not_ready" {
		t.Fatalf("completing an incomplete checkout: %+v", again)
	}
	reopen := func(k int) {
		if r := h.req("PUT", "/checkout-sessions/"+id, UpdateRequest{LineItems: line("credit-1000", 1)}, nthKey(k)); r.checkout().Status != ReadyForComplete {
			t.Fatalf("update did not reopen the checkout: %s", r.raw)
		}
	}
	if r.code != 200 || c.Status != Incomplete || m.Code != "payment_failed" || m.Severity != Recoverable || !strings.Contains(m.Content, "token_expired") {
		t.Fatalf("refused token: %d %s", r.code, r.raw)
	}
	// A bank's request to authenticate is a handoff when there is somewhere to go.
	h.out = func(id string) billing.CompleteOut {
		checkoutsession.Change(id, func(x *checkoutsession.Session) bool {
			x.Status, x.Reason = checkoutsession.Failed, "authentication_required: the card's bank requires authentication"
			return true
		})
		v, _ := billing.ReadCheckout(h.ctx, h.org, "buyer-1", id)
		return billing.CompleteOut{Status: 402, View: v}
	}
	reopen(301)
	r = h.req("POST", "/checkout-sessions/"+id+"/complete", payment(sptHandler, "spt_3ds"), key3)
	c = r.checkout()
	if c.Status != Incomplete || c.Messages[len(c.Messages)-1].Code != "payment_failed" {
		t.Fatalf("without a handoff URL the refusal is a payment failure: %s", r.raw)
	}
	h.c.Continue = func(id string) string { return "https://merchant.example/checkout/" + id }
	reopen(302)
	r = h.req("POST", "/checkout-sessions/"+id+"/complete", payment(sptHandler, "spt_3ds"), key4)
	c = r.checkout()
	m = c.Messages[len(c.Messages)-1]
	if c.Status != RequiresEscalation || c.ContinueURL != "https://merchant.example/checkout/"+id || m.Code != "requires_3ds" || m.Severity != RequiresBuyerInput {
		t.Fatalf("authentication handoff: %s", r.raw)
	}
}

func TestCancelAndWhatCannotBeCanceled(t *testing.T) {
	h := newCommerceHarness(t)
	id := h.req("POST", "/checkout-sessions", CreateRequest{LineItems: line("credit-1000", 1)}, key1).checkout().ID
	r := h.req("POST", "/checkout-sessions/"+id+"/cancel", nil, key2)
	if c := r.checkout(); r.code != 200 || c.Status != Canceled {
		t.Fatalf("cancel: %d %s", r.code, r.raw)
	}
	// Canceled is terminal: no update, no completion, no second cancel.
	r = h.req("PUT", "/checkout-sessions/"+id, UpdateRequest{LineItems: line("credit-500", 1)}, key3)
	if c := r.checkout(); c.Status != Canceled || c.Messages[len(c.Messages)-1].Code != "not_modifiable" {
		t.Fatalf("update canceled: %s", r.raw)
	}
	r = h.req("POST", "/checkout-sessions/"+id+"/complete", payment(sptHandler, "spt_1"), key4)
	if c := r.checkout(); c.Status != Canceled || len(h.pays) != 0 {
		t.Fatalf("complete canceled: %s", r.raw)
	}
	r = h.req("POST", "/checkout-sessions/"+id+"/cancel", nil, "66666666-6666-4666-8666-666666666666")
	if c := r.checkout(); c.Status != Canceled || c.Messages[len(c.Messages)-1].Code != "not_cancelable" {
		t.Fatalf("cancel twice: %s", r.raw)
	}
	// A completed checkout is not canceled either.
	id2 := h.req("POST", "/checkout-sessions", CreateRequest{LineItems: line("credit-500", 1)}, "77777777-7777-4777-8777-777777777777").checkout().ID
	h.req("POST", "/checkout-sessions/"+id2+"/complete", payment(sptHandler, "spt_1"), "88888888-8888-4888-8888-888888888888")
	r = h.req("POST", "/checkout-sessions/"+id2+"/cancel", nil, "99999999-9999-4999-8999-999999999999")
	if c := r.checkout(); c.Status != Completed || c.Messages[len(c.Messages)-1].Code != "not_cancelable" {
		t.Fatalf("cancel completed: %s", r.raw)
	}
}

func TestAnUnknownCallerIsUnauthorized(t *testing.T) {
	h := newCommerceHarness(t, func(c *Commerce) {
		c.Orgs = func(context.Context, string) (*organization.Organization, error) { return nil, http.ErrNoCookie }
	})
	r := h.req("POST", "/checkout-sessions", CreateRequest{LineItems: line("credit-1000", 1)}, key1)
	if r.code != 401 {
		t.Fatalf("status %d: an org that does not resolve is not authenticated", r.code)
	}
	if _, err := NewCommerce(Commerce{}); err == nil {
		t.Fatal("a Commerce without Permalink was accepted")
	}
}

func TestCommerceIdempotencyStore(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	var s Idempotency
	seen, err := s.Begin(ctx, "scope", "key-1", "digest-a")
	if err != nil || seen != nil {
		t.Fatalf("first sighting: %v %v", seen, err)
	}
	seen, err = s.Begin(ctx, "scope", "key-1", "digest-a")
	if err != nil || seen == nil || seen.Done || seen.Digest != "digest-a" {
		t.Fatalf("in flight: %+v %v", seen, err)
	}
	if err := s.Finish(ctx, "scope", "key-1", "digest-a", Stored{Status: 201, ContentType: "application/json", Body: []byte(`{"id":"chk_1"}`)}); err != nil {
		t.Fatal(err)
	}
	seen, err = s.Begin(ctx, "scope", "key-1", "digest-a")
	if err != nil || seen == nil || !seen.Done || seen.Response.Status != 201 || string(seen.Response.Body) != `{"id":"chk_1"}` || seen.Digest != "digest-a" {
		t.Fatalf("replay: %+v %v", seen, err)
	}
	// Another scope is another operation.
	if seen, _ := s.Begin(ctx, "other", "key-1", "digest-a"); seen != nil {
		t.Fatal("keys are scoped")
	}
	// A released key can be used again.
	if err := s.Release(ctx, "other", "key-1"); err != nil {
		t.Fatal(err)
	}
	if seen, err := s.Begin(ctx, "other", "key-1", "digest-b"); err != nil || seen != nil {
		t.Fatalf("after release: %+v %v", seen, err)
	}
}

// Records are kept for the time the specification gives (at least 24 hours,
// 48 recommended) and no longer: an old key is a new key, and Sweep removes
// what nothing will ask about again.
func TestCommerceIdempotencyRecordsExpire(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	now := time.Now()
	s := Idempotency{Now: func() time.Time { return now }}
	for _, k := range []string{"old-1", "old-2"} {
		if seen, err := s.Begin(ctx, "scope", k, "d"); seen != nil || err != nil {
			t.Fatalf("%s: %v %v", k, seen, err)
		}
		if err := s.Finish(ctx, "scope", k, "d", Stored{Status: 200, Body: []byte(`{}`)}); err != nil {
			t.Fatal(err)
		}
	}
	rows := func() int {
		n, err := idempotencykey.Query(s.db(ctx)).Count()
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	// Inside the retention nothing is forgotten or swept.
	now = now.Add(Keep - time.Hour)
	if seen, _ := s.Begin(ctx, "scope", "old-1", "d"); seen == nil || !seen.Done {
		t.Fatal("a record inside the retention was forgotten")
	}
	if n, err := s.Sweep(ctx, 100); err != nil || n != 0 || rows() != 2 {
		t.Fatalf("Sweep inside the retention: removed %d (%v), %d rows", n, err, rows())
	}
	// Past it, the same key is a new request.
	now = now.Add(2 * time.Hour)
	if seen, err := s.Begin(ctx, "scope", "old-1", "d"); seen != nil || err != nil {
		t.Fatalf("an expired key was still remembered: %v %v", seen, err)
	}
	// And what nobody asks about again is swept.
	if n, err := s.Sweep(ctx, 100); err != nil || n < 1 || rows() != 0 {
		t.Fatalf("Sweep past the retention: removed %d (%v), %d rows left", n, err, rows())
	}
	if _, err := (Idempotency{Keep: time.Hour}).Begin(ctx, "scope", "k", "d"); err == nil {
		t.Fatal("a retention under 24 hours was accepted")
	}
}

// A plan is charged at the live catalog price when the subscription is created,
// which can differ from the total the checkout quoted. Completion must refuse
// to charge a price the buyer's platform was not shown.
func TestPlanQuoteIsBoundToTheCharge(t *testing.T) {
	h := newCommerceHarness(t)
	plans, err := billing.ReadPlans(h.ctx, "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	var slug string
	var price int64
	for _, p := range plans {
		if p.Price > 100 && !p.ContactSales {
			slug, price = p.Slug, p.Price
			break
		}
	}
	if slug == "" {
		t.Skip("the catalog sells no paid plan on the default host")
	}
	id := h.req("POST", "/checkout-sessions", CreateRequest{LineItems: line("plan-"+slug, 1)}, nthKey(1)).checkout().ID
	// The catalog has moved since the quote: the session holds an older total.
	if _, _, err := checkoutsession.Change(id, func(s *checkoutsession.Session) bool { s.TotalCents = price - 100; return true }); err != nil {
		t.Fatal(err)
	}
	r := h.req("POST", "/checkout-sessions/"+id+"/complete", payment(sptHandler, "spt_1"), nthKey(2))
	c := r.checkout()
	if len(c.Messages) == 0 {
		t.Fatalf("a stale quote was completed: %d %s", r.code, r.raw)
	}
	m := c.Messages[len(c.Messages)-1]
	if r.code != 200 || c.Status != Incomplete || m.Code != "totals_changed" || m.Severity != Recoverable || m.Path != "$.totals" {
		t.Fatalf("a stale quote: %d %s", r.code, r.raw)
	}
	if c.Totals[1].Amount != price-100 {
		t.Fatalf("the response must still show what was quoted: %+v", c.Totals)
	}
	if len(h.pays) != 0 {
		t.Fatalf("%d charges were made at a price the buyer was not shown", len(h.pays))
	}
	// Update re-quotes at the live price; the buyer's platform sees it, and then
	// the completion is the same price it was shown.
	r = h.req("PUT", "/checkout-sessions/"+id, UpdateRequest{LineItems: line("plan-"+slug, 1)}, nthKey(3))
	if u := r.checkout(); u.Totals[1].Amount != price || u.Status != ReadyForComplete {
		t.Fatalf("update: %s", r.raw)
	}
	r = h.req("POST", "/checkout-sessions/"+id+"/complete", payment(sptHandler, "spt_1"), nthKey(4))
	if r.checkout().Status != Completed || len(h.pays) != 1 {
		t.Fatalf("complete at the quoted price: %s", r.raw)
	}
}
