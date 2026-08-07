package commerce

import (
	"testing"

	"github.com/hanzoai/commerce/billing/depositledger"
)

// TestBootstrapDoesNotStartTheDepositWatcher pins the separation this change
// exists for: Bootstrap BUILDS the app, it does not RUN it.
//
// The watcher was the one background schedule that broke that rule, and the
// breakage was not academic. cmd/grant and cmd/backfill-events are one-shot
// tools that call Bootstrap purely for the DB wiring, do work, and exit without
// ever calling Shutdown — so each of them started a chain scan that writes
// ledger credits and then died in the middle of a pass. Neither wants a money
// mover; neither asked for one.
//
// The asset table here is deliberately NON-EMPTY, because an empty one would
// make this vacuous: the watcher would be inert for want of configuration
// rather than because Bootstrap declined to start it. base/usdc is what the
// live deployment is actually configured with.
func TestBootstrapDoesNotStartTheDepositWatcher(t *testing.T) {
	t.Setenv("CRYPTO_DEPOSIT_RPC_BASE", "https://mainnet.base.org")
	t.Setenv("CRYPTO_DEPOSIT_TOKEN_BASE_USDC", "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913")

	bootTestCommerce(t)

	svc := depositledger.Default()
	if svc == nil {
		t.Fatal("Bootstrap must install the process-wide watcher so the mint gate has something to ask")
	}
	if !svc.Enabled() {
		t.Fatal("precondition: base/usdc is configured, so the watcher is Enabled")
	}
	if svc.Running() {
		t.Fatal("Bootstrap must NOT start the deposit schedule — a one-shot cmd/ tool that calls Bootstrap would otherwise run a money-moving chain scan and exit mid-pass")
	}
}

// TestBootstrapFailsClosedOnAnIncoherentWatchTable: a token with no endpoint is
// money nobody is watching. That must stop the boot rather than silently watch
// less than the operator asked for — and moving Start out of Bootstrap must not
// have moved this with it, since the table is still PARSED at Bootstrap.
func TestBootstrapFailsClosedOnAnIncoherentWatchTable(t *testing.T) {
	// A configured token whose chain has no CRYPTO_DEPOSIT_RPC_* endpoint.
	t.Setenv("CRYPTO_DEPOSIT_TOKEN_BASE_USDC", "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913")
	t.Setenv("CRYPTO_DEPOSIT_RPC_BASE", "")

	dir := t.TempDir()
	cfg := &Config{
		DataDir: dir + "/data", Secret: "test-secret",
		HTTPAddr: "127.0.0.1:0", QueryTimeout: 30e9,
	}
	cfg.IAM.Enabled = false
	cfg.KMS.Enabled = false
	t.Setenv("STRIPE_SECRET_KEY", "")
	t.Setenv("COMMERCE_STRIPE_SEED", "false")
	t.Setenv("SQL_URL", "")
	t.Setenv("COMMERCE_DATA_DIR", cfg.DataDir)
	t.Setenv("COMMERCE_BASE_URL", "")

	app := NewWithConfig(cfg)
	t.Cleanup(func() { _ = app.Shutdown() })
	if err := app.Bootstrap(); err == nil {
		t.Fatal("Bootstrap must refuse a token configured on a chain with no endpoint — that token is money nobody is watching")
	}
}
