package costs

import (
	"cmp"
	"context"
	"net/http"
	"slices"
	"time"
)

// httpTimeout bounds every outbound vendor-API call so a slow/hung vendor can
// never hang the admin report. One shared client per report.
const httpTimeout = 8 * time.Second

// buildReport assembles the full vendor-cost picture for a period. It is the ONE
// place vendors are enumerated — adding a vendor is adding a line here (and, for a
// real-cost one, its client). Every branch is honest: an actual figure when the
// vendor API is reachable, an estimate labeled as such otherwise, never a fake.
//
// The ledger aggregate is computed once and shared: OpenAI's estimate fallback and
// the Anthropic/Fireworks/DO-AI estimates all read from it, so the metering source
// is walked a single time.
// Report is the per-vendor COGS breakdown for a period — the QUERY, with no HTTP
// in it.
//
// It exists so a caller that is not a request can ask: the admin cockpit reads
// this god-view from a process that holds none of these books, and it used to do
// it by re-entering commerce's own /v1/costs endpoint. Re-deriving the walk on that
// side would be a second implementation of one question, and two answers to
// "what did we pay our vendors" is a worse failure than no answer.
//
// It returns the report only. The ledger aggregate buildReport also computes is
// how margin is kept DRY with cost inside this package; it is not a fact a cost
// reader asked for, and sending it would make every caller inherit it.
func Report(ctx context.Context, isTest bool, period string) CostReport {
	report, _ := buildReport(ctx, isTest, period)
	return report
}

func buildReport(ctx context.Context, isTest bool, p string) (CostReport, ledgerUsage) {
	client := &http.Client{Timeout: httpTimeout}
	ledger := aggregateLedger(ctx, isTest, p)

	vendors := []VendorCost{
		// 1) DigitalOcean compute — actual via the DO billing API.
		digitalOceanCost(ctx, client, p),

		// 2) LLM providers we resell.
		//    OpenAI — actual via org-costs, else metered estimate.
		openAICost(ctx, client, p, ledger.ProviderCOGSCents["openai"]),
		//    Anthropic / Fireworks / DO-AI — no cost API; metered estimate (COGS =
		//    tokens x cost-basis). Reported source=estimated, honestly.
		estimatedLLM("anthropic", p, ledger.ProviderCOGSCents["anthropic"]),
		estimatedLLM("fireworks", p, ledger.ProviderCOGSCents["fireworks"]),
		estimatedLLM("digitalocean-ai", p, ledger.ProviderCOGSCents["digitalocean"]),
	}

	// 3) Extensible vendor registry — vendors we pay but haven't wired a cost
	//    source for yet. Reported at 0 with an honest source label + note, never
	//    a fabricated number. Wiring one = replacing its stub with a real client.
	vendors = append(vendors, pendingVendors(p)...)

	// Drop empty estimated LLM lines (no metered usage AND no wired API) so the
	// report isn't padded with zeros, but ALWAYS keep DigitalOcean and the pending
	// stubs (their presence is itself the honest signal).
	vendors = pruneEmpty(vendors)

	slices.SortStableFunc(vendors, func(a, b VendorCost) int {
		return cmp.Or(
			cmp.Compare(b.AmountCents, a.AmountCents), // highest first
			cmp.Compare(a.Vendor, b.Vendor),
		)
	})

	return CostReport{
		Period:     p,
		Vendors:    vendors,
		TotalCents: total(vendors),
		Currency:   "usd",
	}, ledger
}

// estimatedLLM builds a metered-estimate cost line for a provider without a cost
// API. The amount is the ledger-derived COGS (tokens x cost-basis).
func estimatedLLM(vendor, p string, cents int64) VendorCost {
	line := VendorCost{
		Vendor:      vendor,
		Service:     "llm-inference",
		AmountCents: cents,
		Period:      p,
		Source:      SourceEstimated,
		Currency:    "usd",
		Note:        "COGS estimated from metered tokens x cost-basis (no vendor cost API)",
	}
	return line
}

// pendingVendor names a vendor we pay for which no cost source is wired yet.
type pendingVendor struct {
	vendor  string
	service string
}

// pendingRegistry is the extensible list of not-yet-wired vendors. Slotting a new
// vendor in is one line here; wiring its real cost is replacing the stub in
// buildReport with a client (like digitalOceanCost / openAICost).
var pendingRegistry = []pendingVendor{
	{vendor: "cloudflare", service: "cdn-dns"},
	{vendor: "npm", service: "registry"},
	{vendor: "github", service: "vcs-ci"},
}

// pendingVendors renders the pending registry as honest estimated-0 lines.
func pendingVendors(p string) []VendorCost {
	out := make([]VendorCost, 0, len(pendingRegistry))
	for _, pv := range pendingRegistry {
		out = append(out, VendorCost{
			Vendor:      pv.vendor,
			Service:     pv.service,
			AmountCents: 0,
			Period:      p,
			Source:      SourceEstimated,
			Currency:    "usd",
			Note:        "cost source not wired yet — reported 0, not fabricated",
		})
	}
	return out
}

// pruneEmpty drops estimated LLM lines that are 0 (no usage, no API), keeping the
// always-present DigitalOcean line and the pending stubs.
func pruneEmpty(vendors []VendorCost) []VendorCost {
	out := vendors[:0]
	for _, v := range vendors {
		if v.Service == "llm-inference" && v.Vendor != "openai" && v.AmountCents == 0 {
			continue
		}
		out = append(out, v)
	}
	return out
}
