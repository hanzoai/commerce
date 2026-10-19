// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package census_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/db"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/webhook"
	"github.com/hanzoai/commerce/models/webhook/census"
	"github.com/hanzoai/commerce/util/nscontext"
)

// production stands up commerce's datastore the way Bootstrap does: a shared
// system store, and a store of its own for each org (SetOrgDBResolver).
func production(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	cfg := db.DefaultConfig()
	cfg.DataDir = dir
	cfg.EnableVectorSearch = false
	cfg.EnableDatastore = false
	mgr, err := db.NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	sys, err := mgr.Org("system")
	if err != nil {
		t.Fatal(err)
	}
	prev := datastore.New(context.Background()).DB()
	datastore.SetDefaultDB(sys)
	datastore.SetOrgDBResolver(mgr.Org)
	t.Cleanup(func() {
		datastore.SetOrgDBResolver(nil)
		datastore.SetDefaultDB(prev)
		_ = mgr.Close()
		_ = os.RemoveAll(dir)
	})
}

func file(t *testing.T, d *datastore.Datastore, name, u, token string, events webhook.Events) {
	t.Helper()
	w := webhook.New(d)
	w.Name, w.Url, w.AccessToken, w.Events = name, u, token, events
	if err := w.Create(); err != nil {
		t.Fatalf("file %s: %v", name, err)
	}
}

// The census finds the rows in both stores — the org's own, where the CRUD wrote
// them, and the system store the legacy sender read — says which is which, and
// carries no credential: not the AccessToken, not the URL's path or query.
func TestSubscribersReadsBothStoresAndNoCredential(t *testing.T) {
	production(t)
	ctx := context.Background()
	for _, name := range []string{"acme", "quiet"} {
		o := organization.New(datastore.New(ctx))
		o.Name = name
		if err := o.Create(); err != nil {
			t.Fatalf("org %s: %v", name, err)
		}
	}
	file(t, datastore.NewNamespaced(nscontext.WithNamespace(ctx, "acme")), "orders", "https://hooks.example.com/in?key=url-secret", "tok-secret", webhook.Events{"order.paid": true, "user.created": false})
	file(t, datastore.New(ctx), "everything", "https://shared.example.net/all", "tok-2", webhook.Events{"user.created": true})

	subs, err := census.Subscribers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 2 {
		t.Fatalf("census found %d rows, want 2: %+v", len(subs), subs)
	}
	by := map[string]census.Subscriber{}
	for _, s := range subs {
		by[s.Store] = s
	}
	if s := by["org"]; s.Org != "acme" || s.Host != "hooks.example.com" || len(s.Events) != 1 || s.Events[0] != "order.paid" || s.URLHash == "" {
		t.Fatalf("org-store row %+v", s)
	}
	if s := by["system"]; s.Org != "" || s.Host != "shared.example.net" {
		t.Fatalf("system-store row %+v", s)
	}
	b, _ := json.Marshal(subs)
	for _, leak := range []string{"tok-secret", "tok-2", "url-secret", "key="} {
		if strings.Contains(string(b), leak) {
			t.Fatalf("the census carries %q: %s", leak, b)
		}
	}
	orgs := make([]*organization.Organization, 0)
	if _, err := organization.Query(datastore.New(ctx)).GetAll(&orgs); err != nil || len(orgs) != 2 {
		t.Fatalf("orgs after the census: %d, %v — reading must create none", len(orgs), err)
	}
}
