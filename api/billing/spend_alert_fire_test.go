package billing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/spendalert"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// warmNamespace establishes the org's per-org SQLite handle once with a read-only
// query (the ae harness gives each org its own DB via manager.Org; the first access
// opens it), so a later Fiber-driven create and list share visibility. Prod always
// warms the handle via the gateway before any CRUD — a test-only setup step.
func warmNamespace(org *organization.Organization) {
	db := datastore.New(nscontext.WithNamespace(context.Background(), org.Name))
	_, _ = spendalert.Query(db).Ancestor(db.NewKey("synckey", "", 1, nil)).Count()
}

// The fire pass runs on the SAME datastore handle the money path hands it —
// org.Namespaced(c.Context()) inside a request, NOT a bare context.Background()
// one — so these helpers drive seed/fire through a Fiber request (like RecordUsage
// does in prod) to exercise the exact handle that GetById+Update writes through.

// driveSeedUsage writes ONE api-usage debit through a request-scoped db — the exact
// row shape RecordUsage creates and scopeSpentCents sums — without firing (so the
// synchronous fire below is the single, deterministic write under assertion).
func driveSeedUsage(t *testing.T, org *organization.Organization, cents int64, project, service string) {
	t.Helper()
	h := func(c *zip.Ctx) error {
		o := middleware.GetOrganization(c)
		db := datastore.New(o.Namespaced(c.Context()))
		trans := transaction.New(db)
		trans.Type = transaction.Withdraw
		trans.SourceId = o.Name
		trans.SourceKind = "iam-user"
		trans.Currency = currency.Type("usd")
		trans.Amount = currency.Cents(cents)
		trans.Tags = "api-usage"
		trans.Project = spendalert.NormalizeProject(project)
		trans.Service = service
		trans.Test = o.TestMode()
		if err := trans.Create(); err != nil {
			return c.JSON(500, map[string]any{"err": err.Error()})
		}
		return c.JSON(200, map[string]any{"ok": true})
	}
	req := httptest.NewRequest(http.MethodPost, "/seed", nil)
	if w := driveSeeded(capSeed(org), "/seed", req, h); w.StatusCode != 200 {
		t.Fatalf("seed usage status = %d", w.StatusCode)
	}
}

// driveFire runs the fire pass synchronously inside a request, on the request db —
// the SAME handle RecordUsage's detached fire goroutine receives — so the
// GetById+Update lands exactly as it does in prod. ev nil → stamp only (what the
// Budgets UI reads).
func driveFire(t *testing.T, org *organization.Organization, project, service string) {
	t.Helper()
	h := func(c *zip.Ctx) error {
		o := middleware.GetOrganization(c)
		db := datastore.New(o.Namespaced(c.Context()))
		checkAndFireSpendAlerts(c.Context(), db, o.Name, o.TestMode(), project, service, nil)
		return c.JSON(200, map[string]any{"ok": true})
	}
	req := httptest.NewRequest(http.MethodPost, "/fire", nil)
	if w := driveSeeded(capSeed(org), "/fire", req, h); w.StatusCode != 200 {
		t.Fatalf("fire status = %d", w.StatusCode)
	}
}

// stampStale rewrites a cap's TriggeredAt to a PRIOR-period stamp through the
// request db, simulating a fire that happened last month (for the reset test).
func stampStale(t *testing.T, org *organization.Organization, id, stamp string) {
	t.Helper()
	h := func(c *zip.Ctx) error {
		o := middleware.GetOrganization(c)
		db := datastore.New(o.Namespaced(c.Context()))
		a := spendalert.New(db)
		if err := a.GetById(id); err != nil {
			return c.JSON(404, map[string]any{"err": err.Error()})
		}
		a.TriggeredAt = stamp
		if err := saveAlert(db, a); err != nil {
			return c.JSON(500, map[string]any{"err": err.Error()})
		}
		return c.JSON(200, map[string]any{"ok": true})
	}
	req := httptest.NewRequest(http.MethodPost, "/stamp", nil)
	if w := driveSeeded(capSeed(org), "/stamp", req, h); w.StatusCode != 200 {
		t.Fatalf("stamp stale status = %d", w.StatusCode)
	}
}

// triggeredAt reads the single cap's derived triggeredAt stamp via the SAME view
// the console Budgets page renders — so the test asserts exactly what a customer
// (and the fleet read side) observes.
func triggeredAt(t *testing.T, org *organization.Organization) string {
	t.Helper()
	rows := listCaps(t, org)
	if len(rows) != 1 {
		t.Fatalf("want exactly 1 cap, got %d", len(rows))
	}
	s, _ := rows[0]["triggeredAt"].(string)
	return s
}

// levelFor/parseTriggerStamp are pure — assert the ladder and the debounce codec
// directly, no datastore.
func TestSpendAlert_LevelFor_ReusesVerdictBoundaries(t *testing.T) {
	a := &spendalert.SpendAlert{Threshold: 100, SoftPct: 50} // $1 cap, warn at 50%.
	cases := []struct {
		spent int64
		want  alertLevel
	}{
		{0, levelNone},
		{49, levelNone},  // under the 50% soft threshold.
		{50, levelWarn},  // at the soft threshold.
		{99, levelWarn},  // under the cap.
		{100, levelOver}, // at the cap — a further billable request is refused.
		{150, levelOver}, // over.
	}
	for _, c := range cases {
		if got := levelFor(a, c.spent); got != c.want {
			t.Errorf("levelFor(spent=%d) = %v, want %v", c.spent, got, c.want)
		}
	}
	if got := levelFor(&spendalert.SpendAlert{Threshold: 0}, 1_000_000); got != levelNone {
		t.Errorf("unlimited levelFor = %v, want none", got)
	}
}

func TestSpendAlert_TriggerStamp_RoundTripAndLegacy(t *testing.T) {
	p, l := parseTriggerStamp(triggerStamp("2026-07", levelOver))
	if p != "2026-07" || l != levelOver {
		t.Fatalf("round-trip = (%q,%v), want (2026-07,over)", p, l)
	}
	// A legacy bare ISO timestamp (the field's original form) must decode to level
	// none so it never suppresses a real crossing.
	if _, l := parseTriggerStamp("2026-07-01T00:00:00Z"); l != levelNone {
		t.Fatalf("legacy stamp level = %v, want none (must not suppress a crossing)", l)
	}
	if _, l := parseTriggerStamp(""); l != levelNone {
		t.Fatalf("empty stamp level = %v, want none", l)
	}
}

// The core behavior: a debit that crosses the soft threshold fires a WARN once;
// crossing the cap ESCALATES to over once; re-running is DEBOUNCED (no re-fire).
func TestSpendAlert_Fire_WarnEscalateDebounce(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	org := &organization.Organization{}
	org.Name = "fire-warn"
	warmNamespace(org)

	// $1 hard cap, warn at 50%.
	createCap(t, org, `{"title":"cap","threshold":100,"enforce":true,"softPct":50}`)
	if got := triggeredAt(t, org); got != "" {
		t.Fatalf("pre-spend triggeredAt = %q, want empty", got)
	}
	period := currentPeriod()

	// Spend $0.60 — over the 50% soft threshold, under the cap → WARN.
	driveSeedUsage(t, org, 60, "", "")
	driveFire(t, org, "", "")
	if got, want := triggeredAt(t, org), triggerStamp(period, levelWarn); got != want {
		t.Fatalf("after soft crossing triggeredAt = %q, want %q", got, want)
	}

	// Re-run at the SAME level → debounced, stamp unchanged.
	driveFire(t, org, "", "")
	if got, want := triggeredAt(t, org), triggerStamp(period, levelWarn); got != want {
		t.Fatalf("debounce broke: triggeredAt = %q, want %q", got, want)
	}

	// Spend $0.40 more (total $1.00 = cap) → ESCALATE to over.
	driveSeedUsage(t, org, 40, "", "")
	driveFire(t, org, "", "")
	if got, want := triggeredAt(t, org), triggerStamp(period, levelOver); got != want {
		t.Fatalf("after cap crossing triggeredAt = %q, want %q (escalate warn→over)", got, want)
	}

	// Re-run at over → debounced, stays over.
	driveFire(t, org, "", "")
	if got, want := triggeredAt(t, org), triggerStamp(period, levelOver); got != want {
		t.Fatalf("over debounce broke: triggeredAt = %q, want %q", got, want)
	}
}

// An ALERT-ONLY row (enforce=false) never blocks a request, but it MUST still fire
// the alert when its threshold is crossed — the "alert" the row has always been.
func TestSpendAlert_Fire_AlertOnly_StillFires(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	org := &organization.Organization{}
	org.Name = "fire-soft"
	warmNamespace(org)

	createCap(t, org, `{"title":"alert","threshold":100,"enforce":false,"softPct":80}`)
	driveSeedUsage(t, org, 100, "", "") // at the ceiling → over.
	driveFire(t, org, "", "")
	if got, want := triggeredAt(t, org), triggerStamp(currentPeriod(), levelOver); got != want {
		t.Fatalf("alert-only triggeredAt = %q, want %q (a soft alert still fires)", got, want)
	}
}

// Monthly reset: a stamp from a PRIOR period is stale, so the same over-cap spend
// RE-ARMS the alert in the new window — the SAME UTC-month Schedule the cap resets
// on. Proven by pre-seeding a stale stamp and asserting the fire re-stamps to now.
func TestSpendAlert_Fire_MonthlyReset_ReArms(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	org := &organization.Organization{}
	org.Name = "fire-reset"
	warmNamespace(org)

	id := createCapAs(t, org, "owner", `{"title":"cap","threshold":100,"enforce":true}`)
	driveSeedUsage(t, org, 100, "", "")
	stampStale(t, org, id, triggerStamp("2020-01", levelOver)) // a long-past window.

	// The same over-cap spend must RE-FIRE in the current window (stale period != now).
	driveFire(t, org, "", "")
	if got, want := triggeredAt(t, org), triggerStamp(currentPeriod(), levelOver); got != want {
		t.Fatalf("after reset triggeredAt = %q, want %q (window rollover re-arms the alert)", got, want)
	}
}

// Ancestor preservation (regression): editing a cap (PATCH) or firing its alert
// (which stamps TriggeredAt) MUST keep the row under its synckey ancestor so the
// ancestor-scoped ListSpendAlerts/loadOrgScopes/ScopeRules still see it. Before the
// saveAlert fix, GetById reconstructed a root key and Update re-homed the row off
// the ancestor — the budget VANISHED from the customer's list on any edit AND
// enforcement silently died (0 covering rows → no 402/429). This proves both the
// PATCH path and the fire path stay listable.
func TestSpendAlert_Update_PreservesAncestor_StaysListable(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	org := &organization.Organization{}
	org.Name = "fire-anchor"
	warmNamespace(org)

	id := createCapAs(t, org, "owner", `{"title":"c","threshold":100,"enforce":true}`)
	if n := len(listCaps(t, org)); n != 1 {
		t.Fatalf("after create: n=%d, want 1", n)
	}

	// PATCH must keep it listable (the pre-existing bug this fix closes).
	if code := patchCapAs(t, org, id, "owner", "", `{"threshold":200}`); code != 200 {
		t.Fatalf("PATCH status = %d", code)
	}
	if n := len(listCaps(t, org)); n != 1 {
		t.Fatalf("after PATCH: n=%d, want 1 (edit must not un-list the budget)", n)
	}

	// Firing the alert (stamps TriggeredAt) must ALSO keep it listable.
	driveSeedUsage(t, org, 200, "", "")
	driveFire(t, org, "", "")
	rows := listCaps(t, org)
	if len(rows) != 1 {
		t.Fatalf("after fire: n=%d, want 1 (a fired alert must not un-list the cap)", len(rows))
	}
	if got, _ := rows[0]["triggeredAt"].(string); got != triggerStamp(currentPeriod(), levelOver) {
		t.Fatalf("triggeredAt = %q, want over stamp (fire persisted)", got)
	}
}

// Tenant/scope isolation: a debit on project P must not fire an alert whose scope
// is a DIFFERENT project Q. Covering is the same relation the cap verdict uses.
func TestSpendAlert_Fire_ScopeIsolation(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	org := &organization.Organization{}
	org.Name = "fire-iso"
	warmNamespace(org)

	// A $1 cap scoped to project Q.
	createCap(t, org, `{"title":"Q cap","threshold":100,"enforce":true,"project":"Q"}`)

	// Spend $1 on project P (a different scope) and fire P's scope — Q's alert must
	// NOT trip.
	driveSeedUsage(t, org, 100, "P", "")
	driveFire(t, org, "P", "")
	if got := triggeredAt(t, org); got != "" {
		t.Fatalf("Q-scoped alert fired on a P debit: triggeredAt = %q, want empty", got)
	}
}
