package depositledger

import (
	"context"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/commerce/billing/bucket"
	"github.com/hanzoai/commerce/billing/depositwatch"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/cryptopaymentintent"
	"github.com/hanzoai/commerce/models/organization"
	txutil "github.com/hanzoai/commerce/models/transaction/util"
	"github.com/hanzoai/commerce/util/test/ae"
)

// These are REAL-datastore integration tests. They exist because the exactly-once
// guarantee is not a property of the watcher's logic — that is proven with fakes
// in billing/depositwatch — but of the STORAGE KEY meeting a backend that upserts
// on it. A fake ledger cannot prove that; only a real one can.

const (
	// The custody service hands back an EIP-55 checksummed address; chain logs
	// carry it lowercased. Both forms appear below on purpose.
	testAddr      = "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"
	testAddrLower = "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed"
	testContract  = "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48"
)

func testEnv() []string {
	return []string{
		"CRYPTO_DEPOSIT_RPC_ETHEREUM=http://rpc.invalid",
		"CRYPTO_DEPOSIT_TOKEN_ETHEREUM_USDC=" + testContract,
	}
}

// fakeChain is a chain that serves a fixed set of transfers.
type fakeChain struct {
	head      uint64
	transfers []depositwatch.Transfer
}

func (f *fakeChain) BlockNumber(context.Context) (uint64, error) { return f.head, nil }
func (f *fakeChain) Decimals(context.Context) (int, error)       { return 6, nil }
func (f *fakeChain) Symbol(context.Context) (string, error)      { return "USDC", nil }
func (f *fakeChain) TransfersTo(_ context.Context, addrs []string, from, to uint64) ([]depositwatch.Transfer, error) {
	want := map[string]bool{}
	for _, a := range addrs {
		want[strings.ToLower(a)] = true
	}
	var out []depositwatch.Transfer
	for _, t := range f.transfers {
		if t.Block >= from && t.Block <= to && want[strings.ToLower(t.To)] {
			out = append(out, t)
		}
	}
	return out, nil
}

func serviceOver(t *testing.T, chain *fakeChain) *Service {
	t.Helper()
	svc, err := New(testEnv(), WithReader(func(depositwatch.Asset) (depositwatch.Reader, error) { return chain, nil }))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !svc.Enabled() {
		t.Fatal("service is disabled with a configured asset")
	}
	return svc
}

// seedOrg persists a LIVE organization (TestMode false → the live ledger
// partition, which is where real money belongs).
func seedOrg(t *testing.T, name string) {
	t.Helper()
	org := organization.New(datastore.New(context.Background()))
	org.Name = name
	org.Live = true
	if err := org.Create(); err != nil {
		t.Fatalf("create org %s: %v", name, err)
	}
}

// seedIntent persists a minted deposit intent exactly as
// api/billing.CreateCryptoDeposit would.
func seedIntent(t *testing.T, org, subject, addr string) *cryptopaymentintent.CryptoPaymentIntent {
	t.Helper()
	db := orgDB(org)
	in := cryptopaymentintent.New(db)
	in.Currency = "usd"
	in.Chain = cryptopaymentintent.Ethereum
	in.Token = "usdc"
	in.DepositAddress = addr
	in.CustomerRef = subject
	in.Status = cryptopaymentintent.Pending
	in.ExpiresAt = time.Now().Add(24 * time.Hour)
	in.Defaults()
	if err := in.Create(); err != nil {
		t.Fatalf("create intent: %v", err)
	}
	return in
}

// balance reads the customer's spendable split the same way
// GET /v1/billing/me/balance does.
func balance(t *testing.T, org, subject string) bucket.Split {
	t.Helper()
	raw, err := txutil.GetRawByCurrency(orgDB(org).Context, subject, "iam-user", "usd", false)
	if err != nil {
		t.Fatalf("read balance: %v", err)
	}
	return bucket.Compute(raw, subject, time.Now())
}

// usdcTransfer is n cents' worth of 6-decimal USDC.
func usdcTransfer(cents int64, block uint64, txHash string, eventIndex uint64) depositwatch.Transfer {
	return depositwatch.Transfer{
		To:         testAddrLower,
		Units:      new(big.Int).Mul(big.NewInt(cents), big.NewInt(10_000)), // 1 cent = 10^4 base units
		TxHash:     txHash,
		EventIndex: eventIndex,
		Block:      block,
	}
}

// The whole point, end to end: money arrives on chain, the customer can spend
// it, and no amount of re-running credits it twice.
func TestCreditsAConfirmedDepositExactlyOnce(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	const org, subject = "acme", "acme/alice"
	seedOrg(t, org)
	intent := seedIntent(t, org, subject, testAddr)

	head := uint64(1000)
	chain := &fakeChain{head: head, transfers: []depositwatch.Transfer{
		usdcTransfer(2550, head-20, "0xfeedface", 4), // $25.50, 21 blocks deep
	}}
	svc := serviceOver(t, chain)
	if err := (cursorStore{}).Save(context.Background(), "ethereum:usdc", head-100); err != nil {
		t.Fatalf("seed cursor: %v", err)
	}

	n, err := svc.SyncOnce(context.Background())
	if err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}
	if n != 1 {
		t.Fatalf("credited %d deposits, want 1", n)
	}

	split := balance(t, org, subject)
	if int64(split.Balance) != 2550 {
		t.Fatalf("balance = %d cents, want 2550", split.Balance)
	}
	// Real money: it must land in Prepaid, the only bucket GPU spend may draw
	// from. A crypto deposit classified as a grant would silently downgrade what
	// the customer can buy with money they actually paid.
	if int64(split.PrepaidBalance) != 2550 || split.CreditsRemaining != 0 {
		t.Fatalf("deposit landed in the wrong bucket: prepaid=%d credits=%d, want 2550/0",
			split.PrepaidBalance, split.CreditsRemaining)
	}

	// Re-scan: restarts, retries, a second replica, the reorg-window overlap.
	for pass := 2; pass <= 5; pass++ {
		if _, err := svc.SyncOnce(context.Background()); err != nil {
			t.Fatalf("pass %d: %v", pass, err)
		}
		if got := int64(balance(t, org, subject).Balance); got != 2550 {
			t.Fatalf("after pass %d the balance is %d cents, want 2550 — the deposit was credited more than once", pass, got)
		}
	}

	// And a SECOND, independent service value (a second pod) over the same store.
	other := serviceOver(t, chain)
	if _, err := other.SyncOnce(context.Background()); err != nil {
		t.Fatalf("second replica: %v", err)
	}
	if got := int64(balance(t, org, subject).Balance); got != 2550 {
		t.Fatalf("a second replica took the balance to %d cents, want 2550", got)
	}

	// The intent tells the customer the same story the ledger does.
	reloaded := cryptopaymentintent.New(orgDB(org))
	if err := reloaded.GetById(orgDB(org).EncodeKey(intentKey(t, org))); err != nil {
		t.Fatalf("reload intent: %v", err)
	}
	if reloaded.Status != cryptopaymentintent.Succeeded {
		t.Fatalf("intent status = %s, want succeeded", reloaded.Status)
	}
	// The rate is recorded at the precision it was USED at, not rounded to cents:
	// a dollar peg is "1.00000000" in the same format a market rate arrives in,
	// so one field means one thing whichever way the asset was valued.
	if reloaded.TxHash != "0xfeedface" || reloaded.SettlementAmount != 2550 || reloaded.ExchangeRate != "1.00000000" {
		t.Fatalf("intent lost its settlement detail: tx=%s amount=%d rate=%s",
			reloaded.TxHash, reloaded.SettlementAmount, reloaded.ExchangeRate)
	}
	if reloaded.CryptoAmount != "25500000" {
		t.Fatalf("intent crypto amount = %q, want 25500000 base units", reloaded.CryptoAmount)
	}
	_ = intent
}

// intentKey finds the single intent in an org (the tests seed exactly one).
func intentKey(t *testing.T, org string) datastore.Key {
	t.Helper()
	db := orgDB(org)
	intents := make([]*cryptopaymentintent.CryptoPaymentIntent, 0)
	keys, err := cryptopaymentintent.Query(db).Filter("Chain=", "ethereum").GetAll(&intents)
	if err != nil {
		t.Fatalf("query intents: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected exactly 1 seeded intent in %s, found %d", org, len(keys))
	}
	return keys[0]
}

// A deposit shallower than the chain's confirmation depth moves the intent's
// display state and NOT the balance.
func TestUnconfirmedDepositMovesNoMoney(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	const org, subject = "shallow", "shallow/bob"
	seedOrg(t, org)
	seedIntent(t, org, subject, testAddr)

	head := uint64(2000)
	chain := &fakeChain{head: head, transfers: []depositwatch.Transfer{
		usdcTransfer(10_000, head-1, "0xshallow", 0), // 2 blocks deep; ethereum needs 12
	}}
	svc := serviceOver(t, chain)
	if err := (cursorStore{}).Save(context.Background(), "ethereum:usdc", head-100); err != nil {
		t.Fatalf("seed cursor: %v", err)
	}
	if _, err := svc.SyncOnce(context.Background()); err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}
	if got := int64(balance(t, org, subject).Balance); got != 0 {
		t.Fatalf("balance = %d cents on a 2-block sighting, want 0", got)
	}

	in := cryptopaymentintent.New(orgDB(org))
	if err := in.GetById(orgDB(org).EncodeKey(intentKey(t, org))); err != nil {
		t.Fatalf("reload intent: %v", err)
	}
	if in.Status != cryptopaymentintent.Confirming {
		t.Fatalf("intent status = %s, want confirming", in.Status)
	}
	if in.Confirmations != 2 {
		t.Fatalf("intent shows %d confirmations, want 2", in.Confirmations)
	}

	// The chain moves on; now it is deep enough and the money appears.
	chain.head = head + 20
	if _, err := svc.SyncOnce(context.Background()); err != nil {
		t.Fatalf("second SyncOnce: %v", err)
	}
	if got := int64(balance(t, org, subject).Balance); got != 10_000 {
		t.Fatalf("balance = %d cents after confirmation, want 10000", got)
	}
}

// A sighting whose transaction leaves the chain returns the intent to pending
// and never touches the ledger.
func TestReorgedSightingIsRolledBack(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	const org, subject = "reorg", "reorg/carol"
	seedOrg(t, org)
	seedIntent(t, org, subject, testAddr)

	head := uint64(3000)
	chain := &fakeChain{head: head, transfers: []depositwatch.Transfer{
		usdcTransfer(5000, head-3, "0xdoomed", 0),
	}}
	svc := serviceOver(t, chain)
	if err := (cursorStore{}).Save(context.Background(), "ethereum:usdc", head-50); err != nil {
		t.Fatalf("seed cursor: %v", err)
	}
	if _, err := svc.SyncOnce(context.Background()); err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}

	in := cryptopaymentintent.New(orgDB(org))
	if err := in.GetById(orgDB(org).EncodeKey(intentKey(t, org))); err != nil {
		t.Fatal(err)
	}
	if in.Status != cryptopaymentintent.Confirming {
		t.Fatalf("intent status = %s, want confirming", in.Status)
	}

	// The reorg drops it.
	chain.transfers = nil
	chain.head = head + 5
	if _, err := svc.SyncOnce(context.Background()); err != nil {
		t.Fatalf("second SyncOnce: %v", err)
	}

	in = cryptopaymentintent.New(orgDB(org))
	if err := in.GetById(orgDB(org).EncodeKey(intentKey(t, org))); err != nil {
		t.Fatal(err)
	}
	if in.Status != cryptopaymentintent.Pending {
		t.Fatalf("intent status = %s after a reorg, want pending — the customer is stranded on a transaction that no longer exists", in.Status)
	}
	if in.TxHash != "" || in.Confirmations != 0 {
		t.Fatalf("intent still carries the dropped sighting: tx=%q confirmations=%d", in.TxHash, in.Confirmations)
	}
	if got := int64(balance(t, org, subject).Balance); got != 0 {
		t.Fatalf("balance = %d cents for a transaction that left the chain, want 0", got)
	}
}

// Money to an address no intent owns credits nobody, and does not error.
func TestUnknownAddressCreditsNobody(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	const org, subject = "stranger", "stranger/dave"
	seedOrg(t, org)
	seedIntent(t, org, subject, testAddr)

	head := uint64(4000)
	stray := usdcTransfer(999_999, head-30, "0xstray", 0)
	stray.To = "0x000000000000000000000000000000000000dead"
	chain := &fakeChain{head: head, transfers: []depositwatch.Transfer{stray}}
	svc := serviceOver(t, chain)
	if err := (cursorStore{}).Save(context.Background(), "ethereum:usdc", head-100); err != nil {
		t.Fatalf("seed cursor: %v", err)
	}
	n, err := svc.SyncOnce(context.Background())
	if err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}
	if n != 0 {
		t.Fatalf("credited %d deposits for an address nobody owns", n)
	}
	if got := int64(balance(t, org, subject).Balance); got != 0 {
		t.Fatalf("balance = %d cents, want 0", got)
	}
}

// The cursor is what makes a restart resume instead of skipping. Persist it,
// read it back, and confirm a fresh Service sees the same position.
func TestCursorSurvivesAcrossServices(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	ctx := context.Background()
	if err := (cursorStore{}).Save(ctx, "ethereum:usdt", 123456); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := (cursorStore{}).Last(ctx, "ethereum:usdt")
	if err != nil {
		t.Fatalf("Last: %v", err)
	}
	if got != 123456 {
		t.Fatalf("cursor = %d, want 123456", got)
	}
	// A different asset is a different cursor — one chain's progress must never
	// mark another's blocks as scanned.
	if other, err := (cursorStore{}).Last(ctx, "base:usdc"); err != nil || other != 0 {
		t.Fatalf("base:usdc cursor = (%d, %v), want (0, nil)", other, err)
	}
}

// The ledger tag must classify as real money.
func TestDepositTag_IsRealMoney(t *testing.T) {
	if bucket.DepositKind(depositTag) != bucket.Prepaid {
		t.Fatalf("tag %q classifies as a non-cash grant; a paid deposit must be Prepaid (GPU-spendable)", depositTag)
	}
}

// The deterministic key is the exactly-once anchor: same event → same row id,
// different event → different row id.
func TestCreditKey_IsDeterministicPerOnChainEvent(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	db := orgDB("keys")
	a1 := creditKey(db, "ethereum:0xabc:0")
	a2 := creditKey(db, "ethereum:0xabc:0")
	if a1.StringID() != a2.StringID() {
		t.Fatalf("same event produced two row ids: %s vs %s", a1.StringID(), a2.StringID())
	}
	for _, other := range []string{"ethereum:0xabc:1", "base:0xabc:0", "ethereum:0xabd:0"} {
		if creditKey(db, other).StringID() == a1.StringID() {
			t.Fatalf("distinct event %q collided with ethereum:0xabc:0", other)
		}
	}
}

// An unconfigured deploy is inert.
func TestNew_UnconfiguredIsDisabled(t *testing.T) {
	svc, err := New([]string{"PATH=/usr/bin"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if svc.Enabled() {
		t.Fatal("an unconfigured deploy is watching something")
	}
	if n, err := svc.SyncOnce(context.Background()); n != 0 || err != nil {
		t.Fatalf("SyncOnce on a disabled service = (%d, %v), want (0, nil)", n, err)
	}
	svc.Start() // must be a no-op, not a panic
	svc.Stop()
}

// An incoherent config is fatal, not a quiet partial watch.
func TestNew_RefusesIncoherentConfig(t *testing.T) {
	if _, err := New([]string{"CRYPTO_DEPOSIT_TOKEN_BASE_USDC=" + testContract}); err == nil {
		t.Fatal("accepted a token with no RPC endpoint — those deposits would never be seen")
	}
}

// The schedule actually runs: nothing calls SyncOnce here.
func TestStart_CreditsWithoutAnyCaller(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	const org, subject = "scheduled", "scheduled/erin"
	seedOrg(t, org)
	seedIntent(t, org, subject, testAddr)

	head := uint64(6000)
	chain := &fakeChain{head: head, transfers: []depositwatch.Transfer{
		usdcTransfer(1234, head-30, "0xscheduled", 0),
	}}
	svc, err := New(testEnv(),
		WithReader(func(depositwatch.Asset) (depositwatch.Reader, error) { return chain, nil }),
		WithInterval(5*time.Millisecond))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := (cursorStore{}).Save(context.Background(), "ethereum:usdc", head-100); err != nil {
		t.Fatalf("seed cursor: %v", err)
	}

	svc.Start()
	defer svc.Stop()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if int64(balance(t, org, subject).Balance) == 1234 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("the scheduled watcher never credited the deposit (balance %d cents) — a rail nobody triggers is the bug this replaces",
		balance(t, org, subject).Balance)
}

// Stop must actually stop, and must be safe to call twice.
func TestStopIsIdempotent(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	svc, err := New(testEnv(),
		WithReader(func(depositwatch.Asset) (depositwatch.Reader, error) { return &fakeChain{head: 10}, nil }),
		WithInterval(time.Millisecond))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	svc.Start()
	svc.Start() // second Start must not spawn a second loop
	svc.Stop()
	svc.Stop()
}
