// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package acp

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// SignatureHeader carries the signature of an order webhook (openapi.agentic_checkout_webhook.yaml).
const SignatureHeader = "Merchant-Signature"

// Sender delivers order events to the agent.
type Sender interface {
	Send(ctx context.Context, e Event) error
}

// Sign returns the Merchant-Signature of a body signed at a moment: t=<unix_seconds>,v1=<hex of
// HMAC-SHA256 over timestamp + "." + body>.
func Sign(secret []byte, at time.Time, body []byte) string {
	t := strconv.FormatInt(at.Unix(), 10)
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(t + "."))
	h.Write(body)
	return "t=" + t + ",v1=" + hex.EncodeToString(h.Sum(nil))
}

// Webhook sends order events to the agent's receiver, one attempt each: the spec defines no
// retry.
type Webhook struct {
	url    string
	secret []byte
	client *http.Client
}

// NewWebhook returns a Sender to the receiver at target, an https URL, signing with the secret
// shared with the agent. A redirect is never followed.
func NewWebhook(target string, secret []byte, client *http.Client) (*Webhook, error) {
	if u, err := url.Parse(target); err != nil || u.Scheme != "https" || u.Host == "" {
		return nil, errors.New("acp: the webhook receiver must be an absolute https URL")
	}
	if len(secret) == 0 || client == nil {
		return nil, errors.New("acp: the webhook secret and http client are required")
	}
	c := *client
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Webhook{url: target, secret: secret, client: &c}, nil
}

// SendError is a delivery the receiver did not accept; Body is its Error object when it sent one.
type SendError struct {
	Status int
	Body   Error
}

// Error implements error.
func (e *SendError) Error() string {
	return fmt.Sprintf("acp: the webhook receiver answered %d %s: %s", e.Status, e.Body.Code, e.Body.Message)
}

// Send delivers one event. The data is the full order with type, id, checkout_session_id,
// permalink_url and status, which the webhook schema requires.
func (w *Webhook) Send(ctx context.Context, e Event) error {
	d := e.Data
	if e.Type == "" || d.Type != "order" || d.ID == "" || d.CheckoutSessionID == "" || d.PermalinkURL == "" || d.Status == "" {
		return errors.New("acp: the event needs a type and an order with type, id, checkout_session_id, permalink_url and status")
	}
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(SignatureHeader, Sign(w.secret, time.Now(), body))
	res, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	dec := json.NewDecoder(res.Body)
	if res.StatusCode != http.StatusOK {
		se := &SendError{Status: res.StatusCode}
		_ = dec.Decode(&se.Body)
		return se
	}
	var ack struct {
		Received bool `json:"received"`
	}
	if dec.Decode(&ack) != nil || !ack.Received {
		return &SendError{Status: res.StatusCode, Body: Error{Code: "not_received", Message: "the receiver did not confirm the event"}}
	}
	return nil
}
