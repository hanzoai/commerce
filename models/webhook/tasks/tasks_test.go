// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package tasks

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/db"
	"github.com/hanzoai/commerce/models/webhook"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// hook files a webhook row in org that sends every event to url.
func hook(t *testing.T, ctx ae.Context, org, url, token string) {
	t.Helper()
	db := datastore.New(ctx)
	db.SetNamespace(org)
	w := webhook.New(db)
	w.Name, w.Url, w.All, w.Enabled, w.AccessToken = "probe", url, true, true, token
	if err := w.Create(); err != nil {
		t.Fatalf("file webhook: %v", err)
	}
}

// A delivery carries the event and nothing else. The hook's AccessToken used to
// ride in every body, so any subscriber endpoint — and anything that logged one —
// held a credential of the org's.
func TestEmitPostsNoToken(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	prev := client
	client = &http.Client{Timeout: 5 * time.Second}
	defer func() { client = prev }()
	got := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got <- string(b)
	}))
	defer srv.Close()
	hook(t, ctx, "acme", srv.URL, "tok-the-org-holds")

	Emit.Call(ctx, "acme", "subscriber.created", map[string]any{"email": "a@example.com"})
	select {
	case body := <-got:
		if strings.Contains(body, "tok-the-org-holds") || strings.Contains(body, "accessToken") {
			t.Fatalf("the delivery carried the hook's token: %s", body)
		}
		if !strings.Contains(body, "a@example.com") {
			t.Fatalf("the delivery lost the event: %s", body)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no delivery")
	}
}

// A subscriber URL is an address an org wrote, dialled from inside the cluster, so
// it is held to hanzoai/outbound's rule: a private address — loopback here, the
// metadata endpoint in production — is never reached.
func TestEmitRefusesAPrivateAddress(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	reached := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached <- struct{}{} }))
	defer srv.Close()
	hook(t, ctx, "acme", srv.URL, "")

	Emit.Call(ctx, "acme", "subscriber.created", map[string]any{"email": "a@example.com"})
	select {
	case <-reached:
		t.Fatal("a loopback subscriber URL was reached")
	case <-time.After(2 * time.Second):
	}
}

// What the legacy sender reaches, measured on the store layout production boots
// (a system store plus a store per org). A row the merchant CRUD filed in the
// org's own store is never delivered to; a row in the system store is delivered
// every org's events. That is why the census reads both stores and why the
// sender goes rather than being repaired.
func TestEmitReadsOnlyTheSystemStore(t *testing.T) {
	dir := t.TempDir()
	cfg := db.DefaultConfig()
	cfg.DataDir, cfg.EnableVectorSearch, cfg.EnableDatastore = dir, false, false
	mgr, err := db.NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()
	sys, err := mgr.Org("system")
	if err != nil {
		t.Fatal(err)
	}
	prevDB := datastore.New(context.Background()).DB()
	datastore.SetDefaultDB(sys)
	datastore.SetOrgDBResolver(mgr.Org)
	defer func() { datastore.SetOrgDBResolver(nil); datastore.SetDefaultDB(prevDB) }()
	prev := client
	client = &http.Client{Timeout: 5 * time.Second}
	defer func() { client = prev }()

	hits := make(chan string, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits <- r.URL.Path }))
	defer srv.Close()
	ctx := context.Background()
	for _, row := range []struct {
		d    *datastore.Datastore
		path string
	}{
		{datastore.NewNamespaced(nscontext.WithNamespace(ctx, "acme")), "/acme-own"},
		{datastore.New(ctx), "/system"},
	} {
		w := webhook.New(row.d)
		w.Name, w.Url, w.All = "probe", srv.URL+row.path, true
		if err := w.Create(); err != nil {
			t.Fatal(err)
		}
	}

	Emit.Call(ctx, "other-org", "user.created", map[string]any{"email": "b@example.com"})
	select {
	case p := <-hits:
		if p != "/system" {
			t.Fatalf("delivered to %s", p)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the system-store row was not delivered another org's event")
	}
	select {
	case p := <-hits:
		t.Fatalf("a second delivery, to %s: the org-store row was reached", p)
	case <-time.After(time.Second):
	}
}
