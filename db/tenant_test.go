package db

import (
	"context"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	sqlitedrv "github.com/hanzoai/sqlite"
)

type tenantThing struct {
	Name string `json:"name"`
}

// testManager builds a Manager on a temp dir with the tenant bound wound down
// so eviction is reachable in a test rather than after 256 tenants.
func testManager(t *testing.T, maxOpen int) *Manager {
	t.Helper()
	// Match the posture of the build under test. A codec-linked build refuses to
	// open tenant money stores without a master key, so supply one and exercise the
	// same encrypted path production runs. A build without the live codec refuses a
	// key instead, and takes the unencrypted dev path — so give it none.
	if sqlitedrv.CodecLinked() {
		t.Setenv(masterKeyEnv, hex.EncodeToString(testMasterKey()))
	}
	cfg := DefaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.EnableVectorSearch = false
	cfg.MaxOpenTenants = maxOpen
	m, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })
	return m
}

func put(t *testing.T, d DB, kind, id, name string) {
	t.Helper()
	if _, err := d.Put(context.Background(), d.NewKey(kind, id, 0, nil), &tenantThing{Name: name}); err != nil {
		t.Fatalf("put %s/%s: %v", kind, id, err)
	}
}

func get(t *testing.T, d DB, kind, id string) string {
	t.Helper()
	var got tenantThing
	if err := d.Get(context.Background(), d.NewKey(kind, id, 0, nil), &got); err != nil {
		t.Fatalf("get %s/%s: %v", kind, id, err)
	}
	return got.Name
}

// TestManagerBoundsOpenTenantStores is the whole point of the migration: the
// old Manager kept a *SQLiteDB per tenant ever touched and closed them only at
// shutdown, so descriptors and memory grew with the tenant count. Touching more
// tenants than the bound must not hold more stores open than the bound.
func TestManagerBoundsOpenTenantStores(t *testing.T) {
	m := testManager(t, 2)
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		d, err := m.Org(id)
		if err != nil {
			t.Fatalf("Org(%s): %v", id, err)
		}
		put(t, d, "Thing", "1", id)
	}
	if open := m.tenants.Open(); open > 2 {
		t.Fatalf("%d tenant stores open, bound is 2 — the leak is back", open)
	}
	// Evicted is not lost: the file is the state, so a reopened tenant still
	// has its rows.
	d, err := m.Org("a")
	if err != nil {
		t.Fatalf("Org(a) after eviction: %v", err)
	}
	if got := get(t, d, "Thing", "1"); got != "a" {
		t.Errorf("after eviction+reopen got %q, want %q", got, "a")
	}
}

// TestTenantDBOutlivesTheHandleItBorrows covers the bridge: call sites keep the
// DB that Org() returned (Store.DB, the namespaced datastore resolver, the user
// service all do), and it must keep working after the store behind it has been
// evicted and reopened — because it never held that store, it borrows one per
// operation.
func TestTenantDBOutlivesTheHandleItBorrows(t *testing.T) {
	m := testManager(t, 1)
	held, err := m.Org("acme")
	if err != nil {
		t.Fatalf("Org: %v", err)
	}
	put(t, held, "Thing", "1", "before")

	// Push "acme" out with a bound of one.
	if _, err := m.Org("other"); err != nil {
		t.Fatalf("Org(other): %v", err)
	}

	put(t, held, "Thing", "2", "after")
	if got := get(t, held, "Thing", "1"); got != "before" {
		t.Errorf("read through held DB after eviction = %q, want %q", got, "before")
	}
	if got := get(t, held, "Thing", "2"); got != "after" {
		t.Errorf("write through held DB after eviction = %q, want %q", got, "after")
	}
	if open := m.tenants.Open(); open > 1 {
		t.Errorf("%d stores open, bound is 1 — holding a DB must not pin a handle", open)
	}
}

// TestIteratorSurvivesHandleEviction guards the invariant tenantQuery.Run
// depends on. Run borrows a store, starts the query and gives the store back
// before the caller has read a row, so the iterator outlives the borrow. That
// is only sound because an in-flight *sql.Rows keeps its own connection alive
// even when the pool it came from is closed underneath it. If that ever stops
// being true, Run truncates result sets under eviction pressure — silently.
func TestIteratorSurvivesHandleEviction(t *testing.T) {
	m := testManager(t, 1)
	d, err := m.Org("acme")
	if err != nil {
		t.Fatalf("Org: %v", err)
	}
	for _, id := range []string{"1", "2", "3"} {
		put(t, d, "Thing", id, "n"+id)
	}

	ctx := context.Background()
	it := d.Query("Thing").Run(ctx)
	var first tenantThing
	if _, err := it.Next(&first); err != nil {
		t.Fatalf("first Next: %v", err)
	}

	// Evict "acme" mid-iteration.
	if _, err := m.Org("other"); err != nil {
		t.Fatalf("Org(other): %v", err)
	}

	read := 1
	for {
		var row tenantThing
		k, err := it.Next(&row)
		if k == nil || err != nil {
			break
		}
		read++
	}
	if read != 3 {
		t.Fatalf("read %d rows across an eviction, want 3 — the iterator was truncated", read)
	}
}

// TestTenantFilePaths pins the on-disk layout: where a tenant's file lands is a
// fact other things depend on (durability ships the same key to object storage),
// so it is asserted rather than left to whatever the code happens to do.
//
// An ORG's file is placed by hanzoai/namespace — <DataDir>/orgs/<slug>/commerce.db
// — the layout every Hanzo service shares. A USER's file is still commerce's own
// <UserDataDir>/<id>/data.db, because namespace has no layout for a user.
func TestTenantFilePaths(t *testing.T) {
	m := testManager(t, 8)
	if _, err := m.Org("acme"); err != nil {
		t.Fatalf("Org: %v", err)
	}
	if _, err := m.User("bob"); err != nil {
		t.Fatalf("User: %v", err)
	}
	for _, want := range []string{
		filepath.Join(m.config.DataDir, "orgs", "acme", tenantSubsystem+".db"),
		filepath.Join(m.config.UserDataDir, "bob", "data.db"),
	} {
		if _, err := os.Stat(want); err != nil {
			t.Errorf("expected tenant file at %s: %v", want, err)
		}
	}
}

// TestUserAndOrgAreSeparateKeyspaces: an org and a user may share an id and are
// still different tenants with different files. The old Manager got this from
// having two maps; the registry gets it from Tenant.Type.
func TestUserAndOrgAreSeparateKeyspaces(t *testing.T) {
	m := testManager(t, 8)
	org, err := m.Org("acme")
	if err != nil {
		t.Fatalf("Org: %v", err)
	}
	usr, err := m.User("acme")
	if err != nil {
		t.Fatalf("User: %v", err)
	}
	put(t, org, "Thing", "1", "org-row")

	var got tenantThing
	err = usr.Get(context.Background(), usr.NewKey("Thing", "1", 0, nil), &got)
	if err == nil {
		t.Fatalf("user tenant read the org tenant's row (%q) — the keyspaces are shared", got.Name)
	}
	if org.TenantType() != tenantOrg || usr.TenantType() != tenantUser {
		t.Errorf("tenant types = %q/%q, want %q/%q", org.TenantType(), usr.TenantType(), tenantOrg, tenantUser)
	}
}

func TestManagerRejectsUnsafeTenantID(t *testing.T) {
	m := testManager(t, 8)
	// The id reaches here from a gateway-verified owner/subject, so it must
	// never be able to name a file outside the data dir.
	for _, id := range []string{"", "..", "../etc", "a/b", ".hidden"} {
		if _, err := m.Org(id); err == nil {
			t.Errorf("Org(%q) was accepted; it must be rejected", id)
		}
		if _, err := m.User(id); err == nil {
			t.Errorf("User(%q) was accepted; it must be rejected", id)
		}
	}
}

func TestManagerClosedReportsErrDatabaseClosed(t *testing.T) {
	m := testManager(t, 8)
	if _, err := m.Org("acme"); err != nil {
		t.Fatalf("Org: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := m.Org("acme"); !errors.Is(err, ErrDatabaseClosed) {
		t.Errorf("Org after Close = %v, want ErrDatabaseClosed", err)
	}
	if _, err := m.User("bob"); !errors.Is(err, ErrDatabaseClosed) {
		t.Errorf("User after Close = %v, want ErrDatabaseClosed", err)
	}
}

// TestTenantQueryBranchesIndependently: db.Query builders return a new query and
// leave the receiver alone, and callers branch off a partially built one. The
// deferred builder must copy its recorded steps, or the branches would append
// into a shared array and see each other's filters.
func TestTenantQueryBranchesIndependently(t *testing.T) {
	m := testManager(t, 8)
	d, err := m.Org("acme")
	if err != nil {
		t.Fatalf("Org: %v", err)
	}
	put(t, d, "Thing", "1", "keep")
	put(t, d, "Thing", "2", "drop")

	ctx := context.Background()
	base := d.Query("Thing")
	keep := base.Filter("Name=", "keep")
	drop := base.Filter("Name=", "drop")

	for _, tc := range []struct {
		name string
		q    Query
		want int
	}{
		{"base", base, 2},
		{"keep", keep, 1},
		{"drop", drop, 1},
	} {
		n, err := tc.q.Count(ctx)
		if err != nil {
			t.Fatalf("Count(%s): %v", tc.name, err)
		}
		if n != tc.want {
			t.Errorf("Count(%s) = %d, want %d — a branch mutated its parent", tc.name, n, tc.want)
		}
	}
}
