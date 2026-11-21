// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Webhooks delivers order events to platforms (shopping/order, "Events"). The
// payload is the full current state of the order, never a delta; every delivery
// is signed per the signatures specification and carries the business profile
// in UCP-Agent, Webhook-Id and Webhook-Timestamp; a retry redelivers the same
// event with the original Webhook-Id and Webhook-Timestamp.
type Webhooks struct {
	// Signer signs deliveries with a key published in the business profile's keys[].
	Signer *Signer
	// Agent is the URL of the business profile, sent as UCP-Agent.
	Agent string
	// Client posts the deliveries; the default is the guarded client of the
	// profile fetcher, since the URL is the platform's.
	Client *http.Client
	// Attempts bounds the deliveries tried, 5 by default; a delivery that is not
	// acknowledged after that is the host's to retry later.
	Attempts int
	// Backoff is the wait after the first failed attempt, doubling each time;
	// default one second.
	Backoff time.Duration
	// Now is the clock; it defaults to time.Now.
	Now func() time.Time
}

func (w *Webhooks) now() time.Time {
	if w.Now != nil {
		return w.Now()
	}
	return time.Now()
}

// WebhookURL returns the URL a platform's profile names for order events: the
// config.webhook_url of its dev.ucp.shopping.order capability, which must be an
// https URL.
func WebhookURL(platform *Profile, version string) (string, error) {
	for _, c := range platform.UCP.Capabilities[CapOrder] {
		if c.Version != version {
			continue
		}
		var cfg struct {
			URL string `json:"webhook_url"`
		}
		if err := json.Unmarshal(c.Config, &cfg); err != nil || cfg.URL == "" {
			return "", errors.New("the order capability declares no webhook_url")
		}
		if err := checkURL(cfg.URL); err != nil {
			return "", fmt.Errorf("webhook_url: %w", err)
		}
		return cfg.URL, nil
	}
	return "", errors.New("the platform does not declare the order capability")
}

// EventID is the stable Webhook-Id of an order event: one identifier per order
// and event kind, the same on every retry.
func EventID(orderID, kind string) string {
	sum := sha256.Sum256([]byte(orderID + "\x00" + kind))
	return "whevt_" + hex.EncodeToString(sum[:16])
}

// Send delivers the order to url, retrying with the same Webhook-Id and
// Webhook-Timestamp until the platform acknowledges with a 2xx, answers with a
// status that retrying cannot change, or the attempts run out. id names the
// event and at is when it occurred.
func (w *Webhooks) Send(ctx context.Context, url string, o *Order, id string, at time.Time) error {
	if w.Signer == nil || w.Agent == "" {
		return errors.New("ucp: Webhooks needs a Signer and the business profile URL")
	}
	if err := checkURL(url); err != nil {
		return fmt.Errorf("webhook URL: %w", err)
	}
	payload, err := json.Marshal(o)
	if err != nil {
		return err
	}
	client := w.Client
	if client == nil {
		client = guarded(10*time.Second, false)
	}
	attempts, wait := w.Attempts, w.Backoff
	if attempts <= 0 {
		attempts = 5
	}
	if wait <= 0 {
		wait = time.Second
	}
	var last error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return ctx.Err()
			}
			wait *= 2
		}
		retry, err := w.attempt(ctx, client, url, payload, id, at)
		if err == nil {
			return nil
		}
		last = err
		if !retry {
			return err
		}
	}
	return fmt.Errorf("webhook not acknowledged after %d attempts: %w", attempts, last)
}

// attempt makes one delivery; retry says whether trying again can help.
func (w *Webhooks) attempt(ctx context.Context, client *http.Client, url string, payload []byte, id string, at time.Time) (retry bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderAgent, `profile="`+w.Agent+`"`)
	req.Header.Set(HeaderWebhookID, id)
	req.Header.Set(HeaderWebhookTime, strconv.FormatInt(at.Unix(), 10))
	if err := w.Signer.SignRequest(req, payload, Options{}); err != nil {
		return false, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return true, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return false, nil
	case resp.StatusCode == http.StatusRequestTimeout, resp.StatusCode == http.StatusTooManyRequests, resp.StatusCode >= 500:
		return true, fmt.Errorf("platform answered %d", resp.StatusCode)
	}
	return false, fmt.Errorf("platform rejected the event with %d", resp.StatusCode)
}
