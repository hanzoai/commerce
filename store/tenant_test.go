// Package store — tenant repository tests.
//
// These tests exercise the repository against a throwaway SQLite file,
// booted through a real base app so the migration actually runs. The same
// shape will work for Postgres: set COMMERCE_BASE_URL before `go test` and
// the repo talks to Postgres instead. We deliberately do NOT fake base —
// that would let regressions in the JSON-array lookup path slip past CI.
package store

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

// newTestStore constructs an isolated store under t.TempDir(). Cleanup is
// registered via t.Cleanup so the DB pool is closed and the on-disk files
// go away on test completion.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := New(Config{DataDir: filepath.Join(dir, "commerce")})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() {
		_ = s.Close(nil)
	})
	return s
}

// newTestTenant returns a minimal tenant fixture; tests may mutate the
// returned value before passing to Create.
func newTestTenant(name string, hosts ...string) *Tenant {
	return &Tenant{
		Name:      name,
		Hostnames: hosts,
		Brand: BrandConfig{
			DisplayName:  name,
			PrimaryColor: "#0ea5e9",
		},
		IAM: IAMConfig{
			Issuer:   "https://id.example.test",
			ClientID: name + "-client",
		},
		IDV: IDVConfig{
			Provider: "persona",
			Endpoint: "https://withpersona.com/verify",
		},
		Providers: []Provider{
			{Name: "square", Enabled: true, KMSPath: "commerce/" + name + "/square"},
			{Name: "braintree", Enabled: false, KMSPath: "commerce/" + name + "/braintree"},
		},
		BDEndpoint:         "https://bd.example.test",
		ReturnURLAllowlist: []string{"https://example.test"},
	}
}

// ─── Create ──────────────────────────────────────────────────────────────

func TestCreate_AssignsIDAndTimestamps(t *testing.T) {
	s := newTestStore(t)
	tenant := newTestTenant("liquidity", "pay.example.test")

	if err := s.Tenants.Create(tenant); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tenant.ID == "" {
		t.Errorf("expected ID assigned, got empty")
	}
	if tenant.Created.IsZero() {
		t.Errorf("expected Created populated")
	}
	if tenant.Updated.IsZero() {
		t.Errorf("expected Updated populated")
	}
}

func TestCreate_DuplicateNameFails(t *testing.T) {
	s := newTestStore(t)
	a := newTestTenant("acme", "pay.acme.test")
	b := newTestTenant("acme", "pay2.acme.test")

	if err := s.Tenants.Create(a); err != nil {
		t.Fatalf("first create: %v", err)
	}
	err := s.Tenants.Create(b)
	if err != ErrDuplicateTenant {
		t.Errorf("second Create err = %v, want ErrDuplicateTenant", err)
	}
}

func TestCreate_RejectsInvalidHostname(t *testing.T) {
	s := newTestStore(t)
	cases := map[string]string{
		"leading-whitespace":  " pay.example.test",
		"trailing-whitespace": "pay.example.test ",
		"embedded-newline":    "pay.example\ntest",
		"embedded-quote":      `pay"example.test`,
		"empty":               "",
		"ipv6-literal":        "[::1]",
		"raw-ipv4":            "10.0.0.1",
	}
	for name, h := range cases {
		t.Run(name, func(t *testing.T) {
			tenant := newTestTenant("tenant-"+name, h)
			if err := s.Tenants.Create(tenant); err == nil {
				t.Errorf("Create accepted hostname %q — expected rejection", h)
			}
		})
	}
}

func TestCreate_DedupesAndNormalizesHostnames(t *testing.T) {
	s := newTestStore(t)
	tenant := newTestTenant("norm",
		"PAY.EXAMPLE.TEST",
		"pay.example.test",
		"pay.example.test.",    // trailing dot
		"pay.example.test:443", // port
	)
	if err := s.Tenants.Create(tenant); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(tenant.Hostnames) != 1 || tenant.Hostnames[0] != "pay.example.test" {
		t.Errorf("hostnames = %v, want [pay.example.test]", tenant.Hostnames)
	}
}

// ─── FindByHostname ──────────────────────────────────────────────────────

func TestFindByHostname_ExactAndNormalized(t *testing.T) {
	s := newTestStore(t)
	if err := s.Tenants.Create(newTestTenant(
		"liquidity",
		"pay.satschel.test",
		"pay.dev.satschel.test",
	)); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		host string
		want string
	}{
		{"pay.satschel.test", "liquidity"},
		{"pay.dev.satschel.test", "liquidity"},
		{"PAY.SATSCHEL.TEST", "liquidity"},
		{"pay.satschel.test:443", "liquidity"},
		{"pay.satschel.test.", "liquidity"}, // trailing dot
	}
	for _, tc := range cases {
		got, err := s.Tenants.FindByHostname(tc.host)
		if err != nil {
			t.Errorf("FindByHostname(%q) err = %v, want nil", tc.host, err)
			continue
		}
		if got.Name != tc.want {
			t.Errorf("FindByHostname(%q).Name = %q, want %q", tc.host, got.Name, tc.want)
		}
	}
}

func TestFindByHostname_SubdomainMismatch(t *testing.T) {
	s := newTestStore(t)
	if err := s.Tenants.Create(newTestTenant("liquidity", "pay.satschel.test")); err != nil {
		t.Fatal(err)
	}

	// None of these should match — exact-only.
	spoofs := []string{
		"evil.test",
		"satschel.test",
		"xyzpay.satschel.test",
		"pay.satschel.test.evil.test",
		"a.pay.satschel.test",
	}
	for _, h := range spoofs {
		_, err := s.Tenants.FindByHostname(h)
		if err != ErrTenantNotFound {
			t.Errorf("FindByHostname(%q) err = %v, want ErrTenantNotFound", h, err)
		}
	}
}

func TestFindByHostname_EmptyReturnsInvalid(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Tenants.FindByHostname("")
	if err != ErrInvalidHostname {
		t.Errorf("empty host err = %v, want ErrInvalidHostname", err)
	}
}

// A hostname NOT in any tenant's hostnames array must return ErrTenantNotFound
// regardless of how it collates alphabetically. Catches JSON-LIKE pattern
// bugs that could match substrings.
func TestFindByHostname_SubstringOracleResistance(t *testing.T) {
	s := newTestStore(t)
	if err := s.Tenants.Create(newTestTenant(
		"prefix",
		"aaaa.example.test",
	)); err != nil {
		t.Fatal(err)
	}

	// "aaaa.example.test" is stored; "aaa.example.test" must NOT hit.
	if _, err := s.Tenants.FindByHostname("aaa.example.test"); err != ErrTenantNotFound {
		t.Errorf("substring oracle leak: aaa.example.test resolved, err=%v", err)
	}
	// And neither must "aaaaa.example.test" (longer).
	if _, err := s.Tenants.FindByHostname("aaaaa.example.test"); err != ErrTenantNotFound {
		t.Errorf("substring oracle leak: aaaaa.example.test resolved, err=%v", err)
	}
}

// ─── FindByID ────────────────────────────────────────────────────────────

func TestFindByID_Found(t *testing.T) {
	s := newTestStore(t)
	tenant := newTestTenant("byid", "pay.byid.test")
	if err := s.Tenants.Create(tenant); err != nil {
		t.Fatal(err)
	}

	got, err := s.Tenants.FindByID(tenant.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Name != "byid" {
		t.Errorf("name = %q, want byid", got.Name)
	}
	if len(got.Hostnames) != 1 || got.Hostnames[0] != "pay.byid.test" {
		t.Errorf("hostnames = %v", got.Hostnames)
	}
	// Verify JSON round-trip integrity.
	if got.Brand.PrimaryColor != "#0ea5e9" {
		t.Errorf("brand.primary_color lost in round-trip: %q", got.Brand.PrimaryColor)
	}
	if len(got.Providers) != 2 || got.Providers[0].Name != "square" {
		t.Errorf("providers round-trip: %+v", got.Providers)
	}
}

func TestFindByID_Missing(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Tenants.FindByID("does-not-exist"); err != ErrTenantNotFound {
		t.Errorf("err = %v, want ErrTenantNotFound", err)
	}
	if _, err := s.Tenants.FindByID(""); err != ErrTenantNotFound {
		t.Errorf("empty id err = %v, want ErrTenantNotFound", err)
	}
}

// ─── UpdateProviders ─────────────────────────────────────────────────────

func TestUpdateProviders_Replaces(t *testing.T) {
	s := newTestStore(t)
	tenant := newTestTenant("up", "pay.up.test")
	if err := s.Tenants.Create(tenant); err != nil {
		t.Fatal(err)
	}

	next := []Provider{
		{Name: "stripe", Enabled: true, KMSPath: "commerce/up/stripe"},
	}
	if err := s.Tenants.UpdateProviders(tenant.ID, next); err != nil {
		t.Fatalf("UpdateProviders: %v", err)
	}
	got, err := s.Tenants.FindByID(tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Providers) != 1 || got.Providers[0].Name != "stripe" {
		t.Errorf("providers after update = %+v", got.Providers)
	}
}

func TestUpdateProviders_MissingTenant(t *testing.T) {
	s := newTestStore(t)
	err := s.Tenants.UpdateProviders("no-such-id", []Provider{{Name: "x", Enabled: true}})
	if err != ErrTenantNotFound {
		t.Errorf("err = %v, want ErrTenantNotFound", err)
	}
}

// Concurrency model: last-write-wins. Two goroutines writing different
// provider lists MUST NOT corrupt the JSON column. We do not assert which
// writer wins — that is implementation-defined — only that the stored value
// is one of the two attempted payloads (never a merge, never a truncation).
func TestUpdateProviders_ConcurrencyLastWriteWins(t *testing.T) {
	s := newTestStore(t)
	tenant := newTestTenant("race", "pay.race.test")
	if err := s.Tenants.Create(tenant); err != nil {
		t.Fatal(err)
	}

	setA := []Provider{{Name: "a-only", Enabled: true, KMSPath: "kms/a"}}
	setB := []Provider{{Name: "b-only", Enabled: true, KMSPath: "kms/b"}}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = s.Tenants.UpdateProviders(tenant.ID, setA) }()
	go func() { defer wg.Done(); _ = s.Tenants.UpdateProviders(tenant.ID, setB) }()
	wg.Wait()

	got, err := s.Tenants.FindByID(tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Providers) != 1 {
		t.Fatalf("providers len = %d, want 1; got %+v", len(got.Providers), got.Providers)
	}
	if got.Providers[0].Name != "a-only" && got.Providers[0].Name != "b-only" {
		t.Errorf("corrupt provider after race: %+v", got.Providers)
	}
}

// ─── List ────────────────────────────────────────────────────────────────

func TestList_OrderedByName(t *testing.T) {
	s := newTestStore(t)
	for _, name := range []string{"charlie", "alpha", "bravo"} {
		if err := s.Tenants.Create(newTestTenant(name, "pay."+name+".test")); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Tenants.List(10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	want := []string{"alpha", "bravo", "charlie"}
	for i, w := range want {
		if got[i].Name != w {
			t.Errorf("List[%d] = %q, want %q", i, got[i].Name, w)
		}
	}
}

func TestList_LimitAndOffset(t *testing.T) {
	s := newTestStore(t)
	for _, name := range []string{"alpha", "bravo", "charlie", "delta"} {
		if err := s.Tenants.Create(newTestTenant(name, "pay."+name+".test")); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Tenants.List(2, 1)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Name != "bravo" || got[1].Name != "charlie" {
		t.Errorf("page = [%q, %q]", got[0].Name, got[1].Name)
	}
}

// ─── Regression: raw-column inspection ───────────────────────────────────

// Defensive check: the hostnames JSON column stores a canonical array, not
// a quoted string or a whitespace-padded encoding that could weaken the
// LIKE lookup. This is the single strongest anti-regression we have for the
// tenant-resolver security property.
func TestRawHostnamesStoredAsCanonicalArray(t *testing.T) {
	s := newTestStore(t)
	tenant := newTestTenant("canon", "pay.canon.test", "pay2.canon.test")
	if err := s.Tenants.Create(tenant); err != nil {
		t.Fatal(err)
	}
	rec, err := s.App.FindRecordById("commerce_tenants", tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	raw := rec.GetString("hostnames")
	var parsed []string
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("hostnames not valid JSON array: %q", raw)
	}
	if len(parsed) != 2 || parsed[0] != "pay.canon.test" || parsed[1] != "pay2.canon.test" {
		t.Errorf("hostnames canonical array = %v, want [pay.canon.test pay2.canon.test]", parsed)
	}
}

// ─── P8-C1 regression: hostname hijack MUST be rejected ──────────────────
//
// Red finding P8-C1 reproduction — two tenants claiming the same hostname
// must fail the second Create with ErrHostnameClaimed. The claim lives in
// commerce_tenant_hostnames (unique index on hostname) so the collision is
// caught at the SQL engine, not by an application-level check that races
// across replicas.

func TestRed_HostnameHijack_Rejected(t *testing.T) {
	s := newTestStore(t)

	victim := newTestTenant("victim", "pay.shared.test")
	if err := s.Tenants.Create(victim); err != nil {
		t.Fatalf("victim Create: %v", err)
	}

	attacker := newTestTenant("attacker", "pay.shared.test")
	err := s.Tenants.Create(attacker)
	if !errors.Is(err, ErrHostnameClaimed) {
		t.Fatalf("attacker Create err = %v, want ErrHostnameClaimed", err)
	}

	// The attacker tenant row MUST NOT survive the failed hostname insert —
	// the whole create runs in a transaction. If it did, deleting the
	// victim row later would let the dormant attacker row become the
	// resolver target (delayed hijack).
	owner, err := s.Tenants.FindByHostname("pay.shared.test")
	if err != nil || owner.Name != "victim" {
		t.Fatalf("FindByHostname post-hijack returned %+v err=%v, want victim", owner, err)
	}

	// Verify no dangling attacker tenant row exists by scanning for the
	// attacker name: unique-by-name is separate from unique-by-hostname,
	// so a rollback of the hostname insert MUST also roll back the tenant
	// row. If it did not, the second Create of "attacker" would 409.
	recs, _ := s.App.FindRecordsByFilter("commerce_tenants", "name = 'attacker'", "", 10, 0)
	if len(recs) != 0 {
		t.Fatalf("attacker tenant row leaked after failed hostname insert: %d rows", len(recs))
	}
}

// TestRed_HostnameSharingAcrossTenants — an attacker cannot even claim one
// hostname alongside a victim-owned hostname in the same POST; the
// transaction aborts at the colliding entry and no partial hostnames from
// the batch are committed.
func TestRed_HostnameSharingAcrossTenants(t *testing.T) {
	s := newTestStore(t)

	if err := s.Tenants.Create(newTestTenant("victim", "pay.shared.test", "a.victim.test")); err != nil {
		t.Fatal(err)
	}

	attacker := newTestTenant("attacker",
		"b.attacker.test",   // novel, would succeed in isolation
		"pay.shared.test",   // colliding with victim
		"c.attacker.test",   // novel, would succeed in isolation
	)
	err := s.Tenants.Create(attacker)
	if !errors.Is(err, ErrHostnameClaimed) {
		t.Fatalf("attacker Create err = %v, want ErrHostnameClaimed", err)
	}

	// None of the attacker's novel hostnames should have committed — the
	// tx aborts at the colliding entry.
	for _, h := range []string{"b.attacker.test", "c.attacker.test"} {
		if _, err := s.Tenants.FindByHostname(h); !errors.Is(err, ErrTenantNotFound) {
			t.Errorf("novel host %q committed under failed tx: err=%v", h, err)
		}
	}
}

// TestRed_HostnameHijack_RaceSafe — N goroutines race to claim the same
// hostname via distinct tenant Creates. The SQL unique index must ensure at
// most one commits; the others all return ErrHostnameClaimed (or
// ErrDuplicateTenant if the name check fires first). Zero partial commits.
//
// This test is the `-race` bar for P8-C1. Under -race -count=3 it must be
// green with no data races reported.
func TestRed_HostnameHijack_RaceSafe(t *testing.T) {
	const contenders = 8
	s := newTestStore(t)

	var wins, hostnameConflicts, nameConflicts, other int64

	var wg sync.WaitGroup
	wg.Add(contenders)
	for i := 0; i < contenders; i++ {
		name := "claimer" + string(rune('a'+i)) // unique names so only the hostname collides
		go func(n string) {
			defer wg.Done()
			err := s.Tenants.Create(newTestTenant(n, "pay.shared.test"))
			switch {
			case err == nil:
				atomic.AddInt64(&wins, 1)
			case errors.Is(err, ErrHostnameClaimed):
				atomic.AddInt64(&hostnameConflicts, 1)
			case errors.Is(err, ErrDuplicateTenant):
				atomic.AddInt64(&nameConflicts, 1)
			default:
				atomic.AddInt64(&other, 1)
				t.Errorf("unexpected err from contender %q: %v", n, err)
			}
		}(name)
	}
	wg.Wait()

	if wins != 1 {
		t.Fatalf("wins = %d, want exactly 1", wins)
	}
	if other != 0 {
		t.Fatalf("unexpected error kinds = %d, want 0", other)
	}
	if hostnameConflicts+nameConflicts != contenders-1 {
		t.Fatalf("losers = %d (hostname=%d name=%d), want %d",
			hostnameConflicts+nameConflicts, hostnameConflicts, nameConflicts, contenders-1)
	}

	owner, err := s.Tenants.FindByHostname("pay.shared.test")
	if err != nil {
		t.Fatalf("winner lookup err = %v", err)
	}
	if owner == nil || owner.Name == "" {
		t.Fatalf("winner tenant is nil/empty: %+v", owner)
	}
}

// ─── UpdateHostnames ────────────────────────────────────────────────────

func TestUpdateHostnames_HijackRejected(t *testing.T) {
	s := newTestStore(t)

	if err := s.Tenants.Create(newTestTenant("victim", "pay.shared.test")); err != nil {
		t.Fatal(err)
	}
	attacker := newTestTenant("attacker", "a.attacker.test")
	if err := s.Tenants.Create(attacker); err != nil {
		t.Fatal(err)
	}

	err := s.Tenants.UpdateHostnames(attacker.ID, []string{"pay.shared.test"})
	if !errors.Is(err, ErrHostnameClaimed) {
		t.Fatalf("UpdateHostnames hijack err = %v, want ErrHostnameClaimed", err)
	}

	// Attacker's original hostname MUST still be bound — the tx aborts
	// without dropping existing rows.
	owner, err := s.Tenants.FindByHostname("a.attacker.test")
	if err != nil || owner.Name != "attacker" {
		t.Fatalf("attacker lost its original hostname after failed hijack: %+v err=%v", owner, err)
	}
	owner, err = s.Tenants.FindByHostname("pay.shared.test")
	if err != nil || owner.Name != "victim" {
		t.Fatalf("victim lost ownership after failed hijack: %+v err=%v", owner, err)
	}
}

func TestUpdateHostnames_AddsAndRemoves(t *testing.T) {
	s := newTestStore(t)
	tenant := newTestTenant("mut", "old.mut.test")
	if err := s.Tenants.Create(tenant); err != nil {
		t.Fatal(err)
	}

	if err := s.Tenants.UpdateHostnames(tenant.ID, []string{"new.mut.test", "second.mut.test"}); err != nil {
		t.Fatalf("UpdateHostnames: %v", err)
	}

	if _, err := s.Tenants.FindByHostname("old.mut.test"); !errors.Is(err, ErrTenantNotFound) {
		t.Errorf("old hostname still bound after update: %v", err)
	}
	if owner, err := s.Tenants.FindByHostname("new.mut.test"); err != nil || owner.Name != "mut" {
		t.Errorf("new hostname not bound: %+v %v", owner, err)
	}
	if owner, err := s.Tenants.FindByHostname("second.mut.test"); err != nil || owner.Name != "mut" {
		t.Errorf("second hostname not bound: %+v %v", owner, err)
	}
}

// TestFindByHostname_AfterTenantDelete — deleting a tenant must cascade-delete
// its hostname rows so a dormant claim cannot be resurrected.
func TestFindByHostname_AfterTenantDelete(t *testing.T) {
	s := newTestStore(t)
	victim := newTestTenant("victim2", "pay2.shared.test")
	if err := s.Tenants.Create(victim); err != nil {
		t.Fatal(err)
	}

	rec, err := s.App.FindRecordById("commerce_tenants", victim.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.App.Delete(rec); err != nil {
		t.Fatalf("delete victim: %v", err)
	}

	if _, err := s.Tenants.FindByHostname("pay2.shared.test"); !errors.Is(err, ErrTenantNotFound) {
		t.Errorf("FindByHostname post-delete err = %v, want ErrTenantNotFound", err)
	}

	// A subsequent Create by a different tenant with the same hostname must
	// now succeed — the claim was freed by the cascade.
	if err := s.Tenants.Create(newTestTenant("successor", "pay2.shared.test")); err != nil {
		t.Fatalf("successor Create err = %v, want nil post-cascade", err)
	}
}
