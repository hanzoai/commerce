// Copyright © 2026 Hanzo AI. MIT License.

// Package postgres opens a REAL Postgres store for one test, on a fresh
// throwaway database, and SKIPS the test when no Postgres is reachable.
//
// It exists because the in-repo unit context (util/test/ae) is SQLite, and
// SQLite hides two properties production depends on. Its Get has a kind-less
// fallback, so a read that Postgres never matches round-trips anyway; and it
// serialises every write behind one process mutex, so a store operation that
// is NOT atomic still looks atomic to every concurrent test. Both differences
// bite exactly where money is: a guard that cannot find its own row pays twice,
// and a running total that loses an update breaches the ceiling it declares.
//
// So the tests that assert an ATOMICITY property run here, against the backend
// that has to provide it. One harness, one door: a second hand-rolled DSN in a
// test file is a second contract with the store.
package postgres

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"os/user"
	"testing"
	"time"

	"github.com/hanzoai/commerce/db"
)

// DSN is the environment variable naming the Postgres to test against.
//
// SETTING IT MAKES THE GATE BINDING. An operator who names a Postgres has
// declared that these tests must run, so an unreachable one FAILS the run
// instead of skipping it — a gate that silently turns itself off is the exact
// defect these tests exist to catch, and a CI whose store is missing would
// otherwise report green while asserting nothing. Leaving it unset is the
// developer's opt-out, and it says so out loud on every skip.
const DSN = "COMMERCE_TEST_PG_DSN"

// New returns a Postgres-backed store on a database created for this test and
// dropped when it ends.
//
// The database is FRESH per test: these tests count rows and assert totals, and
// a shared database makes every one of those assertions a race with whatever
// else is running.
func New(t *testing.T) db.DB {
	t.Helper()

	declared := os.Getenv(DSN) != ""
	// refuse ends the test the way the caller's own declaration says it should.
	refuse := func(format string, args ...any) {
		t.Helper()
		if declared {
			t.Fatalf(DSN+" is set and "+format, args...)
		}
		t.Skipf("SKIPPED, asserting nothing: no postgres ("+format+"). Set "+DSN+" to run it.", args...)
	}

	base := os.Getenv(DSN)
	if base == "" {
		u := "postgres"
		if cu, err := user.Current(); err == nil && cu.Username != "" {
			u = cu.Username
		}
		base = fmt.Sprintf("postgres://%s@localhost:5432/postgres?sslmode=disable", u)
	}

	admin, err := sql.Open("postgres", base)
	if err != nil {
		refuse("cannot be opened: %v", err)
	}
	if err := admin.Ping(); err != nil {
		admin.Close()
		refuse("is not reachable: %v", err)
	}

	name := fmt.Sprintf("commerce_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
		admin.Close()
		refuse("cannot create a test database: %v", err)
	}

	child, err := url.Parse(base)
	if err != nil {
		admin.Exec("DROP DATABASE " + name)
		admin.Close()
		t.Fatalf("parse %s: %v", DSN, err)
	}
	child.Path = "/" + name

	store, err := db.NewPostgresDB(&db.PostgresDBConfig{DSN: child.String(), TenantID: "system"})
	if err != nil {
		admin.Exec("DROP DATABASE " + name)
		admin.Close()
		t.Fatalf("open %s: %v", name, err)
	}

	t.Cleanup(func() {
		store.Close()
		admin.Exec("DROP DATABASE " + name)
		admin.Close()
	})
	return store
}
