package bitcoinrpc

import (
	"context"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func esplora(t *testing.T, routes map[string]func(w http.ResponseWriter, r *http.Request)) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h, ok := routes[r.URL.Path]; ok {
			h(w, r)
			return
		}
		t.Errorf("unexpected request to %s %s", r.Method, r.URL.Path)
		w.WriteHeader(404)
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL)
}

func json200(body string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, body)
	}
}

// TestUnspentOrderIsStable is not cosmetic.
//
// The order Unspent returns becomes the INPUT ORDER of the transaction we sign.
// If it varied between two calls, one intent would produce two different
// transactions — two signatures over two payloads, both valid, both spending
// the same coins. Esplora makes no ordering promise, so the order is imposed
// here.
func TestUnspentOrderIsStable(t *testing.T) {
	c := esplora(t, map[string]func(http.ResponseWriter, *http.Request){
		"/address/bc1qexample/utxo": json200(`[
			{"txid":"cc","vout":1,"value":300,"status":{"confirmed":true,"block_height":900}},
			{"txid":"aa","vout":0,"value":100,"status":{"confirmed":true,"block_height":800}},
			{"txid":"bb","vout":0,"value":200,"status":{"confirmed":false}},
			{"txid":"aa","vout":2,"value":150,"status":{"confirmed":true,"block_height":800}},
			{"txid":"dd","vout":0,"value":0,"status":{"confirmed":true,"block_height":700}}
		]`),
	})
	got, err := c.Unspent(context.Background(), "bc1qexample")
	if err != nil {
		t.Fatal(err)
	}
	want := []UTXO{
		{TxID: "aa", Vout: 0, Value: 100, Height: 800},
		{TxID: "aa", Vout: 2, Value: 150, Height: 800},
		{TxID: "cc", Vout: 1, Value: 300, Height: 900},
		{TxID: "bb", Vout: 0, Value: 200, Height: 0}, // unconfirmed sorts LAST
	}
	if len(got) != len(want) {
		t.Fatalf("got %d outputs, want %d (the zero-value one must be dropped): %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("output %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
	if got[3].Confirmed() {
		t.Error("an output with no block height reports itself confirmed")
	}
}

// TestFeeRateTakesTheNearestTarget covers the shape Esplora actually answers
// with: a sparse map, string keys, no promise the asked-for target is present.
func TestFeeRateTakesTheNearestTarget(t *testing.T) {
	c := esplora(t, map[string]func(http.ResponseWriter, *http.Request){
		"/fee-estimates": json200(`{"1":25.5,"2":20.0,"6":12.25,"144":1.011,"1008":1.0}`),
	})
	for target, want := range map[int]float64{1: 25.5, 2: 20.0, 3: 12.25, 6: 12.25, 7: 1.011, 144: 1.011, 500: 1.0} {
		got, err := c.FeeRate(context.Background(), target)
		if err != nil {
			t.Fatalf("target %d: %v", target, err)
		}
		if got != want {
			t.Errorf("target %d: got %v sat/vB, want %v", target, got, want)
		}
	}
}

// TestFeeRateNeverInventsARate. A locally invented fee is either a transaction
// that never confirms or one that overpays without limit; neither is a decision
// this package may make silently.
func TestFeeRateNeverInventsARate(t *testing.T) {
	c := esplora(t, map[string]func(http.ResponseWriter, *http.Request){
		"/fee-estimates": json200(`{"1008":1.0}`),
	})
	if _, err := c.FeeRate(context.Background(), 2000); err == nil {
		t.Error("invented a fee rate for a target the endpoint does not cover")
	}
	if _, err := c.FeeRate(context.Background(), 0); err == nil {
		t.Error("accepted a fee target of 0 blocks")
	}
}

// TestFeeRateFloorsAtRelayMinimum. Below one satoshi per vbyte most nodes will
// not forward the transaction at all, so a sweep built at 0.5 sat/vB is a fee
// paid for a transaction that never travels.
func TestFeeRateFloorsAtRelayMinimum(t *testing.T) {
	c := esplora(t, map[string]func(http.ResponseWriter, *http.Request){
		"/fee-estimates": json200(`{"6":0.4}`),
	})
	got, err := c.FeeRate(context.Background(), 6)
	if err != nil {
		t.Fatal(err)
	}
	if got != 1 {
		t.Errorf("got %v sat/vB, want the 1 sat/vB relay floor", got)
	}
}

// TestSendChecksTheShapeNotTheStatus.
//
// Esplora reports some rejections with HTTP 200 and a prose body, so a client
// that trusted the status code would report a rejected transaction as
// broadcast — and a sweep would be recorded as done with the coins still in
// custody.
func TestSendChecksTheShapeNotTheStatus(t *testing.T) {
	txid := strings.Repeat("ab", 32)

	t.Run("accepts a txid", func(t *testing.T) {
		var sent string
		c := esplora(t, map[string]func(http.ResponseWriter, *http.Request){
			"/tx": func(w http.ResponseWriter, r *http.Request) {
				b, _ := io.ReadAll(r.Body)
				sent = string(b)
				io.WriteString(w, txid+"\n")
			},
		})
		got, err := c.Send(context.Background(), []byte{0x01, 0x02, 0x03})
		if err != nil {
			t.Fatal(err)
		}
		if got != txid {
			t.Errorf("got %q, want %q", got, txid)
		}
		if sent != "010203" {
			t.Errorf("posted %q, want the raw transaction as hex", sent)
		}
	})

	t.Run("refuses prose at 200", func(t *testing.T) {
		c := esplora(t, map[string]func(http.ResponseWriter, *http.Request){
			"/tx": func(w http.ResponseWriter, _ *http.Request) {
				io.WriteString(w, "sendrawtransaction RPC error: bad-txns-inputs-missingorspent")
			},
		})
		if _, err := c.Send(context.Background(), []byte{0x01}); err == nil {
			t.Error("read a rejection as a successful broadcast")
		}
	})

	t.Run("refuses a non-hex answer", func(t *testing.T) {
		c := esplora(t, map[string]func(http.ResponseWriter, *http.Request){
			"/tx": func(w http.ResponseWriter, _ *http.Request) {
				io.WriteString(w, strings.Repeat("z", 64))
			},
		})
		if _, err := c.Send(context.Background(), []byte{0x01}); err == nil {
			t.Error("accepted 64 characters that are not hex as a txid")
		}
	})

	t.Run("reports a real error", func(t *testing.T) {
		c := esplora(t, map[string]func(http.ResponseWriter, *http.Request){
			"/tx": func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(400)
				io.WriteString(w, "min relay fee not met")
			},
		})
		_, err := c.Send(context.Background(), []byte{0x01})
		if err == nil || !strings.Contains(err.Error(), "min relay fee") {
			t.Errorf("want the endpoint's reason, got %v", err)
		}
	})
}

// A LIVE read against Bitcoin TESTNET. It never broadcasts.
//
//	BTC_LIVE=1 go test ./billing/bitcoinrpc/ -run TestLiveSpend -v
func TestLiveSpendReads(t *testing.T) {
	if os.Getenv("BTC_LIVE") == "" {
		t.Skip("set BTC_LIVE=1 to read Bitcoin testnet")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	c := NewClient("https://blockstream.info/testnet/api")

	rate, err := c.FeeRate(ctx, 6)
	if err != nil {
		t.Fatalf("FeeRate: %v", err)
	}
	t.Logf("testnet fee rate for 6 blocks: %v sat/vB", rate)
	if rate < 1 {
		t.Errorf("rate %v is below the relay floor", rate)
	}

	// The BIP-173 example address. Its witness program is hash160 of the
	// secp256k1 generator point, so its key is the integer 1 and anybody may
	// spend it — which is exactly why it is safe to name in a test and why it
	// usually holds a little dust.
	const bip173 = "tb1qw508d6qejxtdg4y5r3zarvary0c5xw7kxpjzsx"
	utxos, err := c.Unspent(ctx, bip173)
	if err != nil {
		t.Fatalf("Unspent: %v", err)
	}
	t.Logf("%s has %d unspent outputs", bip173, len(utxos))
	var total int64
	for _, u := range utxos {
		if len(u.TxID) != 64 {
			t.Errorf("txid %q is not 32 bytes of hex", u.TxID)
		}
		if _, err := hex.DecodeString(u.TxID); err != nil {
			t.Errorf("txid %q is not hex", u.TxID)
		}
		if u.Value <= 0 {
			t.Errorf("output %s:%d has value %d", u.TxID, u.Vout, u.Value)
		}
		total += u.Value
	}
	t.Logf("total spendable: %d sats", total)
}
