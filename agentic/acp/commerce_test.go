// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package acp

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"testing"

	"github.com/hanzoai/commerce/api/billing"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/checkoutsession"
	"github.com/hanzoai/commerce/models/idempotencykey"
	orgpkg "github.com/hanzoai/commerce/pkg/org"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

func TestCommerceIsABackend(t *testing.T) {
	var b Backend = Commerce{}
	if b.Currency() != "usd" {
		t.Fatal(b.Currency())
	}
}

// What commerce answers a completion with is stated in the spec's vocabulary.
func TestCommerceRefusalsAreStatedInTheSpecsVocabulary(t *testing.T) {
	p := Product{ID: "credit-500", Name: "$5.00 USD credit"}
	view := func(status string, e *billing.AgentError) *billing.CheckoutView {
		return &billing.CheckoutView{ID: "chk_1", Status: status, TotalCents: 500, Error: e, Items: []billing.ItemIn{{Kind: "credit", UsdCents: 500}}}
	}
	for _, tc := range []struct {
		name   string
		out    billing.CompleteOut
		fault  int    // the fault's status, when the answer is a fault
		code   string // the fault's code
		refuse string // the Refusal's code, when the answer is a failed checkout
		param  string
	}{
		{"a declined card", billing.CompleteOut{Status: 402, View: view("failed", &billing.AgentError{Code: "declined", Message: "Your card was declined."})}, 0, "", "payment_declined", ""},
		{"a revoked token", billing.CompleteOut{Status: 402, View: view("failed", &billing.AgentError{Code: "token_revoked", Message: "the token is revoked"})}, 0, "", "payment_declined", ""},
		{"an expired token", billing.CompleteOut{Status: 402, View: view("failed", &billing.AgentError{Code: "token_expired", Message: "the token has expired"})}, 0, "", "payment_declined", ""},
		{"a token over its limit", billing.CompleteOut{Status: 402, View: view("failed", &billing.AgentError{Code: "token_over_limit", Message: "the amount is over what the token has left"})}, 0, "", "payment_declined", ""},
		{"a token of another seller", billing.CompleteOut{Status: 402, View: view("failed", &billing.AgentError{Code: "token_wrong_merchant", Message: "not granted to this seller"})}, 0, "", "payment_declined", ""},
		{"a token that is not one", billing.CompleteOut{Status: 402, View: view("failed", &billing.AgentError{Code: "token_invalid", Message: "not a token"})}, 0, "", "invalid", "$.payment_data.instrument.credential.token"},
		{"a bank that wants authentication", billing.CompleteOut{Status: 402, View: view("failed", &billing.AgentError{Code: "authentication_required", Message: "3ds"})}, 0, "", "intervention_required", ""},
		{"payments that are not available", billing.CompleteOut{Status: 402, View: view("failed", &billing.AgentError{Code: "payments_unavailable", Message: "cannot accept tokens"})}, 0, "", "payment_declined", ""},
		{"a sale that was refused", billing.CompleteOut{Status: 422, View: view("failed", &billing.AgentError{Code: "sale_refused", Message: "the plan is not sold"})}, 0, "", "payment_declined", ""},
		{"a quote the catalog has moved past", billing.CompleteOut{Status: 409, View: view("failed", &billing.AgentError{Code: "price_changed", Message: "the price is now 900"})}, 0, "", "conflict", ""},
		{"a checkout completed under another key", billing.CompleteOut{Status: 409, View: view("complete", &billing.AgentError{Code: "already_completed", Message: "another key"})}, 409, "conflict", "", ""},
		{"a checkout being completed", billing.CompleteOut{Status: 409, View: view("pending", &billing.AgentError{Code: "in_progress", Message: "being completed"})}, 409, "conflict", "", ""},
		{"a checkout that is gone", billing.CompleteOut{Status: 404, View: view("failed", &billing.AgentError{Code: "not_found", Message: "checkout not found"})}, 404, "not_found", "", ""},
		{"billing unavailable", billing.CompleteOut{Status: 503, View: view("failed", &billing.AgentError{Code: "unavailable", Message: "try again"})}, 503, "service_unavailable", "", ""},
		{"a processor outage", billing.CompleteOut{Status: 502, View: view("failed", &billing.AgentError{Code: "payment_failed", Message: "the processor is down"})}, 503, "service_unavailable", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := outcome(tc.out, p)
			if tc.fault != 0 {
				var f *Fault
				if !errors.As(err, &f) || f.Status != tc.fault || f.Body.Code != tc.code {
					t.Fatalf("err = %v; want a %d %s fault", err, tc.fault, tc.code)
				}
				return
			}
			if err != nil || c.State != Failed || c.Refusal == nil || c.Refusal.Code != tc.refuse || c.Refusal.Param != tc.param || c.Refusal.Message == "" {
				t.Fatalf("outcome = %+v %+v %v; want a failed checkout refused as %s", c, c.Refusal, err, tc.refuse)
			}
			if c.Refusal.Resolution == "" {
				t.Fatal("a refusal without a resolution")
			}
		})
	}

	// A paid checkout, and a payment waiting on a deposit.
	paid := view("complete", nil)
	paid.Order = &billing.OrderView{ID: "chk_1", CheckoutID: "chk_1", Status: "paid", TotalCents: 500}
	c, err := outcome(billing.CompleteOut{Status: 200, View: paid}, p)
	if err != nil || c.State != Paid || c.Receipt == nil || c.Receipt.ID != "chk_1" || c.Total != 500 || c.Item != p {
		t.Fatalf("paid = %+v %v", c, err)
	}
	if c, err = outcome(billing.CompleteOut{Status: 202, View: view("pending", nil)}, p); err != nil || c.State != Paying {
		t.Fatalf("pending = %+v %v", c, err)
	}
	// What commerce must not answer: a request ACP never makes, or an answer with no checkout.
	for _, bad := range []billing.CompleteOut{
		{Status: 400, View: view("failed", &billing.AgentError{Code: "idempotency_key_required"})},
		{Status: 400, View: view("failed", &billing.AgentError{Code: "payment_type_unknown"})},
		{Status: 200, View: view("open", nil)},
		{Status: 200},
	} {
		if _, err := outcome(bad, p); err == nil {
			t.Errorf("%+v accepted", bad)
		}
	}
}

func TestCommerceFailuresAreFaultsOfTheRightKind(t *testing.T) {
	for _, tc := range []struct {
		in     int
		status int
		code   string
	}{
		{401, 401, "unauthorized"}, {404, 404, "not_found"}, {409, 409, "conflict"}, {422, 400, "unsupported"},
		{429, 429, "rate_limited"}, {400, 400, "invalid"}, {403, 400, "invalid"}, {500, 503, "service_unavailable"}, {503, 503, "service_unavailable"},
	} {
		err := refused(&billing.PaymentFault{Status: tc.in, Message: "m"})
		var f *Fault
		if !errors.As(err, &f) || f.Status != tc.status || f.Body.Code != tc.code || f.Body.Message != "m" {
			t.Errorf("%d: %v; want %d %s", tc.in, err, tc.status, tc.code)
		}
	}
}

func TestCommerceChecksOutThroughTheRealCheckoutSurface(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	who := Caller{Org: "acp-seller", Subject: "agent-buyer", Host: ""}
	org, err := orgpkg.Resolve(ctx, who.Org)
	if err != nil {
		t.Fatal(err)
	}
	cat, err := billing.ReadCatalog(ctx, org, who.Host)
	if err != nil || len(cat.CreditPacks) == 0 || len(cat.Plans) == 0 {
		t.Fatalf("catalog = %+v %v", cat, err)
	}
	pack, plan := cat.CreditPacks[0], cat.Plans[0]
	var c Commerce

	// A credit pack opens at the catalog's price, under the catalog's id.
	co, err := c.Open(ctx, who, pack.ID)
	if err != nil {
		t.Fatal(err)
	}
	if co.State != Open || co.Item.ID != pack.ID || co.Total != pack.UsdCents || co.Receipt != nil || co.Refusal != nil || co.Item.Name == "" {
		t.Fatalf("open = %+v", co)
	}
	got, err := c.Read(ctx, who, co.ID)
	if err != nil || got.ID != co.ID || got.Item != co.Item || got.Total != co.Total || got.State != Open {
		t.Fatalf("read = %+v %v", got, err)
	}

	// Change replaces the item with a plan, at the plan's price, under its slug and name.
	ch, err := c.Change(ctx, who, co.ID, plan.Slug)
	if err != nil || ch.Item.ID != plan.Slug || ch.Item.Name != plan.Name || ch.Total != plan.Price {
		t.Fatalf("change = %+v %v", ch, err)
	}
	if got, _ = c.Read(ctx, who, co.ID); got.Item.Name != plan.Name {
		t.Fatalf("read after change = %+v", got)
	}

	// An item the seller does not sell is ErrNoItem, and opens nothing.
	for _, id := range []string{"nope", "credit-1", "credit-999999999", ""} {
		if _, err := c.Open(ctx, who, id); !errors.Is(err, ErrNoItem) {
			t.Errorf("Open(%q) = %v", id, err)
		}
	}
	if _, err := c.Change(ctx, who, co.ID, "nope"); !errors.Is(err, ErrNoItem) {
		t.Errorf("Change to nope = %v", err)
	}

	// Another buyer cannot see it.
	other := who
	other.Subject = "someone-else"
	for name, f := range map[string]func() error{
		"read":   func() error { _, err := c.Read(ctx, other, co.ID); return err },
		"change": func() error { _, err := c.Change(ctx, other, co.ID, pack.ID); return err },
		"cancel": func() error { _, err := c.Cancel(ctx, other, co.ID); return err },
		"pay":    func() error { _, err := c.Pay(ctx, other, co.ID, "k", spt()); return err },
	} {
		var fl *Fault
		if err := f(); !errors.As(err, &fl) || fl.Status != http.StatusNotFound {
			t.Errorf("%s by another buyer = %v", name, err)
		}
	}

	// What commerce cannot take is refused before anything is charged.
	for name, bad := range map[string]Charge{
		"another psp":        {Handler: PaymentHandler{PSP: "adyen"}, Instrument: "card", Credential: "spt", Token: "spt_1"},
		"another instrument": {Handler: stripeHandler(), Instrument: "bank", Credential: "spt", Token: "spt_1"},
		"another credential": {Handler: stripeHandler(), Instrument: "card", Credential: "network_token", Token: "spt_1"},
		"a vault token":      {Handler: stripeHandler(), Instrument: "card", Credential: "spt", Token: "vt_01J8Z3WXYZ9ABC"},
		"a raw card number":  {Handler: stripeHandler(), Instrument: "card", Credential: "spt", Token: "4242424242424242"},
	} {
		var fl *Fault
		if _, err := c.Pay(ctx, who, co.ID, "k", bad); !errors.As(err, &fl) || fl.Body.Code != "unsupported" {
			t.Errorf("%s: %v", name, err)
		}
	}
	if got, _ = c.Read(ctx, who, co.ID); got.State != Open {
		t.Fatalf("a refused payment changed the checkout: %+v", got)
	}

	// Paying with a token on a deployment that cannot read tokens leaves the checkout
	// usable, with the reason; it is never a success.
	paid, err := c.Pay(ctx, who, co.ID, "key-1", spt())
	if err != nil || paid.State != Failed || paid.Receipt != nil || paid.Refusal == nil || paid.Refusal.Code != "payment_declined" {
		t.Fatalf("pay = %+v %+v %v", paid, paid.Refusal, err)
	}
	if got, _ = c.Read(ctx, who, co.ID); got.State != Failed || got.Refusal == nil || got.Refusal.Code != "payment_declined" {
		t.Fatalf("read after a refused payment = %+v", got)
	}

	// A failed checkout can be changed, then canceled, once; a canceled one can do nothing.
	if _, err := c.Change(ctx, who, co.ID, pack.ID); err != nil {
		t.Fatal(err)
	}
	can, err := c.Cancel(ctx, who, co.ID)
	if err != nil || can.State != Canceled || can.Item.ID != pack.ID {
		t.Fatalf("cancel = %+v %v", can, err)
	}
	var fl *Fault
	if _, err := c.Cancel(ctx, who, co.ID); !errors.As(err, &fl) || fl.Status != http.StatusMethodNotAllowed || fl.Body.Code != "not_cancelable" {
		t.Fatalf("second cancel = %v", err)
	}
	if _, err := c.Change(ctx, who, co.ID, pack.ID); !errors.As(err, &fl) || fl.Status != http.StatusConflict {
		t.Fatalf("change after cancel = %v", err)
	}
	if _, err := c.Pay(ctx, who, co.ID, "key-2", spt()); !errors.As(err, &fl) || fl.Status != http.StatusConflict || fl.Body.Code != "conflict" {
		t.Fatalf("pay after cancel = %v", err)
	}
	if got, _ = c.Read(ctx, who, co.ID); got.State != Canceled {
		t.Fatalf("read after cancel = %+v", got)
	}

	// A completed checkout cannot be canceled: the model is the source of truth.
	done, err := c.Open(ctx, who, pack.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := checkoutsession.Change(done.ID, func(s *checkoutsession.Session) bool { s.Status = checkoutsession.Complete; return true }); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Cancel(ctx, who, done.ID); !errors.As(err, &fl) || fl.Status != http.StatusMethodNotAllowed {
		t.Fatalf("cancel of a complete checkout = %v", err)
	}
}

func stripeHandler() PaymentHandler { return PaymentHandler{ID: "card_tokenized", PSP: "stripe"} }

func spt() Charge {
	return Charge{Handler: stripeHandler(), Instrument: "card", Credential: "spt", Token: "spt_1Test"}
}

// The whole API over the real checkout surface: a session is opened, read, changed, paid
// (the deployment has no Stripe, so the payment is refused as a payment_declined message),
// and canceled, and every answer is valid by the spec's schema.
func TestTheAPIOverCommerce(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	who := Caller{Org: "acp-api-seller", Subject: "agent-buyer", Host: ""}
	org, err := orgpkg.Resolve(ctx, who.Org)
	if err != nil {
		t.Fatal(err)
	}
	cat, _ := billing.ReadCatalog(ctx, org, who.Host)
	pack := cat.CreditPacks[0]
	h, _ := TokenizedCard("card_tokenized", "acct_test", "stripe", true, nil, nil)
	svc, err := New(Config{
		Backend:  Commerce{},
		Identify: func(*http.Request) (Caller, error) { return who, nil },
		Handlers: func(context.Context, Caller) ([]PaymentHandler, error) { return []PaymentHandler{h}, nil },
		Order:    func(id string) string { return "https://pay.example/orders/" + id },
		Ledger:   Durable{},
	})
	if err != nil {
		t.Fatal(err)
	}
	r := &rig{t: t, svc: svc, h: svc.API(), be: newFake()}
	body := `{"currency":"usd","line_items":[{"id":"` + pack.ID + `","unit_amount":` + strconv.FormatInt(pack.UsdCents, 10) + `}],"capabilities":{}}`
	w := r.post("/checkout_sessions", body)
	if w.Code != 201 {
		t.Fatalf("create = %d %s", w.Code, w.Body)
	}
	validSession(t, w, "CheckoutSession")
	id := w.get("id").(string)
	if w.get("status") != "ready_for_payment" || len(w.get("messages").([]any)) != 0 || w.get("totals").([]any)[2].(map[string]any)["amount"] != float64(pack.UsdCents) {
		t.Fatalf("create = %s", w.Body)
	}
	// The same key and body replays through the durable ledger: one checkout.
	if again := r.post("/checkout_sessions", body); again.Header().Get("Idempotent-Replayed") != "true" || again.get("id") != id {
		t.Fatalf("replay = %d %s", again.Code, again.Body)
	}
	if w = r.get("/checkout_sessions/" + id); w.Code != 200 {
		t.Fatalf("get = %d %s", w.Code, w.Body)
	}
	validSession(t, w, "CheckoutSession")

	w = r.post("/checkout_sessions/"+id+"/complete", completeBody)
	if w.Code != 200 {
		t.Fatalf("complete = %d %s", w.Code, w.Body)
	}
	validSession(t, w, "CheckoutSession")
	m := w.get("messages").([]any)[0].(map[string]any)
	if w.get("status") != "ready_for_payment" || m["type"] != "error" || m["code"] != "payment_declined" || w.json()["order"] != nil {
		t.Fatalf("complete = %s", w.Body)
	}

	// The same key with another body is the specified conflict, from the durable ledger.
	k := map[string]string{"Idempotency-Key": "durable-1"}
	r.post("/checkout_sessions/"+id, `{"line_items":[{"id":"`+pack.ID+`"}]}`, k)
	wantErr(t, r.post("/checkout_sessions/"+id, `{"line_items":[{"id":"nope"}]}`, k), 422, "invalid_request", "idempotency_conflict", "")

	w = r.post("/checkout_sessions/"+id+"/cancel", spec(t, "cancel_checkout_session_request"))
	if w.Code != 200 || w.get("status") != "canceled" {
		t.Fatalf("cancel = %d %s", w.Code, w.Body)
	}
	validSession(t, w, "CheckoutSession")
	wantErr(t, r.post("/checkout_sessions/"+id+"/cancel", `{}`), 405, "invalid_request", "not_cancelable", "")
	wantErr(t, r.get("/checkout_sessions/chk_nonexistent"), 404, "invalid_request", "not_found", "")
	wantErr(t, r.post("/checkout_sessions", `{"currency":"usd","line_items":[{"id":"nope"}],"capabilities":{}}`), 400, "invalid_request", "invalid_item_id", "$.line_items[0].item.id")
}

func TestDurableLedger(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	var l Durable
	v, _, lease, err := l.Begin(ctx, "acme\x00u\x00POST /checkout_sessions", "k1", "print-a")
	if err != nil || v != Fresh || lease == nil {
		t.Fatalf("first = %v %v", v, err)
	}
	if v, _, _, _ := l.Begin(ctx, "acme\x00u\x00POST /checkout_sessions", "k1", "print-a"); v != Busy {
		t.Fatalf("running: %v", v)
	}
	if err := lease.Settle(ctx, Stored{Status: 201, Body: []byte(`{"id":"checkout_session_1"}`)}); err != nil {
		t.Fatal(err)
	}
	v, got, _, err := l.Begin(ctx, "acme\x00u\x00POST /checkout_sessions", "k1", "print-a")
	if err != nil || v != Replay || got.Status != 201 || string(got.Body) != `{"id":"checkout_session_1"}` {
		t.Fatalf("replay = %v %+v %v", v, got, err)
	}
	if v, _, _, _ := l.Begin(ctx, "acme\x00u\x00POST /checkout_sessions", "k1", "print-b"); v != Mismatch {
		t.Fatalf("other body: %v", v)
	}
	if v, _, _, _ := l.Begin(ctx, "acme\x00u\x00POST /checkout_sessions/x", "k1", "print-a"); v != Fresh {
		t.Fatalf("other scope: %v", v)
	}
	// A release frees the key.
	_, _, l2, _ := l.Begin(ctx, "s", "k2", "p")
	if err := l2.Release(ctx); err != nil {
		t.Fatal(err)
	}
	if v, _, _, _ := l.Begin(ctx, "s", "k2", "p"); v != Fresh {
		t.Fatalf("released: %v", v)
	}
}

// A request whose guard was reclaimed after the stale interval must not free or settle the
// guard of the request that reclaimed it.
func TestDurableLeaseFreesAndSettlesOnlyItsOwnGuard(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	var l Durable
	scope, key := "acme\x00u\x00POST /checkout_sessions", "k-stale"
	_, _, first, err := l.Begin(ctx, scope, key, "p")
	if err != nil {
		t.Fatal(err)
	}
	// The first request stalls; its guard is reclaimed by a second one. The reclaim replaces
	// the row under the same id, as idempotencykey.Begin does for a guard past its TTL.
	db := datastore.New(nscontext.WithNamespace(ctx, "system"))
	old := idempotencykey.New(db)
	if err := old.Get(db.NewKey(old.Kind(), idempotencykey.DeterministicID(scope, key), 0, nil)); err != nil {
		t.Fatal(err)
	}
	if err := old.Delete(); err != nil {
		t.Fatal(err)
	}
	v, _, second, err := l.Begin(ctx, scope, key, "p")
	if err != nil || v != Fresh {
		t.Fatalf("reclaim = %v %v", v, err)
	}
	// The first request finishes late: its release and its settle touch nothing of the second's.
	_ = first.Release(ctx)
	if v, _, _, _ := l.Begin(ctx, scope, key, "p"); v != Busy {
		t.Fatalf("a release by the first request freed the second's guard: %v", v)
	}
	if err := first.Settle(ctx, Stored{Status: 201, Body: []byte(`{"id":"late"}`)}); err == nil {
		t.Fatal("a stale lease settled a guard it no longer holds")
	}
	if v, _, _, _ := l.Begin(ctx, scope, key, "p"); v != Busy {
		t.Fatalf("a settle by the first request completed the second's guard: %v", v)
	}
	// The holder settles and releases its own.
	if err := second.Settle(ctx, Stored{Status: 201, Body: []byte(`{"id":"mine"}`)}); err != nil {
		t.Fatal(err)
	}
	v, got, _, _ := l.Begin(ctx, scope, key, "p")
	if v != Replay || string(got.Body) != `{"id":"mine"}` {
		t.Fatalf("after the holder settled: %v %+v", v, got)
	}
}
