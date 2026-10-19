package square

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hanzoai/commerce/payment/processor"
)

// ValidateWebhook is the one Square webhook verifier. HandleProviderWebhook
// (api/billing/webhooks.go) reaches it through processor.Global(), at the one
// route POST /v1/commerce/webhooks/:provider.
//
// Square signs each notification as
//
//	base64(HMAC-SHA256(signatureKey, notificationURL + rawBody))
//
// where notificationURL is the EXACT URL of the webhook subscription — never one
// derived from the inbound request, which a proxy rewrites. The digests are
// compared as raw bytes with hmac.Equal.
//
// There is no freshness window, and that is deliberate. Square keeps an event's
// created_at across every retry of it (it retries a delivery that got no 2xx for
// up to 72 hours, and the dashboard can resend an event later than that), so a
// window rejects exactly the deliveries that matter most: the ones that arrived
// while the receiver was down. A five-minute window here made every retry of an
// event Square first sent during a deploy a permanent refusal. Replay is stopped
// where it can be stopped exactly — the receiver records each event id once and
// acknowledges a repeat as a duplicate, and every money posting downstream is
// keyed on the provider's payment, refund or dispute id.
//
// Verifying needs only the signature key and the notification URL; it does not
// need the charge credentials, so a deployment that verifies but does not charge
// still verifies.
func (p *Provider) ValidateWebhook(ctx context.Context, payload []byte, signature string) (*processor.WebhookEvent, error) {
	key, url := p.config.WebhookSignatureKey, p.config.WebhookURL
	if key == "" {
		return nil, fmt.Errorf("%w: square webhook signature key not configured", processor.ErrWebhookValidationFailed)
	}
	if url == "" {
		return nil, fmt.Errorf("%w: square webhook URL not configured", processor.ErrWebhookValidationFailed)
	}

	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(url))
	mac.Write(payload)
	provided, err := base64.StdEncoding.DecodeString(strings.TrimSpace(signature))
	if err != nil || !hmac.Equal(provided, mac.Sum(nil)) {
		return nil, processor.ErrWebhookValidationFailed
	}

	var evt struct {
		Type      string `json:"type"`
		EventID   string `json:"event_id"`
		CreatedAt string `json:"created_at"`
		Data      struct {
			Object map[string]interface{} `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &evt); err != nil {
		return nil, fmt.Errorf("square webhook: %w", err)
	}
	if evt.EventID == "" {
		// The event id is the receiver's whole replay defence, so a signed body
		// without one is refused rather than processed unkeyed.
		return nil, fmt.Errorf("%w: square webhook carries no event_id", processor.ErrWebhookValidationFailed)
	}
	ts := time.Now().Unix()
	if t, err := time.Parse(time.RFC3339, evt.CreatedAt); err == nil {
		ts = t.Unix()
	}
	return &processor.WebhookEvent{
		ID:        evt.EventID,
		Type:      eventType(evt.Type),
		Processor: processor.Square,
		Org:       p.config.Org,
		Data:      evt.Data.Object,
		Timestamp: ts,
	}, nil
}

// ReceivesWebhooks reports whether this account can verify a delivery: it holds
// the subscription's signature key and its notification URL.
func (p *Provider) ReceivesWebhooks() bool {
	return p.config.WebhookSignatureKey != "" && p.config.WebhookURL != ""
}

var _ processor.WebhookReceiver = (*Provider)(nil)

// eventType names a Square event in the receiver's vocabulary. Only the names the
// receiver branches on differ from Square's own: an invoice payment settles as
// invoice.paid, and a dispute's change of state is dispute.updated under either of
// the names Square has used for it.
func eventType(t string) string {
	switch t {
	case "invoice.payment_made":
		return "invoice.paid"
	case "dispute.state.updated", "dispute.state.changed":
		return "dispute.updated"
	}
	return t
}
