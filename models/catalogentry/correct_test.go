package catalogentry

import (
	"testing"

	"github.com/hanzoai/commerce/util/test/ae"
)

// A row born wrong stays wrong forever unless something reads the snapshot back
// over it. That is the whole defect this file pins: production's rows were
// created once, years of snapshots later they still carried their birth address,
// and 31 of 84 products pointed at a route the fleet answers 404 for.
func TestCorrect_MovesTheAddressAndNothingElse(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	if _, err := Seed(db); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Age the store the way production aged: one row keeps a stale address, and a
	// human has since edited what humans edit.
	e := New(db)
	if ok, err := e.Query().Filter("Slug=", "wallet").Get(); err != nil || !ok {
		t.Fatalf("wallet row: ok=%v err=%v", ok, err)
	}
	want := e.ApiPath
	e.ApiPath = "/v1/wallet"
	e.ApiRoute = "api.hanzo.ai/v1/wallet"
	e.Name = "Wallets, renamed in the CMS"
	e.Order = 4242
	e.Status = StatusExternal
	if err := e.Update(); err != nil {
		t.Fatalf("age the row: %v", err)
	}

	corrected, err := Correct(db)
	if err != nil {
		t.Fatalf("correct: %v", err)
	}
	if corrected != 1 {
		t.Fatalf("corrected %d rows, want exactly the 1 that had drifted", corrected)
	}

	got := New(db)
	if ok, err := got.Query().Filter("Slug=", "wallet").Get(); err != nil || !ok {
		t.Fatalf("reread wallet: ok=%v err=%v", ok, err)
	}
	if got.ApiPath != want {
		t.Errorf("apiPath = %q, want %q — the address is the fleet's fact and the snapshot is where it is declared", got.ApiPath, want)
	}
	if got.ApiRoute != "api.hanzo.ai"+want {
		t.Errorf("apiRoute = %q, want the host-qualified spelling of %q — left to drift, a reader cannot tell which one lies", got.ApiRoute, want)
	}
	if got.Name != "Wallets, renamed in the CMS" || got.Order != 4242 || got.Status != StatusExternal {
		t.Errorf("name/order/status = %q/%d/%q — a correction that overwrites a human's merchandising is a deploy that undoes admin.hanzo.ai",
			got.Name, got.Order, got.Status)
	}

	// Twice is once. A boot that keeps writing is a boot that keeps churning the
	// store for nothing, and the count is what says so.
	again, err := Correct(db)
	if err != nil {
		t.Fatalf("re-correct: %v", err)
	}
	if again != 0 {
		t.Fatalf("re-correct wrote %d rows, want 0", again)
	}
}

// Correction is not creation. Seed owns birth, and a snapshot row for a product
// this store never had must not quietly add a product to somebody's catalog.
func TestCorrect_CreatesNothing(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	corrected, err := Correct(db)
	if err != nil {
		t.Fatalf("correct: %v", err)
	}
	if corrected != 0 {
		t.Fatalf("corrected %d rows in an empty store, want 0", corrected)
	}
	n, err := Query(db).Count()
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("store holds %d rows after Correct on an empty store, want 0 — birth is Seed's job", n)
	}
}
