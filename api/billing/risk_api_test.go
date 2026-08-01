// Copyright © 2026 Hanzo AI. MIT License.

package billing

// risk_api_test.go drives the risk face through the REAL registration —
// RiskRoute's own group, its own TokenRequired, its own Bind — so what these
// tests prove is the production chain and not a hand-copied middleware list.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/dispute"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/risk"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/test/ae"
)

// answers is the scoring plane these tests install: it answers what the test
// says and records nothing the test does not ask about.
type answers struct{ action risk.Action }

func (a answers) Decide(context.Context, *risk.Ask) (*risk.Decision, error) {
	act := a.action
	if act == "" {
		act = risk.Allow
	}
	return &risk.Decision{ID: "d_test", Action: act, Score: 0.5}, nil
}
func (a answers) Label(context.Context, *risk.Label) error { return nil }

// face mounts the real risk face and returns a caller bound to one org.
//
// The seed sets exactly what a live IAM-authenticated request carries by the
// time it reaches this group — the validated principal's org and its scope —
// and nothing else. TokenRequired then takes its own IAM branch, so the gate
// under test is the production gate.
func face(t *testing.T, ctx context.Context, orgName string) func(method, path, body string) *http.Response {
	t.Helper()
	org := &organization.Organization{}
	org.Name = orgName
	org.Live = true

	app := zip.New(zip.Config{DisableStartupMessage: true, AppName: "risk-api-test"})
	app.Use(func(c *zip.Ctx) error {
		c.SetContext(ctx)
		c.Locals("iam_authenticated", true)
		c.Locals("permissions", bit.Field(permission.Admin|permission.Live))
		c.Locals("organization", org)
		return c.Continue()
	})
	RiskRoute(app)

	return func(method, path, body string) *http.Response {
		var rdr io.Reader
		if body != "" {
			rdr = bytes.NewBufferString(body)
		}
		req := httptest.NewRequest(method, path, rdr)
		req.Header.Set("Content-Type", "application/json")
		res, err := app.Fiber().Test(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		return res
	}
}

func decode(t *testing.T, res *http.Response) map[string]any {
	t.Helper()
	raw, _ := io.ReadAll(res.Body)
	out := map[string]any{}
	if len(raw) == 0 {
		return out
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("body %q: %v", string(raw), err)
	}
	return out
}

func TestRiskAPI_ScreenRecordsAndReturnsTheDecision(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	call := face(t, ctx, "apiscreen")
	res := call(http.MethodPost, "/v1/billing/risk/screen",
		`{"stage":"payment","subjectKind":"customer","subject":"c1","amount":4200,"currency":"usd","signals":{"ip":"203.0.113.7"}}`)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status=%d want 200", res.StatusCode)
	}
	body := decode(t, res)
	if body["action"] != "allow" || body["moves"] != true {
		t.Fatalf("screen=%v", body)
	}
	if body["decision"] != "d_test" {
		t.Fatalf("the decision was not anchored: %v", body)
	}
	if body["allowed"].(float64) != 4200 {
		t.Fatalf("allowed=%v want 4200", body["allowed"])
	}

	// The record is readable back, and the list carries it.
	id := body["id"].(string)
	one := decode(t, call(http.MethodGet, "/v1/billing/risk/screens/"+id, ""))
	if one["id"] != id {
		t.Fatalf("read back %v", one)
	}
	page := decode(t, call(http.MethodGet, "/v1/billing/risk/screens?subjectKind=customer&subject=c1", ""))
	if len(page["screens"].([]any)) != 1 {
		t.Fatalf("page=%v", page)
	}
}

// TestRiskAPI_ControlPlacedThenEnforcedThenReleased is the platform control in
// one arc: declare it, watch it stop the money, lift it.
func TestRiskAPI_ControlPlacedThenEnforcedThenReleased(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	call := face(t, ctx, "apicontrol")

	res := call(http.MethodPost, "/v1/billing/risk/controls",
		`{"effect":"hold","subjectKind":"merchant","subject":"m1","reason":"chargeback spike"}`)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("place: status=%d want 201", res.StatusCode)
	}
	placed := decode(t, res)
	if placed["live"] != true || placed["effect"] != "hold" {
		t.Fatalf("control=%v", placed)
	}

	// A payout for that merchant is now stopped.
	out := decode(t, call(http.MethodPost, "/v1/billing/risk/screen",
		`{"stage":"payout","subjectKind":"merchant","subject":"m1","amount":900,"out":true}`))
	if out["moves"] != false || out["action"] != "block" {
		t.Fatalf("the hold did not stop the payout: %v", out)
	}
	if out["allowed"].(float64) != 0 || out["held"].(float64) != 900 {
		t.Fatalf("allowed=%v held=%v want 0/900", out["allowed"], out["held"])
	}

	// And an inbound charge for the same merchant is not.
	in := decode(t, call(http.MethodPost, "/v1/billing/risk/screen",
		`{"stage":"payment","subjectKind":"merchant","subject":"m1","amount":900}`))
	if in["moves"] != true {
		t.Fatalf("the hold stopped an inbound charge: %v", in)
	}

	// Release it, and the payout moves again.
	rel := decode(t, call(http.MethodDelete, "/v1/billing/risk/controls/"+placed["id"].(string), ""))
	if rel["released"] != true || rel["live"] != false {
		t.Fatalf("release=%v", rel)
	}
	again := decode(t, call(http.MethodPost, "/v1/billing/risk/screen",
		`{"stage":"payout","subjectKind":"merchant","subject":"m1","amount":900,"out":true}`))
	if again["moves"] != true {
		t.Fatalf("a released control still restrains: %v", again)
	}
}

// TestRiskAPI_ReserveHoldsTheExactShare.
func TestRiskAPI_ReserveHoldsTheExactShare(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	call := face(t, ctx, "apireserve")
	if res := call(http.MethodPost, "/v1/billing/risk/controls",
		`{"effect":"reserve","subjectKind":"merchant","subject":"m1","rate":2500}`); res.StatusCode != http.StatusCreated {
		t.Fatalf("place: %d", res.StatusCode)
	}
	out := decode(t, call(http.MethodPost, "/v1/billing/risk/screen",
		`{"stage":"payout","subjectKind":"merchant","subject":"m1","amount":101,"out":true}`))
	if out["held"].(float64) != 26 || out["allowed"].(float64) != 75 {
		t.Fatalf("held=%v allowed=%v want 26/75", out["held"], out["allowed"])
	}
}

// TestRiskAPI_RefusesAnImpossibleReserveRate — a rate outside 0..100% is a
// caller's mistake and is named as one.
func TestRiskAPI_RefusesAnImpossibleReserveRate(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	call := face(t, ctx, "apirate")
	for _, body := range []string{
		`{"effect":"reserve","subjectKind":"merchant","subject":"m1","rate":0}`,
		`{"effect":"reserve","subjectKind":"merchant","subject":"m1","rate":10001}`,
		`{"effect":"nonsense","subjectKind":"merchant","subject":"m1"}`,
		`{"effect":"hold","subjectKind":"org","subject":"acme"}`,
	} {
		if res := call(http.MethodPost, "/v1/billing/risk/controls", body); res.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s: status=%d want 400", body, res.StatusCode)
		}
	}
}

// TestRiskAPI_OutcomeLabelsTheScreen — the post-purchase half of the loop.
func TestRiskAPI_OutcomeLabelsTheScreen(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	call := face(t, ctx, "apioutcome")
	scr := decode(t, call(http.MethodPost, "/v1/billing/risk/screen",
		`{"stage":"payment","subjectKind":"customer","subject":"c1","amount":4200,"currency":"usd"}`))

	res := call(http.MethodPost, "/v1/billing/risk/outcomes",
		`{"event":"dispute","subjectKind":"customer","subject":"c1","screen":"`+scr["id"].(string)+`","idem":"o-1"}`)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("outcome: status=%d want 201", res.StatusCode)
	}
	body := decode(t, res)
	if body["reported"] != true || body["decision"] != "d_test" {
		t.Fatalf("outcome=%v — the label did not reach the plane with its decision", body)
	}

	// Repeating under the same key returns the first record, not a second.
	again := decode(t, call(http.MethodPost, "/v1/billing/risk/outcomes",
		`{"event":"dispute","subjectKind":"customer","subject":"c1","screen":"`+scr["id"].(string)+`","idem":"o-1"}`))
	if again["id"] != body["id"] {
		t.Fatalf("a retried outcome made a second record: %v vs %v", again["id"], body["id"])
	}

	// The merchant standing now counts the dispute.
	st := decode(t, call(http.MethodGet, "/v1/billing/risk/merchants/c1", ""))
	if st["disputes"].(float64) != 0 {
		t.Fatalf("a customer's dispute was counted against a merchant of the same id: %v", st)
	}
}

// TestRiskAPI_MerchantReviewActs — continuous monitoring with the control the
// answer implies.
func TestRiskAPI_MerchantReviewActs(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{action: risk.Restrict})

	call := face(t, ctx, "apireview")

	// A read-only standing changes nothing.
	before := decode(t, call(http.MethodGet, "/v1/billing/risk/merchants/m1", ""))
	if before["screen"] != nil {
		t.Fatalf("a read recorded a judgement: %v", before)
	}

	st := decode(t, call(http.MethodPost, "/v1/billing/risk/merchants/m1/review", `{"act":true,"reserve":1500}`))
	if st["placed"] == nil || st["placed"] == "" {
		t.Fatalf("the review placed nothing: %v", st)
	}
	list := decode(t, call(http.MethodGet, "/v1/billing/risk/controls?live=true", ""))
	controls := list["controls"].([]any)
	if len(controls) != 1 {
		t.Fatalf("%d controls in force, want 1: %v", len(controls), list)
	}
	c := controls[0].(map[string]any)
	if c["effect"] != "reserve" || c["rate"].(float64) != 1500 {
		t.Fatalf("control=%v want a 15%% reserve", c)
	}
}

// TestRiskAPI_DisputeEvidenceAndTheSubmissionGap — assembly is ours; submission
// has no adjudicator, and the API says so instead of returning success.
func TestRiskAPI_DisputeEvidenceAndTheSubmissionGap(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	org := &organization.Organization{}
	org.Name = "apidispute"
	org.Live = true
	db := datastore.New(org.Namespaced(ctx))
	d := dispute.New(db)
	d.Amount = 4200
	d.Currency = currency.USD
	d.Reason = "fraudulent"
	d.MustCreate()

	call := face(t, ctx, "apidispute")
	res := call(http.MethodGet, "/v1/billing/risk/disputes/"+d.Id()+"/evidence", "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("evidence: status=%d", res.StatusCode)
	}
	body := decode(t, res)
	e := body["evidence"].(map[string]any)
	if e["gaps"] == nil || len(e["gaps"].([]any)) == 0 {
		t.Fatalf("a defence with no charge and no judgement must state its gaps: %v", e)
	}

	sub := call(http.MethodPost, "/v1/billing/risk/disputes/"+d.Id()+"/submit", `{}`)
	if sub.StatusCode != http.StatusNotImplemented {
		t.Fatalf("submit: status=%d want 501 — there is no adjudicator and the API must say so", sub.StatusCode)
	}
}

// TestRiskAPI_TenantIsolation — org B, on the same process and the same store,
// reads none of org A's records and is restrained by none of org A's controls.
func TestRiskAPI_TenantIsolation(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	a := face(t, ctx, "isoa")
	b := face(t, ctx, "isob")

	scr := decode(t, a(http.MethodPost, "/v1/billing/risk/screen",
		`{"stage":"payment","subjectKind":"customer","subject":"shared","amount":100}`))
	ctl := decode(t, a(http.MethodPost, "/v1/billing/risk/controls",
		`{"effect":"block","subjectKind":"customer","subject":"shared"}`))

	if res := b(http.MethodGet, "/v1/billing/risk/screens/"+scr["id"].(string), ""); res.StatusCode != http.StatusNotFound {
		t.Fatalf("org B read org A's screen: status=%d", res.StatusCode)
	}
	if page := decode(t, b(http.MethodGet, "/v1/billing/risk/screens", "")); len(page["screens"].([]any)) != 0 {
		t.Fatalf("org B listed org A's screens: %v", page)
	}
	if list := decode(t, b(http.MethodGet, "/v1/billing/risk/controls", "")); len(list["controls"].([]any)) != 0 {
		t.Fatalf("org B listed org A's controls: %v", list)
	}
	if res := b(http.MethodDelete, "/v1/billing/risk/controls/"+ctl["id"].(string), ""); res.StatusCode != http.StatusNotFound {
		t.Fatalf("org B released org A's control: status=%d", res.StatusCode)
	}
	moved := decode(t, b(http.MethodPost, "/v1/billing/risk/screen",
		`{"stage":"payment","subjectKind":"customer","subject":"shared","amount":100}`))
	if moved["moves"] != true {
		t.Fatalf("org A's block restrained org B's money: %v", moved)
	}
	// And org A's own control still stands.
	still := decode(t, a(http.MethodPost, "/v1/billing/risk/screen",
		`{"stage":"payment","subjectKind":"customer","subject":"shared","amount":100}`))
	if still["moves"] != false {
		t.Fatalf("org A's own control stopped restraining: %v", still)
	}
}

// TestRiskAPI_AnUnauthenticatedCallerIsRefused — the group carries its own
// gate rather than inheriting one by path prefix, so the refusal does not
// depend on the order two registrations happened to run in.
func TestRiskAPI_AnUnauthenticatedCallerIsRefused(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	app := zip.New(zip.Config{DisableStartupMessage: true, AppName: "risk-api-unauth"})
	app.Use(func(c *zip.Ctx) error { c.SetContext(ctx); return c.Continue() })
	RiskRoute(app)

	for _, r := range [][2]string{
		{http.MethodPost, "/v1/billing/risk/screen"},
		{http.MethodGet, "/v1/billing/risk/screens"},
		{http.MethodGet, "/v1/billing/risk/controls"},
		{http.MethodPost, "/v1/billing/risk/controls"},
		{http.MethodGet, "/v1/billing/risk/merchants/m1"},
		{http.MethodPost, "/v1/billing/risk/outcomes"},
		{http.MethodGet, "/v1/billing/risk/disputes/d1/evidence"},
	} {
		req := httptest.NewRequest(r[0], r[1], bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		res, err := app.Fiber().Test(req)
		if err != nil {
			t.Fatalf("%s %s: %v", r[0], r[1], err)
		}
		if res.StatusCode != http.StatusUnauthorized && res.StatusCode != http.StatusForbidden {
			t.Fatalf("%s %s answered %d to an unauthenticated caller", r[0], r[1], res.StatusCode)
		}
	}
}
