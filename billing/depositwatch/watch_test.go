package depositwatch

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/billing/husdindex"
	"github.com/hanzoai/commerce/models/cryptopaymentintent"
)

// ── fakes ───────────────────────────────────────────────────────────────────

type fakeReader struct {
	head      uint64
	transfers []husdindex.Transfer
	decimals  int
	symbol    string

	symbolErr error
	logsErr   error

	// observed call shapes, so the chunking guarantees are asserted rather than
	// assumed.
	maxAddrsSeen int
	maxRangeSeen uint64
	calls        int
}

func (r *fakeReader) BlockNumber(context.Context) (uint64, error) { return r.head, nil }
func (r *fakeReader) Decimals(context.Context) (int, error)       { return r.decimals, nil }
func (r *fakeReader) Symbol(context.Context) (string, error) {
	if r.symbolErr != nil {
		return "", r.symbolErr
	}
	return r.symbol, nil
}

func (r *fakeReader) TransfersTo(_ context.Context, addrs []string, from, to uint64) ([]husdindex.Transfer, error) {
	r.calls++
	if len(addrs) > r.maxAddrsSeen {
		r.maxAddrsSeen = len(addrs)
	}
	if span := to - from + 1; span > r.maxRangeSeen {
		r.maxRangeSeen = span
	}
	if r.logsErr != nil {
		return nil, r.logsErr
	}
	want := map[string]bool{}
	for _, a := range addrs {
		want[strings.ToLower(a)] = true
	}
	var out []husdindex.Transfer
	for _, t := range r.transfers {
		if t.Block >= from && t.Block <= to && want[strings.ToLower(t.To)] {
			out = append(out, t)
		}
	}
	return out, nil
}

// fakeStore APPENDS every credit without deduping. That is deliberate: a fake
// that silently collapses duplicates would make a double-credit bug invisible,
// which is the single most expensive bug this package can have. Here a second
// credit shows up as a second element, and the tests assert on the dedup KEYS —
// the thing production actually dedupes on.
type fakeStore struct {
	watched   []Watched
	credits   []Credit
	sights    []Sighting
	unsights  []Sighting
	creditErr error
}

func (s *fakeStore) Watched(context.Context, string, string) ([]Watched, error) {
	return s.watched, nil
}
func (s *fakeStore) Sight(_ context.Context, si Sighting) error {
	s.sights = append(s.sights, si)
	return nil
}
func (s *fakeStore) Unsight(_ context.Context, si Sighting) error {
	s.unsights = append(s.unsights, si)
	return nil
}
func (s *fakeStore) Credit(_ context.Context, c Credit) (bool, error) {
	if s.creditErr != nil {
		return false, s.creditErr
	}
	// Reports "written" the way the real ledger does — false for a key it has
	// already seen — so the pass count means NEW credits and a re-scan cannot
	// make the number climb.
	_, seen := s.distinctCredits()[c.DedupKey]
	s.credits = append(s.credits, c)
	return !seen, nil
}

// distinctCredits collapses the append-only credit log the way the production
// ledger's deterministic row id does, and reports how much money a customer
// would actually end up with.
func (s *fakeStore) distinctCredits() map[string]Credit {
	out := map[string]Credit{}
	for _, c := range s.credits {
		out[c.DedupKey] = c
	}
	return out
}

func (s *fakeStore) totalCents() int64 {
	var sum int64
	for _, c := range s.distinctCredits() {
		sum += c.AmountCents
	}
	return sum
}

type fakeCursor struct {
	last map[string]uint64
	err  error
}

func newCursor() *fakeCursor { return &fakeCursor{last: map[string]uint64{}} }

// running is the cursor of a deployment that has been up for a while — the
// normal case. A ZERO cursor is the cold start, which deliberately begins at the
// confirmation window and is covered by its own test.
func running(head uint64, keys ...string) *fakeCursor {
	c := newCursor()
	if len(keys) == 0 {
		keys = []string{"ethereum:usdc"}
	}
	for _, k := range keys {
		c.last[k] = head - 500
	}
	return c
}

func (c *fakeCursor) Last(_ context.Context, key string) (uint64, error) {
	return c.last[key], c.err
}
func (c *fakeCursor) Save(_ context.Context, key string, b uint64) error {
	if c.err != nil {
		return c.err
	}
	c.last[key] = b
	return nil
}

// ── fixtures ────────────────────────────────────────────────────────────────

const (
	// ethConfirmations must track the model's policy: this suite asserts on
	// concrete depths, so if the policy moves the fixtures must move with it.
	ethConfirmations = 12
	usdcContract     = "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48"
	// depositAddr is stored EIP-55 checksummed, exactly as the MPC custody
	// service returns it; the chain reports it lowercased.
	depositAddr      = "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"
	depositAddrLower = "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed"
)

func usdcAsset() Asset {
	return Asset{Chain: "ethereum", Token: "usdc", Contract: usdcContract, RPCURL: "http://rpc.test"}
}

func usdcReader(head uint64, transfers ...husdindex.Transfer) *fakeReader {
	return &fakeReader{head: head, transfers: transfers, decimals: 6, symbol: "USDC"}
}

func pendingIntent() Watched {
	return Watched{
		Org: "acme", Test: false, IntentID: "cpi_1", Subject: "acme/alice",
		Address: depositAddr, Status: cryptopaymentintent.Pending,
	}
}

// usdc returns a Transfer of n whole USDC (6 decimals).
func usdc(n int64, block uint64, txHash string, logIndex uint64) husdindex.Transfer {
	return husdindex.Transfer{
		To:       depositAddrLower,
		ValueWei: new(big.Int).Mul(big.NewInt(n), big.NewInt(1_000_000)),
		TxHash:   txHash,
		LogIndex: logIndex,
		Block:    block,
	}
}

func newWatcher(a Asset, r Reader, s Store, c Cursor) *Watcher {
	return New([]Bound{{Asset: a, Reader: r}}, s, c)
}

// ── exactly once ────────────────────────────────────────────────────────────

// A confirmed deposit credits the payer once, and NO number of re-scans,
// restarts or concurrent replicas can make it credit twice.
func TestSync_ConfirmedDepositCreditsExactlyOnce(t *testing.T) {
	head := uint64(1000)
	tx := usdc(25, head-ethConfirmations+1, "0xDEAD", 3) // depth == 12
	store := &fakeStore{watched: []Watched{pendingIntent()}}
	w := newWatcher(usdcAsset(), usdcReader(head, tx), store, running(head))

	n, err := w.Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if n != 1 {
		t.Fatalf("first pass credited %d transfers, want 1", n)
	}
	if got := store.totalCents(); got != 2500 {
		t.Fatalf("credited %d cents, want 2500 ($25 of 6-decimal USDC)", got)
	}

	// Re-scan the same window (a restart, a retry, a second tick). The window
	// deliberately overlaps: commit-behind guarantees it.
	for pass := 2; pass <= 4; pass++ {
		again, err := w.Sync(context.Background())
		if err != nil {
			t.Fatalf("pass %d: %v", pass, err)
		}
		if again != 0 {
			t.Fatalf("pass %d reported %d NEW credits for a deposit already credited — a counter that climbs on a re-scan reads exactly like a double credit", pass, again)
		}
	}
	if got := len(store.distinctCredits()); got != 1 {
		t.Fatalf("after 4 passes there are %d distinct credits, want 1 — the dedup key is not stable across passes", got)
	}
	if got := store.totalCents(); got != 2500 {
		t.Fatalf("after 4 passes the customer has %d cents, want 2500", got)
	}
	want := "ethereum:0xdead:3"
	if _, ok := store.distinctCredits()[want]; !ok {
		t.Fatalf("dedup key %q not produced; got %v", want, keysOf(store.distinctCredits()))
	}
}

// Two replicas scanning the same chain into the same ledger must produce the
// SAME row id, because that — not coordination — is what makes exactly-once
// hold without leader election.
func TestSync_ConcurrentReplicasProduceTheSameLedgerRow(t *testing.T) {
	head := uint64(1000)
	tx := usdc(10, head-30, "0xAbCdEf", 0)
	store := &fakeStore{watched: []Watched{pendingIntent()}}
	cursor := running(head)

	// Two independent Watcher values (two pods), one shared store and cursor.
	a := newWatcher(usdcAsset(), usdcReader(head, tx), store, cursor)
	b := newWatcher(usdcAsset(), usdcReader(head, tx), store, cursor)
	for _, w := range []*Watcher{a, b, a, b} {
		if _, err := w.Sync(context.Background()); err != nil {
			t.Fatalf("Sync: %v", err)
		}
	}
	if got := len(store.distinctCredits()); got != 1 {
		t.Fatalf("two replicas produced %d distinct ledger rows, want 1: %v", got, keysOf(store.distinctCredits()))
	}
	if got := store.totalCents(); got != 1000 {
		t.Fatalf("two replicas credited %d cents, want 1000", got)
	}
}

// The dedup key is chain-scoped: the same transaction hash on two chains is two
// deposits, not one. (A pre-EIP-155 transaction really can be replayed across
// EVM chains with an identical hash.)
func TestDedupKey_IsChainScoped(t *testing.T) {
	tr := husdindex.Transfer{TxHash: "0xfeed", LogIndex: 1}
	eth, base := dedupKey("ethereum", tr), dedupKey("base", tr)
	if eth == base {
		t.Fatalf("dedup key ignores the chain: %q == %q — one of two genuine deposits would be swallowed", eth, base)
	}
	if eth != "ethereum:0xfeed:1" {
		t.Fatalf("dedup key = %q, want ethereum:0xfeed:1", eth)
	}
}

// A second deposit to the same address is a second credit — the money is a
// property of the transfer, not of the intent's state machine, so an intent
// already marked succeeded must not swallow it.
func TestSync_SecondDepositToASettledAddressStillCredits(t *testing.T) {
	head := uint64(2000)
	first := usdc(5, head-40, "0x111", 0)
	second := usdc(7, head-20, "0x222", 0)

	settled := pendingIntent()
	settled.Status = cryptopaymentintent.Succeeded
	settled.TxHash = "0x111"
	settled.Block = head - 40

	store := &fakeStore{watched: []Watched{settled}}
	w := newWatcher(usdcAsset(), usdcReader(head, first, second), store, running(head))

	if _, err := w.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if got := len(store.distinctCredits()); got != 2 {
		t.Fatalf("got %d credits, want 2 — a repeat deposit to a settled address was lost: %v", got, keysOf(store.distinctCredits()))
	}
	if got := store.totalCents(); got != 1200 {
		t.Fatalf("credited %d cents, want 1200 ($5 + $7)", got)
	}
}

// ── confirmations ───────────────────────────────────────────────────────────

// A one-block sighting moves the intent's display state and NOTHING else.
func TestSync_ShallowSightingIsNotCredited(t *testing.T) {
	head := uint64(900)
	store := &fakeStore{watched: []Watched{pendingIntent()}}
	w := newWatcher(usdcAsset(), usdcReader(head, usdc(100, head, "0xnew", 0)), store, running(head))

	if _, err := w.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(store.credits) != 0 {
		t.Fatalf("credited %d transfers at depth 1; a 1-block sighting is not money", len(store.credits))
	}
	if len(store.sights) != 1 {
		t.Fatalf("recorded %d sightings, want 1", len(store.sights))
	}
	if store.sights[0].Confirmations != 1 {
		t.Fatalf("sighting recorded %d confirmations, want 1", store.sights[0].Confirmations)
	}
}

// The exact boundary: depth == required credits, depth == required-1 does not.
func TestSync_CreditsAtExactlyTheRequiredDepth(t *testing.T) {
	head := uint64(1000)
	for _, tc := range []struct {
		name       string
		block      uint64
		wantCredit bool
	}{
		{"one block short", head - ethConfirmations + 2, false}, // depth 11
		{"exactly at depth", head - ethConfirmations + 1, true}, // depth 12
		{"deeper", head - ethConfirmations, true},               // depth 13
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeStore{watched: []Watched{pendingIntent()}}
			w := newWatcher(usdcAsset(), usdcReader(head, usdc(1, tc.block, "0xa", 0)), store, running(head))
			if _, err := w.Sync(context.Background()); err != nil {
				t.Fatalf("Sync: %v", err)
			}
			if got := len(store.credits) > 0; got != tc.wantCredit {
				t.Fatalf("block %d (depth %d): credited=%v, want %v",
					tc.block, head-tc.block+1, got, tc.wantCredit)
			}
		})
	}
}

// The depth comes from the model, not from a private table in the scanner.
func TestSync_UsesTheModelsConfirmationPolicy(t *testing.T) {
	if cryptopaymentintent.RequiredConfirmationsForChain("ethereum") != ethConfirmations {
		t.Fatalf("fixture drift: the model no longer requires %d confirmations on ethereum", ethConfirmations)
	}
	// base requires 20; a deposit 15 deep on base must NOT credit even though the
	// same depth would credit on ethereum.
	head := uint64(1000)
	base := Asset{Chain: "base", Token: "usdc", Contract: usdcContract, RPCURL: "http://rpc.test"}
	store := &fakeStore{watched: []Watched{pendingIntent()}}
	w := newWatcher(base, usdcReader(head, usdc(1, head-14, "0xb", 0)), store, running(head, "base:usdc"))
	if _, err := w.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(store.credits) != 0 {
		t.Fatalf("credited a base deposit only 15 blocks deep; base requires %d",
			cryptopaymentintent.RequiredConfirmationsForChain("base"))
	}
}

// ── reorgs ──────────────────────────────────────────────────────────────────

// A sighting that leaves the canonical chain before it was ever credited must
// return the intent to pending — never leave a customer staring at "confirming"
// for a transaction that no longer exists.
func TestSync_ReorgedSightingIsReverted(t *testing.T) {
	head := uint64(1000)
	store := &fakeStore{watched: []Watched{pendingIntent()}}
	cursor := running(head)
	reader := usdcReader(head, usdc(50, head-2, "0xghost", 0))
	w := newWatcher(usdcAsset(), reader, store, cursor)

	if _, err := w.Sync(context.Background()); err != nil {
		t.Fatalf("first Sync: %v", err)
	}
	if len(store.sights) != 1 {
		t.Fatalf("expected the deposit to be sighted, got %d sightings", len(store.sights))
	}

	// The reorg: the transaction is gone from the canonical chain, and the intent
	// now carries the sighting.
	confirming := pendingIntent()
	confirming.Status = cryptopaymentintent.Confirming
	confirming.TxHash = "0xghost"
	confirming.Block = head - 2
	store.watched = []Watched{confirming}
	reader.transfers = nil
	reader.head = head + 3

	if _, err := w.Sync(context.Background()); err != nil {
		t.Fatalf("second Sync: %v", err)
	}
	if len(store.unsights) != 1 {
		t.Fatalf("a vanished transaction was not un-sighted (%d unsights) — the intent is stranded in confirming", len(store.unsights))
	}
	if len(store.credits) != 0 {
		t.Fatalf("credited %d transfers for a transaction that left the chain", len(store.credits))
	}
}

// A reorg that only MOVES the transaction (re-mined at a different height) must
// still credit, exactly once, under the same key.
func TestSync_ReorgThatMovesTheBlockStillCreditsOnce(t *testing.T) {
	head := uint64(1000)
	store := &fakeStore{watched: []Watched{pendingIntent()}}
	cursor := running(head)
	reader := usdcReader(head, usdc(9, head-1, "0xmoved", 0))
	w := newWatcher(usdcAsset(), reader, store, cursor)

	if _, err := w.Sync(context.Background()); err != nil { // sighted, too shallow
		t.Fatalf("first Sync: %v", err)
	}

	confirming := pendingIntent()
	confirming.Status = cryptopaymentintent.Confirming
	confirming.TxHash = "0xmoved"
	confirming.Block = head - 1
	store.watched = []Watched{confirming}

	// Re-mined two blocks earlier, and the chain has since moved on past the
	// confirmation depth.
	reader.transfers = []husdindex.Transfer{usdc(9, head-3, "0xmoved", 0)}
	reader.head = head + 20

	if _, err := w.Sync(context.Background()); err != nil {
		t.Fatalf("second Sync: %v", err)
	}
	if got := len(store.distinctCredits()); got != 1 {
		t.Fatalf("got %d credits after a block-moving reorg, want 1: %v", got, keysOf(store.distinctCredits()))
	}
	if got := store.totalCents(); got != 900 {
		t.Fatalf("credited %d cents, want 900", got)
	}
	if len(store.unsights) != 0 {
		t.Fatalf("un-sighted a transaction that was still on chain (%d) — a moved block is not a dropped one", len(store.unsights))
	}
}

// Silence about a block we never scanned is not evidence of a reorg.
func TestSync_DoesNotUnsightOutsideTheScannedRange(t *testing.T) {
	head := uint64(5000)
	confirming := pendingIntent()
	confirming.Status = cryptopaymentintent.Confirming
	confirming.TxHash = "0xold"
	confirming.Block = 10 // far below the cursor

	store := &fakeStore{watched: []Watched{confirming}}
	cursor := newCursor()
	cursor.last["ethereum:usdc"] = 4900
	w := newWatcher(usdcAsset(), usdcReader(head), store, cursor)

	if _, err := w.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(store.unsights) != 0 {
		t.Fatalf("un-sighted an intent whose block was never scanned (%d)", len(store.unsights))
	}
}

// ── amount and decimals ─────────────────────────────────────────────────────

func TestAmountCents(t *testing.T) {
	oneE := func(n int) *big.Int { return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil) }

	for _, tc := range []struct {
		name     string
		units    *big.Int
		decimals int
		peg      int64
		want     int64
		wantErr  bool
		dust     bool
	}{
		{name: "1 USDC at 6 decimals", units: oneE(6), decimals: 6, peg: 100, want: 100},
		{name: "1 USDT at 6 decimals", units: oneE(6), decimals: 6, peg: 100, want: 100},
		{name: "1 USDC at 18 decimals (BSC)", units: oneE(18), decimals: 18, peg: 100, want: 100},
		{name: "0.01 USDC is one cent", units: big.NewInt(10_000), decimals: 6, peg: 100, want: 1},
		{name: "0.009 USDC truncates down", units: big.NewInt(9_000), decimals: 6, peg: 100, dust: true},
		{name: "sub-cent dust", units: big.NewInt(1), decimals: 6, peg: 100, dust: true},
		{name: "zero", units: big.NewInt(0), decimals: 6, peg: 100, dust: true},
		{name: "1234.56 USDC", units: big.NewInt(1_234_560_000), decimals: 6, peg: 100, want: 123_456},
		{name: "negative refused", units: big.NewInt(-1), decimals: 6, peg: 100, wantErr: true},
		{name: "nil refused", units: nil, decimals: 6, peg: 100, wantErr: true},
		{name: "0 decimals refused", units: oneE(6), decimals: 0, peg: 100, wantErr: true},
		{name: "1 decimal refused", units: oneE(6), decimals: 1, peg: 100, wantErr: true},
		{name: "absurd decimals refused", units: oneE(6), decimals: 99, peg: 100, wantErr: true},
		{name: "unpegged token refused", units: oneE(6), decimals: 6, peg: 0, wantErr: true},
		{name: "overflowing amount refused", units: oneE(40), decimals: 6, peg: 100, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := AmountCents(tc.units, tc.decimals, tc.peg)
			switch {
			case tc.wantErr:
				if err == nil || errors.Is(err, ErrDust) {
					t.Fatalf("AmountCents = (%d, %v), want a hard error", got, err)
				}
			case tc.dust:
				if !errors.Is(err, ErrDust) {
					t.Fatalf("AmountCents = (%d, %v), want ErrDust", got, err)
				}
			default:
				if err != nil {
					t.Fatalf("AmountCents: %v", err)
				}
				if got != tc.want {
					t.Fatalf("AmountCents = %d, want %d", got, tc.want)
				}
			}
		})
	}
}

// The failure the whole verify() dance exists to prevent, stated as arithmetic:
// reading a 6-decimal token as 18 (or the reverse) is not a rounding error.
func TestAmountCents_WrongDecimalsIsCatastrophic(t *testing.T) {
	oneUSDC := big.NewInt(1_000_000) // $1.00 of 6-decimal USDC

	right, err := AmountCents(oneUSDC, 6, 100)
	if err != nil || right != 100 {
		t.Fatalf("AmountCents(1 USDC, 6) = (%d, %v), want 100", right, err)
	}
	// Read as 18 decimals: the customer is under-credited to nothing.
	if _, err := AmountCents(oneUSDC, 18, 100); !errors.Is(err, ErrDust) {
		t.Fatalf("1 USDC read at 18 decimals should be dust, got %v", err)
	}
	// An 18-decimal amount read as 6 decimals: 10^12 times too much.
	eighteen := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	wrong, err := AmountCents(eighteen, 6, 100)
	if err != nil {
		t.Fatalf("AmountCents: %v", err)
	}
	if wrong != 100_000_000_000_000 {
		t.Fatalf("sanity: %d", wrong)
	}
	if wrong/right != 1_000_000_000_000 {
		t.Fatalf("expected a 10^12 error, got %dx", wrong/right)
	}
	// …which is why decimals are never configured. See TestSync_RefusesAnAssetWhoseContractDisagrees.
}

func TestSync_DustIsNotCredited(t *testing.T) {
	head := uint64(1000)
	dust := husdindex.Transfer{
		To: depositAddrLower, ValueWei: big.NewInt(999), // 0.000999 USDC
		TxHash: "0xdust", LogIndex: 0, Block: head - 20,
	}
	store := &fakeStore{watched: []Watched{pendingIntent()}}
	w := newWatcher(usdcAsset(), usdcReader(head, dust), store, running(head))
	if _, err := w.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(store.credits) != 0 {
		t.Fatalf("credited %d cents of sub-cent dust", store.totalCents())
	}
}

// A contract that is not the token we were configured with disables the asset —
// no credits, and no cursor advance that would skip the blocks we refused to
// interpret.
func TestSync_RefusesAnAssetWhoseContractDisagrees(t *testing.T) {
	head := uint64(1000)
	reader := usdcReader(head, usdc(1000, head-30, "0xa", 0))
	reader.symbol = "DAI" // configured as usdc, the address actually holds DAI
	store := &fakeStore{watched: []Watched{pendingIntent()}}
	cursor := newCursor()
	w := newWatcher(usdcAsset(), reader, store, cursor)

	n, err := w.Sync(context.Background())
	if err == nil {
		t.Fatal("Sync accepted a contract whose symbol() disagrees with the config")
	}
	if n != 0 || len(store.credits) != 0 {
		t.Fatalf("credited %d transfers from an unverified contract", len(store.credits))
	}
	if got, _ := cursor.Last(context.Background(), "ethereum:usdc"); got != 0 {
		t.Fatalf("cursor advanced to %d for a refused asset — those blocks would never be re-read", got)
	}
	if !strings.Contains(err.Error(), "DAI") {
		t.Fatalf("error should name what the contract actually is: %v", err)
	}
}

func TestSync_RefusesAContractWithUnusableDecimals(t *testing.T) {
	head := uint64(1000)
	reader := usdcReader(head, usdc(1, head-30, "0xa", 0))
	reader.decimals = 0 // a contract that answered 0x for decimals(), or is not a token
	store := &fakeStore{watched: []Watched{pendingIntent()}}
	w := newWatcher(usdcAsset(), reader, store, newCursor())

	if _, err := w.Sync(context.Background()); err == nil {
		t.Fatal("Sync accepted a contract reporting 0 decimals — every amount would be credited 100x")
	}
	if len(store.credits) != 0 {
		t.Fatalf("credited %d transfers against unusable decimals", len(store.credits))
	}
}

// A transient failure reading the contract must not be cached as a permanent
// refusal: the next pass retries and credits.
func TestSync_RecoversAfterATransientVerifyFailure(t *testing.T) {
	head := uint64(1000)
	reader := usdcReader(head, usdc(3, head-30, "0xa", 0))
	reader.symbolErr = errors.New("rpc timeout")
	store := &fakeStore{watched: []Watched{pendingIntent()}}
	w := newWatcher(usdcAsset(), reader, store, running(head))

	if _, err := w.Sync(context.Background()); err == nil {
		t.Fatal("expected the pass to fail while the contract could not be read")
	}
	reader.symbolErr = nil
	if _, err := w.Sync(context.Background()); err != nil {
		t.Fatalf("second Sync: %v", err)
	}
	if got := store.totalCents(); got != 300 {
		t.Fatalf("credited %d cents after recovery, want 300", got)
	}
}

// ── address matching ────────────────────────────────────────────────────────

// The custody service returns EIP-55 checksummed addresses; chain logs are
// lowercase. Comparing them raw loses every deposit.
func TestSync_MatchesChecksummedAddressAgainstLowercaseLogs(t *testing.T) {
	head := uint64(1000)
	if depositAddr == depositAddrLower {
		t.Fatal("fixture is not actually mixed-case")
	}
	store := &fakeStore{watched: []Watched{pendingIntent()}} // stores the checksummed form
	w := newWatcher(usdcAsset(), usdcReader(head, usdc(42, head-30, "0xa", 0)), store, running(head))
	if _, err := w.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if got := store.totalCents(); got != 4200 {
		t.Fatalf("credited %d cents, want 4200 — the checksummed address did not match the lowercase log", got)
	}
}

// Money sent to an address we never minted credits nobody.
func TestSync_UnknownAddressIsNotCredited(t *testing.T) {
	head := uint64(1000)
	stranger := husdindex.Transfer{
		To: "0x000000000000000000000000000000000000dead", ValueWei: big.NewInt(5_000_000),
		TxHash: "0xstranger", LogIndex: 0, Block: head - 30,
	}
	store := &fakeStore{watched: []Watched{pendingIntent()}}
	w := newWatcher(usdcAsset(), usdcReader(head, stranger), store, running(head))
	if _, err := w.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(store.credits) != 0 {
		t.Fatalf("credited a transfer to an address no intent owns: %+v", store.credits)
	}
}

// If two intents claim one address we cannot say whose money it is, so the
// asset stops rather than guessing.
func TestSync_AmbiguousAddressStopsTheAsset(t *testing.T) {
	head := uint64(1000)
	a, b := pendingIntent(), pendingIntent()
	b.IntentID, b.Subject, b.Org = "cpi_2", "other/bob", "othercorp"
	store := &fakeStore{watched: []Watched{a, b}}
	cursor := newCursor()
	w := newWatcher(usdcAsset(), usdcReader(head, usdc(1, head-30, "0xa", 0)), store, cursor)

	if _, err := w.Sync(context.Background()); err == nil {
		t.Fatal("Sync credited a deposit to an address claimed by two intents")
	}
	if len(store.credits) != 0 {
		t.Fatalf("credited %d transfers on an ambiguous address", len(store.credits))
	}
	if got, _ := cursor.Last(context.Background(), "ethereum:usdc"); got != 0 {
		t.Fatalf("cursor advanced past blocks that were never interpreted (%d)", got)
	}
}

// The credit is addressed to the intent's own payer and org, never to a value
// derived from the transfer.
func TestSync_CreditsTheIntentsOwnPayer(t *testing.T) {
	head := uint64(1000)
	wt := pendingIntent()
	wt.Org, wt.Subject, wt.Test = "globex", "globex/carol", true
	store := &fakeStore{watched: []Watched{wt}}
	w := newWatcher(usdcAsset(), usdcReader(head, usdc(8, head-30, "0xa", 2)), store, running(head))
	if _, err := w.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(store.credits) != 1 {
		t.Fatalf("got %d credits, want 1", len(store.credits))
	}
	c := store.credits[0]
	if c.Org != "globex" || c.Subject != "globex/carol" || !c.Test {
		t.Fatalf("credit addressed to %+v, want org=globex subject=globex/carol test=true", c)
	}
	if c.IntentID != "cpi_1" || c.TxHash != "0xa" || c.LogIndex != 2 || c.Chain != "ethereum" || c.Token != "usdc" {
		t.Fatalf("credit lost its provenance: %+v", c)
	}
	if c.Units != "8000000" || c.PegRate != "1.00" {
		t.Fatalf("credit audit trail = units %q rate %q, want 8000000 / 1.00", c.Units, c.PegRate)
	}
}

// ── scanning ────────────────────────────────────────────────────────────────

func TestSync_CommitsBehindTheHead(t *testing.T) {
	head := uint64(10_000)
	store := &fakeStore{watched: []Watched{pendingIntent()}}
	cursor := newCursor()
	cursor.last["ethereum:usdc"] = head - 100
	w := newWatcher(usdcAsset(), usdcReader(head), store, cursor)

	if _, err := w.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	got, _ := cursor.Last(context.Background(), "ethereum:usdc")
	if want := head - ethConfirmations; got != want {
		t.Fatalf("cursor committed to %d, want %d (head − required) — committing to the head would make the reorg window unscannable", got, want)
	}
}

func TestSync_ChunksAddressesAndBlockRanges(t *testing.T) {
	head := uint64(20_000)
	var watched []Watched
	for i := 0; i < 250; i++ {
		wt := pendingIntent()
		wt.IntentID = fmt.Sprintf("cpi_%d", i)
		wt.Address = fmt.Sprintf("0x%040x", i+1)
		watched = append(watched, wt)
	}
	// The deposit we must still find, on the LAST address, in an early block.
	paid := watched[len(watched)-1]
	tr := husdindex.Transfer{
		To: strings.ToLower(paid.Address), ValueWei: big.NewInt(3_000_000),
		TxHash: "0xchunked", LogIndex: 0, Block: 6_000,
	}
	store := &fakeStore{watched: watched}
	reader := usdcReader(head, tr)
	cursor := newCursor()
	cursor.last["ethereum:usdc"] = 5_000
	w := newWatcher(usdcAsset(), reader, store, cursor)

	if _, err := w.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if reader.maxAddrsSeen > defaultMaxAddrs {
		t.Fatalf("asked for %d addresses in one getLogs call, cap is %d", reader.maxAddrsSeen, defaultMaxAddrs)
	}
	if reader.maxRangeSeen > defaultMaxRange {
		t.Fatalf("asked for a %d-block window, cap is %d", reader.maxRangeSeen, defaultMaxRange)
	}
	if got := store.totalCents(); got != 300 {
		t.Fatalf("credited %d cents, want 300 — chunking dropped a deposit", got)
	}
}

// An RPC failure on one chain must not stop another chain's deposits.
func TestSync_OneBrokenChainDoesNotBlindTheOthers(t *testing.T) {
	head := uint64(1000)
	good := usdcAsset()
	bad := Asset{Chain: "polygon", Token: "usdc", Contract: usdcContract, RPCURL: "http://rpc.test"}

	goodReader := usdcReader(head, usdc(4, head-30, "0xgood", 0))
	badReader := usdcReader(head)
	badReader.logsErr = errors.New("polygon rpc is down")

	store := &fakeStore{watched: []Watched{pendingIntent()}}
	w := New([]Bound{{Asset: bad, Reader: badReader}, {Asset: good, Reader: goodReader}}, store, running(head, "ethereum:usdc", "polygon:usdc"))

	n, err := w.Sync(context.Background())
	if err == nil {
		t.Fatal("expected the broken chain to be reported")
	}
	if n != 1 {
		t.Fatalf("credited %d transfers, want 1 — a broken chain blinded a healthy one", n)
	}
	if got := store.totalCents(); got != 400 {
		t.Fatalf("credited %d cents, want 400", got)
	}
	if !strings.Contains(err.Error(), "polygon") {
		t.Fatalf("error should name the failing asset: %v", err)
	}
}

// A cursor that cannot be read is not an empty cursor: refuse the pass rather
// than re-scan (or skip) an unknown range.
func TestSync_FailsClosedWhenTheCursorIsUnreadable(t *testing.T) {
	store := &fakeStore{watched: []Watched{pendingIntent()}}
	cursor := newCursor()
	cursor.err = errors.New("datastore unavailable")
	w := newWatcher(usdcAsset(), usdcReader(1000, usdc(1, 900, "0xa", 0)), store, cursor)

	if _, err := w.Sync(context.Background()); err == nil {
		t.Fatal("Sync proceeded without knowing where it left off")
	}
	if len(store.credits) != 0 {
		t.Fatalf("credited %d transfers with an unreadable cursor", len(store.credits))
	}
}

// A ledger that refuses the write must abort the pass with the cursor unmoved,
// so the deposit is retried instead of being scanned past.
func TestSync_DoesNotAdvanceThePastAFailedCredit(t *testing.T) {
	head := uint64(1000)
	store := &fakeStore{watched: []Watched{pendingIntent()}, creditErr: errors.New("ledger write failed")}
	cursor := newCursor()
	cursor.last["ethereum:usdc"] = head - 100
	w := newWatcher(usdcAsset(), usdcReader(head, usdc(1, head-50, "0xa", 0)), store, cursor)

	if _, err := w.Sync(context.Background()); err == nil {
		t.Fatal("expected the failing ledger write to fail the pass")
	}
	if got, _ := cursor.Last(context.Background(), "ethereum:usdc"); got != head-100 {
		t.Fatalf("cursor moved to %d despite a failed credit — that deposit would never be retried", got)
	}
}

func keysOf(m map[string]Credit) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// A first deploy starts at the bottom of the confirmation window and commits
// exactly what it scanned: no block is ever marked done without being read, and
// no public chain is cold-scanned from genesis to find deposits that could not
// exist yet.
func TestSync_ColdStartScansTheConfirmationWindowOnly(t *testing.T) {
	head := uint64(10_000)
	store := &fakeStore{watched: []Watched{pendingIntent()}}
	cursor := newCursor() // never run before
	reader := usdcReader(head)
	w := newWatcher(usdcAsset(), reader, store, cursor)

	if _, err := w.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if reader.calls != 1 {
		t.Fatalf("cold start made %d getLogs calls, want 1 — it is scanning history that cannot contain a deposit", reader.calls)
	}
	if reader.maxRangeSeen != ethConfirmations+1 {
		t.Fatalf("cold start scanned a %d-block window, want %d (the confirmation window)", reader.maxRangeSeen, ethConfirmations+1)
	}
	got, _ := cursor.Last(context.Background(), "ethereum:usdc")
	if want := head - ethConfirmations; got != want {
		t.Fatalf("cold start committed %d, want %d — it must not commit a block it never scanned", got, want)
	}
}
