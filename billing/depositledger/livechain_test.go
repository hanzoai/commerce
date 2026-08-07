package depositledger

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/commerce/util/test/ae"
)

// A LIVE read-path probe. Everything else in this package runs against fakes,
// which is right for policy but cannot answer the one question that decides
// whether this rail may be armed: does it actually reach a chain and read a real
// contract correctly?
//
// It is skipped unless CRYPTO_DEPOSIT_* are present, so CI never depends on a
// public RPC. Drive it by hand before flipping cryptoDepositsCanBeCredited:
//
//	CRYPTO_DEPOSIT_RPC_BASE=https://mainnet.base.org \
//	CRYPTO_DEPOSIT_TOKEN_BASE_USDC=0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913 \
//	go test -count=1 -run TestLiveChain -v ./billing/depositledger/
//
// ⚠ -count=1 is not optional. Go caches a passing test even though its inputs
// came from the environment, so a second run with a DIFFERENT contract will
// happily replay the first one's result — it reports the old contract address
// and passes. That nearly recorded a cached success as live evidence.
//
// Reading is all it does. The gate that decides whether an address is ever
// handed out is elsewhere and stays shut; a scan of addresses holding nothing
// credits nothing.
func TestLiveChainReadPath(t *testing.T) {
	if liveEnvMissing() {
		t.Skip("no CRYPTO_DEPOSIT_RPC_* configured — this probe is opt-in")
	}
	c := ae.NewContext()
	_ = c // the watcher resolves watched addresses before it reads the chain

	svc, err := New(os.Environ())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !svc.Enabled() {
		t.Fatal("watcher disabled despite configured assets")
	}
	t.Logf("watching: %+v", svc.Status(context.Background()).Assets)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	n, err := svc.SyncOnce(ctx)
	if err != nil {
		// A symbol mismatch is the probe EARNING ITS KEEP, not a flake: it means
		// the configured contract is not the token it is labelled as. Verified
		// against Base by labelling USDT's contract "usdc" — the scan refused
		// with the on-chain symbol, which is also the proof that decimals are
		// read from the contract rather than assumed.
		if strings.Contains(err.Error(), "reports symbol") {
			t.Fatalf("CONTRACT MISLABELLED — do not arm this asset: %v", err)
		}
		t.Fatalf("SyncOnce against a live chain: %v", err)
	}
	t.Logf("scan completed, %d deposit(s) credited", n)
}

func liveEnvMissing() bool {
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "CRYPTO_DEPOSIT_RPC_") {
			return false
		}
	}
	return true
}
