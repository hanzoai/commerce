package tax

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/taxrate"
	"github.com/hanzoai/commerce/models/taxregion"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// calcOver drives the real Calculate handler with body over a request whose
// context resolves to org `ns` (Calculate reads datastore.New(c.Context())).
func calcOver(t *testing.T, ns string, body calcRequest) calcResponse {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Post("/tax/calculate", func(c *zip.Ctx) error {
		c.SetContext(nscontext.WithNamespace(context.Background(), ns))
		return c.Next()
	}, Calculate)

	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/tax/calculate", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("calculate status = %d, body=%s", resp.StatusCode, b)
	}
	var out calcResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out
}

func seedRegion(t *testing.T, db *datastore.Datastore, country, province string) *taxregion.TaxRegion {
	t.Helper()
	r := taxregion.New(db)
	r.CountryCode = country
	r.ProvinceCode = province
	if err := r.Create(); err != nil {
		t.Fatalf("create region: %v", err)
	}
	return r
}

func seedRate(t *testing.T, db *datastore.Datastore, regionID string, rate float64, isDefault, combinable bool) {
	t.Helper()
	tr := taxrate.New(db)
	tr.TaxRegionId = regionID
	tr.Rate = rate
	tr.IsDefault = isDefault
	tr.IsCombinable = combinable
	if err := tr.Create(); err != nil {
		t.Fatalf("create rate: %v", err)
	}
}

func approx(a, b float64) bool { return math.Abs(a-b) < 0.005 }

// TestCalculate_ProvinceLevelRate proves the region cascade resolves a
// province-level region and applies its default rate per item.
func TestCalculate_ProvinceLevelRate(t *testing.T) {
	const ns = "acme"
	tc := ae.NewContext()
	defer tc.Close()
	db := datastore.New(nscontext.WithNamespace(context.Background(), ns))

	r := seedRegion(t, db, "CA", "ON")
	seedRate(t, db, r.Id(), 0.13, true, false) // 13% HST, default

	out := calcOver(t, ns, calcRequest{
		ShippingAddress: calcAddress{CountryCode: "CA", ProvinceCode: "ON"},
		Items:           []calcItem{{Amount: 100, Quantity: 2}},
	})
	if !approx(out.TotalTax, 26.0) { // 100 * 2 * 0.13
		t.Fatalf("total tax = %v, want 26.00", out.TotalTax)
	}
	if len(out.Items) != 1 || !approx(out.Items[0].TaxRate, 0.13) {
		t.Fatalf("item rate = %+v, want 0.13", out.Items)
	}
}

// TestCalculate_CountryFallback proves that when no province-level region exists,
// the handler falls back to the country-level region (empty province).
func TestCalculate_CountryFallback(t *testing.T) {
	const ns = "acme"
	tc := ae.NewContext()
	defer tc.Close()
	db := datastore.New(nscontext.WithNamespace(context.Background(), ns))

	r := seedRegion(t, db, "US", "") // country-level only
	seedRate(t, db, r.Id(), 0.05, true, false)

	out := calcOver(t, ns, calcRequest{
		ShippingAddress: calcAddress{CountryCode: "US", ProvinceCode: "TX"}, // no TX region
		Items:           []calcItem{{Amount: 200, Quantity: 1}},
	})
	if !approx(out.TotalTax, 10.0) { // 200 * 0.05
		t.Fatalf("total tax = %v, want 10.00 (country fallback)", out.TotalTax)
	}
}

// TestCalculate_NoRegionZeroTax proves an address with no matching region yields
// zero tax (not an error).
func TestCalculate_NoRegionZeroTax(t *testing.T) {
	const ns = "acme"
	tc := ae.NewContext()
	defer tc.Close()

	out := calcOver(t, ns, calcRequest{
		ShippingAddress: calcAddress{CountryCode: "JP", ProvinceCode: ""},
		Items:           []calcItem{{Amount: 100, Quantity: 3}},
	})
	if out.TotalTax != 0 {
		t.Fatalf("total tax = %v, want 0 (no region)", out.TotalTax)
	}
	if len(out.Items) != 1 || out.Items[0].Tax != 0 {
		t.Fatalf("items = %+v, want one zero-tax line", out.Items)
	}
}

// TestCalculate_CombinableRatesSummed proves the rule cascade sums every
// combinable rate for the region (e.g. GST + PST stack).
func TestCalculate_CombinableRatesSummed(t *testing.T) {
	const ns = "acme"
	tc := ae.NewContext()
	defer tc.Close()
	db := datastore.New(nscontext.WithNamespace(context.Background(), ns))

	r := seedRegion(t, db, "CA", "BC")
	seedRate(t, db, r.Id(), 0.05, false, true) // GST 5% combinable
	seedRate(t, db, r.Id(), 0.07, false, true) // PST 7% combinable

	out := calcOver(t, ns, calcRequest{
		ShippingAddress: calcAddress{CountryCode: "CA", ProvinceCode: "BC"},
		Items:           []calcItem{{Amount: 100, Quantity: 1}},
	})
	if !approx(out.Items[0].TaxRate, 0.12) {
		t.Fatalf("effective rate = %v, want 0.12 (0.05 + 0.07)", out.Items[0].TaxRate)
	}
	if !approx(out.TotalTax, 12.0) {
		t.Fatalf("total tax = %v, want 12.00", out.TotalTax)
	}
}

// TestCalculate_DefaultRateWhenNoCombinable proves that with no combinable rates
// the default-flagged rate is used (not summed with the non-default one).
func TestCalculate_DefaultRateWhenNoCombinable(t *testing.T) {
	const ns = "acme"
	tc := ae.NewContext()
	defer tc.Close()
	db := datastore.New(nscontext.WithNamespace(context.Background(), ns))

	r := seedRegion(t, db, "GB", "")
	seedRate(t, db, r.Id(), 0.20, true, false)  // standard VAT, default
	seedRate(t, db, r.Id(), 0.05, false, false) // reduced VAT, not default

	out := calcOver(t, ns, calcRequest{
		ShippingAddress: calcAddress{CountryCode: "GB", ProvinceCode: ""},
		Items:           []calcItem{{Amount: 100, Quantity: 1}},
	})
	if !approx(out.Items[0].TaxRate, 0.20) {
		t.Fatalf("effective rate = %v, want 0.20 (default)", out.Items[0].TaxRate)
	}
}

// TestCalculate_FirstRateWhenNoDefault proves the final fallback: no combinable
// and no default rate → the first rate found is applied.
func TestCalculate_FirstRateWhenNoDefault(t *testing.T) {
	const ns = "acme"
	tc := ae.NewContext()
	defer tc.Close()
	db := datastore.New(nscontext.WithNamespace(context.Background(), ns))

	r := seedRegion(t, db, "AU", "")
	seedRate(t, db, r.Id(), 0.10, false, false) // GST, neither default nor combinable

	out := calcOver(t, ns, calcRequest{
		ShippingAddress: calcAddress{CountryCode: "AU", ProvinceCode: ""},
		Items:           []calcItem{{Amount: 50, Quantity: 2}},
	})
	if !approx(out.Items[0].TaxRate, 0.10) {
		t.Fatalf("effective rate = %v, want 0.10 (first-rate fallback)", out.Items[0].TaxRate)
	}
	if !approx(out.TotalTax, 10.0) { // 50 * 2 * 0.10
		t.Fatalf("total tax = %v, want 10.00", out.TotalTax)
	}
}
