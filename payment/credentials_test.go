// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package payment

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/providers/square"
	"github.com/hanzoai/commerce/secrets"
)

// tenancyStore is the host's secret plane with cloud's tenancy rule (apps/kms
// admit): orgs/<org>/… is served only to a call acting for <org>.
type tenancyStore struct {
	values map[string]string
	asked  []string
}

func (s *tenancyStore) GetSecret(ctx context.Context, ref string) ([]byte, error) {
	s.asked = append(s.asked, ref)
	rest, ok := strings.CutPrefix(ref, "orgs/")
	if !ok {
		return nil, errors.New("not a tenant ref")
	}
	if org, _, _ := strings.Cut(rest, "/"); zip.CallerOf(ctx).Org != org {
		return nil, errors.New("refused: the call does not act for " + org)
	}
	if v, ok := s.values[ref]; ok {
		return []byte(v), nil
	}
	return nil, errors.New("no such secret")
}

func withStore(t *testing.T, s *tenancyStore) {
	t.Helper()
	prev := secrets.Get()
	secrets.Set(s)
	t.Cleanup(func() { secrets.Set(prev) })
}

// A per-org Square credential loads: what the org holds at
// orgs/<org>/square/<NAME> reaches its integration fields, and from there the
// processor its registry charges with — on a request sent by someone else.
func TestHydrateLoadsTheOrgsOwnSquareCredential(t *testing.T) {
	ensureDefaultProcessorPolicy(t)
	s := &tenancyStore{values: map[string]string{
		"orgs/acme/square/SQUARE_SANDBOX_ACCESS_TOKEN":   "sb-acme-token",
		"orgs/acme/square/SQUARE_SANDBOX_LOCATION_ID":    "LOC-ACME",
		"orgs/acme/square/SQUARE_SANDBOX_APPLICATION_ID": "sandbox-sq0idb-acme",
	}}
	withStore(t, s)
	t.Setenv("SQUARE_SANDBOX_ACCESS_TOKEN", "deployment-token")
	t.Setenv("SQUARE_SANDBOX_LOCATION_ID", "DEPLOYMENT-LOC")

	org := &organization.Organization{Name: "acme"} // not Live: sandbox
	shopper := zip.WithCaller(context.Background(), zip.Caller{Org: "someone-else", User: "bob"})
	Hydrate(shopper, org)

	if org.Square.Sandbox.AccessToken != "sb-acme-token" || org.Square.Sandbox.LocationId != "LOC-ACME" {
		t.Fatalf("acme's credential did not load: %+v (asked %v)", org.Square.Sandbox, s.asked)
	}
	p, err := ProcessorsForOrg(org).Get(processor.Square)
	if err != nil {
		t.Fatalf("no Square processor for a hydrated org: %v", err)
	}
	if sp := p.(*square.Provider); sp.LocationID() != "LOC-ACME" || sp.ApplicationID() != "sandbox-sq0idb-acme" {
		t.Fatalf("the registry charges location %q app %q, want acme's own, not the deployment's", sp.LocationID(), sp.ApplicationID())
	}
	for _, ref := range s.asked {
		if !strings.HasPrefix(ref, "orgs/acme/") {
			t.Fatalf("hydrating acme asked for %q", ref)
		}
	}
}

// Narrowed to one provider, the read asks for that provider's names only.
func TestHydrateNarrowsToTheProvidersAskedFor(t *testing.T) {
	s := &tenancyStore{values: map[string]string{
		"orgs/hanzo/wire/WIRE_ACCOUNT_NUMBER": "000123456789",
		"orgs/hanzo/wire/WIRE_BANK_NAME":      "Bank",
	}}
	withStore(t, s)
	org := &organization.Organization{Name: "hanzo"}
	payer := zip.WithCaller(context.Background(), zip.Caller{Org: "acme", User: "alice"})
	Hydrate(payer, org, "wire")
	if org.Wire.AccountNumber != "000123456789" || org.Wire.BankName != "Bank" {
		t.Fatalf("wire did not load: %+v", org.Wire)
	}
	for _, ref := range s.asked {
		if !strings.HasPrefix(ref, "orgs/hanzo/wire/") {
			t.Fatalf("a wire-only hydrate asked for %q", ref)
		}
	}
	if len(s.asked) != 9 {
		t.Fatalf("asked %d wire names, want the 9 the wire set holds", len(s.asked))
	}
}

func TestHydrateWithNoOrgOrNoReaderReadsNothing(t *testing.T) {
	s := &tenancyStore{values: map[string]string{}}
	withStore(t, s)
	Hydrate(context.Background(), nil)
	Hydrate(context.Background(), &organization.Organization{})
	if len(s.asked) != 0 {
		t.Fatalf("asked %v for no org", s.asked)
	}
	secrets.Set(nil)
	org := &organization.Organization{Name: "acme"}
	Hydrate(context.Background(), org)
	if org.Square.Production.AccessToken != "" {
		t.Fatal("a commerce with no reader loaded a credential")
	}
}
