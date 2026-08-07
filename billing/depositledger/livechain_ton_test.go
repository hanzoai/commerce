package depositledger

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/commerce/billing/depositwatch"
	"github.com/hanzoai/commerce/billing/tonrpc"
)

// A LIVE read-path probe for TON, and the sibling of TestLiveChainReadPath,
// TestLiveSolanaReadPath and TestLiveXRPLReadPath.
//
// Skipped unless CRYPTO_DEPOSIT_RPC_TON is set, so CI never depends on a public
// endpoint. Drive it by hand:
//
//	CRYPTO_DEPOSIT_RPC_TON=https://toncenter.com/api/v3 \
//	CRYPTO_DEPOSIT_TOKEN_TON_USDT=EQCxE6mUtQJKFnGfaROTKOt1lZbDiiX1kCixRv7Nw2Id_sDs \
//	go test -count=1 -run TestLiveTON -v ./billing/depositledger/
//
// ⚠ -count=1 is not optional, for the same reason it is not optional on every
// other probe here: Go caches a PASS even though every input came from the
// environment, so a second run against a DIFFERENT jetton master replays the
// first one's result and reports the old master's decimals as live evidence.
//
// ⚠ The public toncenter endpoint allows about ONE request per second without
// an API key, and this probe makes several in a row. A 429 is reported as
// INCONCLUSIVE rather than as an answer about the chain — "the endpoint would
// not talk to us" and "the chain says no" must never look the same.
func TestLiveTONReadPath(t *testing.T) {
	rpc := strings.TrimSpace(os.Getenv("CRYPTO_DEPOSIT_RPC_TON"))
	if rpc == "" {
		t.Skip("no CRYPTO_DEPOSIT_RPC_TON configured — this probe is opt-in")
	}
	asset := liveAsset(t, depositwatch.FamilyTON)
	if asset.Contract == "" {
		t.Skip("CRYPTO_DEPOSIT_RPC_TON is set but no CRYPTO_DEPOSIT_TOKEN_TON_* — nothing to read")
	}
	t.Logf("jetton master %s @ %s", asset.Contract, asset.RPCURL)

	master, err := tonrpc.ParseAddress(asset.Contract)
	if err != nil {
		t.Fatalf("master: %v", err)
	}
	t.Logf("master canonicalised to %s", master)
	c := tonrpc.NewClient(asset.RPCURL, master)

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	// 1. The index is reachable and the masterchain seqno is a real, advancing
	// position. THIS is the scan window: TON is sharded and has no global block
	// height, and the masterchain seqno is the one sequence every transaction
	// enters exactly once.
	head, err := c.BlockNumber(ctx)
	if isThrottled(err) {
		t.Skipf("INCONCLUSIVE — %s throttled us on the first call: %v", asset.RPCURL, err)
	}
	if err != nil {
		t.Fatalf("masterchainInfo: %v", err)
	}
	if head == 0 {
		t.Fatal("masterchain seqno is 0")
	}
	t.Logf("masterchain seqno: %d", head)

	// 2. THE assertion this probe exists for: decimals come off the jetton
	// master's ON-CHAIN content dictionary — not from a constant, and not from
	// the off-chain JSON its content URI points at. If this is 6 for USDT, the
	// scale in every credit came from the chain.
	pace()
	decimals, err := c.Decimals(ctx)
	if isThrottled(err) {
		t.Skipf("INCONCLUSIVE — throttled: %v", err)
	}
	if err != nil {
		t.Fatalf("reading decimals off the jetton master's on-chain content: %v", err)
	}
	if decimals < 2 || decimals > 36 {
		t.Fatalf("master reports %d decimals, which cannot express a cent", decimals)
	}
	t.Logf("jetton decimals (read from the master's ON-CHAIN content): %d", decimals)

	// 3. Identity. A jetton that does not say ON CHAIN what it is cannot be
	// armed, and USDT on TON is exactly that case — it publishes only decimals
	// and a content URI, and the ticker in that off-chain document is "USD₮".
	// Both outcomes are asserted, because both are correct answers about a real
	// token and neither may be silently tolerated.
	// (No HTTP: the master's content was cached by the Decimals call above.)
	symbol, symErr := c.Symbol(ctx)
	switch {
	case symErr != nil:
		t.Logf("jetton publishes NO on-chain symbol: %v", symErr)
		t.Log("THIS IS THE CORRECT OUTCOME for USDT on TON — do not arm it. See tonrpc.Client.Symbol.")
	default:
		t.Logf("jetton on-chain symbol: %q", symbol)
		if !strings.EqualFold(strings.TrimSpace(symbol), asset.Token) {
			t.Fatalf("MASTER MISLABELLED — do not arm this asset: %s reports symbol %q but is configured as %q",
				asset.Contract, symbol, asset.Token)
		}
	}

	// 4. A REAL credit, read through the same code a pass uses.
	//
	// A recent transfer of this jetton is found independently — from the index's
	// SENDER-side jetton_transfers view, which production deliberately never
	// credits from — and its destination owner is then read back through the
	// RECEIVER-side path the client actually uses. Agreement between the two
	// means the client is watching the side where the money lands.
	pace()
	owner, ok := findLiveTONRecipient(ctx, t, asset.RPCURL, master)
	if !ok {
		t.Log("scan INCONCLUSIVE — no recent transfer of this jetton was found to read back")
		return
	}
	t.Logf("reading back the receiving side for owner %s", owner)

	from := head - 2000
	if head < 2000 {
		from = 0
	}
	// Retried, because the public endpoint allows about one request per second
	// and one scan makes several. A retry costs nothing: reads are idempotent,
	// and the client caches the owner→wallet resolution, so each attempt makes
	// strictly fewer calls than the last.
	var transfers []depositwatch.Transfer
	for attempt := 0; ; attempt++ {
		pace()
		transfers, err = c.TransfersTo(ctx, []string{owner}, from, head)
		if !isThrottled(err) {
			break
		}
		if attempt >= 5 {
			t.Logf("scan INCONCLUSIVE — %s kept throttling us: %v", asset.RPCURL, err)
			t.Log("the read path reached the index (head, decimals and identity are confirmed above); re-run against a dedicated endpoint or with an API key to exercise the scan")
			return
		}
	}
	if err != nil {
		t.Fatalf("TransfersTo over masterchain blocks %d..%d: %v", from, head, err)
	}
	t.Logf("scanned masterchain blocks %d..%d: %d inbound transfer(s)", from, head, len(transfers))
	for _, tr := range transfers {
		if tr.To != owner {
			t.Fatalf("a transfer came back addressed to %q, not the owner we asked about (%q)", tr.To, owner)
		}
		if tr.Units == nil || tr.Units.Sign() <= 0 {
			t.Fatalf("a transfer carries a non-positive amount: %+v", tr)
		}
		if tr.Block < from || tr.Block > head {
			t.Fatalf("a transfer came back at masterchain block %d, outside the window %d..%d", tr.Block, from, head)
		}
		if len(tr.TxHash) != 64 {
			t.Fatalf("transaction id %q is not canonical lowercase hex — the dedup key would depend on the endpoint's rendering", tr.TxHash)
		}
		t.Logf("  %s units to %s in %s at masterchain block %d (event %d)", tr.Units, tr.To, tr.TxHash, tr.Block, tr.EventIndex)
	}
	if len(transfers) == 0 {
		t.Log("the window held no inbound transfer for this owner; the resolution and scan path still executed without error")
	}

	// 5. And the whole watcher, wired exactly as production wires it. When the
	// jetton could not identify itself on chain, the ONLY acceptable outcome is
	// a refusal — an unidentifiable token must credit nothing, and asserting it
	// here proves the refusal survives all the way out to the scheduled pass.
	svc, err := New(os.Environ())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !svc.Enabled() {
		t.Fatal("watcher disabled despite configured assets")
	}
	t.Logf("watching: %s", svc.Describe())
	for attempt := 0; ; attempt++ {
		pace()
		n, serr := svc.SyncOnce(ctx)
		if isThrottled(serr) {
			if attempt >= 5 {
				t.Logf("full-watcher check INCONCLUSIVE — throttled: %v", serr)
				return
			}
			continue
		}
		if symErr != nil {
			if serr == nil {
				t.Fatalf("the watcher scanned an unidentifiable jetton and credited %d deposit(s) — it must refuse", n)
			}
			if !strings.Contains(serr.Error(), "symbol") {
				t.Fatalf("the watcher refused for a different reason than identity: %v", serr)
			}
			t.Logf("✓ the watcher refuses this asset end-to-end: %v", serr)
			return
		}
		if serr != nil {
			t.Fatalf("SyncOnce against a live index: %v", serr)
		}
		t.Logf("scan completed, %d deposit(s) credited", n)
		return
	}
}

// pace waits out the public endpoint's rate limit between calls. It is a
// property of the PROBE and of the free tier, not of the rail: a production
// deploy needs a dedicated or keyed endpoint, exactly as the Solana probe
// concludes for the same reason.
func pace() { time.Sleep(1500 * time.Millisecond) }

// findLiveTONRecipient asks the index for a recent transfer of this jetton and
// returns the OWNER it was sent to.
//
// It reads /jetton/transfers — the SENDER-side view that production
// deliberately never credits from, because a record there means a transfer was
// SENT and not that it arrived. Using it only to pick a subject keeps the two
// paths independent.
func findLiveTONRecipient(ctx context.Context, t *testing.T, baseURL string, master tonrpc.Address) (string, bool) {
	t.Helper()
	u, err := url.Parse(strings.TrimRight(baseURL, "/") + "/jetton/transfers")
	if err != nil {
		t.Fatalf("url: %v", err)
	}
	q := u.Query()
	q.Set("jetton_master", master.String())
	q.Set("limit", "1")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("jetton/transfers: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Logf("jetton/transfers answered HTTP %d; skipping the read-back", resp.StatusCode)
		return "", false
	}
	var out struct {
		JettonTransfers []struct {
			Destination  string `json:"destination"`
			JettonMaster string `json:"jetton_master"`
		} `json:"jetton_transfers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, tr := range out.JettonTransfers {
		// Verify rather than trust the filter: several of this API's query
		// parameters are silently ignored rather than rejected.
		if got, err := tonrpc.ParseAddress(tr.JettonMaster); err != nil || got != master {
			continue
		}
		if tr.Destination == "" {
			continue
		}
		return tr.Destination, true
	}
	return "", false
}
