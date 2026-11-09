package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/namespace"
	sqlitedrv "github.com/hanzoai/sqlite"

	"github.com/hanzoai/commerce/util/nscontext"
)

// A host that keeps its stores in one place hands commerce a handle for each tenant,
// and commerce builds its store over it. These tests give it the two handles a host
// has: a SQLite file opened as a single connection, and a schema on a PostgreSQL server
// (env-gated, like every PostgreSQL test here). The same exercise runs on both, so the
// two shapes are held to one behaviour, and then to one table: a tenant's file moved to
// the server as it is must be readable there.

type seamThing struct {
	Name   string `json:"name"`
	Count  int    `json:"count"`
	Active bool   `json:"active"`
}

// fileSeam is the host's SQLite leg: a file per tenant under dir, one connection.
func fileSeam(t *testing.T, dir string) Seam {
	t.Helper()
	return func(ns namespace.Namespace, open func(*sql.DB, bool) (io.Closer, error)) (io.Closer, error) {
		path, err := namespace.Path(dir, ns, tenantSubsystem)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
		h, err := sql.Open("sqlite", sqlitedrv.PragmaDSN(path, plaintextPragmas(SQLiteConfig{})))
		if err != nil {
			return nil, err
		}
		h.SetMaxOpenConns(1)
		c, err := open(h, false)
		if err != nil {
			_ = h.Close()
			return nil, err
		}
		return c, nil
	}
}

// throwaway makes a database on the server dsn names and drops it when the test ends,
// and answers the DSN of it: the schemas below are made in a database of the test's own.
func throwaway(t *testing.T, dsn string) string {
	t.Helper()
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	token := make([]byte, 6)
	_, _ = rand.Read(token)
	name := "commerce_seam_" + hex.EncodeToString(token)
	if _, err := admin.Exec(`CREATE DATABASE ` + name); err != nil {
		t.Fatalf("create the test database: %v", err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(`DROP DATABASE ` + name + ` WITH (FORCE)`) })
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	return u.String()
}

// serverHandle is a pool on a fresh schema of the throwaway database dsn names.
func serverHandle(t *testing.T, dsn, schema string) *sql.DB {
	t.Helper()
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = admin.Close() }()
	if _, err := admin.Exec(`CREATE SCHEMA IF NOT EXISTS "` + schema + `"`); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(dsn)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	h, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal(err)
	}
	h.SetMaxOpenConns(4)
	t.Cleanup(func() { _ = h.Close() })
	return h
}

// serverSeam is the host's PostgreSQL leg: a schema per tenant on the server dsn names,
// found by search_path.
func serverSeam(t *testing.T, dsn string) Seam {
	t.Helper()
	dsn = throwaway(t, dsn)
	return func(ns namespace.Namespace, open func(*sql.DB, bool) (io.Closer, error)) (io.Closer, error) {
		schema := "t_" + hex.EncodeToString([]byte(ns.String()))
		admin, err := sql.Open("postgres", dsn)
		if err != nil {
			return nil, err
		}
		defer func() { _ = admin.Close() }()
		if _, err := admin.Exec(`CREATE SCHEMA IF NOT EXISTS "` + schema + `"`); err != nil {
			return nil, err
		}
		u, err := url.Parse(dsn)
		if err != nil {
			return nil, err
		}
		q := u.Query()
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		h, err := sql.Open("postgres", u.String())
		if err != nil {
			return nil, err
		}
		h.SetMaxOpenConns(4)
		c, err := open(h, true)
		if err != nil {
			_ = h.Close()
			return nil, err
		}
		return c, nil
	}
}

func seamManager(t *testing.T, seam Seam) *Manager {
	t.Helper()
	cfg := DefaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.EnableVectorSearch = false
	cfg.Seam = seam
	m, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })
	return m
}

// exercise is what commerce does to a tenant's store, one operation of each kind, run
// under a deadline: a single connection that two operations wait on is a deadlock, and
// it must read as a failure and not a hung suite.
func exercise(t *testing.T, d DB, org string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ctx = nscontext.WithNamespace(ctx, org)
	key := func(id string) Key { return d.NewKey("Thing", id, 0, nil) }

	for i, name := range []string{"alpha", "beta", "gamma"} {
		if _, err := d.Put(ctx, key(name), &seamThing{Name: name, Count: i, Active: i%2 == 0}); err != nil {
			t.Fatalf("put %s: %v", name, err)
		}
	}
	var got seamThing
	if err := d.Get(ctx, key("beta"), &got); err != nil || got.Name != "beta" || got.Count != 1 {
		t.Fatalf("get beta: %+v %v", got, err)
	}
	if err := d.Get(nscontext.WithNamespace(ctx, org+"-other"), key("beta"), &got); err != ErrNoSuchEntity {
		t.Fatalf("another namespace read beta: %v", err)
	}

	// A request with no namespace has a scope of its own, the one the org's records are in
	// (organization, namespace): a row written there is read there, by key and by query,
	// and is no org's. A file keeps it under no namespace at all, so the server does too.
	bare := context.Background()
	if _, err := d.Put(bare, key("unscoped"), &seamThing{Name: "unscoped"}); err != nil {
		t.Fatalf("put with no namespace: %v", err)
	}
	var plain seamThing
	if err := d.Get(bare, key("unscoped"), &plain); err != nil || plain.Name != "unscoped" {
		t.Fatalf("get with no namespace: %+v %v", plain, err)
	}
	if err := d.Get(ctx, key("unscoped"), &plain); err != ErrNoSuchEntity {
		t.Fatalf("an org read a row written with no namespace: %v", err)
	}
	var found []*seamThing
	if _, err := d.Query("Thing").Filter("Name=", "unscoped").GetAll(bare, &found); err != nil || len(found) != 1 {
		t.Fatalf("query with no namespace: %v %v", found, err)
	}

	// The datastore layer hands GetMulti a slice as long as the keys, to be filled in place.
	many := make([]*seamThing, 2)
	if err := d.GetMulti(ctx, []Key{key("alpha"), key("gamma")}, &many); err != nil || len(many) != 2 || many[1].Name != "gamma" {
		t.Fatalf("getmulti: %v %v", many, err)
	}
	if _, err := d.PutMulti(ctx, []Key{key("delta"), key("epsilon")}, []*seamThing{{Name: "delta"}, {Name: "epsilon"}}); err != nil {
		t.Fatalf("putmulti: %v", err)
	}

	var active []*seamThing
	if _, err := d.Query("Thing").Filter("Active=", true).Order("Name").GetAll(ctx, &active); err != nil || len(active) != 2 || active[0].Name != "alpha" || active[1].Name != "gamma" {
		t.Fatalf("query active: %v %v", active, err)
	}
	if n, err := d.Query("Thing").Count(ctx); err != nil || n != 5 {
		t.Fatalf("count: %d %v", n, err)
	}

	if err := d.Delete(ctx, key("delta")); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := d.Get(ctx, key("delta"), &got); err != ErrNoSuchEntity {
		t.Fatalf("a deleted entity reads back: %v", err)
	}

	// An entity read, then written only while it is still the one that was read.
	var cur map[string]any
	if err := d.Get(ctx, key("alpha"), &cur); err != nil {
		t.Fatal(err)
	}
	if sw, ok := d.(Swapper); ok {
		if wrote, err := sw.Swap(ctx, key("fresh"), "", &seamThing{Name: "fresh"}); err != nil || !wrote {
			t.Fatalf("swap of nothing: %v %v", wrote, err)
		}
		if wrote, err := sw.Swap(ctx, key("fresh"), "", &seamThing{Name: "again"}); err != nil || wrote {
			t.Fatalf("swap over what is there: %v %v", wrote, err)
		}
	}

	seq, ok := d.(Sequencer)
	if !ok {
		t.Fatal("the store has no sequence")
	}
	first, err := seq.NextSequence(ctx, "tags")
	if err != nil {
		t.Fatal(err)
	}
	if second, err := seq.NextSequence(ctx, "tags"); err != nil || second != first+1 {
		t.Fatalf("sequence: %d then %d (%v)", first, second, err)
	}

	err = d.RunInTransaction(ctx, func(tx Transaction) error {
		if _, err := tx.Put(key("txn"), &seamThing{Name: "txn", Count: 9}); err != nil {
			return err
		}
		var in seamThing
		if err := tx.Get(key("txn"), &in); err != nil || in.Count != 9 {
			t.Errorf("a transaction does not read what it wrote: %+v %v", in, err)
		}
		return nil
	}, nil)
	if err != nil {
		t.Fatalf("transaction: %v", err)
	}
	if err := d.Get(ctx, key("txn"), &got); err != nil || got.Count != 9 {
		t.Fatalf("after the transaction: %+v %v", got, err)
	}
}

func TestASeamOpensAnOrgsStoreOnAFile(t *testing.T) {
	m := seamManager(t, fileSeam(t, t.TempDir()))
	d, err := m.Org("acme")
	if err != nil {
		t.Fatalf("Org: %v", err)
	}
	exercise(t, d, "acme")
	if !m.Encrypted() {
		t.Error("a seam's stores are the host's to key, and the manager must say they are")
	}
}

// The seam is for an org; a user's store has no namespace and is opened as it always was.
func TestASeamLeavesAUsersStoreAlone(t *testing.T) {
	m := seamManager(t, func(namespace.Namespace, func(*sql.DB, bool) (io.Closer, error)) (io.Closer, error) {
		t.Error("a user's store was opened through the seam")
		return nil, io.ErrClosedPipe
	})
	u, err := m.User("ann")
	if err != nil {
		t.Fatalf("User: %v", err)
	}
	put(t, u, "Thing", "1", "ann")
	if got := get(t, u, "Thing", "1"); got != "ann" {
		t.Fatalf("a user's store read %q", got)
	}
}

func TestASeamOpensAnOrgsStoreOnAServer(t *testing.T) {
	dsn := os.Getenv("COMMERCE_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("COMMERCE_TEST_POSTGRES_DSN not set")
	}
	m := seamManager(t, serverSeam(t, dsn))
	d, err := m.Org("acme")
	if err != nil {
		t.Fatalf("Org: %v", err)
	}
	exercise(t, d, "acme")
}

// The two stores are one shape, which is what lets a tenant's file be moved to the server
// as it is: the same tables, and in _entities the same columns under the same names.
func TestASeamsFileAndSchemaAreOneShape(t *testing.T) {
	dsn := os.Getenv("COMMERCE_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("COMMERCE_TEST_POSTGRES_DSN not set")
	}
	dsn = throwaway(t, dsn)

	fh, err := sql.Open("sqlite", sqlitedrv.PragmaDSN(filepath.Join(t.TempDir(), "commerce.db"), plaintextPragmas(SQLiteConfig{})))
	if err != nil {
		t.Fatal(err)
	}
	fh.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = fh.Close() })
	if _, err := NewSQLiteDBOn(fh, &SQLiteDBConfig{TenantID: "acme", TenantType: "org"}); err != nil {
		t.Fatal(err)
	}
	ph := serverHandle(t, dsn, "shape")
	if _, err := NewPostgresDBOn(ph, &PostgresDBConfig{TenantID: "acme", TenantType: "org", TenantColumn: seamTenantColumn}); err != nil {
		t.Fatal(err)
	}

	names := func(rows *sql.Rows, err error) map[string]bool {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := map[string]bool{}
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err != nil {
				t.Fatal(err)
			}
			out[n] = true
		}
		return out
	}
	file := names(fh.Query(`SELECT name FROM pragma_table_info('_entities')`))
	server := names(ph.Query(`SELECT column_name FROM information_schema.columns WHERE table_schema = 'shape' AND table_name = '_entities'`))
	if len(file) == 0 || len(file) != len(server) {
		t.Fatalf("_entities has the columns %v on the file and %v on the server", file, server)
	}
	for c := range file {
		if !server[c] {
			t.Errorf("the file's _entities has %q and the server's does not: a move would drop it", c)
		}
	}
	// A row written with no namespace is stored under none, by the file and by the server.
	fd, err := NewSQLiteDBOn(fh, &SQLiteDBConfig{TenantID: "acme", TenantType: "org"})
	if err != nil {
		t.Fatal(err)
	}
	pd, err := NewPostgresDBOn(ph, &PostgresDBConfig{TenantID: "acme", TenantType: "org", TenantColumn: seamTenantColumn})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []DB{fd, pd} {
		if _, err := d.Put(context.Background(), d.NewKey("Thing", "bare", 0, nil), &seamThing{Name: "bare"}); err != nil {
			t.Fatal(err)
		}
	}
	for name, h := range map[string]*sql.DB{"file": fh, "server": ph} {
		var ns string
		if err := h.QueryRow(`SELECT namespace FROM _entities WHERE kind = 'Thing' AND id = 'bare'`).Scan(&ns); err != nil || ns != "" {
			t.Errorf("on the %s a row written with no namespace is under %q (%v)", name, ns, err)
		}
	}
	for _, table := range []string{"_metadata", "_entities", "_sequences"} {
		var inFile, onServer bool
		_ = fh.QueryRow(`SELECT EXISTS (SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?)`, table).Scan(&inFile)
		_ = ph.QueryRow(`SELECT to_regclass($1) IS NOT NULL`, "shape."+table).Scan(&onServer)
		if !inFile || !onServer {
			t.Errorf("table %s: on the file %v, on the server %v", table, inFile, onServer)
		}
	}
}

// Opening a tenant's store again changes nothing in the schema: replicas open a tenant
// while the others write to it, and a trigger dropped and made again waits for every
// writer. The guard still refuses a bearer-shaped tenant under the tenant column's name.
func TestASeamsServerStoreIsMadeOnceAndGuarded(t *testing.T) {
	dsn := os.Getenv("COMMERCE_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("COMMERCE_TEST_POSTGRES_DSN not set")
	}
	base := throwaway(t, dsn)
	h := serverHandle(t, base, "once")
	open := func() *PostgresDB {
		t.Helper()
		pdb, err := NewPostgresDBOn(h, &PostgresDBConfig{TenantID: "acme", TenantType: "org", TenantColumn: seamTenantColumn})
		if err != nil {
			t.Fatal(err)
		}
		return pdb
	}
	trigger := func() (oid uint32) {
		t.Helper()
		if err := h.QueryRow(`SELECT oid::int8 FROM pg_trigger WHERE tgname = 'trg_reject_bearer_tenant'`).Scan(&oid); err != nil {
			t.Fatal(err)
		}
		return oid
	}
	open()
	first := trigger()
	open()
	if again := trigger(); again != first {
		t.Fatalf("opening the store again remade its guard (trigger %d, then %d)", first, again)
	}

	insert := func(tenant string) error {
		_, err := h.Exec(`INSERT INTO _entities (id, kind, namespace, data) VALUES ($1, 'thing', $2, '{}'::jsonb)`, tenant+"-id", tenant)
		return err
	}
	if err := insert("hk-feb5b4b27e2c0"); err == nil {
		t.Fatal("a bearer-shaped tenant was stored")
	}
	if err := insert("acme"); err != nil {
		t.Fatalf("an ordinary tenant was refused: %v", err)
	}

	// A session whose path is another's meets the same guard, and finds the function it
	// calls: a bulk copy into the store, which is how a file's rows are moved there, is
	// such a session.
	elsewhere, err := sql.Open("postgres", base)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = elsewhere.Close() })
	put := func(tenant string) error {
		_, err := elsewhere.Exec(`INSERT INTO "once"._entities (id, kind, namespace, data) VALUES ($1, 'thing', $2, '{}'::jsonb)`, tenant+"-elsewhere", tenant)
		return err
	}
	if err := put("hk-feb5b4b27e2c0"); err == nil || !strings.Contains(err.Error(), "security guard") {
		t.Fatalf("a bearer-shaped tenant written from another path: %v", err)
	}
	if err := put("beta"); err != nil {
		t.Fatalf("an ordinary tenant written from another path was refused: %v", err)
	}
}
