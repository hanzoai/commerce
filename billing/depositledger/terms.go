package depositledger

import (
	"context"
	"fmt"
	"strings"

	"github.com/hanzoai/commerce/billing/depositwatch"
	"github.com/hanzoai/commerce/models/organization"
)

// OrgTerms is the I/O half of depositwatch.TermsResolver: the policy lives
// there, the lookup here. It speaks only when an org has terms of its own; the
// platform default stays on the asset.
type OrgTerms struct {
	// Load reads an org by slug. Injected so this is testable without a database.
	Load func(ctx context.Context, org string) (*organization.Organization, error)
}

// TermsFor implements depositwatch.TermsResolver. ok=false means "no opinion,
// use the default"; an entry that exists and is zero returns ok=true with
// nothing deducted, because that is a real answer.
func (o OrgTerms) TermsFor(ctx context.Context, org, chain string) (depositwatch.Terms, bool, error) {
	if o.Load == nil {
		return depositwatch.Terms{}, false, nil // charging everybody the same is legitimate
	}
	rec, err := o.Load(ctx, org)
	if err != nil {
		// Surfaced, never swallowed: depositwatch fails closed on it.
		return depositwatch.Terms{}, false, fmt.Errorf("load org %q: %w", org, err)
	}
	if rec == nil {
		return depositwatch.Terms{}, false, nil
	}
	// Lowercase everywhere on this rail, so a record written "Ethereum" matches.
	t, ok := rec.CryptoDeposit[strings.ToLower(strings.TrimSpace(chain))]
	if !ok {
		return depositwatch.Terms{}, false, nil
	}
	return depositwatch.Terms{
		FeeCents:    t.FeeCents,
		SlippageBps: t.SlippageBps,
	}, true, nil
}
