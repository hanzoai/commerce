// Copyright (c) 2026-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package db

import (
	"os"
	"strings"
	"testing"
)

// The SQLite guard is exercised on every run; this is the same proof for the
// dialect production actually uses. It is env-gated because it needs a real
// Postgres — the trigger is plpgsql and a regex the embedded store cannot run.
//
// Point COMMERCE_TEST_POSTGRES_DSN at a THROWAWAY database. The test creates
// _entities in it via the normal bootstrap and writes to it.
//
//	kubectl -n hanzo port-forward sql-0 15432:5432
//	psql -h 127.0.0.1 -p 15432 -U hanzo -d postgres -c 'CREATE DATABASE guard_check'
//	COMMERCE_TEST_POSTGRES_DSN='postgres://hanzo@127.0.0.1:15432/guard_check?sslmode=disable' \
//	  go test ./db/ -run TestPostgresGuard -v
func TestPostgresGuard(t *testing.T) {
	dsn := os.Getenv("COMMERCE_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("COMMERCE_TEST_POSTGRES_DSN not set")
	}

	// Opening the store runs initSchema, which installs the guard alongside
	// the table — the property that was missing, and the one under test.
	pdb, err := NewPostgresDB(&PostgresDBConfig{DSN: dsn, TenantID: "acme", TenantType: "org"})
	if err != nil {
		t.Fatalf("NewPostgresDB: %v", err)
	}
	defer pdb.Close()

	// Re-runnable: the permitted names below land real rows, and a second run
	// would otherwise collide on the primary key and read as a guard refusal.
	if _, err := pdb.db.Exec(`TRUNCATE _entities`); err != nil {
		t.Fatalf("clearing the throwaway table: %v", err)
	}

	insert := func(id, kind, tenant, data string) error {
		_, err := pdb.db.Exec(
			`INSERT INTO _entities (id, kind, tenant_id, data) VALUES ($1,$2,$3,$4::jsonb)`,
			id, kind, tenant, data)
		return err
	}

	refused := []struct{ name, value string }{
		{"anthropic", "sk-ant-api03-cPXc0f"},
		{"hk", "hk-feb5b4b27e2c0"},
		{"stripe secret", "sk_live_51H8xQ2eZvKYlo2C"},
		{"stripe publishable", "pk_live_51H8xQ2eZvKYlo2C"},
		{"stripe restricted", "rk_live_51H8xQ2eZvKYlo2C"},
		{"stripe webhook", "whsec_tW3Xy9pLqR2mN8vB"},
		{"bearer header", "Bearer sk-ant-api03"},
		{"bare jwt", "eyJhbGciOiJIUzI1NiJ9.e30.x"},
		{"uppercase", "SK-HZ-UPPER"},
		{"leading space", "  sk-hz-trimmed"},
		{"nbsp", "\u00a0hk-feb5b4b27e2c0"},
		{"zero width space", "\u200bsk-ant-api03"},
		{"bom", "\ufeffhk-feb5b4b27e2c0"},
		{"split marker", "s\u200bk-ant-api03"},
		{"stacked", "\ufeff\u200b\u00a0sk_live_1"},
	}
	for _, c := range refused {
		t.Run("refuse/"+c.name, func(t *testing.T) {
			// As the tenant key.
			err := insert("e1", "order", c.value, `{}`)
			assertRefused(t, err, "tenant_id")

			// As an organization's name.
			err = insert("e2", "organization", "acme", `{"name":`+quoteJSON(c.value)+`}`)
			assertRefused(t, err, "organization name")

			// As a namespace entity's Name.
			err = insert("e3", "namespace", "acme", `{"Name":`+quoteJSON(c.value)+`}`)
			assertRefused(t, err, "namespace Name")
		})
	}

	permitted := []string{
		"hanzo", "adnexus", "iam-user", "lux",
		"2d4d67ab-30f1-474e-b81f-f60461852259",
		"skunkworks", "hkust",
		"sky", // '_' is literal in the regex, not a wildcard
		"pkg", "rkelly", "whsecurity", "eyewear",
	}
	for i, name := range permitted {
		t.Run("permit/"+name, func(t *testing.T) {
			id := "ok" + string(rune('a'+i))
			if err := insert(id, "organization", name, `{"name":`+quoteJSON(name)+`}`); err != nil {
				t.Fatalf("guard refused the real org name %q: %v", name, err)
			}
		})
	}

	// Re-applying must not fail: the bootstrap runs on every open, including
	// against a database restored from backup that already carries the guard.
	t.Run("idempotent", func(t *testing.T) {
		for i := range 3 {
			if err := pdb.initSchema(); err != nil {
				t.Fatalf("re-applying schema (pass %d): %v", i+1, err)
			}
		}
		assertRefused(t, insert("e9", "order", "sk-ant-api03", `{}`), "after re-apply")
	})

	// The guard must not report the credential it caught. The exception text
	// carries the kind and six characters, which is what the audit works from.
	t.Run("does not echo the credential", func(t *testing.T) {
		const key = "sk-ant-api03-SECRETTAIL"
		err := insert("e10", "order", key, `{}`)
		if err == nil {
			t.Fatal("guard permitted a credential-shaped tenant")
		}
		if strings.Contains(err.Error(), "SECRETTAIL") {
			t.Errorf("guard echoed the credential back: %v", err)
		}
	})
}
