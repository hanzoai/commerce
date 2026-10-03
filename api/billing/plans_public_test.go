package billing

import (
	stdjson "encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/test/ae"
)

// The public plan catalog states no AI usage figure. What a plan allows of AI is
// cloud's usage policy (apps/ai/limits), read at /v1/ai/limits; a figure on a
// public plan is a second statement of it that customers read and nothing
// enforces. Live rows were written by an older catalog and by admin edits, so the
// answer is checked against a row that still carries the old figures, not only
// against the embed.
var aiUsage = regexp.MustCompile(`requestsPer|tokensPerMinute|included_cents|session_cents|weekly`)

func TestPublicPlansStateNoAIUsage(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	if _, _, err := SeedPlans(c); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// An admin row written the way live rows were: decoded from the CRUD body,
	// figures beside the capacities that stay.
	pod := plan.New(plan.AuthorityDB(c))
	body := `{"slug":"pod","name":"Pod","category":"agency","price":2499900,"limits":{` +
		`"requestsPerMinute":10000,"tokensPerMinute":20000000,"requestsPerHour":8000,` +
		`"requestsPerDay":50000,"requestsPerWeek":240000,"requestsPerMonth":800000,` +
		`"teamGuests":3,"agents":-1,"bots":1}}`
	if err := json.DecodeBytes([]byte(body), pod); err != nil {
		t.Fatalf("decode admin row: %v", err)
	}
	pod.AdminEdited, pod.Managed = true, true
	if err := pod.Create(); err != nil {
		t.Fatalf("create admin row: %v", err)
	}

	clean := func(name string, raw []byte) {
		t.Helper()
		if m := aiUsage.FindAll(raw, -1); len(m) > 0 {
			t.Errorf("%s states AI usage: %q", name, m)
		}
	}

	// The plane's answer: what cloud forwards as GET /v1/billing/plans.
	plans, err := ReadPlans(c, "", "", nil)
	if err != nil {
		t.Fatalf("read plans: %v", err)
	}
	if _, ok := planAuthorityRows(c); !ok {
		t.Fatal("the authority is empty after the seed, so the embed answered and the row path is untested")
	}
	raw, err := stdjson.Marshal(plans)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	clean("ReadPlans", raw)

	// The endpoints, through real routing.
	get := func(pattern, path string, h zip.Handler) []byte {
		t.Helper()
		r := driveSeeded(func(zc *zip.Ctx) { zc.SetContext(c) }, pattern,
			httptest.NewRequest(http.MethodGet, path, nil), h)
		b, _ := io.ReadAll(r.Body)
		if r.StatusCode != http.StatusOK {
			t.Fatalf("GET %s: status=%d body=%s", path, r.StatusCode, b)
		}
		return b
	}
	clean("GET /v1/billing/plans", get("/v1/billing/plans", "/v1/billing/plans", ListPlans))
	clean("GET /v1/billing/plans/pod", get("/v1/billing/plans/:id", "/v1/billing/plans/pod", GetPlan))
	clean("GET /v1/billing/plans/max-20x", get("/v1/billing/plans/:id", "/v1/billing/plans/max-20x", GetPlan))

	// The embed fallback answers the same way.
	raw, _ = stdjson.Marshal(views(catalog))
	clean("embed", raw)

	// Capacities are not usage, and stay.
	by := map[string]PlanView{}
	for _, p := range plans {
		by[p.Slug] = p
	}
	l := by["pod"].Limits
	if l == nil || l.Agents == nil || *l.Agents != -1 || l.Bots == nil || *l.Bots != 1 || l.TeamGuests == nil || *l.TeamGuests != 3 {
		t.Errorf("pod capacities = %+v, want agents -1, bots 1, teamGuests 3", l)
	}
	if l := by["team"].Limits; l == nil || l.MinSeats == nil || *l.MinSeats != 2 || l.MaxMembers == nil || *l.MaxMembers != 100 {
		t.Errorf("team capacities = %+v, want minSeats 2, maxMembers 100", l)
	}
	if l := by["dns-pro"].Limits; l == nil || l.Zones == nil || *l.Zones != 25 {
		t.Errorf("dns-pro quotas = %+v, want zones 25", l)
	}
}
