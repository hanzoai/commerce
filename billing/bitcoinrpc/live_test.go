package bitcoinrpc

import (
	"context"
	"os"
	"testing"
	"time"
)

// A LIVE read against mainnet.
//
// The stubs prove the UTXO POLICY; only this proves the shape Esplora actually
// answers with, which is what a stub cannot get wrong on our behalf.
//
//	BTC_LIVE=1 go test ./billing/bitcoinrpc/ -run TestLive -v
func TestLiveReadsRealOutputs(t *testing.T) {
	if os.Getenv("BTC_LIVE") == "" {
		t.Skip("set BTC_LIVE=1 to read mainnet")
	}
	c := NewClient("https://blockstream.info/api")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tip, err := c.BlockNumber(ctx)
	if err != nil {
		t.Fatalf("BlockNumber: %v", err)
	}
	t.Logf("chain tip: %d", tip)
	if tip < 800_000 {
		t.Errorf("tip %d is implausibly low for mainnet", tip)
	}

	// A well-known, permanently funded address with a long history. Chosen
	// because it always has confirmed outputs, so a parse failure surfaces
	// rather than reading as "quiet".
	const known = "bc1qgdjqv0av3q56jvd82tkdjpy7gdp9ut8tlqmgrpmv24sq90ecnvqqjwvw97"
	got, err := c.TransfersTo(ctx, []string{known}, tip-50_000, tip)
	if err != nil {
		t.Fatalf("TransfersTo: %v", err)
	}
	t.Logf("outputs paying that address in the last 50k blocks: %d", len(got))
	for i, x := range got {
		if i >= 3 {
			break
		}
		t.Logf("  %s sats  tx=%s  vout=%d  height=%d", x.Units, x.TxHash[:16], x.EventIndex, x.Block)
		if x.Units.Sign() <= 0 {
			t.Errorf("credited a non-positive amount %s", x.Units)
		}
		if x.To != known {
			t.Errorf("To = %q, want the address as given", x.To)
		}
		if x.Block == 0 {
			t.Errorf("credited a transfer with no height")
		}
	}
}
