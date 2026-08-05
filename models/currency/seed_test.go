package currency

import (
	"context"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/store"
	"github.com/hanzoai/commerce/util/test/ae"
)

// defaultDB returns a datastore in the default (global) namespace — the same
// namespace the currency reference table (DefaultNamespace=true) lives in.
func defaultDB(c context.Context) *datastore.Datastore {
	return datastore.New(c)
}

// TestSeed_IdempotentAndComplete proves the first seed creates every common
// currency and a re-seed creates none (idempotent, non-destructive).
func TestSeed_IdempotentAndComplete(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := defaultDB(c)

	created, err := Seed(db)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if created != len(commonCurrencies) {
		t.Fatalf("first seed created %d, want %d (all common currencies)", created, len(commonCurrencies))
	}

	created2, err := Seed(db)
	if err != nil {
		t.Fatalf("re-seed: %v", err)
	}
	if created2 != 0 {
		t.Fatalf("re-seed created %d, want 0 (idempotent)", created2)
	}

	n, err := Query(db).Count()
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != len(commonCurrencies) {
		t.Fatalf("row count = %d, want %d", n, len(commonCurrencies))
	}
}

// TestSeedIfEmpty_GatesOnCount proves SeedIfEmpty seeds an empty table then
// no-ops once any row exists.
func TestSeedIfEmpty_GatesOnCount(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := defaultDB(c)

	created, err := SeedIfEmpty(db)
	if err != nil {
		t.Fatalf("seed-if-empty: %v", err)
	}
	if created != len(commonCurrencies) {
		t.Fatalf("seed-if-empty created %d, want %d", created, len(commonCurrencies))
	}

	created2, err := SeedIfEmpty(db)
	if err != nil {
		t.Fatalf("seed-if-empty again: %v", err)
	}
	if created2 != 0 {
		t.Fatalf("seed-if-empty on populated table created %d, want 0", created2)
	}
}

// TestSeededFields proves the seeded rows carry correct symbol/decimal data for
// both a two-decimal (usd) and a zero-decimal (jpy) currency, table-driven.
func TestSeededFields(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := defaultDB(c)
	if _, err := Seed(db); err != nil {
		t.Fatalf("seed: %v", err)
	}

	cases := []struct {
		code    string
		symbol  string
		decimal int
	}{
		{"usd", "$", 2},
		{"eur", "€", 2},
		{"jpy", "¥", 0},
		{"krw", "₩", 0},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			cur := New(db)
			ok, err := cur.Query().Filter("Code=", tc.code).Get()
			if err != nil {
				t.Fatalf("lookup %s: %v", tc.code, err)
			}
			if !ok {
				t.Fatalf("currency %s not seeded", tc.code)
			}
			if cur.Symbol != tc.symbol {
				t.Fatalf("%s symbol = %q, want %q", tc.code, cur.Symbol, tc.symbol)
			}
			if cur.DecimalDigits != tc.decimal {
				t.Fatalf("%s decimalDigits = %d, want %d", tc.code, cur.DecimalDigits, tc.decimal)
			}
		})
	}
}

// TestStoreReferencesSeededCurrency proves a store's Currency code resolves to a
// currency that exists in the seeded reference table — the reference-integrity
// contract the pickers rely on.
func TestStoreReferencesSeededCurrency(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := defaultDB(c)
	if _, err := Seed(db); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// A store created in an org namespace picks "usd" as its currency.
	sdb := datastore.New(c)
	s := store.New(sdb)
	s.Name = "acme-store"
	s.Currency = "usd"
	if err := s.Create(); err != nil {
		t.Fatalf("create store: %v", err)
	}

	// That currency code must exist in the reference table.
	cur := New(db)
	ok, err := cur.Query().Filter("Code=", string(s.Currency)).Get()
	if err != nil {
		t.Fatalf("resolve store currency: %v", err)
	}
	if !ok {
		t.Fatalf("store currency %q does not exist in the reference table", s.Currency)
	}
	if cur.Code != string(s.Currency) {
		t.Fatalf("resolved currency code = %q, want %q", cur.Code, s.Currency)
	}
}
