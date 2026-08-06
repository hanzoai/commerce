package mpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// An address without its custody handle is half a record, and the half we kept
// was the one that cannot spend.
//
// /keygen returns both `wallet_id` and the per-curve addresses. This code
// parsed the wallet id and then discarded it, because GenerateAddress returned
// a bare string — so commerce minted custody addresses it held no handle to,
// and a credited deposit could never be swept without reconciling against the
// MPC node's own wallet records. The return type is a Wallet now, which makes
// dropping the id something you would have to do on purpose.
func TestGenerateAddress_KeepsTheWalletHandle(t *testing.T) {
	const (
		wantID  = "wal_01HXYZ"
		wantEVM = "0x1111111111111111111111111111111111111111"
		wantBTC = "bc1qexampleexampleexampleexampleexampleex"
		wantSOL = "So11111111111111111111111111111111111111112"
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/keygen" {
			t.Errorf("keygen posted to %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"wallet_id":   wantID,
			"result_type": "keygen",
			"evm_address": wantEVM,
			"btc_address": wantBTC,
			"sol_address": wantSOL,
		})
	}))
	defer srv.Close()

	mp := NewProcessor(Config{KMSEndpoint: srv.URL, MPCEndpoint: srv.URL})

	// Every chain branch must carry the handle — the bug was one shared cause,
	// so covering only "ethereum" would leave two ways to reintroduce it.
	for _, tc := range []struct{ chain, wantAddr string }{
		{"ethereum", wantEVM},
		{"base", wantEVM}, // EVM chains share the secp256k1 address
		{"bitcoin", wantBTC},
		{"solana", wantSOL},
	} {
		t.Run(tc.chain, func(t *testing.T) {
			got, err := mp.GenerateAddress(context.Background(), "hanzo/z@hanzo.ai", tc.chain)
			if err != nil {
				t.Fatalf("GenerateAddress(%s): %v", tc.chain, err)
			}
			if got.Address != tc.wantAddr {
				t.Errorf("address = %q, want %q", got.Address, tc.wantAddr)
			}
			if got.ID != wantID {
				t.Errorf("wallet id = %q, want %q — an address whose wallet we cannot name is one we cannot sweep", got.ID, wantID)
			}
		})
	}
}

// A keygen that returns no address for the requested chain must fail rather
// than hand back an empty destination. Money sent to "" is money sent nowhere,
// and an empty string is exactly what a zero-value struct would return if the
// error path were ever dropped.
func TestGenerateAddress_RefusesAnEmptyDestination(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A node that knows only secp256k1 answers with no sol_address — which
		// is the live shape while no Ed25519 ciphersuite exists.
		_ = json.NewEncoder(w).Encode(map[string]string{
			"wallet_id":   "wal_02",
			"evm_address": "0x2222222222222222222222222222222222222222",
		})
	}))
	defer srv.Close()

	mp := NewProcessor(Config{KMSEndpoint: srv.URL, MPCEndpoint: srv.URL})
	got, err := mp.GenerateAddress(context.Background(), "hanzo", "solana")
	if err == nil {
		t.Fatalf("solana keygen with no sol_address returned %+v, want an error", got)
	}
	if got.Address != "" {
		t.Errorf("failed keygen still produced address %q", got.Address)
	}
}
