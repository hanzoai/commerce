package engine

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/billingevent"
	"github.com/hanzoai/commerce/models/webhookendpoint"
	"github.com/hanzoai/commerce/util/json"

	. "github.com/hanzoai/commerce/types"
)

// Webhook delivery tuning. Retry is bounded and in-process — no queue infra.
const (
	webhookMaxAttempts    = 3
	webhookAttemptTimeout = 10 * time.Second
)

// webhookBackoff is the sleep BEFORE the Nth retry (exponential ×5). It is a var
// so tests can shrink it; production is fixed at 1s/5s/25s (+jitter).
var webhookBackoff = []time.Duration{1 * time.Second, 5 * time.Second, 25 * time.Second}

// Emit fires a billing event to every subscribed webhook endpoint, fully
// detached from the caller's request lifecycle (fire-and-forget). ctx MUST
// already carry the org namespace (org.Namespaced). Errors are logged, never
// returned — webhook delivery must NEVER touch the money path.
func Emit(ctx context.Context, eventType, objectType, objectId, customerId string, data Map) {
	go func() {
		db := datastore.New(ctx)
		if _, err := EmitBillingEvent(db, eventType, objectType, objectId, customerId, data, nil); err != nil {
			log.Error("billing webhook emit %s (%s %s): %v", eventType, objectType, objectId, err)
		}
	}()
}

// EmitBillingEvent creates an append-only billing event record and dispatches
// it to all matching webhook endpoints. It runs synchronously; callers that must
// stay off the money path invoke it via Emit (goroutine + detached context).
func EmitBillingEvent(db *datastore.Datastore, eventType, objectType, objectId, customerId string, data, previousData Map) (*billingevent.BillingEvent, error) {
	evt := billingevent.New(db)
	evt.Type = eventType
	evt.ObjectType = objectType
	evt.ObjectId = objectId
	evt.CustomerId = customerId
	evt.Data = data
	evt.PreviousData = previousData
	evt.Pending = true

	if err := evt.Create(); err != nil {
		return nil, fmt.Errorf("failed to create billing event: %w", err)
	}

	DispatchWebhooks(db, evt)

	return evt, nil
}

// DispatchWebhooks sends the event to all enabled, subscribed webhook endpoints.
func DispatchWebhooks(db *datastore.Datastore, evt *billingevent.BillingEvent) error {
	// Only enabled endpoints (Status filter respects the disabled state).
	// Ancestor(synckey) mirrors the CRUD list query — endpoints are stored
	// under that parent (webhookendpoint.Defaults).
	rootKey := db.NewKey("synckey", "", 1, nil)
	iter := webhookendpoint.Query(db).
		Ancestor(rootKey).
		Filter("Status=", "enabled").
		Run()

	for {
		ep := webhookendpoint.New(db)
		if _, err := iter.Next(ep); err != nil {
			break
		}

		// Respect the endpoint's event-type subscription filter.
		if !ep.MatchesEvent(evt.Type) {
			continue
		}

		// Best-effort: deliverWebhook exhausts its bounded retry and logs each
		// attempt. A failed endpoint never blocks the others or the money path.
		_ = deliverWebhook(ep, evt)
	}

	// Mark event as fully dispatched.
	evt.Pending = false
	_ = evt.Update()

	return nil
}

// deliverWebhook POSTs the event to one endpoint with a bounded, jittered retry.
// The delivery id is stable across the attempt-group so a subscriber can dedupe
// redeliveries. Retries fire only on a network error, 5xx, or 429; a 4xx is
// permanent (client-side reject) and stops immediately.
func deliverWebhook(ep *webhookendpoint.WebhookEndpoint, evt *billingevent.BillingEvent) error {
	payload := json.EncodeBytes(Map{
		"id":           evt.Id(),
		"type":         evt.Type,
		"objectType":   evt.ObjectType,
		"objectId":     evt.ObjectId,
		"customerId":   evt.CustomerId,
		"data":         evt.Data,
		"previousData": evt.PreviousData,
		"livemode":     evt.Livemode,
		"created":      evt.GetCreatedAt(),
	})

	deliveryID := uuid.NewString()
	client := &http.Client{Timeout: webhookAttemptTimeout}

	var lastErr error
	for attempt := 1; attempt <= webhookMaxAttempts; attempt++ {
		status, err := postWebhook(client, ep, evt, payload, deliveryID)

		if err == nil && status >= 200 && status < 300 {
			log.Info("webhook delivered: endpoint=%s event=%s delivery=%s attempt=%d status=%d",
				ep.Id(), evt.Type, deliveryID, attempt, status)
			return nil
		}

		retryable := err != nil || status == 429 || status >= 500
		if err != nil {
			lastErr = fmt.Errorf("webhook delivery failed: %w", err)
			log.Error("webhook attempt failed: endpoint=%s event=%s delivery=%s attempt=%d error=%v",
				ep.Id(), evt.Type, deliveryID, attempt, err)
		} else {
			lastErr = fmt.Errorf("webhook endpoint returned status %d", status)
			log.Error("webhook attempt failed: endpoint=%s event=%s delivery=%s attempt=%d status=%d",
				ep.Id(), evt.Type, deliveryID, attempt, status)
		}

		if !retryable {
			return lastErr // permanent (4xx) — don't hammer the endpoint
		}
		if attempt < webhookMaxAttempts {
			time.Sleep(backoffWithJitter(attempt))
		}
	}

	return lastErr
}

// postWebhook performs a single delivery attempt. Each attempt carries a fresh
// timestamp+signature (so a slow retry is never rejected as stale) over the same
// body and the same stable delivery id.
func postWebhook(client *http.Client, ep *webhookendpoint.WebhookEndpoint, evt *billingevent.BillingEvent, payload []byte, deliveryID string) (int, error) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := computeSignature(timestamp, payload, ep.Secret)

	req, err := http.NewRequest(http.MethodPost, ep.Url, bytes.NewReader(payload))
	if err != nil {
		return 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", "t="+timestamp+",v1="+signature)
	req.Header.Set("X-Webhook-Event", evt.Type)
	req.Header.Set("X-Webhook-Delivery", deliveryID)

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body) // drain so the connection can be reused

	return resp.StatusCode, nil
}

// backoffWithJitter returns the sleep before the retry that follows the given
// (1-indexed) attempt: base delay + a random 0..base/2 jitter.
func backoffWithJitter(attempt int) time.Duration {
	idx := attempt - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(webhookBackoff) {
		idx = len(webhookBackoff) - 1
	}
	base := webhookBackoff[idx]
	half := int64(base) / 2
	if half <= 0 {
		return base
	}
	return base + time.Duration(rand.Int63n(half))
}

// computeSignature creates an HMAC-SHA256 signature for webhook verification.
func computeSignature(timestamp string, payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyWebhookSignature verifies an X-Webhook-Signature value ("t=…,v1=…").
func VerifyWebhookSignature(payload []byte, signatureHeader, secret string) error {
	parts := strings.Split(signatureHeader, ",")
	var timestamp, signature string
	for _, p := range parts {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			timestamp = kv[1]
		case "v1":
			signature = kv[1]
		}
	}

	if timestamp == "" || signature == "" {
		return fmt.Errorf("invalid signature header format")
	}

	expected := computeSignature(timestamp, payload, secret)
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}
