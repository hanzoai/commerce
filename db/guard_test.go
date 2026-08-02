// Copyright © 2026 Hanzo AI. MIT License.

package db

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/secret"
)

// openGuarded opens a fresh tenant store, which applies the schema and the
// credential guard together.
func openGuarded(t *testing.T) *SQLiteDB {
	t.Helper()
	sdb, err := NewSQLiteDB(&SQLiteDBConfig{
		Path:       filepath.Join(t.TempDir(), "guard.db"),
		Config:     DefaultConfig().SQLite,
		TenantID:   "acme",
		TenantType: "org",
	})
	if err != nil {
		t.Fatalf("NewSQLiteDB: %v", err)
	}
	t.Cleanup(func() { sdb.Close() })
	return sdb
}

// A bearer-shaped name must be refused below the ORM. These writes go straight
// at the table with raw SQL — exactly the path that bypasses the Go check, and
// the reason the guard exists at all.
func TestGuardRejectsCredentialNames(t *testing.T) {
	sdb := openGuarded(t)

	// Each case is the same credential arriving by a different route: as the
	// namespace, as an organization's name, as a namespace entity's Name.
	names := []string{
		"sk-ant-api03-cPXc0f",         // the original incident shape
		"hk-feb5b4b27e2c0",            // refused on shape, not on authority
		"sk_live_51H8xQ2eZvKYlo2C",    // Stripe secret — missed before
		"pk_live_51H8xQ2eZvKYlo2C",    // Stripe publishable
		"rk_live_51H8xQ2eZvKYlo2C",    // Stripe restricted
		"whsec_tW3Xy9pLqR2mN8vB",      // Stripe webhook secret
		"Bearer sk-ant-api03",         // a whole header value
		"eyJhbGciOiJIUzI1NiJ9.e30.x",  // a bare JWT
		"SK-HZ-UPPER",                 // case
		"  sk-hz-trimmed",             // ASCII whitespace
		"\u00a0hk-feb5b4b27e2c0",      // non-breaking space, which trim misses
		"\u200bsk-ant-api03",          // zero-width space
		"\ufeffhk-feb5b4b27e2c0",      // BOM
		"s\u200bk-ant-api03",          // zero-width space splitting the marker
		"\ufeff\u200b\u00a0sk_live_1", // stacked
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			// As the namespace/tenant key.
			_, err := sdb.writeDB.Exec(
				`INSERT INTO _entities (id, kind, namespace, data) VALUES (?,?,?,?)`,
				"e1", "order", name, `{}`)
			assertRefused(t, err, "namespace")

			// As an organization's name.
			_, err = sdb.writeDB.Exec(
				`INSERT INTO _entities (id, kind, namespace, data) VALUES (?,?,?,?)`,
				"e2", "organization", "acme", `{"name":`+quoteJSON(name)+`}`)
			assertRefused(t, err, "organization name")

			// As a namespace entity's Name.
			_, err = sdb.writeDB.Exec(
				`INSERT INTO _entities (id, kind, namespace, data) VALUES (?,?,?,?)`,
				"e3", "namespace", "acme", `{"Name":`+quoteJSON(name)+`}`)
			assertRefused(t, err, "namespace Name")
		})
	}
}

// The guard must not cost a real tenant a write. A guard that rejects valid
// orgs would be removed within the day, which is how the last line of defense
// stops being a line of defense.
func TestGuardPermitsRealNames(t *testing.T) {
	sdb := openGuarded(t)

	names := []string{
		"hanzo", "adnexus", "maxpower", "iam-user", "system", "lux",
		"2d4d67ab-30f1-474e-b81f-f60461852259", // UUID
		"skunkworks",                           // shares letters with sk-
		"hkust",                                // shares letters with hk-
		"sky",                                  // '_' must be literal, not a LIKE wildcard
		"pkg", "rkelly", "whsecurity", "eyewear",
	}

	for i, name := range names {
		t.Run(name, func(t *testing.T) {
			id := "ok" + string(rune('a'+i))
			if _, err := sdb.writeDB.Exec(
				`INSERT INTO _entities (id, kind, namespace, data) VALUES (?,?,?,?)`,
				id, "organization", name, `{"name":`+quoteJSON(name)+`}`); err != nil {
				t.Fatalf("guard refused the real org name %q: %v", name, err)
			}
		})
	}
}

// The guard is created by the same bootstrap that creates the table, and that
// bootstrap runs on every open. Re-applying it must not fail, or a restart
// against an existing database — including one restored from backup that
// already carries the trigger — would not come up.
func TestGuardIsIdempotent(t *testing.T) {
	sdb := openGuarded(t)
	for i := range 3 {
		if err := sdb.initSchema(); err != nil {
			t.Fatalf("re-applying schema (pass %d): %v", i+1, err)
		}
	}
	_, err := sdb.writeDB.Exec(
		`INSERT INTO _entities (id, kind, namespace, data) VALUES (?,?,?,?)`,
		"e9", "order", "sk-ant-api03", `{}`)
	assertRefused(t, err, "after re-apply")
}

// Both dialects are generated from the one list, so a marker added to
// secret.Prefixes reaches the database without anyone remembering to edit SQL.
// This is the property that keeps the Go check and the DB check from drifting.
func TestGuardCoversEveryPrefix(t *testing.T) {
	pg := strings.Join(postgresGuardDDL(), "\n")
	lite := strings.Join(sqliteGuardDDL(), "\n")
	for _, p := range secret.Prefixes {
		if !strings.Contains(pg, p) {
			t.Errorf("postgres guard does not carry prefix %q", p)
		}
		if !strings.Contains(lite, p) {
			t.Errorf("sqlite guard does not carry prefix %q", p)
		}
	}
}

func assertRefused(t *testing.T, err error, route string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: guard permitted a credential-shaped name", route)
	}
	if !strings.Contains(err.Error(), "security guard") {
		t.Fatalf("%s: rejected for the wrong reason: %v", route, err)
	}
}

// quoteJSON renders s as a JSON string literal.
func quoteJSON(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}
