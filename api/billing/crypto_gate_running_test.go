package billing

import (
	"context"
	"testing"

	"github.com/hanzoai/commerce/billing/depositledger"
	"github.com/hanzoai/commerce/billing/depositwatch"
)

// installWatcher installs a process-wide watcher for the duration of one test
// and restores whatever was there before.
//
// It uses the SAME accessor Bootstrap uses (depositledger.SetDefault), so what
// is proven here is the real wiring rather than a parallel one built for the
// test. That the package-level default is directly settable is exactly what
// makes the ambient state testable — the seam is the setter.
func installWatcher(t *testing.T, environ []string, start bool) *depositledger.Service {
	t.Helper()
	svc, err := depositledger.New(environ,
		depositledger.WithStore(gateStore{}),
		depositledger.WithCursor(gateCursor{}),
		depositledger.WithReader(func(depositwatch.Asset) (depositwatch.Reader, error) {
			return gateReader{}, nil
		}),
	)
	if err != nil {
		t.Fatalf("depositledger.New: %v", err)
	}
	prev := depositledger.Default()
	depositledger.SetDefault(svc)
	t.Cleanup(func() { svc.Stop(); depositledger.SetDefault(prev) })
	if start {
		svc.Start()
	}
	return svc
}

func baseUSDCEnv() []string {
	return []string{
		"CRYPTO_DEPOSIT_RPC_BASE=https://mainnet.base.org",
		"CRYPTO_DEPOSIT_TOKEN_BASE_USDC=0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913",
	}
}

// TestGateClosedUntilTheWatcherRuns is the invariant the whole rail rests on,
// stated against the ambient accessor the handlers actually read.
//
// The dangerous state is the middle one: an asset is CONFIGURED and the schedule
// is NOT running. A gate that asked "is this configured?" would answer yes there
// and mint a real custody address that nothing on earth will ever look at —
// money received, never credited, which is the exact defect this rail exists to
// end. It must answer no.
func TestGateClosedUntilTheWatcherRuns(t *testing.T) {
	t.Run("configured but not started: CLOSED", func(t *testing.T) {
		svc := installWatcher(t, baseUSDCEnv(), false)
		if !svc.Enabled() {
			t.Fatal("precondition: the asset is configured")
		}
		if got := watchedAssets(); len(got) != 0 {
			t.Fatalf("a configured-but-unstarted watcher must offer NOTHING, got %+v", got)
		}
		if creditable(watchedAssets(), "base", "usdc") {
			t.Fatal("the mint gate must refuse base/usdc while nothing is reading the chain")
		}
	})

	t.Run("configured and started: OPEN", func(t *testing.T) {
		installWatcher(t, baseUSDCEnv(), true)
		if got := watchedAssets(); len(got) != 1 {
			t.Fatalf("a running watcher must offer its assets, got %+v", got)
		}
		if !creditable(watchedAssets(), "base", "usdc") {
			t.Fatal("base/usdc must be creditable while the watcher runs")
		}
		if creditable(watchedAssets(), "polygon", "usdc") {
			t.Fatal("an unconfigured chain stays refused even while the watcher runs")
		}
	})

	t.Run("stopped again: CLOSED", func(t *testing.T) {
		svc := installWatcher(t, baseUSDCEnv(), true)
		if !creditable(watchedAssets(), "base", "usdc") {
			t.Fatal("precondition: open while running")
		}
		svc.Stop()
		if got := watchedAssets(); len(got) != 0 {
			t.Fatalf("stopping the schedule must close the rail, got %+v", got)
		}
		if creditable(watchedAssets(), "base", "usdc") {
			t.Fatal("a stopped watcher must refuse — the chain is no longer being read")
		}
	})
}

// TestPickerAndMintAgreeOnTheRunningWatcher: GetCryptoOptions and
// CreateCryptoDeposit read the same accessor, so a buyer can never be shown an
// asset the mint path would refuse. That must hold in the not-running state too,
// which is the state this change introduced.
func TestPickerAndMintAgreeOnTheRunningWatcher(t *testing.T) {
	mintable := []string{"base", "ethereum", "polygon"}

	installWatcher(t, baseUSDCEnv(), false)
	chains, tokens := offeredFrom(watchedAssets(), mintable)
	if len(chains) != 0 || len(tokens) != 0 {
		t.Fatalf("an unstarted watcher must offer no chains and no tokens, got %v / %v", chains, tokens)
	}

	installWatcher(t, baseUSDCEnv(), true)
	chains, _ = offeredFrom(watchedAssets(), mintable)
	if len(chains) != 1 || chains[0] != "base" {
		t.Fatalf("a running watcher must offer base, got %v", chains)
	}
	for _, ch := range chains {
		if !creditable(watchedAssets(), ch, "usdc") {
			t.Fatalf("the picker offered %q but the mint gate refuses it", ch)
		}
	}
}

// TestPooledAddressRequiresARunningWatcher: the pooled address is handed to a
// payer as a destination. Handing one out while nothing scans that chain is the
// pooled-chain spelling of the same lost-money defect, so it must follow the
// same gate.
func TestPooledAddressRequiresARunningWatcher(t *testing.T) {
	env := []string{
		"CRYPTO_DEPOSIT_RPC_XRPL=https://xrplcluster.com",
		"CRYPTO_DEPOSIT_TOKEN_XRPL_RLUSD=RLUSD.rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De",
		"CRYPTO_DEPOSIT_ADDRESS_XRPL=rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De",
	}

	installWatcher(t, env, false)
	if addr := pooledAddressFor(watchedAssets(), "xrpl", "rlusd"); addr != "" {
		t.Fatalf("an unstarted watcher must hand out no pooled address, got %q", addr)
	}

	installWatcher(t, env, true)
	if addr := pooledAddressFor(watchedAssets(), "xrpl", "rlusd"); addr == "" {
		t.Fatal("a running watcher must hand out the configured pooled account")
	}
	if addr := pooledAddressFor(watchedAssets(), "xrpl", "usdc"); addr != "" {
		t.Fatalf("an unwatched token on a pooled chain must yield no address, got %q", addr)
	}
}

// --- stubs: no chain, no database ---

type gateReader struct{}

func (gateReader) BlockNumber(ctx context.Context) (uint64, error) { return 1, nil }
func (gateReader) TransfersTo(ctx context.Context, _ []string, _, _ uint64) ([]depositwatch.Transfer, error) {
	return nil, nil
}
func (gateReader) Decimals(ctx context.Context) (int, error) { return 6, nil }
func (gateReader) Symbol(ctx context.Context) (string, error) { return "USDC", nil }

type gateStore struct{}

func (gateStore) Watched(ctx context.Context, _, _ string) ([]depositwatch.Watched, error) {
	return nil, nil
}
func (gateStore) Sight(ctx context.Context, _ depositwatch.Sighting) error   { return nil }
func (gateStore) Unsight(ctx context.Context, _ depositwatch.Sighting) error { return nil }
func (gateStore) Credit(ctx context.Context, _ depositwatch.Credit) (bool, error) {
	return false, nil
}
func (gateStore) RecordUnattributed(ctx context.Context, _ depositwatch.Unattributed) error {
	return nil
}

type gateCursor struct{}

func (gateCursor) Last(ctx context.Context, _ string) (uint64, error) { return 0, nil }
func (gateCursor) Save(ctx context.Context, _ string, _ uint64) error { return nil }
