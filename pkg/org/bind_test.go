// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package org

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/types/pricing"
	"github.com/hanzoai/commerce/types/accesstoken"
)

// cachedOrg builds an entry as it sits in the cache after a real resolve: an org
// whose STORED state differs from the platform defaults in every field
// Organization.Defaults() would overwrite.
//
// This exercises bind() directly rather than through a store round-trip. That is
// deliberate: bind is the unit under test, and the in-memory SQLite harness does
// not repopulate noindex fields on reload, so a seed-and-reload test cannot
// observe this property at all (it reads back defaults either way).
func cachedOrg(t *testing.T, name string) *organization.Organization {
	t.Helper()

	o := organization.New(datastore.New(context.Background()))
	o.Name = name
	o.Enabled = false // suspended
	o.Owners = []string{"user_owner_1"}
	o.Admins = []string{"user_admin_1", "user_admin_2"}
	o.Moderators = []string{"user_mod_1"}
	o.Partners = []pricing.Partner{{Id: "partner_1"}}
	o.SecretKey = []byte("super-secret-signing-key")
	o.Tokens = []accesstoken.AccessToken{{String: "tok_1"}}
	o.Fees.Card.Percent = 0.005 // negotiated rate, 10x below the default
	o.Fees.Card.Flat = 0
	return o
}

// assertStoredState fails if any field Defaults() would overwrite has been lost.
func assertStoredState(t *testing.T, label string, o *organization.Organization) {
	t.Helper()

	if o.Enabled {
		t.Errorf("[%s] Enabled = true, want false — a SUSPENDED org served as enabled", label)
	}
	if got := o.Fees.Card.Percent; got != 0.005 {
		t.Errorf("[%s] Fees.Card.Percent = %v, want 0.005 — negotiated rate overwritten", label, got)
	}
	if got := o.Fees.Card.Flat; got != 0 {
		t.Errorf("[%s] Fees.Card.Flat = %v, want 0", label, got)
	}
	if len(o.Admins) != 2 {
		t.Errorf("[%s] Admins = %v, want 2 entries — admin list wiped", label, o.Admins)
	}
	if len(o.Moderators) != 1 {
		t.Errorf("[%s] Moderators = %v, want 1 entry — moderator list wiped", label, o.Moderators)
	}
	if len(o.Partners) != 1 {
		t.Errorf("[%s] Partners = %v, want 1 entry — partner list wiped", label, o.Partners)
	}
}

// TestBind_PreservesStoredFields is the CRITICAL-1 regression.
//
// bind() re-homes a cached org onto the request's datastore. Doing that through
// Init would end in orm.ApplyDefaults, whose Defaulter tail calls
// Organization.Defaults() unconditionally — setting Enabled=true, resetting
// Fees.Card to 5%/50c, and reallocating Admins/Moderators/Partners. Handlers
// persist the org they are handed with full-entity writes, so that would
// permanently write platform defaults over stored state: a suspended org served
// (then saved) as enabled, and a negotiated 0.5% card fee overwritten with 5%.
func TestBind_PreservesStoredFields(t *testing.T) {
	setup(t)
	cached := cachedOrg(t, "suspended")

	bound := bind(cached, context.Background())

	assertStoredState(t, "bound", bound)
	// The cached entry must also be untouched by binding.
	assertStoredState(t, "cached", cached)
}

// TestInitClobbersStoredFields is the control that makes the test above
// meaningful: it pins the exact behavior bind must avoid. If ApplyDefaults ever
// stops overwriting populated entities, this fails and bind's Rebind can be
// reconsidered — otherwise a green TestBind_PreservesStoredFields could be a
// false negative.
func TestInitClobbersStoredFields(t *testing.T) {
	setup(t)
	o := cachedOrg(t, "control")

	o.Init(datastore.New(context.Background()))

	if !o.Enabled {
		t.Error("Init no longer forces Enabled=true; re-evaluate bind's use of Rebind")
	}
	if o.Fees.Card.Percent != 0.05 || o.Fees.Card.Flat != 50 {
		t.Errorf("Init no longer resets Fees.Card (got %+v); re-evaluate bind", o.Fees.Card)
	}
	if len(o.Admins) != 0 || len(o.Moderators) != 0 || len(o.Partners) != 0 {
		t.Error("Init no longer empties Admins/Moderators/Partners; re-evaluate bind")
	}
}

// TestBind_CopiesAreFullyIndependent is the HIGH-2 regression. The struct copy
// in bind is shallow, so every reference field must be cloned. Otherwise the
// cache and every in-flight request for the same org share one backing array,
// and a mutation in request A appears in the cached entry and in request B.
// AddToken appending to Tokens is the live vector: decoded slices commonly have
// cap>len, so two requests write the same backing slot.
func TestBind_CopiesAreFullyIndependent(t *testing.T) {
	setup(t)
	cached := cachedOrg(t, "acme")
	ctx := context.Background()

	a := bind(cached, ctx)
	b := bind(cached, ctx)

	if a == b {
		t.Fatal("bind returned the same pointer twice")
	}

	// Tamper with every reference field through request A's copy.
	a.Owners[0] = "ATTACKER"
	a.Admins[0] = "ATTACKER"
	a.Moderators[0] = "ATTACKER"
	a.Partners[0].Id = "ATTACKER"
	a.SecretKey[0] = 'X'
	a.Tokens[0].String = "ATTACKER"

	for label, o := range map[string]*organization.Organization{"request B": b, "cache": cached} {
		if o.Owners[0] == "ATTACKER" {
			t.Errorf("%s: Owners aliased", label)
		}
		if o.Admins[0] == "ATTACKER" {
			t.Errorf("%s: Admins aliased", label)
		}
		if o.Moderators[0] == "ATTACKER" {
			t.Errorf("%s: Moderators aliased", label)
		}
		if o.Partners[0].Id == "ATTACKER" {
			t.Errorf("%s: Partners aliased", label)
		}
		if o.SecretKey[0] == 'X' {
			t.Errorf("%s: SecretKey aliased — a credential is shared mutable state", label)
		}
		if o.Tokens[0].String == "ATTACKER" {
			t.Errorf("%s: Tokens aliased", label)
		}
	}
}

// TestBind_AppendCannotWriteIntoSharedBacking covers the cap>len case directly:
// AddToken appends, and if two copies share a backing array with spare capacity
// the appends collide in the same slot.
func TestBind_AppendCannotWriteIntoSharedBacking(t *testing.T) {
	setup(t)
	cached := cachedOrg(t, "capacity")
	// Give Tokens spare capacity, as a decoded slice commonly has.
	big := make([]accesstoken.AccessToken, 1, 8)
	big[0] = accesstoken.AccessToken{String: "tok_1"}
	cached.Tokens = big
	ctx := context.Background()

	a := bind(cached, ctx)
	b := bind(cached, ctx)

	a.Tokens = append(a.Tokens, accesstoken.AccessToken{String: "from_a"})
	b.Tokens = append(b.Tokens, accesstoken.AccessToken{String: "from_b"})

	if a.Tokens[1].String != "from_a" {
		t.Errorf("request A's appended token was overwritten by B: %q", a.Tokens[1].String)
	}
	if len(cached.Tokens) != 1 {
		t.Errorf("cached Tokens grew to %d — an append reached the cache", len(cached.Tokens))
	}
}

// TestBind_WalletNotShared pins the transient handle. Wallet is datastore:"-"
// and lazily loaded, so sharing the pointer would hand two requests the same
// mutable wallet.
func TestBind_WalletNotShared(t *testing.T) {
	setup(t)
	cached := cachedOrg(t, "walletorg")

	if got := bind(cached, context.Background()).Wallet; got != nil {
		t.Error("Wallet pointer carried over from the cache; each request must load its own")
	}
}

// TestBind_RebindsAccessTokenBackReference pins that the copy's AccessTokens
// mixin points at the COPY, not at the cached org — otherwise AddToken on a
// request's org would mint a token against the cached entity's identity.
func TestBind_RebindsAccessTokenBackReference(t *testing.T) {
	setup(t)
	cached := cachedOrg(t, "backref")

	bound := bind(cached, context.Background())

	got, ok := bound.AccessTokens.Entity.(*organization.Organization)
	if !ok {
		t.Fatalf("AccessTokens.Entity is %T, want *organization.Organization", bound.AccessTokens.Entity)
	}
	if got == cached {
		t.Error("AccessTokens.Entity still points at the cached org")
	}
	if got != bound {
		t.Error("AccessTokens.Entity does not point at the bound copy")
	}
}

// referenceFields is the set of exported reference-typed fields (slice, map,
// pointer, interface) reachable on Organization at the top level, including
// through its embedded mixins. Every one is shared by a shallow struct copy and
// so must be handled explicitly in bind.
var referenceFields = []string{
	"Admins",       // cloned
	"Entity",       // rebound via AccessTokens.Init
	"Integrations", // cloned ([]Integration; transient, decoded from Integrations_)
	"Moderators",   // cloned
	"Owners",       // cloned
	// Parent is a datastore.Key: an interface whose methods are all read-only
	// accessors (AppID/Encode/Equal/IntID/Kind/...). The value is immutable, so
	// sharing it between the cache and a request copy is safe.
	"Parent",
	"Partners",  // cloned
	"SecretKey", // cloned
	"Tokens",    // cloned
	"Wallet",    // nilled; lazily reloaded per request
	"Websites",  // cloned
}

// TestBind_CoversEveryReferenceField is the future-proofing. Adding a slice,
// map, or pointer field to Organization silently reintroduces aliasing, because
// the shallow copy in bind will share it and nothing else will complain. This
// fails the moment such a field appears, forcing a decision in bind rather than
// a silent regression.
func TestBind_CoversEveryReferenceField(t *testing.T) {
	var found []string

	var walk func(reflect.Type)
	walk = func(typ reflect.Type) {
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			if f.Anonymous && f.Type.Kind() == reflect.Struct {
				walk(f.Type) // flatten embedded mixins
				continue
			}
			if f.PkgPath != "" {
				continue // unexported: not reachable, not assignable by bind
			}
			switch f.Type.Kind() {
			case reflect.Slice, reflect.Map, reflect.Ptr, reflect.Interface:
				found = append(found, f.Name)
			}
		}
	}
	walk(reflect.TypeOf(organization.Organization{}))

	sort.Strings(found)
	want := append([]string(nil), referenceFields...)
	sort.Strings(want)

	if !reflect.DeepEqual(found, want) {
		t.Fatalf("Organization's reference-typed fields changed.\n got: %v\nwant: %v\n\n"+
			"A shallow struct copy SHARES every field above. Handle the new field in "+
			"bind() (clone it, or nil it if it is a transient handle), then update "+
			"referenceFields.", found, want)
	}
}
