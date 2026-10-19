// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

// Package tasks delivers commerce's legacy per-org webhooks while their rows move
// to the platform's one webhook layer, /v1/webhook (hanzo-inc/cloud apps/webhook:
// HMAC-signed, retried, every platform event). It is on its way out: no route
// creates a row any more, and the sender goes once the census
// (census.Subscribers) has been read and its subscribers re-registered there.
//
// Until then a delivery carries the event and nothing else, and it reaches only
// a public address.
package tasks

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hanzoai/outbound"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/delay"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/webhook"
	"github.com/hanzoai/commerce/util/json"
)

// Payload is the delivery body: the event's data. The hook's AccessToken is NOT
// in it — it used to be, so every subscriber endpoint, and anything that logged
// one, held a credential of the org's.
type Payload struct {
	Data interface{} `json:"data"`
}

// client is the one way commerce dials a subscriber URL: hanzoai/outbound, judged
// at the dial on the address connected, following no redirect. A variable so this
// package's tests can reach an httptest server; nothing else assigns it.
var client = outbound.Client(outbound.Options{Timeout: 20 * time.Second, Limit: 64 << 10})

// post delivers one payload. A failure is returned, never swallowed.
func post(ctx context.Context, url string, data interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, json.EncodeBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Hanzo/1.0")
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("status %d", res.StatusCode)
	}
	return nil
}

// Emit fires an org's webhooks for one event.
var Emit = delay.Func("webhook-emit", func(ctx context.Context, org string, event string, data interface{}) {
	db := datastore.New(ctx)
	db.SetNamespace(org)

	hooks := make([]*webhook.Webhook, 0)
	if _, err := webhook.Query(db).GetAll(&hooks); err != nil {
		log.Warn("webhook: list the hooks of %q: %v", org, err, ctx)
		return
	}
	for _, hook := range hooks {
		// Who receives is unchanged while the rows move: every hook that names the
		// event or takes all of them, as before. (Enabled was never read here; the
		// census reports it so the move can.)
		if !hook.All && !hook.Events[event] {
			continue
		}
		dctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		err := post(dctx, hook.Url, Payload{Data: data})
		cancel()
		if err != nil {
			log.Warn("webhook: %q for %q to hook %s: %v", event, org, hook.Id(), err, ctx)
		}
	}
})
