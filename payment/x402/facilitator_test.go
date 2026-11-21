// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package x402

import (
	"crypto/ecdsa"
	"testing"

	"github.com/luxfi/crypto"
)

type keySigner struct{ key *ecdsa.PrivateKey }

func (k keySigner) Address() string { return Hex(crypto.PubkeyToAddress(k.key.PublicKey).Bytes()) }

func (k keySigner) SignDigest(d [32]byte) ([]byte, error) { return crypto.Sign(d[:], k.key) }

// What the client signs is what the facilitator verifies: one EIP-3009
// encoding on both sides, under the token's domain, and every field bound.
func TestTheFacilitatorVerifiesWhatTheClientSigned(t *testing.T) {
	key, _ := crypto.HexToECDSA(refKey)
	c := NewClient(ClientConfig{Signer: keySigner{key}, TokenName: usdcDomain.Name})
	f := NewFacilitator(FacilitatorConfig{TokenName: usdcDomain.Name})
	req := &PaymentRequest{ChainID: usdcDomain.ChainID, Token: usdcDomain.VerifyingContract,
		Facilitator: "0x7e57000000000000000000000000000000000001", Amount: "1000000", ValidFor: 300}
	auth, err := c.createAuthorization(req)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.verifyAuthorization(req, auth); err != nil {
		t.Fatalf("a payment the client signed: %v", err)
	}
	for name, edit := range map[string]func(*PaymentAuthorization, *PaymentRequest){
		"value": func(a *PaymentAuthorization, _ *PaymentRequest) { a.Value = "1000001" },
		"payee": func(a *PaymentAuthorization, _ *PaymentRequest) {
			a.To = "0x" + "22222222222222222222222222222222222222ff"
		},
		"window": func(a *PaymentAuthorization, _ *PaymentRequest) { a.ValidBefore++ },
		"chain":  func(_ *PaymentAuthorization, r *PaymentRequest) { r.ChainID = 8453 },
		"token": func(_ *PaymentAuthorization, r *PaymentRequest) {
			r.Token = "0x833589fcd6edb6e08f4c7c32d4f71b54bda02913"
		},
		"nonce": func(a *PaymentAuthorization, _ *PaymentRequest) { a.Nonce = "0x01" },
	} {
		a, r := *auth, *req
		edit(&a, &r)
		if err := f.verifyAuthorization(&r, &a); err == nil {
			t.Errorf("%s changed and the signature still verified", name)
		}
	}
}
