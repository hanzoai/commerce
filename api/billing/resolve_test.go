package billing

import (
	"context"
	"reflect"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billingaccount"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// nsCtx binds ns as the datastore namespace. ae.NewContext() shares a
// process-global in-memory store keyed by namespace, so each test uses its OWN
// namespace (t.Name()) to stay isolated from other tests' bindings.
func nsCtx(parent context.Context, ns string) context.Context {
	return nscontext.WithNamespace(parent, ns)
}

// TestBillingSubjectFor pins the anchor rule — the value both commerce and ai
// must compute identically or a customer tops up one account and spends another.
func TestBillingSubjectFor(t *testing.T) {
	t.Setenv("PERSONAL_BILLING_ORGS", "hanzo")
	t.Setenv("ORG_BILLING_ORGS", "")
	resetBillingOrgSetsForTest()

	cases := []struct{ org, user, want string }{
		{"hanzo", "alice", "hanzo/alice"},       // personal org → per-user
		{"hanzo", "", "hanzo"},                  // org-owned principal → pool
		{"hanzo", "hanzo/alice", "hanzo/alice"}, // already qualified → no double-prefix
		{"acme", "bob", "acme"},                 // pooled org → the org
		{"acme", "", "acme"},                    // pooled org, no user → the org
		{"", "alice", ""},                       // unresolved org → cannot bill
		{"HANZO", "Alice", "hanzo/alice"},       // normalized lowercase
	}
	for _, c := range cases {
		if got := BillingSubjectFor(c.org, c.user); got != c.want {
			t.Errorf("BillingSubjectFor(%q,%q) = %q; want %q", c.org, c.user, got, c.want)
		}
	}
}

// TestBillingSubjectFor_OrgBillingOverride: an org on ORG_BILLING_ORGS pools even
// though it would otherwise be personal-billing (twin of ai's isOrgBilling).
func TestBillingSubjectFor_OrgBillingOverride(t *testing.T) {
	t.Setenv("PERSONAL_BILLING_ORGS", "hanzo")
	t.Setenv("ORG_BILLING_ORGS", "hanzo")
	resetBillingOrgSetsForTest()

	if got := BillingSubjectFor("hanzo", "alice"); got != "hanzo" {
		t.Fatalf("BillingSubjectFor = %q; want %q (ORG_BILLING_ORGS override pools)", got, "hanzo")
	}
}

// TestResolveBilling_DefaultChainIsSingleSubject is the no-regression proof: with
// only the backfilled org binding present, every scope resolves to exactly the
// one subject the pre-chain code used. "person" is the personal-billing org here.
func TestResolveBilling_DefaultChainIsSingleSubject(t *testing.T) {
	t.Setenv("PERSONAL_BILLING_ORGS", "person")
	t.Setenv("ORG_BILLING_ORGS", "")
	resetBillingOrgSetsForTest()

	// Personal org: a per-user subject, and the backfilled org pool binding must
	// NOT leak into a personal user's chain.
	ctxP := nsCtx(ae.NewContext(), "person")
	dbP := datastore.New(ctxP)
	mustBindPkg(t, dbP, billingaccount.HolderOrg, "person", "person", 100) // backfill pool binding
	assertChain(t, ctxP, Scope{Org: "person", User: "alice"}, []string{"person/alice"})
	// An org-owned principal (no user) pays from the pool.
	assertChain(t, ctxP, Scope{Org: "person", User: ""}, []string{"person"})

	// Pooled org: the anchor and the backfilled org binding are the same id and
	// dedup to a single-element chain.
	ctxA := nsCtx(ae.NewContext(), "poolco")
	dbA := datastore.New(ctxA)
	mustBindPkg(t, dbA, billingaccount.HolderOrg, "poolco", "poolco", 100)
	assertChain(t, ctxA, Scope{Org: "poolco", User: "bob"}, []string{"poolco"})
}

// TestResolveBilling_PersonalOverflowAppendsAfterAnchor: an explicit user-holder
// binding is charged AFTER the personal account (redundancy / personal overflow).
func TestResolveBilling_PersonalOverflowAppendsAfterAnchor(t *testing.T) {
	t.Setenv("PERSONAL_BILLING_ORGS", "persoflow")
	t.Setenv("ORG_BILLING_ORGS", "")
	resetBillingOrgSetsForTest()

	ctx := nsCtx(ae.NewContext(), "persoflow")
	db := datastore.New(ctx)
	// The user holder is keyed by the anchor ("persoflow/alice"); bind a second,
	// lower-priority account as overflow.
	mustBindPkg(t, db, billingaccount.HolderUser, "persoflow/alice", "acct_backup", 100)
	assertChain(t, ctx, Scope{Org: "persoflow", User: "alice"}, []string{"persoflow/alice", "acct_backup"})
}

// TestResolveBilling_PooledRedundancy: an org may bind a second funding account;
// the org pays first, the redundant account is the fallback.
func TestResolveBilling_PooledRedundancy(t *testing.T) {
	t.Setenv("PERSONAL_BILLING_ORGS", "hanzo")
	t.Setenv("ORG_BILLING_ORGS", "")
	resetBillingOrgSetsForTest()

	ctx := nsCtx(ae.NewContext(), "poolred")
	db := datastore.New(ctx)
	mustBindPkg(t, db, billingaccount.HolderOrg, "poolred", "poolred", 100)     // primary (== anchor)
	mustBindPkg(t, db, billingaccount.HolderOrg, "poolred", "acct_reserve", 200) // fallback
	assertChain(t, ctx, Scope{Org: "poolred", User: "bob"}, []string{"poolred", "acct_reserve"})
}

// TestResolveBilling_ProjectHolderOrdersByPriority: a project-scoped prepaid
// account bound below the anchor is charged first.
func TestResolveBilling_ProjectHolderOrdersByPriority(t *testing.T) {
	t.Setenv("PERSONAL_BILLING_ORGS", "hanzo")
	t.Setenv("ORG_BILLING_ORGS", "")
	resetBillingOrgSetsForTest()

	ctx := nsCtx(ae.NewContext(), "poolproj")
	db := datastore.New(ctx)
	mustBindPkg(t, db, billingaccount.HolderProject, "proj1", "acct_project", -10) // preempts anchor
	assertChain(t, ctx, Scope{Org: "poolproj", User: "bob", Project: "proj1"},
		[]string{"acct_project", "poolproj"})
}

// TestResolveBilling_EmptyScope: an unresolved org yields no chain, so callers
// fail exactly as when the old resolver returned "".
func TestResolveBilling_EmptyScope(t *testing.T) {
	ctx := nsCtx(ae.NewContext(), "emptyscope")
	chain, err := resolveBilling(ctx, Scope{})
	if err != nil {
		t.Fatalf("resolveBilling: %v", err)
	}
	if len(chain) != 0 {
		t.Fatalf("chain = %v; want empty", chain)
	}
}

func mustBindPkg(t *testing.T, db *datastore.Datastore, holderKind, holderId, accountId string, priority int) {
	t.Helper()
	if _, err := billingaccount.Bind(db, holderKind, holderId, accountId, priority); err != nil {
		t.Fatalf("bind %s/%s→%s: %v", holderKind, holderId, accountId, err)
	}
}

func assertChain(t *testing.T, ctx context.Context, scope Scope, want []string) {
	t.Helper()
	got, err := resolveBilling(ctx, scope)
	if err != nil {
		t.Fatalf("resolveBilling(%+v): %v", scope, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolveBilling(%+v) = %v; want %v", scope, got, want)
	}
}
