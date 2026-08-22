// Copyright © 2026 Hanzo AI. MIT License.

package rate

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/models/rate"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A RATE IS CROSS-TENANT MONEY: one row decides what every customer pays. So the
// question these tests keep asking is not "does the CRUD work" but "who can
// reach it", and the answer has to be the same at every address under /rates —
// including one added tomorrow by someone who did not read this file.

var admin = &auth.IAMClaims{Owner: "admin"}
var orgAdmin = &auth.IAMClaims{Owner: "acme", IsAdmin: true}

// mount drives the REAL route table — AdminRoute, not a handler in isolation —
// so what these tests exercise is the wiring a request actually meets. A test
// that calls the handler directly proves the handler and says nothing about the
// gate in front of it, which is the half that matters here.
func mount(t *testing.T, claims *auth.IAMClaims, method, target string, body any) (int, []byte) {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	seed := func(c *zip.Ctx) error {
		c.SetContext(context.Background())
		if claims != nil {
			c.Locals("iam_claims", claims)
		}
		return c.Next()
	}
	AdminRoute(app.Group("/v1"), seed)

	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, "/v1"+target, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, out
}

// every address the group serves, with a body where one is required.
func addresses() []struct {
	method, target string
	body           any
} {
	return []struct {
		method, target string
		body           any
	}{
		{http.MethodGet, "/rates/entries", nil},
		{http.MethodPost, "/rates/entries", map[string]any{"product": "x", "meter": "y"}},
		{http.MethodPut, "/rates/entries/x/y", map[string]any{"rate": 1}},
		{http.MethodDelete, "/rates/entries/x/y", nil},
		{http.MethodPost, "/rates/import", []map[string]any{{"product": "x", "meter": "y"}}},
	}
}

// THE GATE IS ON THE ADDRESS. Not on the handler, so a sixth route added under
// this group inherits it whether or not its author thought about it — which is
// the whole reason it moved off the five handlers that each carried a copy.
func TestNobodyButThePlatformReachesAnyRateAddress(t *testing.T) {
	for _, a := range addresses() {
		for who, claims := range map[string]*auth.IAMClaims{
			"anonymous":    nil,
			"an org admin": orgAdmin,
			"a member":     {Owner: "acme"},
		} {
			code, body := mount(t, claims, a.method, a.target, a.body)
			if code != http.StatusForbidden {
				t.Errorf("%s %s as %s → %d, want 403: a rate is cross-tenant money and "+
					"an org's own admin must not price another tenant's calls (%s)",
					a.method, a.target, who, code, body)
			}
		}
	}
}

// An org admin is NOT a platform admin. Stated separately because IsAdmin is
// true for them — the flag that admits them to their own org's settings — and
// reading it here is the privilege escalation this gate exists to stop.
func TestAnOrgAdminIsNotAPlatformAdmin(t *testing.T) {
	if orgAdmin.IsSuperAdmin() {
		t.Fatal("an org admin resolved as SuperAdmin; the gate would admit every tenant")
	}
	code, _ := mount(t, orgAdmin, http.MethodGet, "/rates/entries", nil)
	if code != http.StatusForbidden {
		t.Errorf("org admin listing rates → %d, want 403", code)
	}
}

func TestThePlatformReachesTheList(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	code, body := mount(t, admin, http.MethodGet, "/rates/entries", nil)
	if code != http.StatusOK {
		t.Fatalf("platform admin listing → %d, want 200 (%s)", code, body)
	}
	var rows []*rate.Rate
	if err := json.Unmarshal(body, &rows); err != nil {
		t.Fatalf("list is not an array of rates: %s", body)
	}
}

// Identity is BOTH parts. A rate keyed on the metered thing alone would let one
// product's price overwrite another's for the same name.
func TestCreateRequiresBothPartsOfTheIdentity(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	for _, in := range []map[string]any{
		{"meter": "gb-month"},          // no product
		{"product": "storage"},         // no meter
		{"product": " ", "meter": " "}, // blank is not a name
	} {
		code, _ := mount(t, admin, http.MethodPost, "/rates/entries", in)
		if code != http.StatusBadRequest {
			t.Errorf("create %v → %d, want 400", in, code)
		}
	}
}

func TestCreateThenDuplicateIsRefused(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	row := map[string]any{"product": "storage", "meter": "gb-month", "unit": "GB-month", "rate": 80000000, "currency": "USD"}
	code, body := mount(t, admin, http.MethodPost, "/rates/entries", row)
	if code != http.StatusCreated {
		t.Fatalf("create → %d, want 201 (%s)", code, body)
	}

	var got rate.Rate
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("created row is not a rate: %s", body)
	}
	if got.Slug != "storage/gb-month" {
		t.Errorf("slug = %q, want storage/gb-month — Take binds it from the parts", got.Slug)
	}
	// A row a PERSON created is theirs from the start, so no later import may
	// quietly replace it.
	if !got.AdminEdited {
		t.Error("a hand-created rate is not marked AdminEdited, so the next import " +
			"silently reverts a price someone chose")
	}

	code, _ = mount(t, admin, http.MethodPost, "/rates/entries", row)
	if code != http.StatusConflict {
		t.Errorf("duplicate create → %d, want 409: two rows for one slug means the "+
			"authority answers one question twice", code)
	}
}

func TestUpdateMarksTheRowAsAPersonsDecision(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	seeded := []map[string]any{{"product": "storage", "meter": "gb-month", "unit": "GB-month", "rate": 80000000, "currency": "USD"}}
	if code, body := mount(t, admin, http.MethodPost, "/rates/import", seeded); code != http.StatusOK {
		t.Fatalf("import → %d (%s)", code, body)
	}

	code, body := mount(t, admin, http.MethodPut, "/rates/entries/storage/gb-month",
		map[string]any{"product": "storage", "meter": "gb-month", "unit": "GB-month", "rate": 90000000, "currency": "USD"})
	if code != http.StatusOK {
		t.Fatalf("update → %d, want 200 (%s)", code, body)
	}
	var got rate.Rate
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("updated row is not a rate: %s", body)
	}
	if got.Rate != 90000000 {
		t.Errorf("rate = %d, want 90000000", got.Rate)
	}
	if !got.AdminEdited {
		t.Fatal("an edited rate is not marked, so the next boot's seed reverts it — the " +
			"operator's price would apply, work, and silently disappear")
	}

	// And the seed must now leave it alone, which is what the mark is FOR.
	if code, _ := mount(t, admin, http.MethodPost, "/rates/import", seeded); code != http.StatusOK {
		t.Fatalf("re-import → %d", code)
	}
	_, body = mount(t, admin, http.MethodGet, "/rates/entries?product=storage", nil)
	var rows []*rate.Rate
	if err := json.Unmarshal(body, &rows); err != nil || len(rows) == 0 {
		t.Fatalf("list after re-import: %s", body)
	}
	if rows[0].Rate != 90000000 {
		t.Errorf("the import reverted an admin's price to %d — AdminEdited is not being "+
			"honoured, and every operator edit is temporary", rows[0].Rate)
	}
}

func TestUpdateAndDeleteRefuseAnUnknownRate(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	if code, _ := mount(t, admin, http.MethodPut, "/rates/entries/nope/nothing", map[string]any{"rate": 1}); code != http.StatusNotFound {
		t.Errorf("update unknown → %d, want 404", code)
	}
	if code, _ := mount(t, admin, http.MethodDelete, "/rates/entries/nope/nothing", nil); code != http.StatusNotFound {
		t.Errorf("delete unknown → %d, want 404", code)
	}
}

func TestImportIsIdempotent(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	doc := []map[string]any{
		{"product": "storage", "meter": "gb-month", "unit": "GB-month", "rate": 80000000, "currency": "USD"},
		{"product": "risk", "meter": "screen", "unit": "screen", "rate": 100000, "currency": "USD"},
	}
	code, body := mount(t, admin, http.MethodPost, "/rates/import", doc)
	if code != http.StatusOK {
		t.Fatalf("import → %d (%s)", code, body)
	}
	var first struct{ Received, Created, Corrected, Unchanged int }
	if err := json.Unmarshal(body, &first); err != nil {
		t.Fatalf("import result: %s", body)
	}
	if first.Created != 2 || first.Received != 2 {
		t.Fatalf("first import created=%d received=%d, want 2/2", first.Created, first.Received)
	}

	_, body = mount(t, admin, http.MethodPost, "/rates/import", doc)
	var again struct{ Received, Created, Corrected, Unchanged int }
	if err := json.Unmarshal(body, &again); err != nil {
		t.Fatalf("re-import result: %s", body)
	}
	if again.Created != 0 || again.Corrected != 0 || again.Unchanged != 2 {
		t.Errorf("re-importing the same document wrote created=%d corrected=%d — it must "+
			"be a no-op, or every boot rewrites every row and the audit trail fills with "+
			"writes that changed nothing", again.Created, again.Corrected)
	}
}

func TestImportRefusesADocumentThatSaysNothing(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	if code, _ := mount(t, admin, http.MethodPost, "/rates/import", []map[string]any{}); code != http.StatusBadRequest {
		t.Errorf("empty import → %d, want 400: an empty document is a mistake, and "+
			"treating it as success reports a price change that did not happen", code)
	}
	// A single object where an array is required.
	if code, _ := mount(t, admin, http.MethodPost, "/rates/import", map[string]any{"product": "x", "meter": "y"}); code != http.StatusBadRequest {
		t.Errorf("object import → %d, want 400", code)
	}
}
