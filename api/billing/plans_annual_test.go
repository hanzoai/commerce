package billing

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"

	"github.com/hanzoai/commerce/util/test/ae"
)

// sameCents compares two optional annual prices by value; both-absent is equal.
func sameCents(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func centsText(p *int64) string {
	if p == nil {
		return "null"
	}
	return fmt.Sprint(*p)
}

// TestAnnualPriceIsTheCatalogs: GET /v1/billing/plans serves each plan's
// priceAnnual as the catalog states it — cents where it names a price, null
// where it names none. Advisory and dedicated are sold by the month only and
// enterprise is a sales call; all three were served as a $0 annual price, which
// on a plan that charges advertises a free year of it. Asked of the embed and of
// the seeded authority read back the way the endpoint reads it, as JSON.
func TestAnnualPriceIsTheCatalogs(t *testing.T) {
	want := map[string]string{}
	for _, f := range []struct {
		fs   interface{ ReadFile(string) ([]byte, error) }
		path string
	}{{subscriptionJSON, "plans/subscription.json"}, {dnsJSON, "plans/dns.json"}} {
		raw, err := f.fs.ReadFile(f.path)
		if err != nil {
			t.Fatalf("read %s: %v", f.path, err)
		}
		var rows []canonicalPlan
		if err := json.Unmarshal(raw, &rows); err != nil {
			t.Fatalf("decode %s: %v", f.path, err)
		}
		for _, r := range rows {
			want[r.ID] = "null"
			if r.PriceAnnual != nil {
				want[r.ID] = fmt.Sprint(int64(math.Round(*r.PriceAnnual * 100)))
			}
		}
	}
	for _, slug := range []string{"advisory", "dedicated", "enterprise"} {
		if want[slug] != "null" {
			t.Fatalf("the catalog states an annual price for %s (%s); this test asserts nothing about null", slug, want[slug])
		}
	}

	served := func(name string, plans []PlanView) {
		t.Helper()
		body, err := json.Marshal(plans)
		if err != nil {
			t.Fatalf("%s: marshal: %v", name, err)
		}
		var rows []map[string]json.RawMessage
		if err := json.Unmarshal(body, &rows); err != nil {
			t.Fatalf("%s: unmarshal: %v", name, err)
		}
		if len(rows) != len(want) {
			t.Fatalf("%s serves %d plans, the catalog lists %d", name, len(rows), len(want))
		}
		for _, row := range rows {
			var slug string
			_ = json.Unmarshal(row["slug"], &slug)
			if got := string(row["priceAnnual"]); got != want[slug] {
				t.Errorf("%s: %s priceAnnual = %s, the catalog states %s", name, slug, got, want[slug])
			}
		}
	}

	served("embed", catalog)

	c := ae.NewContext()
	defer c.Close()
	if _, _, err := SeedPlans(c); err != nil {
		t.Fatalf("seed: %v", err)
	}
	plans, err := ReadPlans(c, "", "", nil)
	if err != nil {
		t.Fatalf("read plans: %v", err)
	}
	if _, ok := planAuthorityRows(c); !ok {
		t.Fatal("the authority is empty after the seed, so the embed answered and the row path is untested")
	}
	served("authority", plans)
}
