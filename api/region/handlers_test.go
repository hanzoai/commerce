package region

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
	"github.com/hanzoai/commerce/models/organization"
	regionModel "github.com/hanzoai/commerce/models/region"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// regionAPI wires the real country add/remove/list handlers behind a seed
// middleware binding org `ns`, so they run their real org.Namespaced datastore path.
type regionAPI struct {
	app *zip.App
	db  *datastore.Datastore
}

func newRegionAPI(ns string) *regionAPI {
	base := nscontext.WithNamespace(context.Background(), ns)
	db := datastore.New(base)
	org := organization.New(db)
	org.Name = ns

	app := zip.New(zip.Config{DisableStartupMessage: true})
	seed := func(c *zip.Ctx) error {
		c.SetContext(base)
		c.Locals("organization", org)
		return c.Next()
	}
	app.Get("/:regionid/countries", seed, ListCountries)
	app.Post("/:regionid/countries", seed, AddCountry)
	app.Delete("/:regionid/countries/:countryCode", seed, RemoveCountry)

	return &regionAPI{app: app, db: db}
}

func (a *regionAPI) do(t *testing.T, method, path string, body any) (int, []byte) {
	t.Helper()
	var r io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		r = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, r)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.app.Fiber().Test(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

func (a *regionAPI) seedRegion(t *testing.T) *regionModel.Region {
	t.Helper()
	r := regionModel.New(a.db)
	r.Name = "North America"
	r.CurrencyCode = "usd"
	if err := r.Create(); err != nil {
		t.Fatalf("create region: %v", err)
	}
	return r
}

func (a *regionAPI) reload(t *testing.T, id string) *regionModel.Region {
	t.Helper()
	r := regionModel.New(a.db)
	if err := r.GetById(id); err != nil {
		t.Fatalf("reload region: %v", err)
	}
	return r
}

// TestCountry_AddListRemove drives the region membership lifecycle: add a
// country, list it, remove it.
func TestCountry_AddListRemove(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	api := newRegionAPI("acme")
	r := api.seedRegion(t)

	// ADD
	if code, body := api.do(t, http.MethodPost, "/"+r.Id()+"/countries", map[string]any{
		"iso2": "US", "name": "United States",
	}); code != http.StatusOK {
		t.Fatalf("add country = %d, body=%s", code, body)
	}
	got := api.reload(t, r.Id())
	if len(got.Countries) != 1 || got.Countries[0].ISO2 != "US" {
		t.Fatalf("countries = %+v, want [US]", got.Countries)
	}
	if got.Countries[0].RegionId != r.Id() {
		t.Fatalf("country RegionId = %q, want %q", got.Countries[0].RegionId, r.Id())
	}

	// LIST
	code, body := api.do(t, http.MethodGet, "/"+r.Id()+"/countries", nil)
	if code != http.StatusOK {
		t.Fatalf("list = %d, body=%s", code, body)
	}
	var list []regionModel.Country
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("decode list: %v (%s)", err, body)
	}
	if len(list) != 1 || list[0].ISO2 != "US" {
		t.Fatalf("list = %+v, want [US]", list)
	}

	// REMOVE
	if code, body := api.do(t, http.MethodDelete, "/"+r.Id()+"/countries/US", nil); code != http.StatusOK {
		t.Fatalf("remove = %d, body=%s", code, body)
	}
	if got := api.reload(t, r.Id()); len(got.Countries) != 0 {
		t.Fatalf("countries after remove = %+v, want empty", got.Countries)
	}
}

// TestAddCountry_Duplicate: adding an already-present country is a 409.
func TestAddCountry_Duplicate(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	api := newRegionAPI("acme")
	r := api.seedRegion(t)

	if code, _ := api.do(t, http.MethodPost, "/"+r.Id()+"/countries", map[string]any{"iso2": "US"}); code != http.StatusOK {
		t.Fatalf("first add failed")
	}
	if code, _ := api.do(t, http.MethodPost, "/"+r.Id()+"/countries", map[string]any{"iso2": "US"}); code != http.StatusConflict {
		t.Fatalf("duplicate add = %d, want 409", code)
	}
	if got := api.reload(t, r.Id()); len(got.Countries) != 1 {
		t.Fatalf("countries = %d, want 1 (no duplicate persisted)", len(got.Countries))
	}
}

// TestAddCountry_MissingISO2: a country without an iso2 code is a 400.
func TestAddCountry_MissingISO2(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	api := newRegionAPI("acme")
	r := api.seedRegion(t)

	if code, _ := api.do(t, http.MethodPost, "/"+r.Id()+"/countries", map[string]any{"name": "Nowhere"}); code != http.StatusBadRequest {
		t.Fatalf("missing iso2 = %d, want 400", code)
	}
}

// TestRemoveCountry_NotPresent: removing a country not in the region is a 404.
func TestRemoveCountry_NotPresent(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	api := newRegionAPI("acme")
	r := api.seedRegion(t)

	if code, _ := api.do(t, http.MethodDelete, "/"+r.Id()+"/countries/CA", nil); code != http.StatusNotFound {
		t.Fatalf("remove absent country = %d, want 404", code)
	}
}

// TestRegion_UnknownIsNotFound: operations on a nonexistent region are 404.
func TestRegion_UnknownIsNotFound(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	api := newRegionAPI("acme")
	if code, _ := api.do(t, http.MethodGet, "/nope/countries", nil); code != http.StatusNotFound {
		t.Fatalf("list unknown region = %d, want 404", code)
	}
	if code, _ := api.do(t, http.MethodPost, "/nope/countries", map[string]any{"iso2": "US"}); code != http.StatusNotFound {
		t.Fatalf("add to unknown region = %d, want 404", code)
	}
}

// TestRegion_CrossOrgIsolation: a region created in org "acme" is invisible to
// org "other" — every tenant's regions are namespace-scoped.
func TestRegion_CrossOrgIsolation(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	acme := newRegionAPI("acme")
	r := acme.seedRegion(t)

	other := newRegionAPI("other")
	if code, _ := other.do(t, http.MethodGet, "/"+r.Id()+"/countries", nil); code != http.StatusNotFound {
		t.Fatalf("cross-org region read = %d, want 404", code)
	}
	if code, _ := other.do(t, http.MethodPost, "/"+r.Id()+"/countries", map[string]any{"iso2": "US"}); code != http.StatusNotFound {
		t.Fatalf("cross-org country add = %d, want 404", code)
	}
}
