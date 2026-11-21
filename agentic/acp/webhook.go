// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package acp

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// The order webhook of openapi.agentic_checkout_webhook.yaml: the seller POSTs order_create
// and order_update events to the agent's receiver, signed in the Merchant-Signature header.

// SignatureHeader carries the webhook signature: t=<unix_seconds>,v1=<64_hex>.
const SignatureHeader = "Merchant-Signature"

// Tolerance is the recommended window between a webhook's timestamp and the receiver's
// clock (the webhook specification: "Recommended timestamp tolerance is 300 seconds").
const Tolerance = 300 * time.Second

// Sender delivers order events to the agent.
type Sender interface {
	Send(ctx context.Context, e Event) error
}

// Sign returns the Merchant-Signature value for a body signed at a moment: HMAC-SHA256 of
// timestamp + "." + body under the shared secret, as 64 lowercase hex digits.
func Sign(secret []byte, at time.Time, body []byte) string {
	t := strconv.FormatInt(at.Unix(), 10)
	return "t=" + t + ",v1=" + hex.EncodeToString(mac(secret, t, body))
}

func mac(secret []byte, t string, body []byte) []byte {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(t))
	h.Write([]byte("."))
	h.Write(body)
	return h.Sum(nil)
}

// Reasons a signature is refused, each a 401 invalid_signature at the receiver.
var (
	ErrMissing   = errors.New("acp: the Merchant-Signature header is missing")
	ErrMalformed = errors.New("acp: Merchant-Signature must be t=<timestamp>,v1=<64_hex>")
	ErrMismatch  = errors.New("acp: the webhook signature does not verify")
	ErrExpired   = errors.New("acp: the webhook timestamp is outside the allowed window")
)

// Verify checks a Merchant-Signature header over the raw body: the format, the HMAC in
// constant time, and a timestamp within tolerance of now. It is what a receiver runs.
func Verify(secret []byte, header string, body []byte, now time.Time, tolerance time.Duration) error {
	if header == "" {
		return ErrMissing
	}
	t, sig, ok := strings.Cut(header, ",")
	t, ok1 := strings.CutPrefix(t, "t=")
	sig, ok2 := strings.CutPrefix(sig, "v1=")
	if !ok || !ok1 || !ok2 || len(sig) != 64 || t == "" || len(t) > 18 || strings.Trim(t, "0123456789") != "" {
		return ErrMalformed
	}
	want, err := hex.DecodeString(sig)
	if err != nil {
		return ErrMalformed
	}
	if !hmac.Equal(want, mac(secret, t, body)) {
		return ErrMismatch
	}
	unix, _ := strconv.ParseInt(t, 10, 64)
	if d := now.Sub(time.Unix(unix, 0)); d > tolerance || -d > tolerance {
		return ErrExpired
	}
	return nil
}

// Webhook sends events to the agent's receiver. The spec defines no retry policy, so Send
// makes one attempt and returns what happened: the host's queue owns redelivery.
type Webhook struct {
	url    string
	secret []byte
	client *http.Client
	now    func() time.Time
}

// NewWebhook returns a Sender to the receiver at target, an https URL, signing with the
// secret shared with the agent. The client carries the host's timeout and transport; it
// never follows a redirect, which would carry a signed payload to another host.
func NewWebhook(target string, secret []byte, client *http.Client) (*Webhook, error) {
	if u, err := url.Parse(target); err != nil || u.Scheme != "https" || u.Host == "" {
		return nil, errors.New("acp: the webhook receiver must be an absolute https URL")
	}
	if len(secret) == 0 {
		return nil, errors.New("acp: the webhook secret is required")
	}
	if client == nil {
		return nil, errors.New("acp: the webhook http client is required")
	}
	c := *client
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Webhook{url: target, secret: secret, client: &c, now: time.Now}, nil
}

// SendError is a delivery the receiver did not accept.
type SendError struct {
	Status int
	// Body is the receiver's Error object when it sent one.
	Body Error
}

// Error implements error.
func (e *SendError) Error() string {
	return fmt.Sprintf("acp: the webhook receiver answered %d %s: %s", e.Status, e.Body.Code, e.Body.Message)
}

// Send delivers one event. The event must carry the full order the receiver needs: type
// "order", id, checkout_session_id, permalink_url and status.
func (w *Webhook) Send(ctx context.Context, e Event) error {
	d := e.Data
	switch {
	case e.Type == "":
		return errors.New("acp: the event type is required")
	case d.Type != "order" || d.ID == "" || d.CheckoutSessionID == "" || d.PermalinkURL == "" || d.Status == "":
		return errors.New("acp: the event data must be an order with type, id, checkout_session_id, permalink_url and status")
	}
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	now := w.now()
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(SignatureHeader, Sign(w.secret, now, body))
	req.Header.Set("Request-Id", hex.EncodeToString(id[:]))
	req.Header.Set("Timestamp", now.UTC().Format(time.RFC3339))
	res, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	reply, err := io.ReadAll(io.LimitReader(res.Body, 1<<16))
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		se := &SendError{Status: res.StatusCode}
		_ = json.Unmarshal(reply, &se.Body)
		return se
	}
	var ack struct {
		Received bool `json:"received"`
	}
	if json.Unmarshal(reply, &ack) != nil || !ack.Received {
		return &SendError{Status: res.StatusCode, Body: Error{Code: "not_received", Message: "the receiver did not confirm the event"}}
	}
	return nil
}
