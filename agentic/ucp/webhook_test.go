// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// platformHook is a platform's webhook endpoint: it verifies each delivery as
// the specification requires of a platform and records what it accepted.
type platformHook struct {
	t        *testing.T
	verifier *Verifier
	mu       sync.Mutex
	fail     int // answer 503 this many times first
	status   int // when set, answer this status to every delivery
	seen     []delivery
	got      chan delivery
}

type delivery struct {
	id, timestamp string
	order         Order
	covered       string
}

func (p *platformHook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	signed, err := p.verifier.Request(context.Background(), r, b)
	if err != nil {
		p.t.Errorf("a delivery failed verification: %v", err)
		w.WriteHeader(401)
		return
	}
	if signed.Identity != "https://merchant.example/.well-known/ucp" {
		p.t.Errorf("signer identity = %q", signed.Identity)
	}
	var o Order
	if err := json.Unmarshal(b, &o); err != nil {
		p.t.Errorf("payload: %v", err)
	}
	d := delivery{id: r.Header.Get("Webhook-Id"), timestamp: r.Header.Get("Webhook-Timestamp"), order: o, covered: r.Header.Get("Signature-Input")}
	p.mu.Lock()
	p.seen = append(p.seen, d)
	failing := p.fail > 0
	if failing {
		p.fail--
	}
	status := p.status
	p.mu.Unlock()
	switch {
	case failing:
		w.WriteHeader(503)
	case status != 0:
		w.WriteHeader(status)
	default:
		w.WriteHeader(204)
		if p.got != nil {
			p.got <- d
		}
	}
}

func (p *platformHook) deliveries() []delivery {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]delivery(nil), p.seen...)
}

// hookServer starts the platform endpoint; the client dials it whatever the host.
func hookServer(t *testing.T, signerKey Key) (*platformHook, *http.Client) {
	t.Helper()
	v := &Verifier{Keys: &keyset{identity: "https://merchant.example/.well-known/ucp", keys: []Key{signerKey}}}
	hook := &platformHook{t: t, verifier: v, got: make(chan delivery, 8)}
	srv := httptest.NewTLSServer(hook)
	t.Cleanup(srv.Close)
	tr := srv.Client().Transport.(*http.Transport).Clone()
	tr.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", srv.Listener.Addr().String())
	}
	return hook, &http.Client{Transport: tr}
}

func sampleOrder(t *testing.T) *Order {
	t.Helper()
	var o Order
	if err := json.Unmarshal(fixture(t, "shopping-order-207.json"), &o); err != nil {
		t.Fatal(err)
	}
	return &o
}

func newWebhooks(t *testing.T) (*Webhooks, *platformHook) {
	s, pub := newKeyPair(t, ES256)
	s.KeyID = "merchant-2026"
	pub.KID = "merchant-2026"
	hook, client := hookServer(t, pub)
	return &Webhooks{Signer: s, Agent: "https://merchant.example/.well-known/ucp", Client: client, Attempts: 4, Backoff: time.Millisecond}, hook
}

// The test server's certificate is valid for example.com.
const hookURL = "https://example.com/webhooks/ucp/orders"

func TestWebhookIsSignedAndCarriesTheFullOrder(t *testing.T) {
	w, hook := newWebhooks(t)
	o := sampleOrder(t)
	at := time.Unix(1737000000, 0)
	if err := w.Send(context.Background(), hookURL, o, "evt-1", at); err != nil {
		t.Fatal(err)
	}
	ds := hook.deliveries()
	if len(ds) != 1 {
		t.Fatalf("%d deliveries", len(ds))
	}
	d := ds[0]
	if d.id != "evt-1" || d.timestamp != "1737000000" || !reflect.DeepEqual(d.order.ID, o.ID) || len(d.order.LineItems) != len(o.LineItems) {
		t.Fatalf("delivery = %+v", d)
	}
	// The signature covers the event identity: Webhook-Id and Webhook-Timestamp
	// are the only per-event replay signal.
	for _, c := range []string{`"webhook-id"`, `"webhook-timestamp"`, `"ucp-agent"`, `"content-digest"`, `"content-type"`, `"@method"`, `"@authority"`, `"@path"`} {
		if !strings.Contains(d.covered, c) {
			t.Errorf("Signature-Input %s does not cover %s", d.covered, c)
		}
	}
	if strings.Contains(d.covered, "idempotency-key") {
		t.Error("webhooks carry no Idempotency-Key")
	}
}

func TestWebhookRetriesRedeliverTheSameEvent(t *testing.T) {
	w, hook := newWebhooks(t)
	hook.fail = 2
	o := sampleOrder(t)
	if err := w.Send(context.Background(), hookURL, o, "evt-9", time.Unix(1737000100, 0)); err != nil {
		t.Fatal(err)
	}
	ds := hook.deliveries()
	if len(ds) != 3 {
		t.Fatalf("%d deliveries; two 503s then success", len(ds))
	}
	for _, d := range ds {
		if d.id != "evt-9" || d.timestamp != "1737000100" {
			t.Fatalf("a retry must keep the original Webhook-Id and Webhook-Timestamp: %+v", d)
		}
	}
}

func TestWebhookDoesNotRetryWhatRetryingCannotChange(t *testing.T) {
	w, hook := newWebhooks(t)
	hook.status = 400
	err := w.Send(context.Background(), hookURL, sampleOrder(t), "evt-2", time.Now())
	if err == nil || len(hook.deliveries()) != 1 {
		t.Fatalf("err %v after %d deliveries; a 400 is final", err, len(hook.deliveries()))
	}
	w, hook = newWebhooks(t)
	hook.status = 503
	err = w.Send(context.Background(), hookURL, sampleOrder(t), "evt-3", time.Now())
	if err == nil || len(hook.deliveries()) != 4 {
		t.Fatalf("err %v after %d deliveries; the attempts are bounded", err, len(hook.deliveries()))
	}
}

func TestWebhookURLComesFromThePlatformOrderCapability(t *testing.T) {
	p := platform(Version, CapCheckout, CapOrder)
	if _, err := webhookURL(p, Version); err == nil {
		t.Fatal("an order capability without webhook_url was accepted")
	}
	c := p.UCP.Capabilities[CapOrder][0]
	c.Config = json.RawMessage(`{"webhook_url":"https://platform.example/webhooks/ucp/orders"}`)
	p.UCP.Capabilities[CapOrder] = []Capability{c}
	if u, err := webhookURL(p, Version); err != nil || u != "https://platform.example/webhooks/ucp/orders" {
		t.Fatalf("url %q err %v", u, err)
	}
	if _, err := webhookURL(p, "2026-01-11"); err == nil {
		t.Fatal("the capability at another version was used")
	}
	for _, bad := range []string{"http://platform.example/hook", "https://127.0.0.1/hook", "https://user@platform.example/hook"} {
		c.Config = json.RawMessage(`{"webhook_url":"` + bad + `"}`)
		p.UCP.Capabilities[CapOrder] = []Capability{c}
		if _, err := webhookURL(p, Version); err == nil {
			t.Errorf("%s was accepted", bad)
		}
	}
	w := &Webhooks{Signer: &Signer{}, Agent: "https://m.example/.well-known/ucp", Client: &http.Client{}, Attempts: 1, Backoff: time.Millisecond}
	if err := w.Send(context.Background(), "http://platform.example/hook", sampleOrder(t), "e", time.Now()); err == nil {
		t.Error("a delivery to an http URL was attempted")
	}
	if err := (&Webhooks{}).Send(context.Background(), hookURL, sampleOrder(t), "e", time.Now()); err == nil {
		t.Error("an unsigned sender was accepted")
	}
}

func TestEventIDIsStable(t *testing.T) {
	if eventID("ord_1", "created") != eventID("ord_1", "created") || eventID("ord_1", "created") == eventID("ord_2", "created") || eventID("ord_1", "created") == eventID("ord_1", "shipped") {
		t.Fatal("EventID must be a stable, unique name of one event")
	}
}

// Completing a checkout announces the order to a platform that negotiated the
// Order capability and said where to send it.
func TestCompletionAnnouncesTheOrder(t *testing.T) {
	w, hook := newWebhooks(t)
	h := newHarness(t, gpay(), func(c *Config) { c.Webhooks = w })
	pl := platform(Version, CapCheckout, CapOrder)
	oc := pl.UCP.Capabilities[CapOrder][0]
	oc.Config = json.RawMessage(`{"webhook_url":"` + hookURL + `"}`)
	pl.UCP.Capabilities[CapOrder] = []Capability{oc}
	h.peers.m[platformURL] = pl
	done := fixtureCheckout(t, "shopping-checkout-rest-829.json")
	ready := *done
	ready.Status, ready.Order = ReadyForComplete, nil
	h.backend.next = &ready
	h.backend.order = sampleOrder(t)
	h.backend.order.ID = done.Order.ID
	h.srv.cfg.Backend = &sequenceBackend{fakeBackend: h.backend, then: done}

	resp := h.do(call{method: "POST", path: "/checkout-sessions/chk_1/complete", body: fixture(t, "shopping-checkout-rest-786.json")})
	if resp.Code != 200 {
		t.Fatalf("complete: %d %s", resp.Code, resp.Body)
	}
	select {
	case d := <-hook.got:
		if d.order.ID != done.Order.ID || d.id != eventID(done.Order.ID, "created") || d.timestamp == "" {
			t.Fatalf("delivery = %+v", d)
		}
		if d.order.UCP == nil || len(d.order.UCP.Capabilities[CapOrder]) != 1 {
			t.Fatalf("an order carries the order capability in ucp: %+v", d.order.UCP)
		}
		if _, err := strconv.ParseInt(d.timestamp, 10, 64); err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no webhook was delivered")
	}
}

func TestNoWebhookWithoutTheOrderCapability(t *testing.T) {
	w, hook := newWebhooks(t)
	h := newHarness(t, gpay(), func(c *Config) { c.Webhooks = w })
	done := fixtureCheckout(t, "shopping-checkout-rest-829.json")
	ready := *done
	ready.Status, ready.Order = ReadyForComplete, nil
	h.backend.next = &ready
	h.backend.order = sampleOrder(t)
	h.srv.cfg.Backend = &sequenceBackend{fakeBackend: h.backend, then: done}
	resp := h.do(call{method: "POST", path: "/checkout-sessions/chk_1/complete", body: fixture(t, "shopping-checkout-rest-786.json")})
	if resp.Code != 200 {
		t.Fatalf("complete: %d", resp.Code)
	}
	select {
	case <-hook.got:
		t.Fatal("an order was announced to a platform that did not negotiate the capability")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestGetOrderServesTheOrderAndItsErrors(t *testing.T) {
	h := newHarness(t, gpay())
	h.peers.m[platformURL] = platform(Version, CapOrder)
	h.backend.order = sampleOrder(t)
	w := h.do(call{method: "GET", path: "/orders/order_abc123"})
	var o Order
	decodeBody(t, w, &o)
	if w.Code != 200 || o.ID != h.backend.order.ID || o.UCP == nil || o.UCP.Version != Version || len(o.UCP.Capabilities[CapOrder]) != 1 {
		t.Fatalf("status %d order %+v", w.Code, o)
	}
	for name, tc := range map[string]struct {
		err  error
		code string
	}{"unknown order": {ErrNotFound, "not_found"}, "not the caller's": {ErrForbidden, "unauthorized"}} {
		h.backend.err = tc.err
		w = h.do(call{method: "GET", path: "/orders/order_abc123"})
		var r ErrorResponse
		decodeBody(t, w, &r)
		if w.Code != 200 || r.UCP.Status != "error" || r.Messages[0].Code != tc.code || r.Messages[0].Severity != Unrecoverable || len(r.UCP.Capabilities[CapOrder]) != 1 {
			t.Fatalf("%s: %d %+v", name, w.Code, r)
		}
	}
	// A platform that did not negotiate order cannot read orders.
	h.backend.err = nil
	h.peers.m[platformURL] = platform(Version, CapCheckout)
	w = h.do(call{method: "GET", path: "/orders/order_abc123"})
	var r ErrorResponse
	decodeBody(t, w, &r)
	if w.Code != 200 || r.Messages[0].Code != CodeCapabilitiesIncompatible {
		t.Fatalf("%d %+v", w.Code, r)
	}
	// Negotiation failed before the backend was asked.
	if h.backend.count("order") != 3 {
		t.Fatalf("order reads = %d", h.backend.count("order"))
	}
}

// The webhook URL is the platform's: a redirect from it must never carry the
// signed order to another target (overview, "Fetching": no redirects).
func TestWebhookNeverFollowsARedirect(t *testing.T) {
	var internal, front atomic.Int32
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		internal.Add(1)
		w.WriteHeader(204)
	}))
	defer target.Close()
	redirector := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		front.Add(1)
		http.Redirect(w, r, "https://internal.example.com/secret", http.StatusTemporaryRedirect)
	}))
	defer redirector.Close()
	tr := target.Client().Transport.(*http.Transport).Clone()
	tr.DialContext = func(ctx context.Context, _, addr string) (net.Conn, error) {
		to := redirector.Listener.Addr().String()
		if strings.HasPrefix(addr, "internal.example.com") {
			to = target.Listener.Addr().String()
		}
		return (&net.Dialer{}).DialContext(ctx, "tcp", to)
	}
	s, _ := newKeyPair(t, ES256)
	w := &Webhooks{Signer: s, Agent: "https://merchant.example/.well-known/ucp", Client: &http.Client{Transport: tr}, Attempts: 3, Backoff: time.Millisecond}
	err := w.Send(context.Background(), "https://example.com/hook", sampleOrder(t), "evt-r", time.Now())
	if err == nil {
		t.Fatal("a redirect was taken for an acknowledgement")
	}
	if internal.Load() != 0 {
		t.Fatalf("the signed order was re-posted to the redirect target (%d times)", internal.Load())
	}
	if front.Load() != 1 {
		t.Fatalf("a redirect is final: %d attempts", front.Load())
	}
}
