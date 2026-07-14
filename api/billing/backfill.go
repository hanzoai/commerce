package billing

import (
	"context"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billingaccount"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/types/currency"
)

// backfillOrgPriority is the Priority of an org's own pool binding — its primary
// funding relationship. The anchor is charged first (priority 0) and dedups with
// this binding for a pooled org; redundancy/overflow bindings use higher numbers.
const backfillOrgPriority = 100

// BackfillBillingAccounts ensures every existing org has a first-class
// BillingAccount plus an org→account Binding, both in that org's namespace. It is
// idempotent and boot-safe (mirrors IAM's BackfillDefaultProjects): each account
// ADOPTS the org's existing ledger subject (its slug) as its id, so the
// append-only ledger and vaulted cards carry forward with NO migration, and the
// org→account binding turns BillingSubjectFor's pooled branch into a stored
// default rather than special-case code.
//
// A personal-billing org's per-USER accounts are deliberately NOT created here:
// commerce cannot enumerate IAM users at boot, and resolveBilling already
// synthesises the per-user account (the anchor "<org>/<user>") as the degenerate
// default — its ledger keys on that subject string whether or not an account
// entity yet exists. A per-user account entity and any overflow binding are
// provisioned on demand, not at backfill.
//
// Returns the number of accounts newly created (0 on a fully-backfilled rerun).
// Errors are returned to the caller; boot wiring logs and continues (best-effort,
// never fatal) so a transient datastore error cannot crash startup.
func BackfillBillingAccounts(ctx context.Context) (int, error) {
	// The org registry lives in the ROOT namespace (same enumeration the billing
	// cycle / auto-recharge run-all use).
	rootDb := datastore.New(ctx)
	orgs := make([]*organization.Organization, 0)
	if _, err := organization.Query(rootDb).GetAll(&orgs); err != nil {
		return 0, err
	}

	created := 0
	for _, org := range orgs {
		slug := org.Namespace() // == org.Name; the tenant's subject today
		if slug == "" {
			continue
		}
		db := datastore.New(org.Namespaced(ctx))

		// Account: adopt the slug as the stable id. Key-exact Get so a rerun sees
		// the existing row (idempotent create).
		if _, err := billingaccount.Get(db, slug); err == datastore.ErrNoSuchEntity {
			acct := billingaccount.New(db)
			acct.SetId(slug)
			acct.OwnerKind = billingaccount.HolderOrg
			acct.OwnerId = slug
			acct.DisplayName = org.FullName
			acct.Currency = currency.USD
			if err := acct.Create(); err != nil {
				return created, err
			}
			created++
		} else if err != nil {
			return created, err
		}

		// Org→account binding (deterministic id → idempotent upsert).
		if _, err := billingaccount.Bind(db, billingaccount.HolderOrg, slug, slug, backfillOrgPriority); err != nil {
			return created, err
		}
	}
	return created, nil
}
