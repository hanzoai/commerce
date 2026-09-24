package xrplrpc

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func meta(delivered string) *txMeta {
	m := &txMeta{TransactionResult: "tesSUCCESS"}
	if delivered != "" {
		m.DeliveredAmount = json.RawMessage(delivered)
	}
	return m
}

// THE DISCRIMINATOR IS THE SHAPE, and it is the whole of it: XRPL renders a
// native amount as a bare STRING of drops and an issued amount as an OBJECT.
func TestABareStringIsNativeDrops(t *testing.T) {
	got, ok, err := nativeAmount(meta(`"25000000"`), "HASH")
	if err != nil {
		t.Fatalf("nativeAmount: %v", err)
	}
	if !ok {
		t.Fatal("a bare drops string credited nothing")
	}
	if got.String() != "25000000" {
		t.Errorf("drops = %s, want 25000000 (25 XRP)", got)
	}
}

func TestAnIssuedAmountIsNotNativeXRP(t *testing.T) {
	// RLUSD, USDC, anybody's token — a different asset, and not an error.
	_, ok, err := nativeAmount(meta(`{"currency":"524C555344000000000000000000000000000000","issuer":"rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De","value":"10"}`), "HASH")
	if err != nil {
		t.Fatalf("nativeAmount: %v", err)
	}
	if ok {
		t.Error("an issued currency was credited as native XRP")
	}
}

// The one case where the amount is genuinely unknowable. Refused rather than
// guessed — crediting anything here would be inventing it.
func TestUnavailableIsRefused(t *testing.T) {
	_, _, err := nativeAmount(meta(`"unavailable"`), "HASH")
	if err == nil {
		t.Fatal(`delivered_amount "unavailable" was accepted`)
	}
	if !strings.Contains(err.Error(), "does not report") {
		t.Errorf("error does not explain: %v", err)
	}
}

func TestNoDeliveredAmountCreditsNothing(t *testing.T) {
	for _, m := range []*txMeta{meta(""), meta("null")} {
		if _, ok, err := nativeAmount(m, "HASH"); ok || err != nil {
			t.Errorf("a non-delivering payment credited (ok=%v err=%v)", ok, err)
		}
	}
}

func TestZeroDropsCreditsNothing(t *testing.T) {
	if _, ok, _ := nativeAmount(meta(`"0"`), "HASH"); ok {
		t.Error("zero drops was credited")
	}
}

func TestAnUnreadableAmountIsAnError(t *testing.T) {
	for _, bad := range []string{`"abc"`, `"-5"`, `"1.5"`} {
		if _, _, err := nativeAmount(meta(bad), "HASH"); err == nil {
			t.Errorf("delivered_amount %s was accepted as drops", bad)
		}
	}
}

// Drops are 6 decimals, NOT the issued-currency Scale of 15. Using Scale would
// credit 10^9 times too much.
func TestNativeDecimalsAreDropsNotScale(t *testing.T) {
	n := NewNative("https://xrpl.invalid")
	d, err := n.Decimals(context.Background())
	if err != nil {
		t.Fatalf("Decimals: %v", err)
	}
	if d != DropDecimals {
		t.Errorf("Decimals = %d, want %d", d, DropDecimals)
	}
	if d == Scale {
		t.Error("native decimals equal the issued-currency Scale — that credits 10^9 too much")
	}
}

// A native reader must answer Symbol WITHOUT a ledger round-trip: there is no
// issuer to interrogate, and asking about a zero issuer would fail.
func TestNativeSymbolNeedsNoLedger(t *testing.T) {
	n := NewNative("https://xrpl.invalid") // deliberately unreachable
	got, err := n.Symbol(context.Background())
	if err != nil {
		t.Fatalf("Symbol: %v", err)
	}
	if got != "XRP" {
		t.Errorf("Symbol = %q, want XRP", got)
	}
}

// The issued reader must keep ignoring native XRP, or the two would both credit
// the same payment.
func TestTheIssuedReaderStillIgnoresNativeXRP(t *testing.T) {
	c := NewClient("https://xrpl.invalid", Issued{})
	if _, ok, err := c.delivered(meta(`"25000000"`), "HASH"); ok || err != nil {
		t.Errorf("the issued reader credited native drops (ok=%v err=%v)", ok, err)
	}
}
