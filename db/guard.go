// Copyright (c) 2026-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package db

import (
	"fmt"
	"strings"

	"github.com/hanzoai/commerce/secret"
)

// The credential guard on _entities, below the ORM.
//
// A raw API key presented as a tenant selector must never be persisted as an
// org name or tenant id (incident 2026-07-02). The Go check on the provisioning
// path is the first gate; this trigger is the backstop that also covers writes
// which never pass through the model layer — a direct SQL session, a bulk
// import, a future code path that forgets.
//
// It was created out of band on the production database and existed in no
// migration, so it was absent from SQLite, from local development, from every
// fresh cluster, and — the case that matters — from any restore. A restore
// silently came back up without the last line of defense. It is defined here,
// in the same schema bootstrap that creates the table it guards, so the table
// and its guard can no longer arrive separately.
//
// Both dialects are generated from secret.Prefixes and secret.Blank, the same
// values behind secret.Like, so the database and the Go check cannot disagree.
//
// The guard fires on INSERT only, matching the deployed trigger. Extending it
// to UPDATE would also reject writes to the rows that predate the guard, which
// are being audited rather than modified.

// sqlText renders s as a single-quoted SQL literal.
func sqlText(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// dedent strips the Go source indentation from a SQL literal. Postgres stores a
// function body verbatim, so without this the definition an operator reads back
// with pg_get_functiondef arrives wrapped in this file's tabs.
func dedent(s string) string {
	lines := strings.Split(strings.Trim(s, "\n"), "\n")
	depth := -1
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		if n := len(l) - len(strings.TrimLeft(l, "\t")); depth < 0 || n < depth {
			depth = n
		}
	}
	for i, l := range lines {
		if len(l) >= depth {
			lines[i] = l[depth:]
		}
	}
	return strings.Join(lines, "\n")
}

// postgresGuardDDL creates the credential predicate and the trigger that
// enforces it. Every statement is idempotent, so it is safe to run against a
// database that already carries the out-of-band original.
func postgresGuardDDL() []string {
	// Normalize exactly as secret.Normalize does: strip Blank runes, trim,
	// lowercase. btrim removes ASCII spaces only, which is why the non-ASCII
	// spaces are in Blank and stripped rather than trimmed.
	norm := fmt.Sprintf("lower(btrim(translate(coalesce($1, ''), %s, '')))",
		sqlText(string(secret.Blank)))
	match := fmt.Sprintf("%s ~ %s", norm,
		sqlText("^("+strings.Join(secret.Prefixes, "|")+")"))

	return []string{
		dedent(fmt.Sprintf(`
			CREATE OR REPLACE FUNCTION secret_like(text) RETURNS boolean
			LANGUAGE sql IMMUTABLE AS $fn$ SELECT %s $fn$
		`, match)),

		dedent(`
			CREATE OR REPLACE FUNCTION reject_bearer_tenant() RETURNS trigger
			LANGUAGE plpgsql AS $fn$
			BEGIN
			  IF secret_like(NEW.tenant_id)
			     OR (NEW.kind = 'organization' AND secret_like(NEW.data->>'name'))
			     OR (NEW.kind = 'namespace'    AND secret_like(NEW.data->>'Name'))
			  THEN
			    RAISE EXCEPTION 'security guard: refusing bearer-shaped identifier (kind=%, tenant=%...) — a raw API key must not become a commerce org/tenant', NEW.kind, left(NEW.tenant_id, 6)
			      USING ERRCODE = 'check_violation',
			            HINT = 'blocked by trg_reject_bearer_tenant (incident 2026-07-02)';
			  END IF;
			  RETURN NEW;
			END;
			$fn$
		`),

		dedent(`DROP TRIGGER IF EXISTS trg_reject_bearer_tenant ON _entities`),

		dedent(`
			CREATE TRIGGER trg_reject_bearer_tenant
			BEFORE INSERT ON _entities
			FOR EACH ROW EXECUTE FUNCTION reject_bearer_tenant()
		`),
	}
}

// sqliteGuardDDL is the same guard for the embedded store, which keys entities
// on namespace rather than tenant_id.
//
// SQLite has no regular expressions, so the prefix test is GLOB rather than
// LIKE: GLOB treats '_' literally, while in LIKE it is a single-character
// wildcard that would make 'sk_' match the org "sky".
func sqliteGuardDDL() []string {
	norm := func(expr string) string {
		e := fmt.Sprintf("coalesce(%s, '')", expr)
		for _, r := range secret.Blank {
			e = fmt.Sprintf("replace(%s, char(%d), '')", e, r)
		}
		return fmt.Sprintf("lower(trim(%s))", e)
	}
	match := func(expr string) string {
		tests := make([]string, 0, len(secret.Prefixes))
		for _, p := range secret.Prefixes {
			tests = append(tests, fmt.Sprintf("%s GLOB %s", norm(expr), sqlText(p+"*")))
		}
		return "(" + strings.Join(tests, " OR ") + ")"
	}

	return []string{
		dedent(`DROP TRIGGER IF EXISTS trg_reject_bearer_tenant`),

		dedent(fmt.Sprintf(`
			CREATE TRIGGER trg_reject_bearer_tenant
			BEFORE INSERT ON _entities
			FOR EACH ROW WHEN %s
			   OR (NEW.kind = 'organization' AND %s)
			   OR (NEW.kind = 'namespace'    AND %s)
			BEGIN
			  SELECT RAISE(ABORT, 'security guard: refusing bearer-shaped identifier — a raw API key must not become a commerce org/tenant (incident 2026-07-02)');
			END
		`,
			match("NEW.namespace"),
			match("json_extract(NEW.data, '$.name')"),
			match("json_extract(NEW.data, '$.Name')"),
		)),
	}
}
