package depositledger

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/commerce/billing/depositwatch"
	"github.com/hanzoai/commerce/billing/xrplrpc"
)

// A LIVE read-path probe for XRPL, and the sibling of TestLiveChainReadPath and
// TestLiveSolanaReadPath.
//
// Everything else about this rail runs against fakes, which is right for policy
// and useless for the question that decides whether XRPL may be armed: does the
// reader actually reach the ledger, identify the token by asking its issuer,
// and read a real delivered amount at the right scale?
//
// Skipped unless CRYPTO_DEPOSIT_RPC_XRPL is set, so CI never depends on a public
// endpoint. Drive it by hand:
//
//	CRYPTO_DEPOSIT_RPC_XRPL=https://xrplcluster.com/ \
//	CRYPTO_DEPOSIT_TOKEN_XRPL_RLUSD=RLUSD.rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De \
//	CRYPTO_DEPOSIT_ADDRESS_XRPL=rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De \
//	go test -count=1 -run TestLiveXRPL -v ./billing/depositledger/
//
// The ADDRESS is required even though this probe only READS: XRPL pools every
// payer onto one custody account, and AssetsFromEnv refuses to build a pooled
// asset that has no account to be paid to. Any r-address will do here — the
// read path never sends anything to it.
//
// ⚠ -count=1 is not optional, for the same reason it is not optional on the EVM
// and Solana probes: Go caches a PASS even though every input came from the
// environment, so a second run against a DIFFERENT issuer replays the first
// one's result and reports the old issuer's evidence as live.
func TestLiveXRPLReadPath(t *testing.T) {
	rpc := strings.TrimSpace(os.Getenv("CRYPTO_DEPOSIT_RPC_XRPL"))
	if rpc == "" {
		t.Skip("no CRYPTO_DEPOSIT_RPC_XRPL configured — this probe is opt-in")
	}
	asset := liveAsset(t, depositwatch.FamilyXRPL)
	if asset.Contract == "" {
		t.Skip("CRYPTO_DEPOSIT_RPC_XRPL is set but no CRYPTO_DEPOSIT_TOKEN_XRPL_* — nothing to read")
	}
	t.Logf("token %s @ %s", asset.Contract, asset.RPCURL)

	token, err := xrplrpc.ParseIssued(asset.Contract)
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	c := xrplrpc.NewClient(asset.RPCURL, token)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// 1. The ledger is reachable and `validated` is a real, advancing position.
	// This is FINALITY on XRPL, not a depth: what this client can see cannot be
	// reorganised away.
	head, err := c.BlockNumber(ctx)
	if err != nil {
		t.Fatalf("ledger(validated): %v", err)
	}
	if head == 0 {
		t.Fatal("validated ledger index is 0")
	}
	t.Logf("validated ledger index: %d", head)

	// 2. The token identifies itself — by asking the ISSUER what it issues. A
	// configured pair that the issuer does not actually issue is the probe
	// EARNING ITS KEEP: it means the asset must not be armed.
	symbol, err := c.Symbol(ctx)
	if err != nil {
		t.Fatalf("TOKEN UNIDENTIFIABLE — do not arm this asset: %v", err)
	}
	t.Logf("issuer confirms it issues: %q", symbol)
	if !strings.EqualFold(strings.TrimSpace(symbol), asset.Token) {
		t.Fatalf("TOKEN MISLABELLED — do not arm this asset: %s issues %q but is configured as %q",
			asset.Contract, symbol, asset.Token)
	}

	// 3. The scale. There is nothing on this ledger to READ it from — an issued
	// currency has no base unit — so the guarantee is that the parse and the
	// reported scale are the same thing. Asserted properly in
	// xrplrpc.TestParseValue_RoundTripsAtDecimals; asserted here only to be sure
	// the live client reports what the arithmetic downstream will use.
	decimals, err := c.Decimals(ctx)
	if err != nil {
		t.Fatalf("Decimals: %v", err)
	}
	if decimals != xrplrpc.Scale {
		t.Fatalf("client reports %d decimals but renders at %d", decimals, xrplrpc.Scale)
	}
	t.Logf("rendering scale: %d decimal places", decimals)

	// 4. THE assertion this probe exists for: read a REAL delivery of this token
	// through the same code a pass uses, and check it against the ledger's own
	// account_tx answer for the same transaction.
	//
	// A payment is found independently — by asking the issuer's own transaction
	// history, which production never does — so agreement between the two paths
	// means something.
	dest, ledger, wantValue, wantHash, tag, ok := findLiveXRPLPayment(ctx, t, asset.RPCURL, token)
	if !ok {
		t.Log("scan INCONCLUSIVE — no recent Payment of this token was found in the issuer's history")
		t.Log("the read path reached the ledger (head, identity and scale are confirmed above); re-run when the token has recent traffic to exercise the scan")
		return
	}
	t.Logf("found a real delivery: %s to %s in ledger %d (tag %q)", wantValue, dest, ledger, tag)

	transfers, err := c.TransfersTo(ctx, []string{dest}, ledger, ledger)
	if err != nil {
		t.Fatalf("TransfersTo over ledger %d: %v", ledger, err)
	}
	var got *depositwatch.Transfer
	for i := range transfers {
		if transfers[i].TxHash == strings.ToLower(wantHash) {
			got = &transfers[i]
		}
	}
	if got == nil {
		t.Fatalf("READ PATH MISSED A REAL DELIVERY — do not arm this asset: %s in ledger %d was not returned (%d transfer(s) found)", wantHash, ledger, len(transfers))
	}
	if got.To != dest {
		t.Fatalf("a transfer came back addressed to %q, not the account we asked about (%q)", got.To, dest)
	}
	if got.Tag != tag {
		t.Fatalf("destination tag read as %q, the ledger says %q — a pooled deposit would be routed to the wrong customer", got.Tag, tag)
	}
	if got.Block != ledger {
		t.Fatalf("ledger index read as %d, want %d", got.Block, ledger)
	}
	// The scale check that matters: units, divided by 10^decimals, must be the
	// decimal number the ledger stated. A misread here is a 10^n credit.
	wantUnits, err := xrplrpc.ParseValue(wantValue)
	if err != nil {
		t.Fatal(err)
	}
	if got.Units.Cmp(wantUnits) != 0 {
		t.Fatalf("AMOUNT MISREAD — do not arm this asset: the ledger delivered %s, the reader made that %s units, want %s",
			wantValue, got.Units, wantUnits)
	}
	t.Logf("  read back: %s units to %s in %s at ledger %d (event %d, tag %q)",
		got.Units, got.To, got.TxHash, got.Block, got.EventIndex, got.Tag)

	// 5. And the whole watcher, wired exactly as production wires it. The gate
	// that hands an address out is elsewhere and stays shut, so a scan of
	// addresses holding nothing credits nothing.
	svc, err := New(os.Environ())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !svc.Enabled() {
		t.Fatal("watcher disabled despite configured assets")
	}
	t.Logf("watching: %s", svc.Describe())
}

// findLiveXRPLPayment asks the ISSUER's own transaction history for a recent
// Payment delivering this token, and reports (destination, ledger, delivered
// value, tx hash, destination tag).
//
// It is deliberately a RAW call written here in the test: production reads
// account_tx on the CUSTODY account, never on the issuer, so the two paths are
// genuinely independent and agreement between them is evidence.
func findLiveXRPLPayment(ctx context.Context, t *testing.T, rpcURL string, token xrplrpc.Issued) (dest string, ledger uint64, value, hash, tag string, ok bool) {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"method": "account_tx",
		"params": []any{map[string]any{
			"account": token.Issuer, "ledger_index_min": -1, "ledger_index_max": -1,
			"binary": false, "forward": false, "limit": 400, "api_version": 1,
		}},
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
		t.Fatalf("account_tx: %v", err)
	}
	defer resp.Body.Close()

	var out struct {
		Result struct {
			Status       string `json:"status"`
			Error        string `json:"error"`
			Transactions []struct {
				Tx struct {
					TransactionType string  `json:"TransactionType"`
					Destination     string  `json:"Destination"`
					DestinationTag  *uint32 `json:"DestinationTag"`
					Hash            string  `json:"hash"`
					LedgerIndex     uint64  `json:"ledger_index"`
				} `json:"tx"`
				Meta struct {
					TransactionResult string `json:"TransactionResult"`
					// RAW, because delivered_amount is an OBJECT for an issued
					// currency and a bare STRING of drops for native XRP. A
					// decoder that assumed the object shape fails on the first
					// XRP payment it meets — which is exactly what this helper
					// did on its first live run.
					DeliveredAmount json.RawMessage `json:"delivered_amount"`
				} `json:"meta"`
			} `json:"transactions"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Result.Error != "" {
		t.Fatalf("account_tx: %s", out.Result.Error)
	}
	for _, e := range out.Result.Transactions {
		if e.Tx.TransactionType != "Payment" || e.Meta.TransactionResult != "tesSUCCESS" {
			continue
		}
		var d struct {
			Currency string `json:"currency"`
			Issuer   string `json:"issuer"`
			Value    string `json:"value"`
		}
		if json.Unmarshal(e.Meta.DeliveredAmount, &d) != nil {
			continue // a bare string: native XRP, not this token
		}
		cur, err := xrplrpc.ParseCurrency(d.Currency)
		if err != nil || cur != token.Currency || d.Issuer != token.Issuer {
			continue
		}
		tag := ""
		if e.Tx.DestinationTag != nil {
			tag = strconv.FormatUint(uint64(*e.Tx.DestinationTag), 10)
		}
		return e.Tx.Destination, e.Tx.LedgerIndex, d.Value, e.Tx.Hash, tag, true
	}
	return "", 0, "", "", "", false
}

// liveAsset picks the configured asset of one chain family out of the
// environment, so each live probe reads the same table production reads.
func liveAsset(t *testing.T, family depositwatch.Family) depositwatch.Asset {
	t.Helper()
	assets, err := depositwatch.AssetsFromEnv(os.Environ())
	if err != nil {
		t.Fatalf("AssetsFromEnv: %v", err)
	}
	var out depositwatch.Asset
	for _, a := range assets {
		if a.Family() == family {
			out = a
		}
	}
	return out
}
