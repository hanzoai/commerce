package deploy

import (
	"strings"
	"testing"
)

func mustRender(t *testing.T, product string, tn Tenant) string {
	t.Helper()
	p, err := Lookup(product)
	if err != nil {
		t.Fatalf("Lookup(%q): %v", product, err)
	}
	out, err := Render(p, tn)
	if err != nil {
		t.Fatalf("Render(%q): %v", product, err)
	}
	return out
}

func TestRender_Vector_MeteredSidecar(t *testing.T) {
	out := mustRender(t, "vector", Tenant{
		Org:             "acme",
		Tag:             "1.16.3",
		MeterProxyImage: "ghcr.io/hanzoai/meter-proxy",
		MeterProxyTag:   "0.1.0",
		IngressHost:     "acme.vector.hanzo.ai",
	})

	must := []string{
		"apiVersion: hanzo.ai/v1",
		"kind: Service",
		"name: vector-acme",                       // per-tenant CR name
		"hanzo.ai/tenant: acme",                   // tenant label
		"repository: ghcr.io/hanzoai/meter-proxy", // proxy is the main container
		"servicePort: 16333",                      // public port = product 6333 + 10000
		`METER_PROXY_UPSTREAM`,
		`value: "http://127.0.0.1:6333"`, // forwards to product on localhost
		`METER_PROXY_PROVIDER`,
		`value: "vector"`,
		`METER_PROXY_PRICES`,
		"POST|/collections|2", // the vector price table
		`COMMERCE_SERVICE_ORG`,
		`value: "acme"`, // debits land in the tenant's org
		"sidecars:",
		"name: vector", // product runs as sidecar (reached on localhost; no port map)
		"image: ghcr.io/hanzoai/vector:1.16.3",
		"mountPath: /qdrant/storage",      // per-tenant storage
		"claimName: vector-acme-data",     // per-tenant PVC
		"- acme.vector.hanzo.ai",          // ingress host
	}
	for _, s := range must {
		if !strings.Contains(out, s) {
			t.Errorf("rendered CR missing %q\n---\n%s", s, out)
		}
	}

	// The service token must come from a secretKeyRef, NEVER inlined.
	if !strings.Contains(out, "secretKeyRef:") || !strings.Contains(out, "key: commerceToken") {
		t.Errorf("token must be wired via secretKeyRef commerceToken\n%s", out)
	}
	if strings.Contains(out, "COMMERCE_SERVICE_TOKEN\n      value:") {
		t.Errorf("token must NEVER be inlined as a literal value\n%s", out)
	}
}

func TestRender_Search_PriceTableAndStorage(t *testing.T) {
	out := mustRender(t, "search", Tenant{
		Org:             "zoo",
		Tag:             "1.37.0",
		MeterProxyImage: "ghcr.io/hanzoai/meter-proxy",
		MeterProxyTag:   "0.1.0",
		Test:            true,
	})
	for _, s := range []string{
		"name: search-zoo",
		`value: "search"`,
		"POST|/indexes/|3",       // index a document costs 3c
		"mountPath: /meili_data",
		"claimName: search-zoo-data",
		"METERING_TEST",          // test ledger requested
		`COMMERCE_SERVICE_ORG`,
		`value: "zoo"`,
	} {
		if !strings.Contains(out, s) {
			t.Errorf("search CR missing %q\n---\n%s", s, out)
		}
	}
	// No ingress requested -> ingress disabled.
	if !strings.Contains(out, "enabled: false") {
		t.Errorf("search CR should disable ingress when no host set\n%s", out)
	}
}

func TestRender_Base_NonMetered_NoSidecar_NoProxyRequired(t *testing.T) {
	// base bills in-process, so Render must NOT require a proxy image and must
	// NOT emit a sidecar — but MUST still inject the commerce env so the
	// in-process meter can bill the right org.
	out := mustRender(t, "base", Tenant{Org: "hanzo", Tag: "1.0.0"})

	if strings.Contains(out, "sidecars:") {
		t.Errorf("base must not get a proxy sidecar (bills in-process)\n%s", out)
	}
	if strings.Contains(out, "meter-proxy") {
		t.Errorf("base must not reference the proxy image\n%s", out)
	}
	for _, s := range []string{
		"name: base-hanzo",
		"repository: ghcr.io/hanzoai/base", // base is the main container
		"servicePort: 8090",                // product port direct (no +10000)
		`COMMERCE_SERVICE_ORG`,
		`value: "hanzo"`, // in-process meter still scoped per-org
		"mountPath: /data",
		"claimName: base-hanzo-data",
	} {
		if !strings.Contains(out, s) {
			t.Errorf("base CR missing %q\n---\n%s", s, out)
		}
	}
}

func TestRender_RejectsFloatingTag(t *testing.T) {
	p, _ := Lookup("vector")
	if _, err := Render(p, Tenant{Org: "acme", MeterProxyImage: "x", MeterProxyTag: "1"}); err == nil {
		t.Fatal("Render must reject a missing/floating tag")
	}
}

func TestRender_RejectsMeterableWithoutProxy(t *testing.T) {
	p, _ := Lookup("vector")
	if _, err := Render(p, Tenant{Org: "acme", Tag: "1.16.3"}); err == nil {
		t.Fatal("Render must reject a meterable product with no proxy image/tag")
	}
}

func TestRender_RejectsEmptyOrg(t *testing.T) {
	p, _ := Lookup("base")
	if _, err := Render(p, Tenant{Tag: "1.0.0"}); err == nil {
		t.Fatal("Render must reject an empty tenant org (debits would go to the wrong namespace)")
	}
}

func TestCatalog_EveryProductHasBillableUnit(t *testing.T) {
	for name, p := range Catalog {
		if p.Unit == "" {
			t.Errorf("catalog %q has no billable Unit", name)
		}
		if p.Image == "" || p.Port == 0 {
			t.Errorf("catalog %q missing Image/Port", name)
		}
		if p.Meterable && strings.TrimSpace(p.Prices) == "" {
			t.Errorf("meterable product %q must declare a price table", name)
		}
	}
}
