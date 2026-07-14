package billingaccount

import (
	"context"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

func nsDB(parent context.Context, ns string) *datastore.Datastore {
	return datastore.New(nscontext.WithNamespace(parent, ns))
}

// TestAccountAdoptsSubjectID proves an account can adopt an arbitrary stable id
// (the tenant's existing ledger subject) and round-trips by that exact id — the
// invariant that lets the append-only ledger carry forward unmigrated.
func TestAccountAdoptsSubjectID(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := nsDB(c, "hanzo")

	a := New(db)
	a.SetId("hanzo/alice") // adopt the pre-existing per-user subject verbatim
	a.OwnerKind = HolderUser
	a.OwnerId = "hanzo/alice"
	a.DisplayName = "Alice (personal)"
	a.Currency = currency.Type("usd")
	if err := a.Create(); err != nil {
		t.Fatalf("create: %v", err)
	}
	if a.Id() != "hanzo/alice" {
		t.Fatalf("account id = %q; want the adopted subject %q", a.Id(), "hanzo/alice")
	}

	got, err := Get(db, "hanzo/alice")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.OwnerId != "hanzo/alice" || got.OwnerKind != HolderUser {
		t.Fatalf("owner = %s/%s; want user/hanzo/alice", got.OwnerKind, got.OwnerId)
	}
	if got.Currency != currency.Type("usd") {
		t.Fatalf("currency = %q; want usd", got.Currency)
	}
}

// TestGetMissingAccount fails closed: a missing account is ErrNoSuchEntity, never
// a zero-value account that could be mistaken for a real one.
func TestGetMissingAccount(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := nsDB(c, "hanzo")

	if _, err := Get(db, "acct_does_not_exist"); err != datastore.ErrNoSuchEntity {
		t.Fatalf("get missing = %v; want ErrNoSuchEntity", err)
	}
}

// TestNewAccountID mints opaque, unique ids that never encode a tenant slug.
func TestNewAccountID(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for i := 0; i < 1000; i++ {
		id := NewAccountID()
		if !strings.HasPrefix(id, "acct_") {
			t.Fatalf("id %q missing acct_ prefix", id)
		}
		if len(id) != len("acct_")+24 {
			t.Fatalf("id %q has unexpected length %d", id, len(id))
		}
		if seen[id] {
			t.Fatalf("duplicate id minted: %q", id)
		}
		seen[id] = true
	}
}
