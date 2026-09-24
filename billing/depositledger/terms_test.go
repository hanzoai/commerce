package depositledger

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/types/pricing"
)

func loader(orgs map[string]*organization.Organization, err error) func(context.Context, string) (*organization.Organization, error) {
	return func(_ context.Context, name string) (*organization.Organization, error) {
		if err != nil {
			return nil, err
		}
		return orgs[name], nil
	}
}

func withTerms(chain string, t pricing.CryptoDeposit) *organization.Organization {
	o := &organization.Organization{}
	o.CryptoDeposit = map[string]pricing.CryptoDeposit{chain: t}
	return o
}

// No loader is a legitimate deployment — everybody on the platform default —
// and must not look like a failure.
func TestNoLoaderHasNoOpinion(t *testing.T) {
	_, ok, err := OrgTerms{}.TermsFor(context.Background(), "acme", "ethereum")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if ok {
		t.Error("claimed an opinion with no loader configured")
	}
}

func TestAnOrgWithNoEntryHasNoOpinion(t *testing.T) {
	r := OrgTerms{Load: loader(map[string]*organization.Organization{"acme": {}}, nil)}
	_, ok, err := r.TermsFor(context.Background(), "acme", "ethereum")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if ok {
		t.Error("an org with no CryptoDeposit entry claimed terms of its own")
	}
}

func TestAnUnknownOrgHasNoOpinion(t *testing.T) {
	r := OrgTerms{Load: loader(map[string]*organization.Organization{}, nil)}
	_, ok, err := r.TermsFor(context.Background(), "nobody", "ethereum")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if ok {
		t.Error("an org that does not exist claimed terms")
	}
}

func TestAnOrgsTermsAreReturned(t *testing.T) {
	r := OrgTerms{Load: loader(map[string]*organization.Organization{
		"acme": withTerms("ethereum", pricing.CryptoDeposit{FeeCents: 250, SlippageBps: 50}),
	}, nil)}
	got, ok, err := r.TermsFor(context.Background(), "acme", "ethereum")
	if err != nil || !ok {
		t.Fatalf("TermsFor = (%+v, %v, %v)", got, ok, err)
	}
	if got.FeeCents != 250 || got.SlippageBps != 50 {
		t.Errorf("terms = %+v, want fee 250 / slippage 50", got)
	}
}

// THE distinction the ok flag exists for.
func TestAnOrgNegotiatedToNothingSaysSo(t *testing.T) {
	r := OrgTerms{Load: loader(map[string]*organization.Organization{
		"bigco": withTerms("ethereum", pricing.CryptoDeposit{}),
	}, nil)}
	got, ok, err := r.TermsFor(context.Background(), "bigco", "ethereum")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !ok {
		t.Fatal("an org with an explicit zero entry reported no opinion — it would be put back on the platform fee it negotiated away")
	}
	if got.Deducts() {
		t.Errorf("terms = %+v, want nothing deducted", got)
	}
}

// Terms are per CHAIN: an org that negotiated Ethereum has said nothing about
// Base, and must not be given Ethereum's terms there.
func TestTermsDoNotLeakAcrossChains(t *testing.T) {
	r := OrgTerms{Load: loader(map[string]*organization.Organization{
		"acme": withTerms("ethereum", pricing.CryptoDeposit{FeeCents: 250}),
	}, nil)}
	if _, ok, _ := r.TermsFor(context.Background(), "acme", "base"); ok {
		t.Error("Ethereum's terms answered for Base")
	}
}

func TestChainLookupIsCaseInsensitive(t *testing.T) {
	// Every chain name on this rail is lowercase, but a record written by hand
	// should not silently miss.
	r := OrgTerms{Load: loader(map[string]*organization.Organization{
		"acme": withTerms("ethereum", pricing.CryptoDeposit{FeeCents: 250}),
	}, nil)}
	got, ok, err := r.TermsFor(context.Background(), "acme", "  Ethereum ")
	if err != nil || !ok {
		t.Fatalf("TermsFor = (%+v, %v, %v)", got, ok, err)
	}
	if got.FeeCents != 250 {
		t.Errorf("terms = %+v", got)
	}
}

func TestALoadFailureIsSurfacedNotSwallowed(t *testing.T) {
	// depositwatch fails CLOSED on this. Swallowing it would credit the org on
	// the platform default — usually cheaper than what they agreed to — with
	// nothing anywhere saying the lookup failed.
	r := OrgTerms{Load: loader(nil, errors.New("store down"))}
	_, ok, err := r.TermsFor(context.Background(), "acme", "ethereum")
	if err == nil {
		t.Fatal("a load failure was reported as 'no opinion'")
	}
	if ok {
		t.Error("claimed an opinion despite failing")
	}
	if !strings.Contains(err.Error(), "acme") {
		t.Errorf("error does not name the org: %v", err)
	}
}
