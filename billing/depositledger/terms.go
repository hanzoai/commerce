package depositledger

import (
	"context"
	"fmt"
	"strings"

	"github.com/hanzoai/commerce/billing/depositwatch"
	"github.com/hanzoai/commerce/models/organization"
)

// OrgTerms answers what ONE org is charged on a chain, from that org's own
// record. It is the I/O half of depositwatch.TermsResolver — the policy lives
// there, the lookup lives here, the same split the two packages already use for
// everything else.
//
// The platform default (CRYPTO_DEPOSIT_{FEE,SLIPPAGE}_<CHAIN>, KMS-injected)
// stays on the asset; this only speaks when an org has terms of its own.
type OrgTerms struct {
	// Load reads an org by its slug. Injected rather than reaching for the
	// datastore directly so this stays testable without a database, and so the
	// caller decides what "an org" means in its context.
	Load func(ctx context.Context, org string) (*organization.Organization, error)
}

// TermsFor implements depositwatch.TermsResolver.
//
// ok=false means "this org has no opinion, use the platform default". An entry
// that EXISTS and is zero returns ok=true with nothing deducted, because an org
// negotiated down to nothing is a real answer and must not be silently put back
// on the platform's fee.
func (o OrgTerms) TermsFor(ctx context.Context, org, chain string) (depositwatch.Terms, bool, error) {
	if o.Load == nil {
		// No loader is "nobody has configured per-org terms", not an error: a
		// deployment that charges everybody the same is legitimate.
		return depositwatch.Terms{}, false, nil
	}
	rec, err := o.Load(ctx, org)
	if err != nil {
		// Surfaced, never swallowed. depositwatch fails CLOSED on this — it will
		// credit nothing on the pass rather than fall back to a default that is
		// usually cheaper than what the org actually agreed to.
		return depositwatch.Terms{}, false, fmt.Errorf("load org %q: %w", org, err)
	}
	if rec == nil {
		return depositwatch.Terms{}, false, nil
	}
	// Chain names are lowercase everywhere on this rail; normalising here means a
	// record written "Ethereum" still matches rather than silently missing.
	t, ok := rec.CryptoDeposit[strings.ToLower(strings.TrimSpace(chain))]
	if !ok {
		return depositwatch.Terms{}, false, nil
	}
	return depositwatch.Terms{
		FeeCents:    t.FeeCents,
		SlippageBps: t.SlippageBps,
	}, true, nil
}
