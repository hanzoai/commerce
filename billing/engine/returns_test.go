package engine

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/db"
	"github.com/hanzoai/commerce/models/billinginvoice"
)

// replicas opens one store file twice, each with its own connections and its own
// write lock, as two cloud replicas sharing one store would: nothing in either
// process serializes the other's writes.
func replicas(t *testing.T) (*datastore.Datastore, *datastore.Datastore) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "books.db")
	open := func() *datastore.Datastore {
		d, err := db.NewSQLiteDB(&db.SQLiteDBConfig{Path: path, Config: db.DefaultConfig().SQLite, TenantID: "acme", TenantType: "org"})
		if err != nil {
			t.Fatalf("open %s: %v", path, err)
		}
		t.Cleanup(func() { _ = d.Close() })
		return datastore.NewWithDB(context.Background(), d)
	}
	return open(), open()
}

// Refunds and dispute reports for one invoice, delivered at once to two replicas
// over one store, every refund delivered twice: each refund lands once, none is
// lost to another, and the dispute ends at its latest report.
func TestReturnsAcrossTwoReplicasAreNeitherLostNorDoubled(t *testing.T) {
	a, b := replicas(t)
	inv := billinginvoice.New(a)
	inv.UserId, inv.AmountPaid = "acme", 100_000
	if err := inv.Create(); err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	id := inv.Id()

	const refunds = 24
	var wg sync.WaitGroup
	errs := make(chan error, 4*refunds)
	for i := range refunds {
		for _, d := range []*datastore.Datastore{a, b} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs <- RecordRefund(d, id, fmt.Sprintf("re_%02d", i), int64(100+i))
			}()
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			d := a
			if i%2 == 1 {
				d = b
			}
			errs <- RecordDispute(d, id, "UNDER_REVIEW", int64(i+1), time.Time{})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("record: %v", err)
		}
	}

	got := billinginvoice.New(b)
	if err := got.GetById(id); err != nil {
		t.Fatalf("read invoice: %v", err)
	}
	var want int64
	for i := range refunds {
		ref := fmt.Sprintf("refund:re_%02d", i)
		if got.Refunds[ref] != int64(100+i) {
			t.Errorf("refund %s holds %d, want %d", ref, got.Refunds[ref], 100+i)
		}
		want += int64(100 + i)
	}
	if len(got.Refunds) != refunds || got.Refunded() != want {
		t.Errorf("%d refunds totalling %d, want %d totalling %d", len(got.Refunds), got.Refunded(), refunds, want)
	}
	if got.DisputeVersion != refunds {
		t.Errorf("dispute at version %d, want the latest, %d", got.DisputeVersion, refunds)
	}
}

// A swap writes only over the entity the caller read: a stale read writes nothing,
// and an entity never stored is written once.
func TestASwapWritesOnlyOverWhatWasRead(t *testing.T) {
	a, b := replicas(t)
	first := billinginvoice.New(a)
	first.UserId = "acme"
	if ok, err := first.Swap(); err != nil || !ok {
		t.Fatalf("swap in a new invoice: %v %v", ok, err)
	}
	stale := billinginvoice.New(b)
	if err := stale.GetById(first.Id()); err != nil {
		t.Fatalf("read: %v", err)
	}
	fresh := billinginvoice.New(b)
	if err := fresh.GetById(first.Id()); err != nil {
		t.Fatalf("read: %v", err)
	}
	fresh.CustomerEmail = "fresh@acme.test"
	if ok, err := fresh.Swap(); err != nil || !ok {
		t.Fatalf("swap over the stored invoice: %v %v", ok, err)
	}
	stale.CustomerEmail = "stale@acme.test"
	if ok, err := stale.Swap(); err != nil || ok {
		t.Fatalf("a stale read swapped: %v %v", ok, err)
	}
	got := billinginvoice.New(a)
	if err := got.GetById(first.Id()); err != nil || got.CustomerEmail != "fresh@acme.test" {
		t.Fatalf("invoice reads %q (%v), want the fresh write kept", got.CustomerEmail, err)
	}
}
