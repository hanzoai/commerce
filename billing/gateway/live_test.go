// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package gateway

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/luxfi/crypto"

	"github.com/hanzoai/commerce/billing/creditledger"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/remittance"
	"github.com/hanzoai/commerce/payment/x402"
	"github.com/hanzoai/commerce/util/test/ae"
)

// TestLiveBaseSepolia runs the whole rail against Base Sepolia: real USDC, a
// real relayer transaction, real confirmations. It moves testnet money, so it
// runs only when asked:
//
//	USDC_LIVE=1 USDC_LIVE_PAYER_KEY=<hex key holding test USDC> \
//	  [USDC_LIVE_RPC=https://sepolia.base.org] [USDC_LIVE_CENTS=500] \
//	  go test -count=1 -run Live -v ./billing/gateway
//
// The relayer and the treasury are derived from the payer's key, so a run
// needs no other secret and its USDC stays recoverable: the relayer must hold
// a little Base Sepolia ETH (the test names its address when it does not).
func TestLiveBaseSepolia(t *testing.T) {
	if os.Getenv("USDC_LIVE") != "1" {
		t.Skip("USDC_LIVE=1 runs this against Base Sepolia")
	}
	payer, err := crypto.HexToECDSA(strings.TrimPrefix(os.Getenv("USDC_LIVE_PAYER_KEY"), "0x"))
	if err != nil {
		t.Fatal("USDC_LIVE_PAYER_KEY must be a hex secp256k1 key")
	}
	derive := func(label string) (*ecdsa.PrivateKey, string) {
		k, err := crypto.ToECDSA(crypto.Keccak256([]byte("hanzo/usdc-live/"+label), crypto.FromECDSA(payer)))
		if err != nil {
			t.Fatal(err)
		}
		return k, x402.Hex(crypto.PubkeyToAddress(k.PublicKey).Bytes())
	}
	relayer, relayerAddr := derive("relayer")
	_, treasuryAddr := derive("treasury")
	from := x402.Hex(crypto.PubkeyToAddress(payer.PublicKey).Bytes())
	rpc := os.Getenv("USDC_LIVE_RPC")
	if rpc == "" {
		rpc = "https://sepolia.base.org"
	}
	cents := int64(500)
	if v := os.Getenv("USDC_LIVE_CENTS"); v != "" {
		n, ok := new(big.Int).SetString(v, 10)
		if !ok || !n.IsInt64() {
			t.Fatal("USDC_LIVE_CENTS must be whole cents")
		}
		cents = n.Int64()
	}

	c := ae.NewContext()
	t.Cleanup(c.Close)
	led := newBook()
	creditledger.Set(led)
	t.Cleanup(func() { creditledger.Set(nil) })

	chain := testnet[0]
	chain.RPC = rpc
	s := New(Config{Testnet: true, Treasury: treasuryAddr, Relayer: relayer, Chains: []Chain{chain}})
	org := &organization.Organization{}
	org.Name = "usdclive"
	ctx := context.Background()
	t.Logf("payer %s, relayer %s, treasury %s", from, relayerAddr, treasuryAddr)

	l, err := s.ready(ctx, chain.ID)
	if err != nil {
		t.Fatalf("Base Sepolia's USDC does not match the table: %v", err)
	}
	gas, err := l.rpc().NativeBalance(ctx, relayerAddr)
	if err != nil {
		t.Fatal(err)
	}
	held, _ := l.balance(ctx, from)
	t.Logf("payer holds %s USDC base units; relayer holds %s wei", held, gas)
	if gas.Sign() == 0 {
		t.Fatalf("the relayer %s holds no Base Sepolia ETH; fund it and run again", relayerAddr)
	}
	before, _ := l.balance(ctx, treasuryAddr)

	terms, err := s.Terms(ctx, org, org.Name)
	if err != nil {
		t.Fatalf("terms: %v", err)
	}
	now := time.Now()
	a := x402.Authorization{
		From: from, To: terms.PayTo,
		Value:       new(big.Int).Mul(big.NewInt(cents), big.NewInt(10_000)).String(),
		ValidAfter:  big.NewInt(now.Add(-time.Minute).Unix()).String(),
		ValidBefore: big.NewInt(now.Add(time.Hour).Unix()).String(),
		Nonce:       terms.Nonce,
	}
	sub := Submission{Network: chain.ID, Salt: terms.Salt, Authorization: a, Signature: signWith(t, payer, terms.Accepts[0].Domain, a)}

	v, err := s.Submit(ctx, org, org.Name, sub)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	t.Logf("submitted: %s %s tx %s", v.ID, v.Status, v.TxHash)
	deadline := time.Now().Add(5 * time.Minute)
	for v.Status != remittance.Succeeded && time.Now().Before(deadline) {
		time.Sleep(4 * time.Second)
		s.advance(ctx, v.ID)
		if v, err = s.Get(ctx, org, org.Name, v.ID); err != nil {
			t.Fatal(err)
		}
		t.Logf("read: %s, %d/%d confirmations, tx %s, credited %v", v.Status, v.Confirmations, v.RequiredConfirmations, v.TxHash, v.Credited)
	}
	if v.Status != remittance.Succeeded || !v.Credited {
		t.Fatalf("not credited: %+v", v)
	}
	after, _ := l.balance(ctx, treasuryAddr)
	moved := new(big.Int).Sub(after, before)
	n, _ := led.Balance(ctx, "usdclive", org.Name, "usd", true)
	t.Logf("treasury received %s base units; ledger holds %d cents", moved, n)
	if moved.Cmp(big.NewInt(cents*10_000)) != 0 || n != cents {
		t.Fatalf("moved %s, credited %d, want %d cents both", moved, n, cents)
	}
}

// TestLiveTheTokenTakesTheSignedCall asks Base Sepolia's USDC, by eth_call and
// with no gas, whether it would execute the exact transferWithAuthorization the
// relayer sends for a payment the rail verified — and refuse the same call with
// one field changed. Nothing is spent and no nonce is used.
//
//	USDC_LIVE=1 USDC_LIVE_PAYER_KEY=<hex key holding test USDC> go test -count=1 -run Live -v ./billing/gateway
func TestLiveTheTokenTakesTheSignedCall(t *testing.T) {
	if os.Getenv("USDC_LIVE") != "1" {
		t.Skip("USDC_LIVE=1 runs this against Base Sepolia")
	}
	payer, err := crypto.HexToECDSA(strings.TrimPrefix(os.Getenv("USDC_LIVE_PAYER_KEY"), "0x"))
	if err != nil {
		t.Fatal("USDC_LIVE_PAYER_KEY must be a hex secp256k1 key")
	}
	from := x402.Hex(crypto.PubkeyToAddress(payer.PublicKey).Bytes())
	relayer, _ := crypto.ToECDSA(crypto.Keccak256([]byte("hanzo/usdc-live/relayer"), crypto.FromECDSA(payer)))
	relayerAddr := x402.Hex(crypto.PubkeyToAddress(relayer.PublicKey).Bytes())
	chain := testnet[0]
	chain.RPC = "https://sepolia.base.org"
	if v := os.Getenv("USDC_LIVE_RPC"); v != "" {
		chain.RPC = v
	}
	s := New(Config{Testnet: true, Treasury: treasury, Relayer: relayer, Chains: []Chain{chain}})
	ctx := context.Background()
	l, err := s.ready(ctx, chain.ID)
	if err != nil {
		t.Fatalf("Base Sepolia's USDC does not match the table: %v", err)
	}
	salt := [32]byte{1}
	nonce := bind("usdclive", "usdclive", salt)
	now := time.Now()
	a := x402.Authorization{
		From: from, To: treasury, Value: "1000000",
		ValidAfter:  big.NewInt(now.Add(-time.Minute).Unix()).String(),
		ValidBefore: big.NewInt(now.Add(time.Hour).Unix()).String(),
		Nonce:       x402.Hex(nonce[:]),
	}
	sub := Submission{Network: chain.ID, Salt: x402.Hex(salt[:]), Authorization: a, Signature: signWith(t, payer, chain.Domain(), a)}
	c, err := verify(sub, []Chain{chain}, treasury, "usdclive", "usdclive")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	row := &remittance.Remittance{From: c.auth.From, To: c.auth.To, Value: c.auth.Value,
		ValidAfter: c.auth.ValidAfter, ValidBefore: c.auth.ValidBefore, Nonce: c.auth.Nonce, Signature: c.sig}
	data, err := transferCall(row)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.rpc().Call(ctx, relayerAddr, chain.USDC, data); err != nil {
		t.Fatalf("the token would refuse the call the relayer sends: %v", err)
	}
	t.Logf("Base Sepolia USDC %s would execute transferWithAuthorization from %s (relayer %s, %d bytes of calldata)", chain.USDC, from, relayerAddr, len(data))
	row.Value = "1000001"
	data, _ = transferCall(row)
	_, err = l.rpc().Call(ctx, relayerAddr, chain.USDC, data)
	if reason, _ := revert(errOr(err)); reason != ReasonSignature {
		t.Fatalf("a changed value: %v, want the token's invalid signature", err)
	}
	t.Logf("the same call with the value changed: %v", err)

	// More than the wallet holds, signed properly: the token names the balance.
	held, err := l.balance(ctx, from)
	if err != nil {
		t.Fatal(err)
	}
	a.Value = new(big.Int).Add(held, big.NewInt(10_000)).String()
	sub.Authorization, sub.Signature = a, signWith(t, payer, chain.Domain(), a)
	if c, err = verify(sub, []Chain{chain}, treasury, "usdclive", "usdclive"); err != nil {
		t.Fatalf("verify: %v", err)
	}
	row.Value, row.Signature = c.auth.Value, c.sig
	data, _ = transferCall(row)
	_, err = l.rpc().Call(ctx, relayerAddr, chain.USDC, data)
	if reason, _ := revert(errOr(err)); reason != ReasonBalance {
		t.Fatalf("more than the wallet holds: %v, want the token's balance refusal", err)
	}
	t.Logf("more than the wallet holds: %v", err)
}

func errOr(err error) error {
	if err == nil {
		return errors.New("accepted")
	}
	return err
}
