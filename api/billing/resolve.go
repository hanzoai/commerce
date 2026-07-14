package billing

import (
	"context"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billingaccount"
)

// The billing SUBJECT is the account money lands in and the gate reads out of.
// It is NOT always the org: hanzoai/ai (object/billing_subject.go) resolves a
// personal-billing org's members to a per-user subject, and the LLM gate reads
// that subject. Commerce and ai must compute the same subject or a customer tops
// up one account and spends from another.
//
// One rule, mirrored from ai so the two can never drift:
//
//	ORG_BILLING_ORGS org      → "<org>"        (explicit pooled override — wins)
//	personal-billing org      → "<org>/<user>" (per-user; default set {hanzo})
//	pooled org                → "<org>"        (per-org)
//
// resolveBilling generalises this single subject into an ordered CHAIN of billing
// accounts (this file's BillingSubjectFor is the chain's anchor). The 2-way
// personal/pooled branch is therefore the degenerate default of the binding
// model: with only the backfilled bindings present the chain is exactly
// [BillingSubjectFor(org,user)] — byte-identical to the pre-chain behavior.

var (
	personalBillingOrgsOnce sync.Once
	personalBillingOrgs     map[string]struct{}

	orgBillingOrgsOnce sync.Once
	orgBillingOrgs     map[string]struct{}
)

// parseOrgSet parses a comma-separated list of org slugs into a lowercased set —
// the ONE parser both billing-org allowlists share (mirrors ai's parseOrgSet).
func parseOrgSet(raw string) map[string]struct{} {
	m := make(map[string]struct{})
	for _, o := range strings.Split(raw, ",") {
		if o = strings.ToLower(strings.TrimSpace(o)); o != "" {
			m[o] = struct{}{}
		}
	}
	return m
}

// loadPersonalBillingOrgs parses PERSONAL_BILLING_ORGS (default "hanzo"). Lazy +
// cached; the fail-safe default keeps the shared catch-all per-user even if unset.
func loadPersonalBillingOrgs() map[string]struct{} {
	personalBillingOrgsOnce.Do(func() {
		raw := os.Getenv("PERSONAL_BILLING_ORGS")
		if strings.TrimSpace(raw) == "" {
			raw = "hanzo" // same fail-safe default as ai; keep them identical
		}
		personalBillingOrgs = parseOrgSet(raw)
	})
	return personalBillingOrgs
}

// loadOrgBillingOrgs parses ORG_BILLING_ORGS (default EMPTY). An org listed here
// shares ONE pooled account even when it would otherwise be personal-billing —
// the scoped, additive override that wins over the personal default. Mirrors ai.
func loadOrgBillingOrgs() map[string]struct{} {
	orgBillingOrgsOnce.Do(func() {
		orgBillingOrgs = parseOrgSet(os.Getenv("ORG_BILLING_ORGS"))
	})
	return orgBillingOrgs
}

// IsPersonalBillingOrg reports whether members of org hold individual balances
// rather than sharing the org's pooled one. isOrgBilling overrides it.
func IsPersonalBillingOrg(org string) bool {
	_, ok := loadPersonalBillingOrgs()[strings.ToLower(strings.TrimSpace(org))]
	return ok
}

// isOrgBilling reports whether org is on the ORG_BILLING_ORGS allowlist — its
// members share ONE pooled account (subject = org), the explicit switch that WINS
// over the personal-billing default.
func isOrgBilling(org string) bool {
	_, ok := loadOrgBillingOrgs()[strings.ToLower(strings.TrimSpace(org))]
	return ok
}

// BillingSubjectFor is the canonical account id for an (org, user) identity — the
// chain's anchor. It must produce byte-for-byte what ai's BillingSubject does, or
// deposits and the gate diverge:
//
//	ORG_BILLING_ORGS org → "<org>"        (override — wins)
//	personal-billing org → "<org>/<user>" (per-user)
//	pooled org           → "<org>"        (per-org)
//
// A caller may already hand a fully-qualified "<org>/<user>" as user (usage
// records carry the resolved subject, not a bare name), so an already-qualified
// user is never double-prefixed. Empty org yields "" (caller cannot bill).
func BillingSubjectFor(org, user string) string {
	org = strings.ToLower(strings.TrimSpace(org))
	if org == "" {
		return ""
	}
	// ORG-billing allowlist wins: the whole org shares one pooled account.
	if isOrgBilling(org) {
		return org
	}
	if !IsPersonalBillingOrg(org) {
		return org // pooled: one account for the org
	}
	user = strings.ToLower(strings.TrimSpace(user))
	if user == "" {
		return org // an org-owned principal (application / service key) pays from the pool
	}
	if strings.HasPrefix(user, org+"/") {
		return user // already the per-user subject — do not double-prefix
	}
	return org + "/" + user
}

// resetBillingOrgSetsForTest re-reads PERSONAL_BILLING_ORGS / ORG_BILLING_ORGS.
// The caches are sync.Once, so a test that sets the env after first use would
// otherwise read a stale map.
func resetBillingOrgSetsForTest() {
	personalBillingOrgsOnce = sync.Once{}
	personalBillingOrgs = nil
	orgBillingOrgsOnce = sync.Once{}
	orgBillingOrgs = nil
}

// Scope names whose billing a request targets, in already-effective terms: Org is
// the resolved namespace (from the validated X-Org-Id, after EdgeAuth's
// SuperAdmin ?org= override + subject-lock), User is the caller (X-User-Id, or a
// usage record's resolved subject), Project is X-Project-Id. resolveBilling turns
// a Scope into the ordered account chain.
type Scope struct {
	Org     string
	User    string
	Project string
}

// holder-class tiebreak ranks (used only when two candidates share a Priority):
// the anchor is always first, then project, org, user — matching the documented
// "bindings(project) ++ bindings(org) ++ bindings(user)" default grouping.
const (
	rankAnchor  = -1
	rankProject = 0
	rankOrg     = 1
	rankUser    = 2

	// anchorPriority makes the anchor the primary (charged first) account by
	// default; an explicit binding with a lower Priority may still preempt it.
	anchorPriority = 0
)

// resolveBilling returns the ordered chain of account ids to charge/authorize for
// scope — the account charged first, first. The chain is the anchor
// (BillingSubjectFor, always present and primary) merged with the explicit
// bindings for the project, user, and org holders, ordered by (Priority asc,
// holder-class, AccountId) and deduped.
//
// No-regression invariant: with ONLY the backfilled bindings present (no
// operator-added overflow), the chain is exactly [anchor] — identical to the
// pre-chain single-subject behavior. A personal-billing user's org POOL is
// deliberately NOT added to their chain (that would let one member drain the
// shared pool, the leak per-user billing closed); the pool is reached only by an
// org-owned principal (no user → anchor == org) or an explicit overflow binding.
//
// An unresolved scope (no org) yields an empty chain; callers then behave exactly
// as when BillingSubjectFor returned "" (401/402), so nothing regresses.
func resolveBilling(ctx context.Context, scope Scope) ([]string, error) {
	anchor := BillingSubjectFor(scope.Org, scope.User)
	if anchor == "" {
		return nil, nil
	}
	orgSlug := strings.ToLower(strings.TrimSpace(scope.Org))
	personalUser := anchor != orgSlug // per-user subject ("<org>/<user>")

	db := datastore.New(ctx)

	type cand struct {
		accountId string
		priority  int
		rank      int
	}
	cands := []cand{{accountId: anchor, priority: anchorPriority, rank: rankAnchor}}

	collect := func(holderKind, holderId string, rank int) error {
		bs, err := billingaccount.ForHolder(db, holderKind, holderId)
		if err != nil {
			return err
		}
		for _, b := range bs {
			cands = append(cands, cand{accountId: b.AccountId, priority: b.Priority, rank: rank})
		}
		return nil
	}

	if err := collect(billingaccount.HolderProject, scope.Project, rankProject); err != nil {
		return nil, err
	}
	// The user holder is keyed by the anchor: for a personal user that is their
	// per-user subject; for a pooled principal it is the org (overflow via the
	// org holder). Both paths (usage debit, balance read) compute the same anchor,
	// so an overflow binding is found consistently.
	if err := collect(billingaccount.HolderUser, anchor, rankUser); err != nil {
		return nil, err
	}
	if !personalUser {
		if err := collect(billingaccount.HolderOrg, orgSlug, rankOrg); err != nil {
			return nil, err
		}
	}

	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].priority != cands[j].priority {
			return cands[i].priority < cands[j].priority
		}
		if cands[i].rank != cands[j].rank {
			return cands[i].rank < cands[j].rank
		}
		return cands[i].accountId < cands[j].accountId
	})

	seen := make(map[string]struct{}, len(cands))
	chain := make([]string, 0, len(cands))
	for _, cn := range cands {
		if _, dup := seen[cn.accountId]; dup {
			continue
		}
		seen[cn.accountId] = struct{}{}
		chain = append(chain, cn.accountId)
	}
	return chain, nil
}

// primarySubject returns the first (primary) account in scope's chain, or "" when
// the scope resolves to nobody. It is the drop-in for the old single-subject
// resolvers (orgBillingKey / the branch's userBillingKey): the account today's
// code would have used, now sourced through the one chain resolver.
func primarySubject(ctx context.Context, scope Scope) (string, error) {
	chain, err := resolveBilling(ctx, scope)
	if err != nil || len(chain) == 0 {
		return "", err
	}
	return chain[0], nil
}
