// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package circle

import (
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/luxfi/crypto"
)

// The reference vector was produced by eth-account 0.13.7 (encode_typed_data +
// sign_message), not by this package, for the well-known eth-account docs key
// 0x4c0883a6…2318. Its domain is Gateway's batched domain on Base Sepolia, its
// payee is a test address.
var (
	refKey       = "4c0883a69102937d6231471b5dbb6204fe5129617082792ae468d01a3f362318"
	refAddress   = "0x2c7536e3605d9c16a7a3d7b1898e529396a65c23"
	refDomainSep = "c3742368491d62d35db4f2606f8715a4814d22826819aa828e2b09d0e1c64f25"
	refDigest    = "73ef2f5d70f93a6941e2aa133074293af7cd0a6bdd0e356b2512b1184ec57d46"
	refSignature = "0xf57147202312319938cc69f1171dafa3502e8db9ca64de50897218309e94574e1254c32f30d973bf2c9f132822cd12f1200a3eea1c7164ad50f839d948856f601b"

	refDomain = Domain{Name: Name, Version: Release, ChainID: 84532, VerifyingContract: "0x0077777d7EBA4688BDeF3E311b846F25870A19B9"}
	refAuth   = Authorization{
		From:        refAddress,
		To:          "0x7e57000000000000000000000000000000000001",
		Value:       "1000000",
		ValidAfter:  "1700000000",
		ValidBefore: "1700604900",
		Nonce:       "0x" + strings.Repeat("11", 32),
	}
)

func TestTypeHashesAreTheStandardOnes(t *testing.T) {
	// EIP-3009's TRANSFER_WITH_AUTHORIZATION_TYPEHASH as USDC's FiatTokenV2
	// publishes it, and EIP-712's domain typehash for the four-field domain.
	cases := map[string]string{
		"TransferWithAuthorization": "7c7c6cdb67a18743f49ec6fa9b35f50d52ed05cbed4cc592e13b44501c1a2267",
		"EIP712Domain":              "8b73c3c69bb8fe3d512ecc4cf759cc79239f7b179b0ffacaa9a75d522b39400f",
	}
	for name, want := range cases {
		if got := hex.EncodeToString(typeHash(name)); got != want {
			t.Errorf("typehash(%s) = %s, want %s (encodeType %q)", name, got, want, encoding(name))
		}
	}
}

func TestDigestMatchesAnIndependentImplementation(t *testing.T) {
	p, err := refAuth.Parse()
	if err != nil {
		t.Fatal(err)
	}
	d, err := refDomain.Digest(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(d[:]); got != refDigest {
		t.Fatalf("digest = %s, eth-account computed %s", got, refDigest)
	}
	signer, err := Recover(d, refSignature)
	if err != nil {
		t.Fatal(err)
	}
	if got := Hex(signer[:]); got != refAddress {
		t.Fatalf("recovered %s, eth-account signed as %s", got, refAddress)
	}
}

func TestDomainSeparatorMatches(t *testing.T) {
	contract, _ := Address(refDomain.VerifyingContract)
	var dom []byte
	dom = append(dom, typeHash("EIP712Domain")...)
	dom = append(dom, crypto.Keccak256([]byte(refDomain.Name))...)
	dom = append(dom, crypto.Keccak256([]byte(refDomain.Version))...)
	dom = append(dom, word(big.NewInt(refDomain.ChainID))...)
	dom = append(dom, pad(contract[:])...)
	if got := hex.EncodeToString(crypto.Keccak256(dom)); got != refDomainSep {
		t.Fatalf("domain separator = %s, eth-account computed %s", got, refDomainSep)
	}
}

// Every signed field must reach the digest: a field left out of the encoding
// would let a payer's signature cover a payment it never described.
func TestEveryFieldIsSigned(t *testing.T) {
	p0, _ := refAuth.Parse()
	base, _ := refDomain.Digest(p0)
	mutate := map[string]func(*Authorization, *Domain){
		"from":              func(a *Authorization, _ *Domain) { a.From = "0x" + strings.Repeat("22", 20) },
		"to":                func(a *Authorization, _ *Domain) { a.To = "0x" + strings.Repeat("33", 20) },
		"value":             func(a *Authorization, _ *Domain) { a.Value = "1000001" },
		"validAfter":        func(a *Authorization, _ *Domain) { a.ValidAfter = "1700000001" },
		"validBefore":       func(a *Authorization, _ *Domain) { a.ValidBefore = "1700604901" },
		"nonce":             func(a *Authorization, _ *Domain) { a.Nonce = "0x" + strings.Repeat("12", 32) },
		"name":              func(_ *Authorization, d *Domain) { d.Name = "GatewayWallet" },
		"version":           func(_ *Authorization, d *Domain) { d.Version = "2" },
		"chainId":           func(_ *Authorization, d *Domain) { d.ChainID = 8453 },
		"verifyingContract": func(_ *Authorization, d *Domain) { d.VerifyingContract = "0x77777777Dcc4d5A8B6E418Fd04D8997ef11000eE" },
	}
	for name, fn := range mutate {
		a, d := refAuth, refDomain
		fn(&a, &d)
		p, err := a.Parse()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got, err := d.Digest(p)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got == base {
			t.Errorf("changing %s did not change the digest", name)
		}
	}
}

func TestRecoverAcceptsBothRecoveryIDSpellings(t *testing.T) {
	key, _ := crypto.HexToECDSA(refKey)
	p, _ := refAuth.Parse()
	d, _ := refDomain.Digest(p)
	sig, err := crypto.Sign(d[:], key)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []byte{sig[64], sig[64] + 27} {
		s := append(append([]byte{}, sig[:64]...), v)
		got, err := Recover(d, Hex(s))
		if err != nil {
			t.Fatalf("v=%d: %v", v, err)
		}
		if Hex(got[:]) != refAddress {
			t.Fatalf("v=%d recovered %s", v, Hex(got[:]))
		}
	}
}

// (r, n−s, v^1) is a second valid signature over the same digest. Gateway's
// verifier refuses it, so this does too.
func TestRecoverRefusesHighS(t *testing.T) {
	sig, _ := hexBytes(refSignature)
	n := crypto.S256().Params().N
	s := new(big.Int).SetBytes(sig[32:64])
	high := new(big.Int).Sub(n, s)
	flipped := make([]byte, 65)
	copy(flipped, sig[:32])
	high.FillBytes(flipped[32:64])
	flipped[64] = (sig[64] - 27) ^ 1 // the other recovery id, 0/1 form
	d, _ := hex.DecodeString(refDigest)
	var digest [32]byte
	copy(digest[:], d)

	// The twin really is a valid signature by the same key — so the refusal
	// below can only be the low-s rule, not a malformed input.
	pub, err := crypto.SigToPub(digest[:], flipped)
	if err != nil || Hex(crypto.PubkeyToAddress(*pub).Bytes()) != refAddress {
		t.Fatalf("the high-s twin does not recover to the signer (err %v); the test is not testing low-s", err)
	}
	if _, err := Recover(digest, Hex(flipped)); err == nil {
		t.Fatal("a high-s signature was accepted")
	}
}

func TestRecoverRefusesMalformedSignatures(t *testing.T) {
	var d [32]byte
	copy(d[:], []byte("digest"))
	good, _ := hexBytes(refSignature)
	bad := map[string]string{
		"empty":     "",
		"no prefix": refSignature[2:],
		"not hex":   "0x" + strings.Repeat("zz", 65),
		"64 bytes":  Hex(good[:64]),
		"66 bytes":  Hex(append(append([]byte{}, good...), 0)),
		"v=2":       Hex(append(append([]byte{}, good[:64]...), 2)),
		"v=29":      Hex(append(append([]byte{}, good[:64]...), 29)),
		"r=0":       Hex(append(make([]byte, 32), good[32:]...)),
		"s=0":       Hex(append(append(append([]byte{}, good[:32]...), make([]byte, 32)...), good[64])),
	}
	for name, sig := range bad {
		if _, err := Recover(d, sig); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestParseIsStrict(t *testing.T) {
	bad := map[string]func(*Authorization){
		"from no prefix":        func(a *Authorization) { a.From = strings.TrimPrefix(a.From, "0x") },
		"from short":            func(a *Authorization) { a.From = "0x" + strings.Repeat("ab", 19) },
		"to long":               func(a *Authorization) { a.To = "0x" + strings.Repeat("ab", 21) },
		"to not hex":            func(a *Authorization) { a.To = "0x" + strings.Repeat("zz", 20) },
		"value empty":           func(a *Authorization) { a.Value = "" },
		"value negative":        func(a *Authorization) { a.Value = "-1" },
		"value leading zero":    func(a *Authorization) { a.Value = "01000000" },
		"value decimal point":   func(a *Authorization) { a.Value = "1.5" },
		"value hex":             func(a *Authorization) { a.Value = "0x10" },
		"value plus":            func(a *Authorization) { a.Value = "+1" },
		"value past uint256":    func(a *Authorization) { a.Value = new(big.Int).Lsh(big.NewInt(1), 256).String() },
		"validAfter space":      func(a *Authorization) { a.ValidAfter = " 1" },
		"validBefore past 2^64": func(a *Authorization) { a.ValidBefore = "18446744073709551616" },
		"validBefore negative":  func(a *Authorization) { a.ValidBefore = "-5" },
		"nonce 31 bytes":        func(a *Authorization) { a.Nonce = "0x" + strings.Repeat("11", 31) },
		"nonce 33 bytes":        func(a *Authorization) { a.Nonce = "0x" + strings.Repeat("11", 33) },
		"nonce no prefix":       func(a *Authorization) { a.Nonce = strings.Repeat("11", 32) },
		"nonce short spelling":  func(a *Authorization) { a.Nonce = "0x01" },
	}
	for name, fn := range bad {
		a := refAuth
		fn(&a)
		if _, err := a.Parse(); err == nil {
			t.Errorf("%s: parsed", name)
		}
	}
	// The largest uint256 is a legal value.
	a := refAuth
	a.Value = maxWord.String()
	if _, err := a.Parse(); err != nil {
		t.Fatalf("2^256-1: %v", err)
	}
}

// A malformed verifyingContract or a non-positive chain id is an error, never
// a digest over zeroes.
func TestDigestRefusesABadDomain(t *testing.T) {
	p, _ := refAuth.Parse()
	for _, d := range []Domain{
		{Name: Name, Version: Release, ChainID: 84532, VerifyingContract: "0x1234"},
		{Name: Name, Version: Release, ChainID: 0, VerifyingContract: refDomain.VerifyingContract},
		{Name: Name, Version: Release, ChainID: -1, VerifyingContract: refDomain.VerifyingContract},
	} {
		if _, err := d.Digest(p); err == nil {
			t.Errorf("%+v: digested", d)
		}
	}
}
