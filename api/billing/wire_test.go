package billing

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/secrets"
	"github.com/hanzoai/commerce/util/test/ae"
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

// brandStore is the host's secret plane with cloud's tenancy rule: orgs/<org>/…
// answers only a call acting for <org>.
type brandStore map[string]string

func (s brandStore) GetSecret(ctx context.Context, ref string) ([]byte, error) {
	rest, ok := strings.CutPrefix(ref, "orgs/")
	if !ok {
		return nil, errors.New("not a tenant ref")
	}
	if org, _, _ := strings.Cut(rest, "/"); zip.CallerOf(ctx).Org != org {
		return nil, errors.New("refused: the call acts for another org")
	}
	if v, ok := s[ref]; ok {
		return []byte(v), nil
	}
	return nil, errors.New("no such secret")
}

// The brand that owns the host is where the wire is sent, so a customer of any
// org paying on pay.lux.network reads lux's bank and one on pay.hanzo.ai reads
// Hanzo's — each from orgs/<brand>/wire, asked as the brand, whoever is asking.
func TestWireForReadsTheBrandsOwnBank(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	webhookOrg(t, ctx, "hanzo", true)
	webhookOrg(t, ctx, "lux", true)
	prev := secrets.Get()
	secrets.Set(brandStore{
		"orgs/hanzo/wire/WIRE_ACCOUNT_NUMBER": "000488108457",
		"orgs/hanzo/wire/WIRE_BANK_NAME":      "Hanzo Bank",
		"orgs/lux/wire/WIRE_ACCOUNT_NUMBER":   "000999900001",
		"orgs/lux/wire/WIRE_BANK_NAME":        "Lux Bank",
	})
	t.Cleanup(func() { secrets.Set(prev) })

	payer := zip.WithCaller(ctx, zip.Caller{Org: "acme", User: "alice"})
	for host, want := range map[string]string{"pay.hanzo.ai": "000488108457", "pay.lux.network": "000999900001"} {
		w, err := WireFor(payer, host, "acme/alice")
		if err != nil {
			t.Fatalf("%s: %v", host, err)
		}
		if w.AccountNumber != want || w.Reference != "TOPUP-ACME-ALICE" {
			t.Fatalf("%s: account %q reference %q, want %q and the payer's reference", host, w.AccountNumber, w.Reference, want)
		}
	}
}

// A brand with no bank in its secrets has nowhere to send the money: one refusal.
func TestWireForWithNoBankRefuses(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	webhookOrg(t, ctx, "hanzo", true)
	prev := secrets.Get()
	secrets.Set(brandStore{})
	t.Cleanup(func() { secrets.Set(prev) })
	if _, err := WireFor(ctx, "pay.hanzo.ai", "acme/alice"); !IsWireUnconfigured(err) {
		t.Fatalf("err = %v, want the wire rail's one refusal", err)
	}
}

// GET /v1/billing/wire, through the handler: the request's caller reaches the
// host's KMS (Forward), so a customer of acme paying on pay.hanzo.ai reads
// Hanzo's bank.
func TestWireEndpointReadsTheBrandsBankAsTheCaller(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	webhookOrg(t, ctx, "hanzo", true)
	acme := webhookOrg(t, ctx, "acme", true)
	prev := secrets.Get()
	secrets.Set(brandStore{
		"orgs/hanzo/wire/WIRE_ACCOUNT_NUMBER": "000488108457",
		"orgs/hanzo/wire/WIRE_BANK_NAME":      "Hanzo Bank",
	})
	t.Cleanup(func() { secrets.Set(prev) })

	req := httptest.NewRequest(http.MethodGet, "/v1/billing/wire", nil)
	req.Host = "pay.hanzo.ai"
	req.Header.Set(zip.HeaderOrg, "acme")
	req.Header.Set(zip.HeaderUser, "alice")
	resp := driveSeeded(func(c *zip.Ctx) {
		c.SetContext(ctx)
		c.Locals("organization", acme)
	}, "/v1/billing/wire", req, GetBillingWireInstructions)
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(raw), "000488108457") {
		t.Fatalf("GET /v1/billing/wire = %d %s, want Hanzo's bank", resp.StatusCode, raw)
	}
}
