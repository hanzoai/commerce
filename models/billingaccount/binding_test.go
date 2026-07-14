package billingaccount

import (
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/util/test/ae"
)

// TestBindIsIdempotent proves re-binding the same (holder, account) pair upserts
// onto one row and updates its priority in place — never forks a duplicate. This
// is what makes the boot backfill safe to run on every start.
func TestBindIsIdempotent(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := nsDB(c, "bind-idem")

	if _, err := Bind(db, HolderOrg, "acme", "acme", 100); err != nil {
		t.Fatalf("first bind: %v", err)
	}
	// Re-bind with a different priority — must update, not duplicate.
	if _, err := Bind(db, HolderOrg, "acme", "acme", 50); err != nil {
		t.Fatalf("re-bind: %v", err)
	}

	got, err := ForHolder(db, HolderOrg, "acme")
	if err != nil {
		t.Fatalf("for holder: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("bindings = %d; want exactly 1 (re-bind must upsert)", len(got))
	}
	if got[0].Priority != 50 {
		t.Fatalf("priority = %d; want 50 (re-bind updates in place)", got[0].Priority)
	}
}

// TestForHolderIsPriorityOrdered proves a holder's chain is returned lowest
// Priority first (charged first), with a deterministic AccountId tie-break.
func TestForHolderIsPriorityOrdered(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := nsDB(c, "bind-order")

	// Insert out of order; expect priority-ascending back.
	mustBind(t, db, HolderUser, "alice", "acct_c", 30)
	mustBind(t, db, HolderUser, "alice", "acct_a", 10)
	mustBind(t, db, HolderUser, "alice", "acct_b", 10) // tie with acct_a → AccountId breaks it

	got, err := ForHolder(db, HolderUser, "alice")
	if err != nil {
		t.Fatalf("for holder: %v", err)
	}
	want := []string{"acct_a", "acct_b", "acct_c"} // (10,a),(10,b),(30,c)
	if len(got) != len(want) {
		t.Fatalf("bindings = %d; want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].AccountId != w {
			t.Fatalf("chain[%d] = %q; want %q (order=%v)", i, got[i].AccountId, w, accountIDs(got))
		}
	}
}

// TestForHolderEmptyHolderYieldsNothing: an unresolved holder never leaks another
// holder's chain (fail-closed).
func TestForHolderEmptyHolderYieldsNothing(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := nsDB(c, "bind-empty")
	mustBind(t, db, HolderOrg, "acme", "acme", 100)

	got, err := ForHolder(db, HolderOrg, "")
	if err != nil {
		t.Fatalf("for holder: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("empty holder returned %d bindings; want 0", len(got))
	}
}

func mustBind(t *testing.T, db *datastore.Datastore, holderKind, holderId, accountId string, priority int) {
	t.Helper()
	if _, err := Bind(db, holderKind, holderId, accountId, priority); err != nil {
		t.Fatalf("bind %s/%s→%s: %v", holderKind, holderId, accountId, err)
	}
}

func accountIDs(bs []*Binding) []string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = b.AccountId
	}
	return out
}
