package db

import (
	"context"
	"strings"
	"testing"
)

// The sort and filter field paths are caller input on every generic REST list
// route (?sort= at util/rest/rest.go) and they land inside a single-quoted SQL
// string literal that the builder concatenates:
//
//	SQLite:    json_extract(data, '$.<path>')
//	Postgres:  data->>'<path>'
//
// There is no bindable position there. toJSONFieldName is not a guard — it only
// lowercases the first rune of each dot-segment — so an ALL-LOWERCASE payload
// passes through it byte for byte and a single quote closes the literal.
//
// These assert on the SQL the real builders emit, so they fail if the
// normalization underneath the whitelist changes.

// payloads are lowercase on purpose: that is what survives toJSONFieldName.
var injectionPayloads = []string{
	`a') , (select 1) --`,
	`a') , (select count(*) from _entities) --`,
	`a'||(select group_concat(name) from sqlite_master)||'`,
	`a') , iif((select count(*) from _entities)>0, id, rowid) --`,
	`a'), (select 1) union select 1 --`,
	`a' --`,
	`a; drop table _entities --`,
}

var injectionFragments = []string{
	"select 1", "sqlite_master", "count(*)", "--", "union", "iif(", "drop table", "tenant_id <>",
}

func assertNoLeak(t *testing.T, what, payload, sql string) {
	t.Helper()
	for _, frag := range injectionFragments {
		if strings.Contains(payload, frag) && strings.Contains(sql, frag) {
			t.Errorf("%s(%q) leaked %q into SQL: %s", what, payload, frag, sql)
		}
	}
}

type fieldPathThing struct {
	Name   string `json:"name"`
	Secret string `json:"secret"`
}

func fieldPathDB(t *testing.T) *SQLiteDB {
	t.Helper()
	cfg := DefaultConfig()
	d, err := NewSQLiteDB(&SQLiteDBConfig{
		Path:       t.TempDir() + "/fieldpath.db",
		Config:     cfg.SQLite,
		TenantID:   "acme",
		TenantType: "org",
	})
	if err != nil {
		t.Fatalf("NewSQLiteDB: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestSQLiteOrderFieldCannotEscapeJSONPath(t *testing.T) {
	d := fieldPathDB(t)

	for _, payload := range injectionPayloads {
		q := d.Query("thing").Order(payload).(*sqliteQuery)
		sql, _ := q.buildSQL()
		assertNoLeak(t, "Order", payload, sql)
		if !strings.Contains(sql, "ORDER BY rowid ASC") {
			t.Errorf("Order(%q) did not fall back to the default: %s", payload, sql)
		}
	}
}

func TestSQLiteFilterFieldCannotEscapeJSONPath(t *testing.T) {
	d := fieldPathDB(t)

	for _, payload := range injectionPayloads {
		q := d.Query("thing").Filter(payload+"=", "x").(*sqliteQuery)
		sql, _ := q.buildSQL()
		assertNoLeak(t, "Filter", payload, sql)
		// A filter that cannot be expressed must match nothing, never be dropped.
		if !strings.Contains(sql, "1 = 0") {
			t.Errorf("Filter(%q) did not fail closed: %s", payload, sql)
		}
	}
}

// The Postgres store is SHARED by every tenant — isolation is the tenant_id
// column in the WHERE, which a subquery smuggled into ORDER BY does not have to
// respect. That makes the same escape a cross-tenant read there, so the guard
// has to hold on this backend too.
func TestPostgresOrderAndFilterFieldCannotEscapeJSONPath(t *testing.T) {
	crossTenant := `a', (case when (select count(*) from _entities ` +
		`where tenant_id <> $2 and data->>'email' like 'v%')>0 ` +
		`then data->>'name' else '' end) desc --`

	for _, payload := range append(injectionPayloads, crossTenant) {
		oq := (&postgresQuery{kind: "thing"}).Order(payload).(*postgresQuery)
		sql := oq.buildOrderBy()
		assertNoLeak(t, "Order", payload, sql)
		if sql != "" {
			t.Errorf("Order(%q) emitted an ORDER BY: %s", payload, sql)
		}

		fq := (&postgresQuery{kind: "thing"}).Filter(payload+"=", "x").(*postgresQuery)
		where, _ := fq.buildWhere("acme")
		assertNoLeak(t, "Filter", payload, where)
		if !strings.Contains(where, "1 = 0") {
			t.Errorf("Filter(%q) did not fail closed: %s", payload, where)
		}
	}
}

// The whitelist must not break real queries. Callers send Go struct field
// paths — the "-" descending prefix, dotted paths, and the datastore key
// filter's "__key__" all have to keep working.
func TestFieldPathLegitimateNamesStillWork(t *testing.T) {
	d := fieldPathDB(t)

	for field, want := range map[string]string{
		"UpdatedAt":                      "updatedAt",
		"Created":                        "created",
		"Name":                           "name",
		"SKU":                            "sKU",
		"Slug":                           "slug",
		"createdAt":                      "createdAt",
		"Code_":                          "code_",
		"__key__":                        "__key__",
		"Account.BitcoinTransactionTxId": "account.bitcoinTransactionTxId",
		"Claims.UserId":                  "claims.userId",
	} {
		q := d.Query("thing").Order(field).(*sqliteQuery)
		sql, _ := q.buildSQL()
		if !strings.Contains(sql, "'$."+want+"') ASC") {
			t.Errorf("Order(%q) should sort by %q ASC, SQL = %s", field, want, sql)
		}

		fq := d.Query("thing").Filter(field+"=", "x").(*sqliteQuery)
		fsql, args := fq.buildSQL()
		if !strings.Contains(fsql, "'$."+want+"') = ?") {
			t.Errorf("Filter(%q=) should filter on %q, SQL = %s", field, want, fsql)
		}
		if len(args) != 3 || args[2] != "x" {
			t.Errorf("Filter(%q=) lost its bound value: %v", field, args)
		}
	}

	q := d.Query("thing").Order("-UpdatedAt").(*sqliteQuery)
	sql, _ := q.buildSQL()
	if !strings.Contains(sql, "'$.updatedAt') DESC") {
		t.Errorf(`Order("-UpdatedAt") lost DESC: %s`, sql)
	}
}

// End to end on the real driver: the injected clause must have no effect on the
// rows a list endpoint returns. Before the whitelist, this payload executed and
// reordered the page according to a subquery over a DIFFERENT kind in the same
// tenant file — one bit of another kind's data per request.
func TestInjectedSortHasNoEffectOnResults(t *testing.T) {
	d := fieldPathDB(t)
	ctx := context.Background()

	for _, id := range []string{"a", "b", "c"} {
		if _, err := d.Put(ctx, d.NewKey("movie", id, 0, nil), &fieldPathThing{Name: id}); err != nil {
			t.Fatalf("put movie %s: %v", id, err)
		}
	}
	if _, err := d.Put(ctx, d.NewKey("user", "u1", 0, nil), &fieldPathThing{Name: "u1", Secret: "topsecret"}); err != nil {
		t.Fatalf("put user: %v", err)
	}

	names := func(sortField string) []string {
		var got []fieldPathThing
		if _, err := d.Query("movie").Order(sortField).GetAll(ctx, &got); err != nil {
			t.Fatalf("GetAll(%q): %v", sortField, err)
		}
		out := make([]string, len(got))
		for i, g := range got {
			out[i] = g.Name
		}
		return out
	}

	base := names("Name")
	if strings.Join(base, ",") != "a,b,c" {
		t.Fatalf("baseline order is not a,b,c: %v", base)
	}

	// A 1-bit read oracle over a DIFFERENT kind in the same tenant file. The
	// leading key ties for every row, so the ordering is decided entirely by
	// the second key: true reverses the page, false leaves it alone. Repeat
	// with a different LIKE prefix and the whole secret comes out a bit at a
	// time. The trailing "--" comments away the builder's own "') ASC".
	oracle := func(prefix string) string {
		return `a'), iif((select count(*) from _entities where kind='user' and cast(data as text) like '%"secret":"` +
			prefix + `%')>0, json_extract(data,'$.name'), '') desc --`
	}

	// "topsecret" starts with "t", not with "z". A vulnerable build answers
	// those two differently; a guarded build cannot tell them apart because
	// neither reaches the SQL.
	hit, miss := names(oracle("t")), names(oracle("z"))
	if strings.Join(hit, ",") != strings.Join(miss, ",") {
		t.Errorf("injected sort leaked one bit of another kind's row: t=>%v z=>%v", hit, miss)
	}
	if strings.Join(hit, ",") != strings.Join(base, ",") {
		t.Errorf("injected sort changed the result order: base=%v injected=%v", base, hit)
	}
}
