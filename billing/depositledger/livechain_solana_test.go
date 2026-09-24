package depositledger

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/commerce/billing/depositwatch"
	"github.com/hanzoai/commerce/billing/solanarpc"
)

// A LIVE read-path probe for Solana, and the sibling of TestLiveChainReadPath.
//
// Everything else about this rail runs against fakes, which is right for policy
// and useless for the one question that decides whether Solana may be armed:
// does the reader actually reach the cluster, derive the right account, and take
// the token's scale from the chain rather than from us?
//
// Skipped unless CRYPTO_DEPOSIT_RPC_SOLANA is set, so CI never depends on a
// public RPC. Drive it by hand:
//
//	CRYPTO_DEPOSIT_RPC_SOLANA=https://api.mainnet-beta.solana.com \
//	CRYPTO_DEPOSIT_TOKEN_SOLANA_USDC=EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v \
//	go test -count=1 -run TestLiveSolana -v ./billing/depositledger/
//
// ⚠ -count=1 is not optional, for the same reason it is not optional on the EVM
// probe: Go caches a PASS even though every input came from the environment, so
// a second run against a DIFFERENT mint replays the first one's result and
// reports the old mint's decimals as live evidence.
func TestLiveSolanaReadPath(t *testing.T) {
	rpc := strings.TrimSpace(os.Getenv("CRYPTO_DEPOSIT_RPC_SOLANA"))
	if rpc == "" {
		t.Skip("no CRYPTO_DEPOSIT_RPC_SOLANA configured — this probe is opt-in")
	}
	assets, err := depositwatch.AssetsFromEnv(os.Environ())
	if err != nil {
		t.Fatalf("AssetsFromEnv: %v", err)
	}
	var asset depositwatch.Asset
	for _, a := range assets {
		if a.Family() == depositwatch.FamilySolana {
			asset = a
		}
	}
	if asset.Contract == "" {
		t.Skip("CRYPTO_DEPOSIT_RPC_SOLANA is set but no CRYPTO_DEPOSIT_TOKEN_SOLANA_* — nothing to read")
	}
	t.Logf("mint %s @ %s", asset.Contract, asset.RPCURL)

	mint, err := solanarpc.ParsePublicKey(asset.Contract)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	c := solanarpc.NewClient(asset.RPCURL, mint)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// 1. The cluster is reachable and finalized is a real, advancing position.
	head, err := c.BlockNumber(ctx)
	if err != nil {
		t.Fatalf("getSlot(finalized): %v", err)
	}
	if head == 0 {
		t.Fatal("finalized slot is 0")
	}
	t.Logf("finalized slot: %d", head)

	// 2. THE assertion this probe exists for: decimals come off the mint
	// account. If this is 6 for USDC, the scale in every credit came from the
	// chain and not from a constant anybody could mistype.
	decimals, err := c.Decimals(ctx)
	if err != nil {
		t.Fatalf("reading decimals off the mint account: %v", err)
	}
	if decimals < 2 || decimals > 36 {
		t.Fatalf("mint reports %d decimals, which cannot express a cent", decimals)
	}
	t.Logf("mint decimals (read from the mint account): %d", decimals)

	// 3. The token identifies itself. A symbol that disagrees with the label is
	// the probe EARNING ITS KEEP — it means the configured mint is not the
	// token it was configured as, and the asset must not be armed.
	symbol, err := c.Symbol(ctx)
	if err != nil {
		t.Fatalf("reading the on-chain symbol: %v", err)
	}
	t.Logf("mint symbol (Metaplex metadata): %q", symbol)
	if !strings.EqualFold(strings.TrimSpace(symbol), asset.Token) {
		t.Fatalf("MINT MISLABELLED — do not arm this asset: %s reports symbol %q but is configured as %q",
			asset.Contract, symbol, asset.Token)
	}

	// 4. THE OTHER assertion this probe exists for: our ATA derivation is what
	// the cluster itself calls this owner's token account.
	//
	// An SPL transfer lands in the ATA, never at the address a customer is
	// shown, so a derivation that is merely self-consistent watches an account
	// nobody funds — and every deposit is invisible while everything looks
	// healthy. The node is asked independently, with getTokenAccountsByOwner,
	// and must agree.
	checked := 0
	for _, owner := range probeOwners {
		accounts := tokenAccountsByOwner(ctx, t, asset.RPCURL, owner, asset.Contract)
		if len(accounts) == 0 {
			t.Logf("  %s holds no %s token account right now; skipping", owner, strings.ToUpper(asset.Token))
			continue
		}
		ownerKey, err := solanarpc.ParsePublicKey(owner)
		if err != nil {
			t.Fatalf("probe owner: %v", err)
		}
		ata, err := solanarpc.AssociatedTokenAddress(ownerKey, solanarpc.TokenProgramID, mint)
		if err != nil {
			t.Fatalf("AssociatedTokenAddress: %v", err)
		}
		if !contains(accounts, ata.String()) {
			t.Fatalf("DERIVATION DISAGREES WITH THE CLUSTER — do not arm this asset: we would watch %s for owner %s, the node says %v",
				ata, owner, accounts)
		}
		t.Logf("  ATA(%s) = %s — confirmed by the cluster", owner, ata)
		checked++
	}
	if checked == 0 {
		t.Fatal("no probe owner held a token account, so the ATA derivation was never checked against the cluster")
	}

	// 5. A real scan over a real window, through the same code a pass uses.
	// Whatever comes back must be addressed to the OWNER we asked about and
	// carry a positive amount; an empty window is a fine answer and not a
	// failure, because we do not control what those addresses receive.
	//
	// The window is SMALL on purpose. A pass costs one getSignaturesForAddress
	// per address plus one getTransaction per transaction found, so scanning a
	// busy exchange wallet over minutes of slots is thousands of calls — which
	// api.mainnet-beta.solana.com answers with 429. Real deposit addresses see a
	// handful of transactions in their lifetime, so this is a property of the
	// probe's choice of address, not of the rail. It is still the reason a
	// production deploy needs a dedicated endpoint.
	from := head - 25
	transfers, err := c.TransfersTo(ctx, probeOwners, from, head)
	if isThrottled(err) {
		// A rate limit is not an answer about the chain. Reporting it as a
		// failure would be as dishonest as reporting it as a success.
		t.Logf("scan INCONCLUSIVE — %s throttled us: %v", asset.RPCURL, err)
		t.Log("the read path reached the chain (head, decimals, symbol and the ATA derivation are all confirmed above); re-run against a dedicated endpoint to exercise the scan")
		return
	}
	if err != nil {
		t.Fatalf("TransfersTo over slots %d..%d: %v", from, head, err)
	}
	t.Logf("scanned slots %d..%d for %d address(es): %d inbound transfer(s)", from, head, len(probeOwners), len(transfers))
	for _, tr := range transfers {
		if !contains(probeOwners, tr.To) {
			t.Fatalf("a transfer came back addressed to %q, which is not one of the owners we asked about", tr.To)
		}
		if tr.Units == nil || tr.Units.Sign() <= 0 {
			t.Fatalf("a transfer carries a non-positive amount: %+v", tr)
		}
		t.Logf("  %s units to %s in %s at slot %d (event %d)", tr.Units, tr.To, tr.TxHash, tr.Block, tr.EventIndex)
	}

	// 6. And the whole watcher, wired exactly as production wires it. The store
	// is real, the chain is real; the gate that hands an address out is
	// elsewhere and stays shut, so a scan of addresses holding nothing credits
	// nothing.
	svc, err := New(os.Environ())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !svc.Enabled() {
		t.Fatal("watcher disabled despite configured assets")
	}
	t.Logf("watching: %+v", svc.Status(context.Background()).Assets)
}

// probeOwners are real mainnet wallets that hold USDC. They are used only to
// ask the cluster a question we can already answer ("what is this owner's token
// account?"), never to move anything.
var probeOwners = []string{
	"GJRs4FwHtemZ5ZE9x3FNvJ8TMwitKTh21yxdRPqn7npE",
	"9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM",
	"8meoEbDNDAogUcAm88F5coEASwyLbqcAMr47WwhpukUx",
}

// tokenAccountsByOwner asks the node to resolve an owner's token accounts.
//
// It is deliberately a RAW call written here in the test and not a method on
// the client: production never asks this question — it derives the answer — so
// the two paths are genuinely independent, and agreement between them means
// something.
func tokenAccountsByOwner(ctx context.Context, t *testing.T, rpcURL, owner, mint string) []string {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "getTokenAccountsByOwner",
		"params": []any{owner, map[string]any{"mint": mint}, map[string]any{"encoding": "jsonParsed", "commitment": "finalized"}},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("getTokenAccountsByOwner: %v", err)
	}
	defer resp.Body.Close()
	var out struct {
		Result struct {
			Value []struct {
				Pubkey string `json:"pubkey"`
			} `json:"value"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Error != nil {
		t.Fatalf("getTokenAccountsByOwner: %s", out.Error.Message)
	}
	accounts := make([]string, 0, len(out.Result.Value))
	for _, v := range out.Result.Value {
		accounts = append(accounts, v.Pubkey)
	}
	return accounts
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// isThrottled reports whether an error is a public endpoint's rate limit rather
// than an answer about the chain. Distinguishing the two is the whole value of a
// probe: "the node would not talk to us" and "the chain says no" must never be
// reported as the same thing.
func isThrottled(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "429") || strings.Contains(msg, "too many requests") || strings.Contains(msg, "rate limit")
}
