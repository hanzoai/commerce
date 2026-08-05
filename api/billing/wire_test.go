package billing

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/secrets"
)

// wireReference must survive a bank memo field: uppercase [A-Z0-9-] only, and
// distinct payers must stay distinct after the trip (reconciliation depends on
// recovering the payer from exactly this alphabet).
func TestWireReference(t *testing.T) {
	cases := []struct{ payer, want string }{
		{"hanzo", "TOPUP-HANZO"},
		{"hanzo/z", "TOPUP-HANZO-Z"},
		{"acme-labs/alice.b", "TOPUP-ACME-LABS-ALICE-B"},
		{"org_1", "TOPUP-ORG-1"},
	}
	for _, tc := range cases {
		if got := wireReference(tc.payer); got != tc.want {
			t.Errorf("wireReference(%q) = %q, want %q", tc.payer, got, tc.want)
		}
	}
}

// TestWireFromHostReadsTheOrgScopedAddress pins the KMS ADDRESS, because getting
// it wrong is silent: both doors reach the same store keyed by (path, name, env),
// so a read at a path nothing writes returns "" and the rail answers "Wire
// transfer not configured" exactly as it would if no one had ever stored the
// bank. It read /tenants/hanzo/wire/<NAME> while cloud writes every in-process
// secret under /orgs/{org}/… — one spelling each, and no test in between.
//
// The fake records what was ASKED FOR rather than what came back, which is the
// only way a test can tell a missing value from a mis-addressed one.
func TestWireFromHostReadsTheOrgScopedAddress(t *testing.T) {
	asked := []string{}
	prev := secrets.Get()
	secrets.Set(refRecorder{asked: &asked, values: map[string]string{
		"/orgs/hanzo/wire/WIRE_ACCOUNT_NUMBER": "488108457818",
		"/orgs/hanzo/wire/WIRE_SWIFT":          "BOFAUS3N",
	}})
	t.Cleanup(func() { secrets.Set(prev) })

	w := wireFromHost(context.Background(), "hanzo")
	if w.AccountNumber != "488108457818" || w.SWIFT != "BOFAUS3N" {
		t.Fatalf("wireFromHost read nothing at the org-scoped address; asked for %v", asked)
	}
	for _, ref := range asked {
		if strings.HasPrefix(ref, "/tenants/") {
			t.Fatalf("still reading the address nothing writes: %q", ref)
		}
	}
}

// TestWireFromHostIsPerBrand — the brand serving the page decides whose bank is
// shown, so a lux page must not read Hanzo's account out of the same store.
func TestWireFromHostIsPerBrand(t *testing.T) {
	prev := secrets.Get()
	secrets.Set(refRecorder{asked: &[]string{}, values: map[string]string{
		"/orgs/hanzo/wire/WIRE_ACCOUNT_NUMBER": "488108457818",
		"/orgs/lux/wire/WIRE_ACCOUNT_NUMBER":   "999900001111",
	}})
	t.Cleanup(func() { secrets.Set(prev) })

	if got := wireFromHost(context.Background(), "lux").AccountNumber; got != "999900001111" {
		t.Fatalf("lux read %q — a brand must resolve its OWN bank", got)
	}
}

type refRecorder struct {
	asked  *[]string
	values map[string]string
}

func (r refRecorder) GetSecret(_ context.Context, ref string) ([]byte, error) {
	*r.asked = append(*r.asked, ref)
	if v, ok := r.values[ref]; ok {
		return []byte(v), nil
	}
	return nil, errNoSecret
}

var errNoSecret = errors.New("no such secret")
