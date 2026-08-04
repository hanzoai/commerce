// Copyright (c) 2026-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package organization

import (
	"errors"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/util/test/ae"
)

// bearerNames are credential-shaped values a caller can type at the untrusted
// org header. If any of these is ever persisted as an org name it is
// simultaneously a credential leak (the value is stored in plaintext as a
// tenant identifier — incident 2026-07-02) and an unbounded-cardinality bug:
// one 5.25KB Organization per distinct token, which is heap growth with no
// ceiling. Refusal turns on shape, not on whether the value opens a door.
var bearerNames = []string{
	"hk-2ff139c7-4dd5-4f23-9de1-df7b67331b6e",
	"sk-live-0123456789abcdef",
	"HK-UPPERCASE-KEY",
	"  sk-leading-space",
}

// TestBeforeCreate_RefusesBearerShapedName pins the chokepoint. Every create
// path — Create, MustCreate, GetOrCreate, and anything added later — runs
// BeforeCreate, and orm.Model.CreateCtx aborts the write when it errors. The
// guard therefore holds for callers that never consulted secret.Like
// themselves, which is the point: upstream guards are defense in depth, this is
// the invariant.
func TestBeforeCreate_RefusesBearerShapedName(t *testing.T) {
	for _, name := range bearerNames {
		o := &Organization{Name: name}
		if err := o.BeforeCreate(); !errors.Is(err, ErrSecretLikeName) {
			t.Errorf("BeforeCreate(%q) = %v, want ErrSecretLikeName", name, err)
		}
	}
}

// TestBeforeCreate_AllowsOrdinaryName guards the guard: a normal org name must
// still provision. A predicate that rejects everything would fail closed into an
// outage. Names near the boundary are included deliberately — "hkust" and
// "skunkworks" contain the prefixes but are not bearer-SHAPED, and rejecting
// them would deny real tenants.
func TestBeforeCreate_AllowsOrdinaryName(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	for _, name := range []string{"hanzo", "lux", "zoo", "skunkworks", "hkust", "sky"} {
		o := New(datastore.New(ctx))
		o.Name = name
		if err := o.BeforeCreate(); err != nil {
			t.Errorf("BeforeCreate(%q) = %v, want nil — ordinary org must provision", name, err)
		}
	}
}

// TestGetOrCreate_NeverPersistsBearerShapedName is the end-to-end proof against
// a real store: GetOrCreate is the exact call the auth path makes, and it must
// refuse to write, leaving NO org row behind for the credential.
func TestGetOrCreate_NeverPersistsBearerShapedName(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	for _, name := range bearerNames {
		db := datastore.New(ctx)
		o := New(db)
		o.Name = name
		o.Enabled = true

		if err := o.GetOrCreate("Name=", name); err == nil {
			t.Errorf("GetOrCreate(%q) succeeded — a credential was persisted as an org", name)
		}

		// Read-only probe: no org row may exist under that name afterwards.
		probe := New(datastore.New(ctx))
		found, err := probe.Query().Filter("Name=", name).Get()
		if err != nil {
			t.Fatalf("probe query for %q: %v", name, err)
		}
		if found {
			t.Errorf("an org named %q was persisted despite the refused create", name)
		}
	}
}

// TestGetOrCreate_PersistsOrdinaryName is the positive control for the test
// above: the same call with a normal name must still create the org, so a
// passing refusal test cannot be an artifact of a broken harness.
func TestGetOrCreate_PersistsOrdinaryName(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	o := New(datastore.New(ctx))
	o.Name = "acme"
	o.Enabled = true
	if err := o.GetOrCreate("Name=", "acme"); err != nil {
		t.Fatalf("GetOrCreate(acme) = %v, want success", err)
	}
	if o.Name != "acme" {
		t.Fatalf("created org name = %q, want acme", o.Name)
	}
}
