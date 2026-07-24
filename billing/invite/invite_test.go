package invite

import (
	"errors"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/util/test/ae"
)

func newDB(t *testing.T) (*datastore.Datastore, func()) {
	t.Helper()
	ctx := ae.NewContext()
	return datastore.New(ctx), func() { ctx.Close() }
}

// TestMint_Idempotent proves minting the same code twice resolves to ONE row
// (deterministic id), regardless of casing/whitespace.
func TestMint_Idempotent(t *testing.T) {
	db, done := newDB(t)
	defer done()

	a, err := Mint(db, "welcome", "beta")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	b, err := Mint(db, "  WELCOME  ", "")
	if err != nil {
		t.Fatalf("re-mint: %v", err)
	}
	if a.Id() != b.Id() {
		t.Fatalf("re-mint minted a new row %q != %q, want idempotent", a.Id(), b.Id())
	}
	if b.Code != "WELCOME" {
		t.Fatalf("normalized code = %q, want WELCOME", b.Code)
	}
}

// TestRedeem_FirstTouch proves the first org to redeem a code claims it, a repeat
// by the SAME org is an idempotent replay, and a DIFFERENT org is refused.
func TestRedeem_FirstTouch(t *testing.T) {
	db, done := newDB(t)
	defer done()

	if _, err := Mint(db, "VIP", ""); err != nil {
		t.Fatalf("mint: %v", err)
	}

	inv, redeemed, err := Redeem(db, "vip", "acme")
	if err != nil || !redeemed || inv.Org != "acme" || !inv.Redeemed {
		t.Fatalf("first redeem = %+v redeemed:%v err=%v, want acme bound", inv, redeemed, err)
	}

	// Same org replays (no second claim, not an error).
	inv2, redeemed2, err := Redeem(db, "VIP", "acme")
	if err != nil || redeemed2 || inv2.Org != "acme" {
		t.Fatalf("replay = %+v redeemed:%v err=%v, want idempotent no-op", inv2, redeemed2, err)
	}

	// A different org is refused — first-touch wins.
	if _, _, err := Redeem(db, "VIP", "beta"); !errors.Is(err, ErrAlreadyRedeemed) {
		t.Fatalf("cross-org redeem err = %v, want ErrAlreadyRedeemed", err)
	}
}

// TestRedeem_UnknownCode proves an unminted code is rejected.
func TestRedeem_UnknownCode(t *testing.T) {
	db, done := newDB(t)
	defer done()

	if _, _, err := Redeem(db, "NOPE", "acme"); !errors.Is(err, ErrUnknownCode) {
		t.Fatalf("unknown-code redeem err = %v, want ErrUnknownCode", err)
	}
}

// TestOrgRedeemed proves the paywall lookup: true only for an org that actually
// redeemed a code.
func TestOrgRedeemed(t *testing.T) {
	db, done := newDB(t)
	defer done()

	if _, err := Mint(db, "GRANT", ""); err != nil {
		t.Fatalf("mint: %v", err)
	}
	if _, _, err := Redeem(db, "GRANT", "acme"); err != nil {
		t.Fatalf("redeem: %v", err)
	}

	if ok, err := OrgRedeemed(db, "acme"); err != nil || !ok {
		t.Fatalf("OrgRedeemed(acme) = %v err=%v, want true", ok, err)
	}
	if ok, err := OrgRedeemed(db, "beta"); err != nil || ok {
		t.Fatalf("OrgRedeemed(beta) = %v err=%v, want false", ok, err)
	}
}
