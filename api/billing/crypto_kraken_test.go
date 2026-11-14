package billing

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/hanzoai/commerce/payment/routing"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/providers/kraken"
	"github.com/hanzoai/commerce/util/test/ae"
)

// TestCreateCryptoDeposit_KrakenOneFreshAddressPerPayment proves that when
// Kraken is configured as our crypto exchange processor:
//  1. cryptoProcessor(ctx) selects processor.Kraken.
//  2. Every call to CreateCryptoDeposit calls Kraken's DepositAddresses API
//     with new=true and issues a brand-new deposit address (1 address per
//     payment) — never reusing an old address across payments, so external
//     observers cannot track or correlate payments across our wallets.
func TestCreateCryptoDeposit_KrakenOneFreshAddressPerPayment(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	installWatcher(t, baseUSDCEnv(), true)
	// The policy's crypto rails send USDC to Stripe and everything else to Kraken, which
	// an admin turns on; here Kraken is on and is asked for USDC by its own rule.
	krakenOn := routing.Default()
	setOn(&krakenOn, "kraken", true)
	krakenOn.Accounts = accountsOf(baseUSDCEnv())
	krakenOn.CryptoRails = []routing.CryptoRoute{{Asset: "usdc", Processor: "kraken"}}
	if _, f := WriteRouting(c, "", krakenOn, "test", ""); f != nil {
		t.Fatalf("policy: %v", f.Err)
	}

	var addrSeq int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/0/private/DepositMethods":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": []string{},
				"result": []kraken.DepositMethod{
					{Method: "USDC (Base)", GenAddress: true},
				},
			})
		case "/0/private/DepositAddresses":
			if vals.Get("new") != "true" {
				t.Errorf("expected new=true on Kraken DepositAddresses, got %q", vals.Get("new"))
			}
			n := atomic.AddInt64(&addrSeq, 1)
			addr := fmt.Sprintf("0x%040x", n)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": []string{},
				"result": []kraken.DepositAddress{
					{Address: addr, ExpireTm: "0", New: true},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	secret := base64.StdEncoding.EncodeToString([]byte("test-kraken-secret-key-for-hmac-sha512"))
	kp := kraken.NewProcessor(kraken.Config{
		APIKey:    "test-kraken-key",
		APISecret: secret,
		APIURL:    srv.URL,
	})
	processor.Register(kp)
	t.Cleanup(func() {
		processor.Register(kraken.NewProcessor(kraken.Config{}))
	})

	db := datastore.New(c)
	org := organization.New(db)
	org.Name = "acme"
	if err := org.Create(); err != nil {
		t.Fatalf("create org: %v", err)
	}

	// Payment 1
	dep1, err := CreateCryptoDeposit(context.Background(), org, "acme/alice", "base", "usdc", 2000)
	if err != nil {
		t.Fatalf("CreateCryptoDeposit 1: %v", err)
	}
	// Payment 2 (same payer, second payment) -> MUST get a brand-new address!
	dep2, err := CreateCryptoDeposit(context.Background(), org, "acme/alice", "base", "usdc", 5000)
	if err != nil {
		t.Fatalf("CreateCryptoDeposit 2: %v", err)
	}
	// Payment 3 (second payer) -> MUST get a brand-new address!
	dep3, err := CreateCryptoDeposit(context.Background(), org, "acme/bob", "base", "usdc", 2000)
	if err != nil {
		t.Fatalf("CreateCryptoDeposit 3: %v", err)
	}

	if dep1.DepositAddress == "" || dep2.DepositAddress == "" || dep3.DepositAddress == "" {
		t.Fatalf("missing deposit address: dep1=%+v dep2=%+v dep3=%+v", dep1, dep2, dep3)
	}
	if dep1.DepositAddress == dep2.DepositAddress {
		t.Fatalf("reused deposit address across two payments for same payer: %q", dep1.DepositAddress)
	}
	if dep2.DepositAddress == dep3.DepositAddress || dep1.DepositAddress == dep3.DepositAddress {
		t.Fatalf("reused deposit address across payers: %q / %q / %q", dep1.DepositAddress, dep2.DepositAddress, dep3.DepositAddress)
	}
	if dep1.ID == dep2.ID {
		t.Fatalf("reused ID across two separate payments: %q", dep1.ID)
	}
}
