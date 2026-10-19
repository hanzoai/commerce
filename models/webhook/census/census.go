// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

// Package census reads who commerce's legacy webhook rows deliver to, so the move
// to /v1/webhook can name every subscriber before anything is deleted.
//
// A row can sit in one of two stores, and which one decides whether it was ever
// delivered to:
//
//   - "org": the org's own store, where the merchant CRUD (/v1/commerce/webhook)
//     wrote every row. The legacy sender never read it — it set a namespace on a
//     datastore whose queries ignore the namespace — so these rows received
//     nothing.
//   - "system": the shared store the legacy sender DID read. A row there was sent
//     every org's events, whichever org it was meant for.
package census

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/webhook"
	"github.com/hanzoai/commerce/util/nscontext"
)

// Subscriber is one legacy webhook row as the census reports it: where it is,
// where it delivers and what it takes — and nothing that is a credential. The URL
// is reduced to its host and a digest of the whole, because a subscriber URL can
// carry a token in its path or query.
type Subscriber struct {
	Store   string   `json:"store"` // "org" or "system", see the package doc
	Org     string   `json:"org"`   // the org whose store holds it; "" in the system store
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Host    string   `json:"host"`
	URLHash string   `json:"urlHash"`
	All     bool     `json:"all"`
	Events  []string `json:"events"`
	Enabled bool     `json:"enabled"`
	Live    bool     `json:"live"`
}

// Subscribers lists every legacy webhook row in the system store and in every
// org's own store, reading only: the orgs from the root namespace (never creating
// one) and each store's rows.
func Subscribers(ctx context.Context) ([]Subscriber, error) {
	system := datastore.New(ctx)
	out, err := rows(system, "system", "")
	if err != nil {
		return nil, err
	}
	orgs := make([]*organization.Organization, 0)
	if _, err := organization.Query(system).GetAll(&orgs); err != nil {
		return out, err
	}
	for _, o := range orgs {
		own := datastore.NewNamespaced(nscontext.WithNamespace(ctx, o.Name))
		if own.DB() == nil || own.DB() == system.DB() {
			continue // no store of its own (reserved, or no per-org routing): nothing more to read
		}
		got, err := rows(own, "org", o.Name)
		if err != nil {
			return out, err
		}
		out = append(out, got...)
	}
	return out, nil
}

func rows(db *datastore.Datastore, store, org string) ([]Subscriber, error) {
	hooks := make([]*webhook.Webhook, 0)
	if _, err := webhook.Query(db).GetAll(&hooks); err != nil {
		return nil, err
	}
	out := make([]Subscriber, 0, len(hooks))
	for _, h := range hooks {
		s := Subscriber{Store: store, Org: org, ID: h.Id(), Name: h.Name, All: h.All, Enabled: h.Enabled, Live: h.Live}
		if u, err := url.Parse(h.Url); err == nil {
			s.Host = u.Host
		}
		sum := sha256.Sum256([]byte(h.Url))
		s.URLHash = hex.EncodeToString(sum[:6])
		for ev, on := range h.Events {
			if on {
				s.Events = append(s.Events, ev)
			}
		}
		sort.Strings(s.Events)
		out = append(out, s)
	}
	return out, nil
}
