package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/hanzoai/commerce/metering"
)

// Config configures a metering reverse proxy.
type Config struct {
	// Upstream is the product's real address, e.g. "http://127.0.0.1:6333"
	// (vector) or "http://127.0.0.1:7700" (search). The proxy forwards every
	// allowed request here unchanged (path, query, body, headers).
	Upstream string

	// Provider labels recorded usage (e.g. "vector", "search"). Stored on the
	// commerce transaction so spend is attributable per product.
	Provider string

	// Prices is the per-request billing policy. Successful requests are charged
	// the matched rule's cents; non-success is free.
	Prices PriceTable

	// SkipPaths are path prefixes that bypass metering entirely (health checks,
	// readiness, metrics). No gate, no charge. Upstream still serves them.
	SkipPaths []string

	// Meter is the metering client (built from metering.FromEnv()). When it is
	// not Enabled (no COMMERCE_URL) the proxy still forwards but neither gates
	// nor records — so a product can deploy the sidecar before billing is wired.
	Meter *metering.Client

	// FlushInterval bounds streaming response flushing (vector/search are
	// request/response, so 0 is fine; set for SSE-style upstreams).
	FlushInterval time.Duration
}

// New builds the metered reverse-proxy handler. It returns an error only for an
// unparseable Upstream URL.
func New(cfg Config) (http.Handler, error) {
	target, err := url.Parse(strings.TrimSpace(cfg.Upstream))
	if err != nil || target.Scheme == "" || target.Host == "" {
		return nil, fmt.Errorf("meter-proxy: invalid upstream %q", cfg.Upstream)
	}

	rp := httputil.NewSingleHostReverseProxy(target)
	rp.FlushInterval = cfg.FlushInterval
	// Preserve the upstream's own error shape; on dial failure return 502 so
	// the caller can distinguish "billing denied" (402/503 from the gate) from
	// "upstream down" (502 from here).
	rp.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":{"message":"upstream unavailable","type":"upstream_error","code":"bad_gateway"}}`))
	}

	prices := cfg.Prices
	provider := cfg.Provider
	skip := append([]string(nil), cfg.SkipPaths...)

	mw := cfg.Meter.Middleware(metering.MiddlewareConfig{
		Provider: provider,
		Price: func(r *http.Request, status int, _ metering.AuthInput) int64 {
			return prices.Price(r.Method, r.URL.Path, clampStatus(status))
		},
		Skip: func(r *http.Request) bool {
			for _, p := range skip {
				if p != "" && strings.HasPrefix(r.URL.Path, p) {
					return true
				}
			}
			return false
		},
	})

	return mw(rp), nil
}
