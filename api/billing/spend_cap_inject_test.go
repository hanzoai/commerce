package billing

import (
	"context"
	"testing"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/test/ae"
)

// The co-resident host injects a period-spend reader (the finance ledger). With it
// set, the cap must enforce on the INJECTED spend even though ZERO commerce
// transactions exist — the exact unified-binary reality (usage is recorded on the
// finance path, commerce's transaction store is empty). Without the seam the cap
// summed 0 and never tripped in prod.
func TestSpendCap_InjectedReader_EnforcesOnFinanceSpend(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	org := &organization.Organization{}
	org.Name = "inject-cap"
	warmNamespace(org)

	var gotOrg, gotProj, gotSvc string
	SetPeriodSpendReader(func(_ context.Context, o string, _ bool, project, service string) (int64, error) {
		gotOrg, gotProj, gotSvc = o, project, service
		return 100, nil // $1.00 of finance-ledger spend, org-wide.
	})
	defer SetPeriodSpendReader(nil)

	// A $1.00 hard cap. NO commerce usage is recorded — only the injected spend.
	createCap(t, org, `{"title":"cap","threshold":100,"enforce":true}`)

	v := authorize(t, org, "user=inject-cap&amount=1")
	if v.Allow || v.Reason != "spend_cap" {
		t.Fatalf("authorize = %+v, want deny spend_cap (injected finance spend must drive the cap)", v)
	}
	if v.SpentCents != 100 {
		t.Fatalf("spentCents = %d, want 100 (from the injected reader, not commerce transactions)", v.SpentCents)
	}
	if gotOrg != "inject-cap" || gotProj != "" || gotSvc != "" {
		t.Fatalf("reader got org=%q proj=%q svc=%q, want inject-cap / '' / '' (org-wide scope)", gotOrg, gotProj, gotSvc)
	}
}

// SPEND_CAP_ENFORCE gates whether a spend_cap verdict DENIES. Default OFF (fail-open,
// shadow): the same over-cap scope ALLOWS (so an auto-deployed cap never blocks a real
// customer before the canary proof); ON: it denies. This is the auto-ship safety net.
func TestSpendCap_EnforceFlag_ShadowWhenOff(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	org := &organization.Organization{}
	org.Name = "enforce-flag"
	warmNamespace(org)

	// $1 hard cap already exhausted (via the injected reader).
	SetPeriodSpendReader(func(_ context.Context, _ string, _ bool, _, _ string) (int64, error) {
		return 100, nil
	})
	defer SetPeriodSpendReader(nil)
	createCap(t, org, `{"title":"cap","threshold":100,"enforce":true}`)

	// Flag ON (the package default) → DENY.
	if v := authorize(t, org, "user=enforce-flag&amount=1"); v.Allow || v.Reason != "spend_cap" {
		t.Fatalf("flag ON: authorize = %+v, want deny spend_cap", v)
	}

	// Flag OFF → SHADOW: the scope is still over cap, but the verdict ALLOWS (no block).
	t.Setenv("SPEND_CAP_ENFORCE", "false")
	if v := authorize(t, org, "user=enforce-flag&amount=1"); !v.Allow || v.Reason != "" {
		t.Fatalf("flag OFF: authorize = %+v, want ALLOW (shadow, no deny)", v)
	}
}

// The alert fires off the INJECTED spend too (the host calls FireSpendAlerts after
// fin.RecordUsage), so the "alert" half also works on the finance path with no
// commerce transactions.
func TestSpendCap_InjectedReader_AlertFiresOnFinanceSpend(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	org := &organization.Organization{}
	org.Name = "inject-fire"
	warmNamespace(org)

	SetPeriodSpendReader(func(_ context.Context, _ string, _ bool, _, _ string) (int64, error) {
		return 100, nil
	})
	defer SetPeriodSpendReader(nil)

	createCap(t, org, `{"title":"cap","threshold":100,"enforce":true}`)
	driveFire(t, org, "", "") // exercises checkAndFireSpendAlerts → scopeSpentCents → injected reader.
	if ta := triggeredAt(t, org); ta != triggerStamp(currentPeriod(), levelOver) {
		t.Fatalf("triggeredAt = %q, want over stamp (alert must fire off injected finance spend)", ta)
	}
}

// Clearing the reader restores the standalone transaction-ledger path (no leakage
// across the unified-binary boundary).
func TestSpendCap_NilReader_UsesTransactionLedger(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	org := &organization.Organization{}
	org.Name = "inject-nil"
	warmNamespace(org)
	SetPeriodSpendReader(nil) // explicit: standalone.

	createCap(t, org, `{"title":"cap","threshold":100,"enforce":true}`)
	// No injected reader and no commerce usage → spent 0 → ALLOW (transaction path).
	if v := authorize(t, org, "user=inject-nil&amount=1"); !v.Allow {
		t.Fatalf("nil reader + no usage must ALLOW (transaction ledger, spent 0): %+v", v)
	}
	// Record real commerce usage → the transaction path sums it → cap trips.
	driveSeedUsage(t, org, 100, "", "")
	if v := authorize(t, org, "user=inject-nil&amount=1"); v.Allow || v.Reason != "spend_cap" {
		t.Fatalf("nil reader + $1 commerce usage must deny spend_cap: %+v", v)
	}
}
