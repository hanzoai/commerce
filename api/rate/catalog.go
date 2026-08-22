// Copyright © 2026 Hanzo AI. MIT License.

package rate

import (
	"context"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/rate"
)

// The meters the platform charges for that are NOT priced anywhere else.
//
// Two products already have an authority and are deliberately absent:
//
//	ai     — a model's price is a ModelRoute row (InputPrice/OutputPrice),
//	         resolved org-first and read AHEAD of config and the compiled table.
//	         A second source here would make two rows answer one question.
//	tools  — a tool's price is its marketplace LISTING, set by the publisher who
//	         is paid for it. A platform rate card cannot price someone else's tool.
//
// What is left is the platform's own metered work, and each one is currently a
// compiled constant plus an env override, with the same eight-line resolver
// copied beside it. That shape has no audit trail and no way to answer "what did
// we charge in March", because an env var keeps no history.
//
// SEEDED VALUES EQUAL THE CONSTANTS THEY REPLACE, so adopting this changes no
// charge. The rows exist so a price can be MOVED deliberately, in one place,
// with a record of who moved it.
//
// It is not seeded from @hanzo/plans, and that is deliberate: services.json says
// of itself that "nothing bills off this file, and a number here must never
// become a grant". Those are display rate cards for the pricing page. A billing
// number and a marketing number are different facts that happen to agree today,
// and the estate already decided they must not share a home.
func catalog() []*rate.Rate {
	return []*rate.Rate{
		{
			Product: "storage", Meter: "block-gb-month",
			Label: "Block storage", Unit: "GB-month",
			// $0.08/GB-month — apps/provisioning/dedicated.go
			// defaultStoragePriceCents = 8.
			Rate: 80_000_000, Currency: "USD", Source: "catalog",
		},
		{
			Product: "translate", Meter: "bulk-chars",
			Label: "Bulk translation", Unit: "1k characters",
			// 20 µUSD per 1k chars — apps/translate/engine.go
			// defaultBulkPriceUUSDPer1kChars = 20.
			Rate: 20_000, Currency: "USD", Source: "catalog",
		},
		{
			Product: "risk", Meter: "screen",
			Label: "Risk screen", Unit: "screen",
			// 100 µUSD per screen — apps/risk/typed.go
			// defaultScreenUUSD = 100.
			Rate: 100_000, Currency: "USD", Source: "catalog",
		},
	}
}

// SeedRates reconciles the meter authority to the catalog above on every boot,
// exactly as SeedPlans does for plans: it creates what is missing, corrects a row
// that drifted from the published value, and leaves an admin-edited row alone
// because an operator's price outranks the file it came from.
func SeedRates(ctx context.Context) (created, corrected int, err error) {
	return rate.Seed(rate.AuthorityDB(ctx), catalog())
}

// Seeded reports the catalog for callers that want the published value without
// touching the authority — the fail-soft floor a reader falls back to when the
// authority cannot be reached.
func Seeded() []*rate.Rate { return catalog() }

// LogSeed runs the seed and reports it. A boot that silently reprices metered
// work is a boot nobody can audit afterwards.
func LogSeed(ctx context.Context) {
	created, corrected, err := SeedRates(ctx)
	if err != nil {
		log.Error("meter authority seed failed: %v", err, nil)
		return
	}
	if created > 0 || corrected > 0 {
		log.Info("meter authority seeded: created=%d corrected=%d", created, corrected)
	}
}
