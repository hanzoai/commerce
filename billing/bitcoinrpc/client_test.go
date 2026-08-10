package bitcoinrpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const addr = "bc1qgdjqv0av3q56jvd82tkdjpy7gdp9ut8tlqmgrpmv24sq90ecnvqqjwvw97"

// oneTx builds the Esplora shape for a confirmed transaction paying addr once.
func oneTx(txid string, height uint64, outs ...map[string]any) map[string]any {
	if len(outs) == 0 {
		outs = []map[string]any{{"scriptpubkey_address": addr, "value": 250000}}
	}
	return map[string]any{
		"txid":   txid,
		"status": map[string]any{"confirmed": true, "block_height": height},
		"vout":   outs,
	}
}

// serve stands up an Esplora that answers one page and then empties, so the
// walk terminates.
func serve(t *testing.T, tip string, pages map[string][]map[string]any) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/blocks/tip/height" {
			_, _ = w.Write([]byte(tip))
			return
		}
		page, ok := pages[r.URL.Path]
		if !ok {
			_, _ = w.Write([]byte("[]"))
			return
		}
		_ = json.NewEncoder(w).Encode(page)
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL)
}

func TestBlockNumberReadsABareInteger(t *testing.T) {
	// The tip endpoint answers a plain number, NOT JSON — decoding it as JSON
	// would fail on a perfectly good response.
	c := serve(t, "961810", nil)
	got, err := c.BlockNumber(context.Background())
	if err != nil {
		t.Fatalf("BlockNumber: %v", err)
	}
	if got != 961810 {
		t.Errorf("tip = %d, want 961810", got)
	}
}

func TestAnUnsyncedEndpointIsRefused(t *testing.T) {
	// Height 0 is not a young chain, it is an endpoint with no data. Accepting
	// it would make every window `0..0` and silently watch nothing.
	c := serve(t, "0", nil)
	if _, err := c.BlockNumber(context.Background()); err == nil {
		t.Fatal("a tip height of 0 was accepted")
	}
}

func TestAnOutputPayingUsIsATransfer(t *testing.T) {
	c := serve(t, "961810", map[string][]map[string]any{
		"/address/" + addr + "/txs": {oneTx("aa", 961800)},
	})
	got, err := c.TransfersTo(context.Background(), []string{addr}, 961000, 961810)
	if err != nil {
		t.Fatalf("TransfersTo: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d transfers, want 1", len(got))
	}
	if got[0].Units.String() != "250000" || got[0].TxHash != "aa" || got[0].Block != 961800 {
		t.Errorf("transfer = %+v", got[0])
	}
}

// THE UTXO TRAP. Two outputs in one transaction can both pay us, and they are
// two separate pieces of value. Collapsing them into one credit works until a
// wallet splits a payment and then silently credits half.
func TestTwoOutputsInOneTransactionAreTwoTransfers(t *testing.T) {
	c := serve(t, "961810", map[string][]map[string]any{
		"/address/" + addr + "/txs": {oneTx("bb", 961800,
			map[string]any{"scriptpubkey_address": addr, "value": 100000},
			map[string]any{"scriptpubkey_address": "someone-else", "value": 999},
			map[string]any{"scriptpubkey_address": addr, "value": 50000},
		)},
	})
	got, err := c.TransfersTo(context.Background(), []string{addr}, 961000, 961810)
	if err != nil {
		t.Fatalf("TransfersTo: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d transfers, want 2 — the second output was dropped or summed away", len(got))
	}
	// The EventIndex must be the OUTPUT'S OWN position, never a counter over
	// matches: a counter renumbers when the first output stops matching, and
	// renumbering a dedup key is a double credit.
	if got[0].EventIndex != 0 || got[1].EventIndex != 2 {
		t.Errorf("event indexes = %d,%d — want the vout positions 0 and 2",
			got[0].EventIndex, got[1].EventIndex)
	}
	if got[0].Units.String() != "100000" || got[1].Units.String() != "50000" {
		t.Errorf("amounts = %s,%s", got[0].Units, got[1].Units)
	}
}

func TestOutputsToOtherAddressesAreIgnored(t *testing.T) {
	c := serve(t, "961810", map[string][]map[string]any{
		"/address/" + addr + "/txs": {oneTx("cc", 961800,
			map[string]any{"scriptpubkey_address": "not-ours", "value": 500000},
		)},
	})
	got, err := c.TransfersTo(context.Background(), []string{addr}, 961000, 961810)
	if err != nil {
		t.Fatalf("TransfersTo: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("credited %d transfers for an address that is not ours", len(got))
	}
}

// The mempool is not a shallower confirmation — it is a claim that has not
// happened and can be replaced.
func TestAnUnconfirmedTransactionIsSkipped(t *testing.T) {
	c := serve(t, "961810", map[string][]map[string]any{
		"/address/" + addr + "/txs": {
			{
				"txid":   "pending",
				"status": map[string]any{"confirmed": false},
				"vout":   []map[string]any{{"scriptpubkey_address": addr, "value": 700000}},
			},
			oneTx("confirmed", 961800),
		},
	})
	got, err := c.TransfersTo(context.Background(), []string{addr}, 961000, 961810)
	if err != nil {
		t.Fatalf("TransfersTo: %v", err)
	}
	// The unconfirmed one is skipped WITHOUT stopping the walk — the confirmed
	// rows behind it are still wanted.
	if len(got) != 1 || got[0].TxHash != "confirmed" {
		t.Fatalf("got %+v, want only the confirmed transaction", got)
	}
}

func TestTransactionsOutsideTheWindowAreNotCredited(t *testing.T) {
	c := serve(t, "961810", map[string][]map[string]any{
		"/address/" + addr + "/txs": {
			oneTx("tooNew", 961809),
			oneTx("inside", 961500),
			oneTx("tooOld", 900000),
		},
	})
	got, err := c.TransfersTo(context.Background(), []string{addr}, 961000, 961800)
	if err != nil {
		t.Fatalf("TransfersTo: %v", err)
	}
	if len(got) != 1 || got[0].TxHash != "inside" {
		t.Fatalf("got %+v, want only the in-window transaction", got)
	}
}

func TestAZeroValueOutputCreditsNothing(t *testing.T) {
	c := serve(t, "961810", map[string][]map[string]any{
		"/address/" + addr + "/txs": {oneTx("dd", 961800,
			map[string]any{"scriptpubkey_address": addr, "value": 0},
		)},
	})
	got, _ := c.TransfersTo(context.Background(), []string{addr}, 961000, 961810)
	if len(got) != 0 {
		t.Errorf("a zero-value output was credited")
	}
}

func TestADuplicateWatchedAddressIsRefused(t *testing.T) {
	// "Which intent owns this?" has no safe answer.
	c := serve(t, "961810", nil)
	_, err := c.TransfersTo(context.Background(), []string{addr, addr}, 961000, 961810)
	if err == nil {
		t.Fatal("the same address watched twice was accepted")
	}
	if !strings.Contains(err.Error(), "twice") {
		t.Errorf("error does not explain: %v", err)
	}
}

func TestDecimalsAndSymbolAreProtocolConstants(t *testing.T) {
	c := NewClient("https://esplora.invalid")
	if d, err := c.Decimals(context.Background()); err != nil || d != 8 {
		t.Errorf("Decimals = (%d, %v), want 8", d, err)
	}
	// Answered without a round-trip — the endpoint above is unreachable.
	if s, err := c.Symbol(context.Background()); err != nil || s != "BTC" {
		t.Errorf("Symbol = (%q, %v), want BTC", s, err)
	}
}
