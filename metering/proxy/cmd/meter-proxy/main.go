// Command meter-proxy is the ONE binary that commercializes any non-Go Hanzo
// product. It runs as a sidecar next to the product (vector, search, …),
// terminates the product's port, gates every request on the caller's prepaid
// commerce balance (fail-closed) and records per-org usage — then forwards to
// the product on localhost. The product itself needs zero changes.
//
// All configuration is env vars the operator wires (the commerce service token
// from a KMS-backed secret), so one image serves every product:
//
//	METER_PROXY_LISTEN     address to listen on            (default :8080)
//	METER_PROXY_UPSTREAM   the product's real address      (required, e.g. http://127.0.0.1:6333)
//	METER_PROXY_PROVIDER   usage label                     (required, e.g. vector)
//	METER_PROXY_PRICES     price-table spec                (see metering/proxy.ParsePriceTable)
//	METER_PROXY_SKIP       comma list of path prefixes to bypass (health/metrics)
//
//	COMMERCE_URL           commerce base url               (default in-cluster commerce)
//	COMMERCE_SERVICE_TOKEN admin S2S token (KMS-backed)    (required to bill)
//	COMMERCE_SERVICE_ORG   default org when a request carries none
//	METERING_TIER_AWARE    "true" to honor included plan allotment before prepaid
//	METERING_FAIL_OPEN     "true" to allow-on-error (default fail-closed)
//	METERING_TEST          "true" to write commerce's TEST ledger (staging/sandbox)
//	METERING_DISABLED      "true" to forward without gating/recording (local dev)
package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hanzoai/commerce/metering"
	"github.com/hanzoai/commerce/metering/proxy"
)

func main() {
	listen := envOr("METER_PROXY_LISTEN", ":8080")
	upstream := strings.TrimSpace(os.Getenv("METER_PROXY_UPSTREAM"))
	provider := strings.TrimSpace(os.Getenv("METER_PROXY_PROVIDER"))
	pricesSpec := os.Getenv("METER_PROXY_PRICES")
	skip := splitList(os.Getenv("METER_PROXY_SKIP"))

	if upstream == "" {
		log.Fatal("meter-proxy: METER_PROXY_UPSTREAM is required")
	}
	if provider == "" {
		log.Fatal("meter-proxy: METER_PROXY_PROVIDER is required")
	}

	meter, err := metering.FromEnv()
	if err != nil {
		log.Fatalf("meter-proxy: metering config: %v", err)
	}

	handler, err := proxy.New(proxy.Config{
		Upstream:  upstream,
		Provider:  provider,
		Prices:    proxy.ParsePriceTable(pricesSpec),
		SkipPaths: skip,
		Meter:     meter,
	})
	if err != nil {
		log.Fatalf("meter-proxy: %v", err)
	}

	srv := &http.Server{
		Addr:              listen,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("meter-proxy: provider=%s listen=%s upstream=%s billing_enabled=%t",
		provider, listen, upstream, meter.Enabled())
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("meter-proxy: serve: %v", err)
	}
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
