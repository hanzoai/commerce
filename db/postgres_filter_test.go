// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package db

import (
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// A numeric or boolean filter binds as the type it compares, not as text: pgx cannot
// encode an int into the text that data->>'f' yields, and a text comparison of
// numbers orders "10" before "9".
func TestPostgresFilterBindsTheValuesType(t *testing.T) {
	cases := []struct {
		name  string
		op    string
		value any
		want  string
		arg   any
	}{
		{"int greater", ">", 0, "END > $3", int64(0)},
		{"int64 less", "<=", int64(250), "END <= $3", int64(250)},
		{"uint", ">=", uint(7), "END >= $3", int64(7)},
		{"float", "<", 1.5, "END < $3", 1.5},
		{"int equal", "=", 5, "END = $3", int64(5)},
		{"bool true", "=", true, "data->>'f' = $3", "true"},
		{"bool not", "!=", true, "data->>'f' != $3", "true"},
		{"string", "=", "paid", "data->>'f' = $3", "paid"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q := (&postgresQuery{kind: "k"}).FilterField("F", c.op, c.value).(*postgresQuery)
			where, args := q.buildWhere("acme")
			if !strings.Contains(where, c.want) {
				t.Fatalf("where = %q, want it to contain %q", where, c.want)
			}
			if len(args) != 2 || args[1] != c.arg {
				t.Fatalf("args = %#v, want the tenant then %#v (%T)", args, c.arg, c.arg)
			}
		})
	}
}

// The same filters against a real Postgres: a number compares as a number ("10" is
// above "9"), a row whose field is not one matches nothing, and a bool binds.
//
// Point COMMERCE_TEST_POSTGRES_DSN at a throwaway database, as TestPostgresGuard does.
func TestPostgresFilterRunsAgainstARealServer(t *testing.T) {
	dsn := os.Getenv("COMMERCE_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("COMMERCE_TEST_POSTGRES_DSN not set")
	}
	pdb, err := NewPostgresDB(&PostgresDBConfig{DSN: dsn, TenantID: "acme", TenantType: "org"})
	if err != nil {
		t.Fatalf("NewPostgresDB: %v", err)
	}
	defer pdb.Close()
	if _, err := pdb.db.Exec(`TRUNCATE _entities`); err != nil {
		t.Fatal(err)
	}
	rows := map[string]string{
		"zero": `{"balance":0}`, "nine": `{"balance":9}`, "ten": `{"balance":10}`, "word": `{"balance":"x"}`,
		"none": `{}`, "on": `{"live":true}`, "off": `{"live":false}`,
	}
	for id, data := range rows {
		if _, err := pdb.db.Exec(`INSERT INTO _entities (id, kind, tenant_id, data) VALUES ($1,'k','acme',$2::jsonb)`, id, data); err != nil {
			t.Fatal(err)
		}
	}
	ids := func(field, op string, v any) []string {
		t.Helper()
		keys, err := pdb.Query("k").FilterField(field, op, v).Order("__key__").Keys(nsCtx("acme"))
		if err != nil {
			t.Fatalf("%s %s %v: %v", field, op, v, err)
		}
		var out []string
		for _, k := range keys {
			out = append(out, k.StringID())
		}
		sort.Strings(out)
		return out
	}
	for _, c := range []struct {
		field, op string
		v         any
		want      []string
	}{
		{"Balance", ">", 0, []string{"nine", "ten"}},
		{"Balance", ">", 9, []string{"ten"}},
		{"Balance", "<=", int64(9), []string{"nine", "zero"}},
		{"Balance", "=", 10, []string{"ten"}},
		{"Balance", ">=", 9.5, []string{"ten"}},
		{"Live", "=", true, []string{"on"}},
		{"Live", "=", false, []string{"nine", "none", "off", "ten", "word", "zero"}},
	} {
		if got := ids(c.field, c.op, c.v); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s %s %v = %v, want %v", c.field, c.op, c.v, got, c.want)
		}
	}
}
