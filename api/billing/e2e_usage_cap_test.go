package billing

import (
	"testing"
	"time"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/test/ae"
)

// TestE2E_SelfServiceUsageCap_Flow walks the ENTIRE self-service usage-cap flow the
// way a live customer experiences it, driving the REAL handlers end to end:
//
//  1. customer creates a usage cap (spend-alert, ENFORCE) via the CRUD;
//  2. billable usage accrues under the cap → the gate ALLOWS;
//  3. usage crosses the soft threshold → still ALLOWED, but the gate WARNS (warnPct);
//  4. the alert FIRES on that crossing (TriggeredAt stamped, debounced);
//  5. usage reaches the cap → the next request is STOPPED with the spend_cap verdict
//     (distinct from insufficient_balance), carrying capCents/spentCents;
//  6. the alert ESCALATES to over;
//  7. the cap view exposes the monthly window (period + resetsAt = first of next UTC
//     month) the spend resets on.
//
// This is the deterministic money-logic proof behind the ship gate: cap set → hit →
// stopped + alerted → resets, on one org, in one test.
func TestE2E_SelfServiceUsageCap_Flow(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	org := &organization.Organization{}
	org.Name = "e2e-cap"
	warmNamespace(org)
	period := currentPeriod()

	// (1) The customer sets a $1.00 HARD cap that warns at 50%.
	createCap(t, org, `{"title":"monthly cap","threshold":100,"enforce":true,"softPct":50}`)

	// A brand-new cap has not fired and exposes its window.
	rows := listCaps(t, org)
	if len(rows) != 1 {
		t.Fatalf("want 1 cap, got %d", len(rows))
	}
	if ta, _ := rows[0]["triggeredAt"].(string); ta != "" {
		t.Fatalf("fresh cap triggeredAt = %q, want empty", ta)
	}
	if p, _ := rows[0]["period"].(string); p != period {
		t.Fatalf("cap period = %q, want %q", p, period)
	}
	wantReset := periodResetsAtUTC().Format(time.RFC3339)
	if r, _ := rows[0]["resetsAt"].(string); r != wantReset {
		t.Fatalf("cap resetsAt = %q, want %q (first of next UTC month)", r, wantReset)
	}

	// (2) A $0.40 request under the cap is AUTHORIZED.
	if v := authorize(t, org, "user=e2e-cap&amount=40"); !v.Allow {
		t.Fatalf("under-cap request denied: %+v", v)
	}

	// (3) Usage climbs to $0.60 — over the 50% soft threshold, still under the cap.
	driveSeedUsage(t, org, 60, "", "")
	warn := authorize(t, org, "user=e2e-cap&amount=1")
	if !warn.Allow {
		t.Fatalf("at 60%% utilization the request must still ALLOW: %+v", warn)
	}
	if warn.WarnPct < 60 {
		t.Fatalf("warnPct = %d, want >=60 (soft-warn signal for X-Spend-Warn)", warn.WarnPct)
	}

	// (4) The alert FIRES on the soft crossing (warn), debounced.
	driveFire(t, org, "", "")
	if ta := triggeredAt(t, org); ta != triggerStamp(period, levelWarn) {
		t.Fatalf("after soft crossing triggeredAt = %q, want warn stamp", ta)
	}

	// (5) Usage reaches the $1.00 cap — the next billable request is STOPPED.
	driveSeedUsage(t, org, 40, "", "") // 60 + 40 = 100 = cap.
	stop := authorize(t, org, "user=e2e-cap&amount=1")
	if stop.Allow {
		t.Fatalf("at the cap the request must be STOPPED, got allow: %+v", stop)
	}
	if stop.Reason != "spend_cap" {
		t.Fatalf("stop reason = %q, want spend_cap (DISTINCT from insufficient_balance)", stop.Reason)
	}
	if stop.CapCents != 100 || stop.SpentCents != 100 {
		t.Fatalf("stop cap/spent = %d/%d, want 100/100", stop.CapCents, stop.SpentCents)
	}
	if stop.WarnPct != 0 {
		t.Fatalf("a deny carries no warn, got warnPct=%d", stop.WarnPct)
	}

	// (6) The alert ESCALATES to over.
	driveFire(t, org, "", "")
	if ta := triggeredAt(t, org); ta != triggerStamp(period, levelOver) {
		t.Fatalf("after cap crossing triggeredAt = %q, want over stamp", ta)
	}

	// (7) The view reflects the exhausted, over-cap state + the reset window.
	rows = listCaps(t, org)
	if over, _ := rows[0]["over"].(bool); !over {
		t.Fatalf("cap view over = false, want true (a further billable request is refused)")
	}
	if spent, _ := rows[0]["periodSpentCents"].(float64); int64(spent) != 100 {
		t.Fatalf("periodSpentCents = %v, want 100", spent)
	}
	if r, _ := rows[0]["resetsAt"].(string); r != wantReset {
		t.Fatalf("resetsAt = %q, want %q — the spend resets at the monthly rollover", r, wantReset)
	}
	// The window reset itself (a prior-period stamp re-arms the same over-cap spend)
	// is proven deterministically by TestSpendAlert_Fire_MonthlyReset_ReArms; here the
	// exposed resetsAt is the concrete boundary the customer sees.
}
