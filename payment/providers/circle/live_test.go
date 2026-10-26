// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package circle

import (
	"context"
	"crypto/rand"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/luxfi/crypto"
)

// Live proof against Circle's TESTNET verifier, opt-in:
//
//	CIRCLE_LIVE=1 go test -count=1 -run Live ./payment/providers/circle
//
// It signs an authorization with a throwaway key — no balance, no deposit — and
// asks Gateway's own POST /v1/x402/verify whether the signature is good. Verify
// is read-only and checks scheme, network, token, signature and window. Then it
// settles the same payment through Client.Settle, which Gateway must refuse on
// BALANCE alone (the key holds nothing, so nothing is locked or moved). A
// tampered copy must be refused as invalid_signature, which proves the verifier
// really read the signature rather than answering yes to anything.
//
// -count=1 is not optional: Go caches a PASS whose inputs came from the network.
func TestLiveTestnetVerifiesOurSignature(t *testing.T) {
	if os.Getenv("CIRCLE_LIVE") != "1" {
		t.Skip("set CIRCLE_LIVE=1 to verify against Circle's testnet")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c := New(Testnet.Base())

	info, err := c.Info(ctx)
	if err != nil || len(info.Domains) == 0 {
		t.Fatalf("info: %v (%d domains)", err, len(info.Domains))
	}
	kinds, err := c.Supported(ctx)
	if err != nil {
		t.Fatal(err)
	}
	const network = "eip155:84532" // Base Sepolia
	var kind Kind
	var asset Asset
	for _, k := range kinds {
		if k.Network != network {
			continue
		}
		for _, a := range k.Extra.Assets {
			if a.Symbol == USDC {
				kind, asset = k, a
			}
		}
	}
	if asset.Address == "" {
		t.Fatalf("testnet supports no USDC on %s", network)
	}
	if kind.Extra.Name != Name || kind.Extra.Version != Release {
		t.Fatalf("testnet signs %q v%q; this package signs %q v%q", kind.Extra.Name, kind.Extra.Version, Name, Release)
	}

	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	from := crypto.PubkeyToAddress(key.PublicKey).Bytes()
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	auth := Authorization{
		From:        Hex(from),
		To:          "0x7e57000000000000000000000000000000000001",
		Value:       "1000000",
		ValidAfter:  strconv.FormatInt(now-600, 10),
		ValidBefore: strconv.FormatInt(now+kind.Extra.MinValidity+3600, 10),
		Nonce:       Hex(nonce[:]),
	}
	req := Requirements{
		Scheme: Scheme, Network: network, Asset: asset.Address, Amount: auth.Value,
		PayTo: auth.To, MaxTimeoutSeconds: kind.Extra.MinValidity + 100,
		Extra: Signing{Name: kind.Extra.Name, Version: kind.Extra.Version, VerifyingContract: kind.Extra.VerifyingContract},
	}
	chain, _ := strconv.ParseInt(network[len("eip155:"):], 10, 64)
	dom := Domain{Name: kind.Extra.Name, Version: kind.Extra.Version, ChainID: chain, VerifyingContract: kind.Extra.VerifyingContract}

	sign := func(a Authorization) string {
		p, err := a.Parse()
		if err != nil {
			t.Fatal(err)
		}
		d, err := dom.Digest(p)
		if err != nil {
			t.Fatal(err)
		}
		sig, err := crypto.Sign(d[:], key)
		if err != nil {
			t.Fatal(err)
		}
		sig[64] += 27
		return Hex(sig)
	}
	res := Resource{URL: "/v1/billing/crypto/gateway/live-proof", Description: "Hanzo balance top-up", MimeType: "application/json"}
	verify := func(a Authorization, sig string) (bool, string) {
		var out struct {
			Valid  bool   `json:"isValid"`
			Reason string `json:"invalidReason"`
		}
		if err := c.do(ctx, http.MethodPost, "/v1/x402/verify", nil, submission(res, req, Payload{a, sig}), &out); err != nil {
			t.Fatal(err)
		}
		return out.Valid, out.Reason
	}

	sig := sign(auth)
	if ok, why := verify(auth, sig); !ok {
		t.Fatalf("Circle's testnet verifier refused our signature: %s", why)
	}

	// Settle, through the production method, with a key that holds nothing:
	// Gateway must read the body, accept the signature, and refuse on balance
	// alone. Nothing is locked because there is nothing to lock.
	st, err := c.Settle(ctx, res, req, Payload{auth, sig})
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	if st.Success || st.Reason != "insufficient_balance" {
		t.Fatalf("settle from an empty wallet answered %+v, want success=false insufficient_balance", st)
	}

	// The same signature over a different value must be refused.
	tampered := auth
	tampered.Value = "2000000"
	req.Amount = tampered.Value
	if ok, why := verify(tampered, sig); ok {
		t.Fatal("Circle accepted a signature over a different value — the verifier is not reading it")
	} else if why != "invalid_signature" {
		t.Fatalf("tampered copy refused as %q, want invalid_signature", why)
	}
}
