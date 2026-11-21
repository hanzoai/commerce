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
	"net/http"
	"strconv"
	"time"
)

// Webhooks delivers order events to platforms (shopping/order, "Events"): the
// full current order, signed per the signatures specification, with the business
// profile in UCP-Agent and a Webhook-Id and Webhook-Timestamp that a retry
// repeats. Every field is required.
type Webhooks struct {
	// Signer signs deliveries with a key published in the business profile's keys[].
	Signer *Signer
	// Agent is the URL of the business profile, sent as UCP-Agent.
	Agent string
	// Client posts the deliveries; NewClient builds one. The URL is the
	// platform's, so redirects are never followed.
	Client *http.Client
	// Attempts bounds the deliveries tried; after that the host retries later.
	Attempts int
	// Backoff is the wait after the first failed attempt, doubling each time.
	Backoff time.Duration
}

// webhookURL returns the config.webhook_url of the platform's order capability
// at version, which must be an https URL (shopping/order, "Webhook URL Configuration").
func webhookURL(platform *Profile, version string) (string, error) {
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

// eventID is the Webhook-Id of an order event: one per order and event kind,
// the same on every retry.
func eventID(orderID, kind string) string {
	sum := sha256.Sum256([]byte(orderID + "\x00" + kind))
	return "whevt_" + hex.EncodeToString(sum[:16])
}

// Send delivers the order to url, repeating the same Webhook-Id and
// Webhook-Timestamp on each retry, until the platform answers 2xx, answers with
// a status retrying cannot change, or the attempts run out. id names the event
// and at is when it occurred.
func (w *Webhooks) Send(ctx context.Context, url string, o *Order, id string, at time.Time) error {
	if w.Signer == nil || w.Agent == "" || w.Client == nil || w.Attempts <= 0 || w.Backoff <= 0 {
		return errors.New("ucp: Webhooks needs Signer, Agent, Client, Attempts and Backoff")
	}
	if err := checkURL(url); err != nil {
		return fmt.Errorf("webhook URL: %w", err)
	}
	payload, err := json.Marshal(o)
	if err != nil {
		return err
	}
	client, wait := withoutRedirects(w.Client), w.Backoff
	var last error
	for i := 0; i < w.Attempts; i++ {
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
		if last = err; !retry {
			return err
		}
	}
	return fmt.Errorf("webhook not acknowledged after %d attempts: %w", w.Attempts, last)
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
	resp.Body.Close()
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return false, nil
	case resp.StatusCode == http.StatusRequestTimeout, resp.StatusCode == http.StatusTooManyRequests, resp.StatusCode >= 500:
		return true, fmt.Errorf("platform answered %d", resp.StatusCode)
	}
	return false, fmt.Errorf("platform rejected the event with %d", resp.StatusCode)
}
