package billing

import (
	"testing"
	"time"

	"github.com/hanzoai/commerce/models/pendingintent"
	"github.com/hanzoai/commerce/util/test/ae"
)

// The reconciler's population: intents recorded and not yet settled, once their
// grace has passed — and only those.
func TestPendingIntentsOpenUntilSettled(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	for _, id := range []string{"pi_open_a", "pi_open_b"} {
		if err := pendingintent.Put(id, "hanzo", "hanzo/alice", pendingintent.Topup, 2500, "usd", true); err != nil {
			t.Fatal(err)
		}
	}
	has := func(id string, rows []*pendingintent.Intent) bool {
		for _, r := range rows {
			if r.Id() == id {
				return true
			}
		}
		return false
	}
	future := time.Now().Add(time.Hour)
	rows, err := pendingintent.Open(future)
	if err != nil || !has("pi_open_a", rows) || !has("pi_open_b", rows) {
		t.Fatalf("open intents = %d (%v); want both", len(rows), err)
	}
	if rows, _ := pendingintent.Open(time.Now().Add(-time.Hour)); has("pi_open_a", rows) {
		t.Fatal("an intent inside its grace is open to the reconciler")
	}
	if won, err := pendingintent.Settle("pi_open_a"); err != nil || !won {
		t.Fatalf("settle = %v %v", won, err)
	}
	if won, _ := pendingintent.Settle("pi_open_a"); won {
		t.Fatal("an intent was settled twice")
	}
	if rows, _ := pendingintent.Open(future); has("pi_open_a", rows) || !has("pi_open_b", rows) {
		t.Fatal("a settled intent is still open, or an open one is not")
	}
}

// What the vault wrote about a saved card is not the caller's to change: the
// customer and token a charge is routed by, and the fingerprint the per-card limits
// count. The caller's own extras still merge.
func TestMergeExtrasLeavesTheVaultsFacts(t *testing.T) {
	held := map[string]interface{}{"customerId": "cus_1", "cardId": "pm_1", "fingerprint": "fp_1", "country": "US", "nickname": "work"}
	got := mergeExtras(held, map[string]interface{}{
		"customerId": "cus_attacker", "cardId": "pm_attacker", "fingerprint": "fp_other", "country": "FR",
		"squareCustomerId": "x", "nickname": "personal", "note": "n",
	})
	for k, want := range map[string]string{"customerId": "cus_1", "cardId": "pm_1", "fingerprint": "fp_1", "country": "US", "nickname": "personal", "note": "n"} {
		if got[k] != want {
			t.Fatalf("%s = %v; want %s", k, got[k], want)
		}
	}
	if _, ok := got["squareCustomerId"]; ok {
		t.Fatal("a caller wrote a vault key")
	}
	if held["nickname"] != "work" {
		t.Fatal("the held metadata was changed in place")
	}
}
