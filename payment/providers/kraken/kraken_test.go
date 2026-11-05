package kraken

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/hanzoai/commerce/payment/processor"
)

func TestKrakenGenerateAddress_NewAddressPerPayment(t *testing.T) {
	rawSecret := []byte("super-secret-kraken-hmac-key-material-64-bytes-long-for-sha512!!")
	b64Secret := base64.StdEncoding.EncodeToString(rawSecret)
	const apiKey = "kraken-test-api-key"

	var addrSeq int64
	var methodCalls int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method %s", r.Method)
		}
		if got := r.Header.Get("API-Key"); got != apiKey {
			t.Errorf("API-Key = %q, want %q", got, apiKey)
		}

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		postData := string(bodyBytes)
		vals, err := url.ParseQuery(postData)
		if err != nil {
			t.Fatalf("parse form: %v", err)
		}
		nonce := vals.Get("nonce")
		if nonce == "" {
			t.Errorf("missing nonce in form: %s", postData)
		}

		// Verify Kraken HMAC-SHA512 signature:
		// base64(HMAC-SHA512(uriPath + SHA256(nonce + postData), secret))
		sha := sha256.Sum256([]byte(nonce + postData))
		mac := hmac.New(sha512.New, rawSecret)
		mac.Write([]byte(r.URL.Path))
		mac.Write(sha[:])
		wantSig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		if gotSig := r.Header.Get("API-Sign"); gotSig != wantSig {
			t.Errorf("API-Sign mismatch: got %q, want %q", gotSig, wantSig)
		}

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/0/private/DepositMethods":
			atomic.AddInt64(&methodCalls, 1)
			asset := vals.Get("asset")
			var methods []DepositMethod
			switch asset {
			case "USDC":
				methods = []DepositMethod{
					{Method: "USDC (Unified EVM)", GenAddress: true},
					{Method: "USDC (Solana)", GenAddress: true},
				}
			case "XBT":
				methods = []DepositMethod{
					{Method: "Bitcoin Lightning", GenAddress: true},
					{Method: "Bitcoin", GenAddress: true},
				}
			default:
				methods = []DepositMethod{
					{Method: "Ethereum (ERC20)", GenAddress: true},
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":  []string{},
				"result": methods,
			})

		case "/0/private/DepositAddresses":
			// Every payment MUST pass new=true so Kraken issues a fresh address.
			if vals.Get("new") != "true" {
				t.Errorf("expected new=true on DepositAddresses, got %q (form=%s)", vals.Get("new"), postData)
			}
			n := atomic.AddInt64(&addrSeq, 1)
			addr := fmt.Sprintf("0xKrakenFreshAddr%040d", n)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": []string{},
				"result": []DepositAddress{
					{Address: addr, ExpireTm: "0", New: true},
				},
			})

		case "/0/private/DepositStatus":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": []string{},
				"result": []DepositRecord{
					{
						Method: "USDC (Unified EVM)",
						Asset:  "USDC",
						RefID:  "REF-123",
						TxID:   "0xabc123",
						Info:   "0xKrakenFreshAddr0000000000000000000000000000000000000001",
						Amount: "25.00",
						Status: "Success",
						Time:   1700000000,
					},
				},
			})

		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p := NewProcessor(Config{
		APIKey:    apiKey,
		APISecret: b64Secret,
		APIURL:    srv.URL,
	})

	ctx := processor.WithToken(context.Background(), "usdc")

	// Payment 1 -> gets fresh address #1
	w1, err := p.GenerateAddress(ctx, "acme/alice", "base")
	if err != nil {
		t.Fatalf("GenerateAddress 1: %v", err)
	}
	// Payment 2 (same payer!) -> gets fresh address #2 (1 per payment)
	w2, err := p.GenerateAddress(ctx, "acme/alice", "base")
	if err != nil {
		t.Fatalf("GenerateAddress 2: %v", err)
	}
	// Payment 3 (different chain: solana) -> gets fresh address #3
	w3, err := p.GenerateAddress(ctx, "acme/bob", "solana")
	if err != nil {
		t.Fatalf("GenerateAddress 3 (solana): %v", err)
	}

	if w1.Address == "" || w2.Address == "" || w3.Address == "" {
		t.Fatalf("empty address generated: w1=%+v w2=%+v w3=%+v", w1, w2, w3)
	}
	if w1.Address == w2.Address {
		t.Fatalf("reused the same address across two payments: %q", w1.Address)
	}
	if w2.Address == w3.Address {
		t.Fatalf("reused address between payment 2 and 3: %q", w2.Address)
	}

	// DepositMethods for USDC should have been cached after the first call.
	if got := atomic.LoadInt64(&methodCalls); got != 1 {
		t.Fatalf("DepositMethods calls for USDC = %d, want 1 (cached)", got)
	}

	// Verify Bitcoin skips "Bitcoin Lightning" and selects "Bitcoin".
	wBTC, err := p.GenerateAddress(processor.WithToken(context.Background(), "btc"), "acme/alice", "bitcoin")
	if err != nil {
		t.Fatalf("GenerateAddress bitcoin: %v", err)
	}
	if wBTC.Address == "" {
		t.Fatal("empty bitcoin address")
	}

	// Verify GetBalance reads DepositStatus for w1.Address.
	bal, err := p.GetBalance(ctx, w1.Address, "base")
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if bal.Available != 2500 {
		t.Fatalf("GetBalance.Available = %d, want 2500", bal.Available)
	}
}
