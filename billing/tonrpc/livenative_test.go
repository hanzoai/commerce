package tonrpc

import (
	"context"
	"os"
	"testing"
	"time"
)

// A LIVE read of native TON against a real, busy mainnet account.
//
// The stubs prove the DISCRIMINATOR; only this proves the shape the index
// actually answers with — which is what a stub cannot get wrong on our behalf.
//
//	TON_LIVE=1 go test ./billing/tonrpc/ -run TestLiveNative -v
func TestLiveNativeReadsRealTransfers(t *testing.T) {
	if os.Getenv("TON_LIVE") == "" {
		t.Skip("set TON_LIVE=1 to read the real chain")
	}
	n := NewNative("https://toncenter-mainnet.tac.build/api/v3")
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	head, err := n.BlockNumber(ctx)
	if err != nil {
		t.Fatalf("BlockNumber: %v", err)
	}
	t.Logf("masterchain head: %d", head)
	if d, err := n.Decimals(ctx); err != nil || d != 9 {
		t.Errorf("Decimals = (%d, %v), want 9", d, err)
	}
	if s, err := n.Symbol(ctx); err != nil || s != "TON" {
		t.Errorf("Symbol = (%q, %v), want TON", s, err)
	}

	// A REAL account, in raw form, taken from the chain itself rather than
	// invented — the first address I made up failed its CRC, which is the
	// parser working and the test author not.
	const busy = "0:9EEADAD8F1E53E2C3B0276AB8FCABDE006984C2CD430B0CEC0B8F2035067183C"
	got, err := n.TransfersTo(ctx, []string{busy}, head-400, head)
	if err != nil {
		t.Fatalf("TransfersTo: %v", err)
	}
	t.Logf("native transfers in the last 2000 masterchain blocks: %d", len(got))
	for i, x := range got {
		if i >= 3 {
			break
		}
		t.Logf("  %s nanotons  tx=%s  block=%d", x.Units, x.TxHash[:16], x.Block)
		if x.Units.Sign() <= 0 {
			t.Errorf("credited a non-positive amount %s", x.Units)
		}
		if x.To != busy {
			t.Errorf("To = %q, want the owner string as given", x.To)
		}
	}
}
