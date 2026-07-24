package engine

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billingevent"
	"github.com/hanzoai/commerce/models/webhookendpoint"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// fastWebhookBackoff shrinks the retry backoff so retry tests don't sleep for
// real seconds; the production schedule (1s/5s/25s) is restored on cleanup.
func fastWebhookBackoff(t *testing.T) {
	t.Helper()
	prev := webhookBackoff
	webhookBackoff = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	t.Cleanup(func() { webhookBackoff = prev })
}

// newTestEvent creates a persisted billing event in an isolated namespace so
// evt.Id()/GetCreatedAt() are valid (a hand-built model has no orm key and
// panics). Returns the namespaced datastore for endpoint setup.
func newTestEvent(t *testing.T, ns, eventType string) (*datastore.Datastore, *billingevent.BillingEvent) {
	t.Helper()
	c := ae.NewContext()
	t.Cleanup(c.Close)

	db := datastore.New(nscontext.WithNamespace(c, ns))
	evt := billingevent.New(db)
	evt.Type = eventType
	evt.ObjectType = "order"
	evt.ObjectId = "ord_" + ns
	evt.CustomerId = "cus_" + ns
	if err := evt.Create(); err != nil {
		t.Fatalf("create billing event: %v", err)
	}
	return db, evt
}

func newEndpoint(t *testing.T, db *datastore.Datastore, url, secret, status string, events []string) *webhookendpoint.WebhookEndpoint {
	t.Helper()
	ep := webhookendpoint.New(db)
	ep.Url = url
	ep.Secret = secret
	ep.Status = status
	ep.Events = events
	if err := ep.Create(); err != nil {
		t.Fatalf("create webhook endpoint: %v", err)
	}
	return ep
}

// ---------------------------------------------------------------------------
// deliverWebhook — headers, signature, retry policy
// ---------------------------------------------------------------------------

func TestDeliverWebhook_ConformingHeaders(t *testing.T) {
	var gotSig, gotEvent, gotDelivery, gotContentType string
	var gotBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		gotSig = r.Header.Get("X-Webhook-Signature")
		gotEvent = r.Header.Get("X-Webhook-Event")
		gotDelivery = r.Header.Get("X-Webhook-Delivery")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(200)
	}))
	defer server.Close()

	db, evt := newTestEvent(t, "wht_headers", "payment_intent.succeeded")
	ep := newEndpoint(t, db, server.URL, "whsec_headers", "enabled", nil)

	if err := deliverWebhook(ep, evt); err != nil {
		t.Fatalf("delivery should succeed: %v", err)
	}

	// The canonical, vendor-free headers — no Commerce-* names survive.
	if gotContentType != "application/json" {
		t.Fatalf("Content-Type = %q", gotContentType)
	}
	if gotEvent != "payment_intent.succeeded" {
		t.Fatalf("X-Webhook-Event = %q", gotEvent)
	}
	if gotDelivery == "" {
		t.Fatal("X-Webhook-Delivery must be set")
	}
	if !strings.HasPrefix(gotSig, "t=") || !strings.Contains(gotSig, ",v1=") {
		t.Fatalf("X-Webhook-Signature malformed: %q", gotSig)
	}

	// Recompute the signature over the exact body the subscriber received.
	var ts, sig string
	for _, p := range strings.Split(gotSig, ",") {
		if kv := strings.SplitN(p, "=", 2); len(kv) == 2 {
			switch kv[0] {
			case "t":
				ts = kv[1]
			case "v1":
				sig = kv[1]
			}
		}
	}
	if want := computeSignature(ts, gotBody, "whsec_headers"); sig != want {
		t.Fatalf("signature mismatch: got %s want %s", sig, want)
	}
	// And it must verify through the public verifier a subscriber would use.
	if err := VerifyWebhookSignature(gotBody, gotSig, "whsec_headers"); err != nil {
		t.Fatalf("VerifyWebhookSignature: %v", err)
	}
}

func TestDeliverWebhook_RetryThenSuccess(t *testing.T) {
	fastWebhookBackoff(t)

	var hits int32
	var deliveryIDs sync.Map
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deliveryIDs.Store(r.Header.Get("X-Webhook-Delivery"), true)
		if atomic.AddInt32(&hits, 1) < 3 {
			w.WriteHeader(500) // fail the first two attempts
			return
		}
		w.WriteHeader(200) // succeed on the third
	}))
	defer server.Close()

	db, evt := newTestEvent(t, "wht_retry", "invoice.paid")
	ep := newEndpoint(t, db, server.URL, "whsec_retry", "enabled", nil)

	if err := deliverWebhook(ep, evt); err != nil {
		t.Fatalf("should succeed after retries: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Fatalf("expected 3 attempts (500,500,200): got %d", got)
	}

	// One stable delivery id across the whole attempt-group (dedupe key).
	n := 0
	deliveryIDs.Range(func(k, _ any) bool {
		if k.(string) == "" {
			t.Fatal("every attempt must carry X-Webhook-Delivery")
		}
		n++
		return true
	})
	if n != 1 {
		t.Fatalf("delivery id must be stable across retries: saw %d ids", n)
	}
}

func TestDeliverWebhook_429IsRetryable(t *testing.T) {
	fastWebhookBackoff(t)

	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&hits, 1) == 1 {
			w.WriteHeader(429) // rate-limited — retry
			return
		}
		w.WriteHeader(200)
	}))
	defer server.Close()

	db, evt := newTestEvent(t, "wht_429", "invoice.paid")
	ep := newEndpoint(t, db, server.URL, "whsec_429", "enabled", nil)

	if err := deliverWebhook(ep, evt); err != nil {
		t.Fatalf("429 then 200 should succeed: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Fatalf("expected 2 attempts (429,200): got %d", got)
	}
}

func TestDeliverWebhook_4xxIsPermanent(t *testing.T) {
	fastWebhookBackoff(t) // applies only if it (wrongly) retried

	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(400)
	}))
	defer server.Close()

	db, evt := newTestEvent(t, "wht_4xx", "invoice.paid")
	ep := newEndpoint(t, db, server.URL, "whsec_400", "enabled", nil)

	err := deliverWebhook(ep, evt)
	if err == nil || !strings.Contains(err.Error(), "status 400") {
		t.Fatalf("expected a 400 error, got %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("4xx must not retry: got %d attempts, want 1", got)
	}
}

func TestDeliverWebhook_5xxExhausts(t *testing.T) {
	fastWebhookBackoff(t)

	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(503)
	}))
	defer server.Close()

	db, evt := newTestEvent(t, "wht_5xx", "invoice.paid")
	ep := newEndpoint(t, db, server.URL, "whsec_503", "enabled", nil)

	if err := deliverWebhook(ep, evt); err == nil {
		t.Fatal("persistent 5xx should return an error")
	}
	if got := atomic.LoadInt32(&hits); got != webhookMaxAttempts {
		t.Fatalf("5xx should retry to exhaustion: got %d, want %d", got, webhookMaxAttempts)
	}
}

// ---------------------------------------------------------------------------
// DispatchWebhooks — endpoint matching (Events filter + Status)
// ---------------------------------------------------------------------------

func TestDispatchWebhooks_EventAndStatusFiltering(t *testing.T) {
	var matchHits, allHits, otherHits, disabledHits int32
	counter := func(n *int32) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(n, 1)
			w.WriteHeader(200)
		}
	}
	matchSrv := httptest.NewServer(counter(&matchHits))
	allSrv := httptest.NewServer(counter(&allHits))
	otherSrv := httptest.NewServer(counter(&otherHits))
	disabledSrv := httptest.NewServer(counter(&disabledHits))
	defer matchSrv.Close()
	defer allSrv.Close()
	defer otherSrv.Close()
	defer disabledSrv.Close()

	db, evt := newTestEvent(t, "wht_filter", "order.completed")
	newEndpoint(t, db, matchSrv.URL, "s1", "enabled", []string{"order.completed"})     // exact subscribe
	newEndpoint(t, db, allSrv.URL, "s2", "enabled", nil)                               // empty = all events
	newEndpoint(t, db, otherSrv.URL, "s3", "enabled", []string{"order.created"})       // not subscribed
	newEndpoint(t, db, disabledSrv.URL, "s4", "disabled", []string{"order.completed"}) // disabled

	if err := DispatchWebhooks(db, evt); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	if got := atomic.LoadInt32(&matchHits); got != 1 {
		t.Fatalf("exact-subscribed endpoint: got %d deliveries, want 1", got)
	}
	if got := atomic.LoadInt32(&allHits); got != 1 {
		t.Fatalf("subscribe-all endpoint: got %d deliveries, want 1", got)
	}
	if got := atomic.LoadInt32(&otherHits); got != 0 {
		t.Fatalf("unsubscribed endpoint must be skipped: got %d", got)
	}
	if got := atomic.LoadInt32(&disabledHits); got != 0 {
		t.Fatalf("disabled endpoint must be skipped: got %d", got)
	}

	// The event is marked fully dispatched.
	if evt.Pending {
		t.Fatal("event should be marked not-pending after dispatch")
	}
}

// ---------------------------------------------------------------------------
// EmitBillingEvent / Emit — the emission the lifecycle points invoke
// ---------------------------------------------------------------------------

func TestEmitBillingEvent_CreatesRecordAndDelivers(t *testing.T) {
	var gotEvent string
	delivered := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEvent = r.Header.Get("X-Webhook-Event")
		w.WriteHeader(200)
		select {
		case delivered <- struct{}{}:
		default:
		}
	}))
	defer server.Close()

	c := ae.NewContext()
	defer c.Close()
	db := datastore.New(nscontext.WithNamespace(c, "wht_emit"))
	newEndpoint(t, db, server.URL, "whsec_emit", "enabled", nil)

	evt, err := EmitBillingEvent(db, "order.created", "order", "ord_1", "cus_1",
		map[string]interface{}{"revenue": 12.34, "currency": "usd"}, nil)
	if err != nil {
		t.Fatalf("EmitBillingEvent: %v", err)
	}
	if evt.Id() == "" {
		t.Fatal("billing event should be persisted with an id")
	}

	select {
	case <-delivered:
	case <-time.After(2 * time.Second):
		t.Fatal("EmitBillingEvent did not deliver to the subscribed endpoint")
	}
	if gotEvent != "order.created" {
		t.Fatalf("X-Webhook-Event = %q, want order.created", gotEvent)
	}
}

// TestEmit_FiresAsync proves the fire-and-forget helper the checkout lifecycle
// points call (order.created/completed/refunded, checkout.started) actually
// delivers, off the caller's stack.
func TestEmit_FiresAsync(t *testing.T) {
	var gotEvent, gotDelivery string
	delivered := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEvent = r.Header.Get("X-Webhook-Event")
		gotDelivery = r.Header.Get("X-Webhook-Delivery")
		w.WriteHeader(200)
		select {
		case delivered <- struct{}{}:
		default:
		}
	}))
	defer server.Close()

	c := ae.NewContext()
	defer c.Close()
	ctx := nscontext.WithNamespace(c, "wht_async")
	db := datastore.New(ctx)
	newEndpoint(t, db, server.URL, "whsec_async", "enabled", []string{"order.completed"})

	Emit(ctx, "order.completed", "order", "ord_9", "cus_9",
		map[string]interface{}{"revenue": 99.0, "currency": "usd"})

	select {
	case <-delivered:
	case <-time.After(2 * time.Second):
		t.Fatal("Emit did not deliver asynchronously")
	}
	if gotEvent != "order.completed" {
		t.Fatalf("X-Webhook-Event = %q, want order.completed", gotEvent)
	}
	if gotDelivery == "" {
		t.Fatal("X-Webhook-Delivery must be set")
	}
}
