package billing

import (
	"io"
	"net/http"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/billing/tier"
	"github.com/hanzoai/commerce/util/test/ae"
)

func TestPaidEcosystemOrgs_Entitlements(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	orgs := []string{"admin", "hanzo", "lux", "zoo", "adnexus", "bootnode", "osage", "pars"}

	for _, orgName := range orgs {
		t.Run(orgName, func(t *testing.T) {
			org := moneyOrg(orgName)
			tv, err := ReadTier(ctx, org, orgName, tier.Free)
			if err != nil {
				t.Fatalf("ReadTier error for %s: %v", orgName, err)
			}
			if tv.Tier.Name != tier.Enterprise {
				t.Errorf("tier name = %s, want %s", tv.Tier.Name, tier.Enterprise)
			}
			if !tv.Tier.UnlimitedAgents {
				t.Errorf("expected unlimitedAgents=true for %s", orgName)
			}
			if len(tv.Tier.AllowedModels) == 0 || tv.Tier.AllowedModels[0] != "*" {
				t.Errorf("expected allowedModels=['*'] for %s, got %v", orgName, tv.Tier.AllowedModels)
			}
			if tv.Balance.EffectiveAvailable < DefaultEcosystemCreditCents {
				t.Errorf("effectiveAvailable = %d, want at least %d", tv.Balance.EffectiveAvailable, DefaultEcosystemCreditCents)
			}

			// TierOf check
			resolvedTier, err := TierOf(ctx, org, orgName)
			if err != nil {
				t.Fatalf("TierOf error for %s: %v", orgName, err)
			}
			if resolvedTier != tier.Enterprise {
				t.Errorf("TierOf = %s, want %s", resolvedTier, tier.Enterprise)
			}
		})
	}
}

func TestPaidEcosystemOrgs_EnsureCreditsTopup(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	org := moneyOrg("hanzo")
	db := datastore.New(org.Namespaced(ctx))

	// Initial grant
	if err := EnsureEcosystemCredits(ctx, db, "hanzo"); err != nil {
		t.Fatalf("EnsureEcosystemCredits failed: %v", err)
	}

	grants, err := getActiveGrants(db, "hanzo")
	if err != nil {
		t.Fatalf("getActiveGrants failed: %v", err)
	}
	var total int64
	for _, g := range grants {
		total += g.RemainingCents
	}
	if total != DefaultEcosystemCreditCents {
		t.Fatalf("total credits = %d, want %d", total, DefaultEcosystemCreditCents)
	}

	// Burn 30000 credits
	rem, err := BurnCredits(db, "hanzo", 30000, "")
	if err != nil || rem != 0 {
		t.Fatalf("BurnCredits failed: rem=%d, err=%v", rem, err)
	}

	// Verify balance after burn
	grantsAfter, _ := getActiveGrants(db, "hanzo")
	t.Logf("grantsAfter count=%d", len(grantsAfter))
	for idx, g := range grantsAfter {
		t.Logf("grant %d: id=%s amount=%d remaining=%d", idx, g.Id(), g.AmountCents, g.RemainingCents)
	}
	var remaining int64
	for _, g := range grantsAfter {
		remaining += g.RemainingCents
	}
	if remaining != 70000 {
		t.Fatalf("remaining credits = %d, want 70000", remaining)
	}

	// Topup back to 100000
	if err := EnsureEcosystemCredits(ctx, db, "hanzo"); err != nil {
		t.Fatalf("EnsureEcosystemCredits topup failed: %v", err)
	}

	grantsTopup, _ := getActiveGrants(db, "hanzo")
	var toppedUpTotal int64
	for _, g := range grantsTopup {
		toppedUpTotal += g.RemainingCents
	}
	if toppedUpTotal != DefaultEcosystemCreditCents {
		t.Fatalf("after topup credits = %d, want %d", toppedUpTotal, DefaultEcosystemCreditCents)
	}
}

func TestPaidEcosystemOrgs_SubscribeWithCredits_PerSeat(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	org := moneyOrg("lux")
	// team = 2400c/mo per seat, minSeats 2. quantity 2 -> 4800c
	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"credits","planId":"team","quantity":2}`, nil)
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s, want 201", resp.StatusCode, string(body))
	}
	out := jsonBody(t, resp)
	if out["amountCents"].(float64) != 4800 {
		t.Fatalf("amountCents = %v, want 4800", out["amountCents"])
	}
	if out["paymentMethodId"] != "credits" {
		t.Fatalf("paymentMethodId = %v, want credits", out["paymentMethodId"])
	}

	db := datastore.New(org.Namespaced(ctx))
	sub := parentSub(t, db, "lux", "team")
	if sub == nil {
		t.Fatal("no team subscription created for lux")
	}
	if sub.ProviderType != "credit" {
		t.Fatalf("sub ProviderType = %s, want credit", sub.ProviderType)
	}

	invs := invoicesForSub(t, db, sub.Id())
	if len(invs) != 1 {
		t.Fatalf("invoices count = %d, want 1", len(invs))
	}
	if invs[0].AmountDue != 4800 || invs[0].AmountPaid != 4800 {
		t.Fatalf("invoice amountDue=%d amountPaid=%d, want 4800/4800", invs[0].AmountDue, invs[0].AmountPaid)
	}
}

func TestPaidEcosystemOrgs_SubscribeNoCardNeeded(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	org := moneyOrg("zoo")
	// Ecosystem partner org subscribing with no card nonce at all -> defaults to operating credits!
	resp := invokeSubscribeCard(org, ctx, `{"planId":"team","quantity":2}`, nil)
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s, want 201", resp.StatusCode, string(body))
	}
	out := jsonBody(t, resp)
	if out["paymentMethodId"] != "credits" {
		t.Fatalf("paymentMethodId = %v, want credits", out["paymentMethodId"])
	}

	db := datastore.New(org.Namespaced(ctx))
	sub := parentSub(t, db, "zoo", "team")
	if sub == nil {
		t.Fatal("no team subscription created for zoo")
	}
	if sub.ProviderType != "credit" {
		t.Fatalf("sub ProviderType = %s, want credit", sub.ProviderType)
	}
}

func TestSquareSandbox_SubscribeWithCardNonceOk(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	org := moneyOrg("acme-square-sandbox")
	m := squareMock("cust_sandbox", "ccof_sandbox", "sqpay_sandbox_123")
	withFakeSquare(t, m)

	// Using Square sandbox test card nonce "cnon:card-nonce-ok"
	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:card-nonce-ok","planId":"team","quantity":2}`, nil)
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s, want 201", resp.StatusCode, string(body))
	}
	out := jsonBody(t, resp)
	if out["amountCents"].(float64) != 4800 {
		t.Fatalf("amountCents = %v, want 4800", out["amountCents"])
	}
	if m.lastChargeAmount != 4800 {
		t.Fatalf("charged amount = %d, want 4800", m.lastChargeAmount)
	}
}
