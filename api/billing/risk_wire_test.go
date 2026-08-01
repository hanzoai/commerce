// Copyright © 2026 Hanzo AI. MIT License.

package billing

// risk_wire_test.go is the gate behind the sentence "every route on the risk
// face is a typed zip op". An untyped route is invisible to the OpenAPI
// document, the MCP tool list, the CLI and every generated SDK — so a rule
// stated in prose and checked by nobody is a rule that lasts one commit.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/zap-proto/zip"
)

// untypedByDesign is a CLOSED list. A route on the risk face that is neither a
// typed op nor named here fails this test. It is empty, and it stays empty
// unless a route genuinely cannot be typed — the two reasons that exist in this
// fleet are a health probe (which must answer a non-2xx carrying its report as
// the body, and a typed error renders as zip's own envelope instead) and an op
// that must render the pre-work billing denial in band. The risk face has
// neither: it has no probe of its own, and it meters nothing before it works.
var untypedByDesign = map[string]string{}

// riskApp builds the face on a throwaway app — the SAME registration production
// runs, so this test cannot pass against a copy of the wiring.
func riskApp(t *testing.T) *zip.App {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true, AppName: "risk-wire-test"})
	RiskRoute(app)
	return app
}

const facePrefix = "/v1/billing/risk"

func TestRisk_EveryRouteIsATypedOp(t *testing.T) {
	app := riskApp(t)

	typed := map[string]bool{}
	for path, item := range paths(t, app) {
		for method := range item {
			typed[strings.ToUpper(method)+" "+routerSpelling(path)] = true
		}
	}

	var untyped []string
	for _, r := range app.Declaration().Routes {
		if !strings.HasPrefix(r.Pattern, facePrefix) {
			continue
		}
		key := r.Method + " " + r.Pattern
		if typed[key] || untypedByDesign[key] != "" {
			continue
		}
		untyped = append(untyped, key)
	}
	if len(untyped) > 0 {
		sort.Strings(untyped)
		t.Fatalf("routes on the risk face that are neither typed nor named in untypedByDesign:\n  %s",
			strings.Join(untyped, "\n  "))
	}
}

// TestRisk_TheFaceIsExactlyTheseOps freezes the surface. Adding an op is a
// decision recorded here; losing one is caught before an SDK is regenerated
// without it.
func TestRisk_TheFaceIsExactlyTheseOps(t *testing.T) {
	want := []string{
		"riskControlPlace",
		"riskControlRelease",
		"riskControls",
		"riskEvidence",
		"riskMerchant",
		"riskMerchantReview",
		"riskOutcome",
		"riskScreen",
		"riskScreenView",
		"riskScreens",
		"riskSubmit",
	}
	got := riskApp(t).Declaration().Ops
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ops:\n got %v\nwant %v", got, want)
	}
}

// TestRisk_NoOpTakesTheTenantAsAnArgument is the isolation gate at the
// contract. An input field is caller-supplied, so a tenant read from one is a
// cross-tenant read the caller asserted for itself; the org comes from the
// validated principal and from nowhere else.
func TestRisk_NoOpTakesTheTenantAsAnArgument(t *testing.T) {
	forbidden := map[string]bool{
		"org": true, "orgid": true, "organization": true, "organizationid": true,
		"tenant": true, "tenantid": true, "namespace": true, "owner": true, "brand": true,
	}
	schemas, ok := spec(t, riskApp(t))["components"].(map[string]any)
	if !ok {
		t.Fatal("the document has no components")
	}
	defs, ok := schemas["schemas"].(map[string]any)
	if !ok {
		t.Fatal("the document has no schemas")
	}
	for name, raw := range defs {
		s, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		props, ok := s["properties"].(map[string]any)
		if !ok {
			continue
		}
		for field := range props {
			if forbidden[strings.ToLower(field)] {
				t.Fatalf("schema %s takes %q as an input field — the tenant must come from the principal", name, field)
			}
		}
	}
}

// TestRisk_SchemaNamesAreFaceScoped — the fleet's schema namespace is FLAT, and
// the weave refuses one name meaning two shapes across apps. Every type this
// face introduces is prefixed, so it cannot collide with another app's.
func TestRisk_SchemaNamesAreFaceScoped(t *testing.T) {
	defs := spec(t, riskApp(t))["components"].(map[string]any)["schemas"].(map[string]any)
	if len(defs) == 0 {
		t.Fatal("the face declares no schemas")
	}
	for name := range defs {
		if !strings.HasPrefix(name, "risk") && !strings.HasPrefix(name, "Evidence") &&
			!strings.HasPrefix(name, "evidence") && !strings.HasPrefix(name, "Dispute") {
			t.Fatalf("schema %q is not scoped to this face; prefix it so the flat fleet namespace cannot collide", name)
		}
	}
}

// TestRisk_DocumentRegeneratesUnchanged is the regen gate: the committed
// document is DERIVED from the source, so it cannot drift from it. Run with
// RISK_OPENAPI_UPDATE=1 to rewrite it after an intended change.
func TestRisk_DocumentRegeneratesUnchanged(t *testing.T) {
	golden := filepath.Join("risk.openapi.json")

	got, err := json.MarshalIndent(spec(t, riskApp(t)), "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got = append(got, '\n')

	if os.Getenv("RISK_OPENAPI_UPDATE") == "1" {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		t.Log("regenerated " + golden)
		return
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read %s: %v (run with RISK_OPENAPI_UPDATE=1 to generate it)", golden, err)
	}
	if string(got) != string(want) {
		t.Fatalf("%s no longer regenerates from source — a route or a type changed and the document did not.\n"+
			"Run: RISK_OPENAPI_UPDATE=1 go test ./api/billing -run TestRisk_DocumentRegeneratesUnchanged", golden)
	}
}

// TestRisk_EveryOpCarriesItsProse — zipdoc lifts the doc comments into
// zipdoc_gen.go, which is committed and compiled in. If that file is missing or
// stale the document ships with no descriptions and nothing else notices.
func TestRisk_EveryOpCarriesItsProse(t *testing.T) {
	for path, item := range paths(t, riskApp(t)) {
		if !strings.HasPrefix(path, facePrefix) {
			continue
		}
		for method, raw := range item {
			op, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if s, _ := op["summary"].(string); s == "" {
				t.Fatalf("%s %s has no summary", strings.ToUpper(method), path)
			}
			if d, _ := op["description"].(string); d == "" {
				t.Fatalf("%s %s has no description — regenerate zipdoc_gen.go", strings.ToUpper(method), path)
			}
		}
	}
}

// spec is the app's document, round-tripped through JSON so every nested value
// is the shape a client actually receives — the builder uses concrete map types
// internally and a test that asserted those would be testing the builder, not
// the document.
func spec(t *testing.T, app *zip.App) map[string]any {
	t.Helper()
	raw, err := json.Marshal(app.OpenAPISpec())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("the app produced no document")
	}
	return out
}

func paths(t *testing.T, app *zip.App) map[string]map[string]any {
	t.Helper()
	raw, ok := spec(t, app)["paths"].(map[string]any)
	if !ok || len(raw) == 0 {
		t.Fatal("the document has no paths")
	}
	out := map[string]map[string]any{}
	for path, item := range raw {
		if m, ok := item.(map[string]any); ok {
			out[path] = m
		}
	}
	return out
}

// routerSpelling converts the document's "{id}" back to the router's ":id" —
// the two spellings of one path, so a comparison of route to op compares like
// with like.
func routerSpelling(path string) string {
	out := path
	for {
		open := strings.IndexByte(out, '{')
		if open < 0 {
			return out
		}
		close := strings.IndexByte(out[open:], '}')
		if close < 0 {
			return out
		}
		out = out[:open] + ":" + out[open+1:open+close] + out[open+close+1:]
	}
}
