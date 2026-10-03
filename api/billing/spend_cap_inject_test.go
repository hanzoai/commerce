package billing

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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

// An exhausted enforcing cap denies, and nothing in the environment can talk it out of
// that. The verdict is a function of the row and the spend — the two things an operator
// can see — so a cap reading "$100" cannot be quietly serving an unbounded one.
//
// The environment is set here to the spelling that used to disable enforcement, because
// the property worth pinning is that it now means nothing: a deployment that still
// carries the old variable, or a shell that exports it out of habit, does not reopen the
// ceiling.
func TestSpendCap_ExhaustedCapDeniesRegardlessOfEnvironment(t *testing.T) {
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

	if v := authorize(t, org, "user=enforce-flag&amount=1"); v.Allow || v.Reason != "spend_cap" {
		t.Fatalf("authorize = %+v, want deny spend_cap", v)
	}

	t.Setenv("SPEND_CAP_ENFORCE", "false")
	if v := authorize(t, org, "user=enforce-flag&amount=1"); v.Allow || v.Reason != "spend_cap" {
		t.Fatalf("with SPEND_CAP_ENFORCE=false: authorize = %+v, want deny spend_cap — the cap is not switchable", v)
	}
}

// An ENFORCED cap whose spend cannot be read reaches no verdict: the ceiling exists
// and nothing can say the request fits under it, so AuthorizeCap answers ErrCapUnread
// and the endpoint 503s rather than allowing the spend.
func TestSpendCap_ReaderError_EnforcedCapReachesNoVerdict(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	org := &organization.Organization{}
	org.Name = "reader-error"
	warmNamespace(org)

	SetPeriodSpendReader(func(_ context.Context, _ string, _ bool, _, _ string) (int64, error) {
		return 0, errors.New("finance store busy")
	})
	defer SetPeriodSpendReader(nil)
	createCap(t, org, `{"title":"cap","threshold":100,"enforce":true}`)

	if v, err := AuthorizeCap(context.Background(), org, "", "", 1, false); !errors.Is(err, ErrCapUnread) {
		t.Fatalf("AuthorizeCap = %+v, %v; want ErrCapUnread", v, err)
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/billing/alerts/authorize?user=reader-error&amount=1", nil)
	if w := driveSeeded(capSeed(org), "/v1/billing/alerts/authorize", req, AuthorizeSpendCap); w.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("AuthorizeSpendCap status = %d, want 503", w.StatusCode)
	}
}

// A SOFT cap never denies, so its unreadable spend costs only the warn figure.
func TestSpendCap_ReaderError_SoftCapAllows(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	org := &organization.Organization{}
	org.Name = "reader-error-soft"
	warmNamespace(org)

	SetPeriodSpendReader(func(_ context.Context, _ string, _ bool, _, _ string) (int64, error) {
		return 0, errors.New("finance store busy")
	})
	defer SetPeriodSpendReader(nil)
	createCap(t, org, `{"title":"cap","threshold":100,"enforce":false}`)

	if v := authorize(t, org, "user=reader-error-soft&amount=1"); !v.Allow || v.WarnPct != 0 {
		t.Fatalf("authorize = %+v, want allow with no warn figure", v)
	}
}

// An org with no cap reads no spend at all: a reader that would fail is never asked.
func TestSpendCap_NoCap_AllowsWithoutReadingSpend(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	org := &organization.Organization{}
	org.Name = "no-cap"
	warmNamespace(org)

	asked := false
	SetPeriodSpendReader(func(_ context.Context, _ string, _ bool, _, _ string) (int64, error) {
		asked = true
		return 0, errors.New("finance store busy")
	})
	defer SetPeriodSpendReader(nil)

	if v := authorize(t, org, "user=no-cap&amount=1"); !v.Allow || asked {
		t.Fatalf("authorize = %+v (spend read: %v), want allow without a spend read", v, asked)
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
