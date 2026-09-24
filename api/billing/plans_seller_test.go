package billing

import (
	"testing"

	"github.com/hanzoai/commerce/util/test/ae"
)

// TestReadPlansSellsOnlyOnTheSellersHosts: GET /v1/billing/plans answers the
// catalog of the brand the host resolves to. Every brand's pay host reads it,
// and each listed Hanzo's ladder under its own name — pay.lux.cloud and
// pay.zoo.cloud offered Pro and Max as if Lux and Zoo sold them.
func TestReadPlansSellsOnlyOnTheSellersHosts(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	if _, _, err := SeedPlans(c); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Hanzo's hosts, and a caller that names no host (the deployment default).
	for _, host := range []string{"pay.hanzo.ai", "api.hanzo.ai", "PAY.HANZO.AI:443", ""} {
		rows, err := ReadPlans(c, host, "", nil)
		if err != nil {
			t.Fatalf("%q: %v", host, err)
		}
		if len(rows) != len(catalog) {
			t.Errorf("%q lists %d plans, the catalog has %d", host, len(rows), len(catalog))
		}
	}

	// Every other brand's pay host sells none — an empty list, never Hanzo's, and
	// never nil, which a JSON reader would take for a failure.
	for _, host := range []string{"pay.lux.cloud", "pay.lux.tel", "pay.zoo.cloud", "Pay.Zoo.Cloud:443", "pay.pars.network"} {
		rows, err := ReadPlans(c, host, "", nil)
		if err != nil {
			t.Fatalf("%q: %v", host, err)
		}
		if rows == nil || len(rows) != 0 {
			t.Errorf("%q lists %v, want an empty list", host, rows)
		}
		if dns, _ := ReadPlans(c, host, "dns", nil); len(dns) != 0 {
			t.Errorf("%q lists %d dns plans, want none", host, len(dns))
		}
	}
}

// TestSellsFollowsTheOrgEndpointsTable: the catalog's seller is decided by the
// same host→brand table the org endpoint wears, so a spoof suffix is not Hanzo.
func TestSellsFollowsTheOrgEndpointsTable(t *testing.T) {
	for host, want := range map[string]bool{
		"pay.hanzo.ai":           true,
		"hanzo.ai":               true,
		"pay.lux.cloud":          false,
		"lux.id":                 false,
		"zoo.ngo":                false,
		"pay.hanzo.ai.lux.tel":   false,
		"pay.lux.cloud.hanzo.ai": true,
	} {
		if got := Sells(host); got != want {
			t.Errorf("Sells(%q) = %v, want %v", host, got, want)
		}
	}
}
