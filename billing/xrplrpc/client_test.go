package xrplrpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Reader-level tests over the REAL wire shapes, so the decisions that depend on
// how rippled words things — which field the amount comes from, what a failed
// command looks like — are proven rather than assumed.

const (
	testIssuer = "rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De"
	testPooled = "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"
	rlusdHex   = "524C555344000000000000000000000000000000"
)

// serve stands up a fake rippled that answers each method from a table.
func serve(t *testing.T, answers map[string]string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		body, ok := answers[req.Method]
		if !ok {
			t.Errorf("unexpected method %q", req.Method)
			body = `{"result":{"status":"error","error":"unknownCmd"}}`
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	token, err := ParseIssued("RLUSD." + testIssuer)
	if err != nil {
		t.Fatal(err)
	}
	return NewClient(srv.URL, token)
}

// entry renders one account_tx transaction the way rippled does.
func entry(txType, dest, tag, amount, delivered, result, hash string, ledger int) string {
	tagField := ""
	if tag != "" {
		tagField = `"DestinationTag":` + tag + `,`
	}
	return `{"tx":{"TransactionType":"` + txType + `","Account":"rSENDER","Destination":"` + dest + `",` +
		tagField + `"Amount":` + amount + `,"hash":"` + hash + `","ledger_index":` + itoa(ledger) + `},` +
		`"meta":{"TransactionResult":"` + result + `","TransactionIndex":7,"delivered_amount":` + delivered + `},` +
		`"validated":true}`
}

func iou(value string) string {
	return `{"currency":"` + rlusdHex + `","issuer":"` + testIssuer + `","value":"` + value + `"}`
}

// THE XRPL exploit, and the reason meta.delivered_amount exists.
//
// With tfPartialPayment set, tx.Amount is the MOST the sender was willing to
// deliver and the ledger may deliver an arbitrarily smaller fraction of it. An
// exchange that credits Amount hands out a million dollars for a payment that
// delivered a cent. This asserts the reader credits what ARRIVED.
func TestTransfersTo_CreditsDeliveredAmountNotAmount(t *testing.T) {
	c := serve(t, map[string]string{
		"account_tx": `{"result":{"status":"success","transactions":[` +
			entry("Payment", testPooled, "42", iou("1000000"), iou("0.01"), "tesSUCCESS", strings.Repeat("A", 64), 900) +
			`]}}`,
	})
	got, err := c.TransfersTo(context.Background(), []string{testPooled}, 800, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("read %d transfers, want 1", len(got))
	}
	// 0.01 at scale 15 — NOT 1000000.
	if got[0].Units.String() != "10000000000000" {
		t.Fatalf("PARTIAL PAYMENT EXPLOIT: credited %s units; the ledger delivered 0.01 (10000000000000 units), the sender merely OFFERED 1000000",
			got[0].Units)
	}
	if got[0].Tag != "42" {
		t.Fatalf("tag = %q, want 42", got[0].Tag)
	}
	if got[0].EventIndex != 0 {
		t.Fatalf("event index = %d, want 0", got[0].EventIndex)
	}
	// Canonical lowercase hex, whatever the server sent.
	if got[0].TxHash != strings.ToLower(strings.Repeat("A", 64)) {
		t.Fatalf("transaction id %q is not canonical lowercase hex — the dedup key would depend on the server's rendering", got[0].TxHash)
	}
}

// Everything that is NOT a successful Payment of our exact token into our exact
// address must be read as no deposit at all.
func TestTransfersTo_IgnoresWhatIsNotOurDeposit(t *testing.T) {
	other := strings.Repeat("B", 64)
	for _, tc := range []struct{ name, entry string }{
		{
			"a failed payment",
			entry("Payment", testPooled, "42", iou("5"), iou("5"), "tecPATH_DRY", other, 900),
		},
		{
			"a payment to somebody else",
			entry("Payment", "rSOMEONEELSE", "42", iou("5"), iou("5"), "tesSUCCESS", other, 900),
		},
		{
			// The impersonation attack: our ticker, an issuer we never
			// configured. That token is worth whatever its issuer says, which
			// may be nothing.
			"our currency from a different issuer",
			`{"tx":{"TransactionType":"Payment","Account":"rSENDER","Destination":"` + testPooled + `","DestinationTag":42,"hash":"` + other + `","ledger_index":900},` +
				`"meta":{"TransactionResult":"tesSUCCESS","TransactionIndex":1,"delivered_amount":{"currency":"` + rlusdHex + `","issuer":"rIMPOSTOR","value":"1000000"}},"validated":true}`,
		},
		{
			"native XRP, which this rail cannot price",
			entry("Payment", testPooled, "42", `"5000000"`, `"5000000"`, "tesSUCCESS", other, 900),
		},
		{
			// Value can reach an account other ways, and none of them can carry
			// a payer's routing tag — so none of them is a customer's deposit.
			"a check being cashed",
			entry("CheckCash", testPooled, "42", iou("5"), iou("5"), "tesSUCCESS", other, 900),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := serve(t, map[string]string{
				"account_tx": `{"result":{"status":"success","transactions":[` + tc.entry + `]}}`,
			})
			got, err := c.TransfersTo(context.Background(), []string{testPooled}, 800, 1000)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 0 {
				t.Fatalf("read %d transfers from %s: %+v", len(got), tc.name, got)
			}
		})
	}
}

// An untagged payment still comes BACK from the reader. Deciding it belongs to
// nobody is the watcher's call; a reader that filtered it away would make the
// money invisible to the layer that records it.
func TestTransfersTo_ReturnsUntaggedPaymentsWithAnEmptyTag(t *testing.T) {
	c := serve(t, map[string]string{
		"account_tx": `{"result":{"status":"success","transactions":[` +
			entry("Payment", testPooled, "", iou("3"), iou("3"), "tesSUCCESS", strings.Repeat("C", 64), 900) +
			`,` +
			entry("Payment", testPooled, "0", iou("4"), iou("4"), "tesSUCCESS", strings.Repeat("D", 64), 901) +
			`]}}`,
	})
	got, err := c.TransfersTo(context.Background(), []string{testPooled}, 800, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("read %d transfers, want 2", len(got))
	}
	if got[0].Tag != "" {
		t.Fatalf("a payment with no DestinationTag came back with tag %q", got[0].Tag)
	}
	if got[1].Tag != "0" {
		t.Fatalf("a payment with DestinationTag 0 came back with tag %q — 0 is a real tag somebody holds", got[1].Tag)
	}
}

// XRPL reports a command failure INSIDE result, with HTTP 200 and no top-level
// error. A client that only checked the HTTP status would read a failed
// account_tx as an empty one — "this account received nothing" when the truth is
// "we could not look", which is a missed deposit.
func TestCall_TreatsAnInResultErrorAsAFailure(t *testing.T) {
	c := serve(t, map[string]string{
		"account_tx": `{"result":{"status":"error","error":"lgrNotFound","error_message":"ledgerIndexMin is out of range."}}`,
	})
	got, err := c.TransfersTo(context.Background(), []string{testPooled}, 800, 1000)
	if err == nil {
		t.Fatalf("a failed account_tx was read as %d transfers — silence became 'nothing arrived'", len(got))
	}
	if !strings.Contains(err.Error(), "lgrNotFound") {
		t.Fatalf("error %q does not carry the ledger's own reason", err)
	}
}

// Only VALIDATED ledgers are final, and this client reads nothing else. An
// unvalidated entry inside a validated range means the server disagrees with
// itself, and believing it would credit money that can still change.
func TestBlockNumber_RefusesAnUnvalidatedLedger(t *testing.T) {
	c := serve(t, map[string]string{
		"ledger": `{"result":{"status":"success","ledger_index":106121435,"validated":false}}`,
	})
	if _, err := c.BlockNumber(context.Background()); err == nil {
		t.Fatal("accepted a ledger the server does not call validated")
	}

	ok := serve(t, map[string]string{
		"ledger": `{"result":{"status":"success","ledger_index":106121435,"validated":true}}`,
	})
	head, err := ok.BlockNumber(context.Background())
	if err != nil || head != 106121435 {
		t.Fatalf("BlockNumber = %d, %v", head, err)
	}
}

// Identity comes from asking the ISSUER what it issues. A pair the issuer does
// not issue is refused before a single deposit is credited at a dollar.
func TestSymbol_RefusesAnIssuerThatDoesNotIssueIt(t *testing.T) {
	c := serve(t, map[string]string{
		"gateway_balances": `{"result":{"status":"success","obligations":{"USD":"100"}}}`,
	})
	if sym, err := c.Symbol(context.Background()); err == nil {
		t.Fatalf("Symbol = %q for an issuer that only issues USD", sym)
	}

	ok := serve(t, map[string]string{
		"gateway_balances": `{"result":{"status":"success","obligations":{"` + rlusdHex + `":"845703146.2"}}}`,
	})
	sym, err := ok.Symbol(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if sym != "RLUSD" {
		t.Fatalf("Symbol = %q, want RLUSD", sym)
	}

	none := serve(t, map[string]string{
		"gateway_balances": `{"result":{"status":"success"}}`,
	})
	if _, err := none.Symbol(context.Background()); err == nil {
		t.Fatal("an account that issues nothing was accepted as an issuer")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
