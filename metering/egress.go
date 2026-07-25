package metering

import "context"

// Egress / bandwidth metering.
//
// Outbound bandwidth was UNMETERED — a per-org unbounded leak. A tenant could
// egress unlimited TB while the upstream provider (DigitalOcean / Hetzner)
// billed Hanzo for the overage (~$0.01/GB) and NOTHING debited the tenant. This
// makes egress a first-class billable dimension on the canonical usage path
// (POST /v1/billing/usage) so it debits per-org exactly like search / functions
// / vector debit their unit — no new server surface, the generic usageRequest
// already stores an arbitrary Provider label.
//
// Egress is a per-GB "sweeper" dimension (the per-seat / per-GB-month shape in
// metering/CLAUDE.md), not a per-request one: a periodic job reads each org's
// outbound GB for the period and calls RecordEgress. That keeps it off the hot
// request path while closing the leak.
//
// SCOPE: this adds the metering PRIMITIVE (dimension + rate + typed debit). The
// bandwidth-sweeper that reads provider egress stats and calls RecordEgress, and
// a hard per-org egress CAP as defense-in-depth, are follow-ups — flagged for
// Red. Until the sweeper lands, subtract the plan's included transfer allowance
// before calling this so only overage is billed.

// ProviderEgress is the canonical Provider label for outbound-bandwidth usage.
// One name, everywhere, so egress spend is attributable per org across products.
const ProviderEgress = "egress"

// EgressCentsPerGB is the default cost-recovery rate for metered outbound GB.
// DigitalOcean / Hetzner bill egress overage at ~$0.01/GB; billing at least that
// per GB stops the leak from running at a loss. A plan that prices egress
// differently overrides it via EgressUsage.CentsPerGB.
const EgressCentsPerGB = 1 // $0.01/GB

// EgressUsage is one egress debit: billable outbound GB for an org over a period.
type EgressUsage struct {
	User  string // per-org billing key (org slug) — the debit destination (matches the gate).
	Actor string // org/sub identity for the audit trail.
	Org   string // routed via X-Org-Id, not the body.
	GB    int64  // billable outbound GB (overage beyond the plan's included transfer).
	// CentsPerGB overrides EgressCentsPerGB when a plan prices egress differently.
	// Zero (the default) uses the cost-recovery rate.
	CentsPerGB int64
	Project    string // optional scope for the per-scope spend cap (issue #70).
	Service    string // optional scope.
	RequestID  string
}

// RecordEgress meters outbound bandwidth as a first-class egress usage event on
// the canonical /v1/billing/usage path. AmountCents = GB × rate, debited via
// Record — so the fail-closed, per-org billing semantics are identical to every
// other metered dimension. Zero or negative GB is a no-op (nil, nil), matching
// Record's zero-amount contract.
func (c *Client) RecordEgress(ctx context.Context, e EgressUsage) (*RecordResult, error) {
	if e.GB <= 0 {
		return nil, nil
	}
	rate := e.CentsPerGB
	if rate <= 0 {
		rate = EgressCentsPerGB
	}
	return c.Record(ctx, Usage{
		User:        e.User,
		Actor:       e.Actor,
		Org:         e.Org,
		AmountCents: e.GB * rate,
		Provider:    ProviderEgress,
		Project:     e.Project,
		Service:     e.Service,
		RequestID:   e.RequestID,
	})
}
