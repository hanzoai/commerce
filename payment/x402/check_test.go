// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package x402

import (
	"context"
	"crypto/ecdsa"
	"strings"
	"testing"
	"time"

	"github.com/luxfi/crypto"
)

// The addresses below are test values with no meaning: a made-up token and
// payee, and the eth-account documentation key the digest vectors already use.
const (
	testToken = "0x00000000000000000000000000000000000a11c3"
	testPayee = "0x7e57000000000000000000000000000000000001"
)

func testKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, err := crypto.HexToECDSA(refKey)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func testAsset() Asset {
	return Asset{Network: "eip155:84532", Address: testToken, Name: "Test Dollar", Version: "2",
		Decimals: 6, PayTo: testPayee, MaxTimeoutSeconds: 60}
}

func testRequirements() PaymentRequirements { return testAsset().Requirements("10000", "") }

func signed(t *testing.T, req PaymentRequirements, after, before uint64) PaymentPayload {
	t.Helper()
	var nonce [32]byte
	for i := range nonce {
		nonce[i] = byte(i + 1)
	}
	p, err := Sign(req, testKey(t), nonce, after, before)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSignVerifyRoundTrip(t *testing.T) {
	req := testRequirements()
	p := signed(t, req, 0, 2000000000)
	if err := Verify(req, p); err != nil {
		t.Fatal(err)
	}
	if p.Payload.Authorization.From != refAddress && !strings.EqualFold(p.Payload.Authorization.From, refAddress) {
		t.Fatalf("from %s", p.Payload.Authorization.From)
	}
	// The signature is the spelling a token takes: lowercase hex, v 27 or 28.
	if s := p.Payload.Signature; s != strings.ToLower(s) || (!strings.HasSuffix(s, "1b") && !strings.HasSuffix(s, "1c")) {
		t.Fatalf("signature %s", s)
	}
}

// Every field of the offered terms, the echo of them, and the authorization is
// bound: changing any one is refused, and for the reason the spec names.
func TestVerifyRefusesEachTamper(t *testing.T) {
	req := testRequirements()
	good := func() PaymentPayload { return signed(t, req, 0, 2000000000) }
	cases := []struct {
		name   string
		mutate func(*PaymentPayload)
		want   string
	}{
		{"version", func(p *PaymentPayload) { p.X402Version = 1 }, ReasonInvalidVersion},
		{"echo scheme", func(p *PaymentPayload) { p.Accepted.Scheme = "upto" }, ReasonInvalidScheme},
		{"echo network", func(p *PaymentPayload) { p.Accepted.Network = "eip155:8453" }, ReasonInvalidNetwork},
		{"echo amount", func(p *PaymentPayload) { p.Accepted.Amount = "1" }, ReasonValueMismatch},
		{"echo asset", func(p *PaymentPayload) { p.Accepted.Asset = "0x0000000000000000000000000000000000000001" }, ReasonInvalidRequirements},
		{"echo payTo", func(p *PaymentPayload) { p.Accepted.PayTo = "0x0000000000000000000000000000000000000001" }, ReasonRecipientMismatch},
		{"echo domain name", func(p *PaymentPayload) { e := *p.Accepted.Extra; e.Name = "Other"; p.Accepted.Extra = &e }, ReasonInvalidRequirements},
		{"echo domain version", func(p *PaymentPayload) { e := *p.Accepted.Extra; e.Version = "1"; p.Accepted.Extra = &e }, ReasonInvalidRequirements},
		{"authorized payee", func(p *PaymentPayload) {
			p.Payload.Authorization.To = "0x0000000000000000000000000000000000000001"
		}, ReasonRecipientMismatch},
		{"authorized value", func(p *PaymentPayload) { p.Payload.Authorization.Value = "9999" }, ReasonValueMismatch},
		{"authorized value larger", func(p *PaymentPayload) { p.Payload.Authorization.Value = "10001" }, ReasonValueMismatch},
		{"validAfter", func(p *PaymentPayload) { p.Payload.Authorization.ValidAfter = "1" }, ReasonSignature},
		{"validBefore", func(p *PaymentPayload) { p.Payload.Authorization.ValidBefore = "2000000001" }, ReasonSignature},
		{"nonce", func(p *PaymentPayload) { p.Payload.Authorization.Nonce = "0x" + strings.Repeat("22", 32) }, ReasonSignature},
		{"payer", func(p *PaymentPayload) { p.Payload.Authorization.From = "0x0000000000000000000000000000000000000001" }, ReasonSignature},
		{"short nonce", func(p *PaymentPayload) { p.Payload.Authorization.Nonce = "0x01" }, ReasonInvalidPayload},
		{"signature", func(p *PaymentPayload) { p.Payload.Signature = "0x" + strings.Repeat("00", 65) }, ReasonSignature},
		{"signature length", func(p *PaymentPayload) { p.Payload.Signature = "0x1234" }, ReasonSignature},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := good()
			c.mutate(&p)
			if got := ReasonOf(Verify(req, p), ""); got != c.want {
				t.Fatalf("reason %q, want %q (%v)", got, c.want, Verify(req, p))
			}
		})
	}
}

// An absent assetTransferMethod means eip3009, so a client that omits it and a
// server that states it agree; any other method is refused, not guessed at.
func TestAssetTransferMethodDefaultsToEIP3009(t *testing.T) {
	req := testRequirements()
	p := signed(t, req, 0, 2000000000)
	echo := *p.Accepted.Extra
	echo.AssetTransferMethod = ""
	p.Accepted.Extra = &echo
	if err := Verify(req, p); err != nil {
		t.Fatalf("omitted method: %v", err)
	}
	other := req
	e := *req.Extra
	e.AssetTransferMethod = "permit2"
	other.Extra = &e
	if _, err := Sign(other, testKey(t), [32]byte{1}, 0, 1); ReasonOf(err, "") != ReasonInvalidScheme {
		t.Fatalf("permit2 signed: %v", err)
	}
	p.Accepted = other
	if got := ReasonOf(Verify(other, p), ""); got != ReasonInvalidScheme {
		t.Fatalf("permit2 verified: %q", got)
	}
}

func TestInWindowBoundsAreExclusive(t *testing.T) {
	a := refAuth
	a.ValidAfter, a.ValidBefore = "100", "200"
	at := func(s int64) error { return InWindow(a, time.Unix(s, 0)) }
	for s, want := range map[int64]string{99: ReasonValidAfter, 100: ReasonValidAfter, 101: "", 199: "", 200: ReasonValidBefore, 201: ReasonValidBefore} {
		if got := ReasonOf(at(s), ""); got != want {
			t.Errorf("t=%d: %q, want %q", s, got, want)
		}
	}
}

func TestSignerNeverSpendsUnlessAllowed(t *testing.T) {
	required := PaymentRequired{X402Version: Version, Resource: ResourceInfo{URL: "https://x.test/a"}, Accepts: []PaymentRequirements{testRequirements()}}
	if _, err := (Signer{Key: testKey(t)}).Pay(context.Background(), required); err != ErrDeclined {
		t.Fatalf("no Allow: %v", err)
	}
	var seen PaymentRequirements
	s := Signer{Key: testKey(t), Now: func() time.Time { return time.Unix(1000, 0) },
		Allow: func(r ResourceInfo, req PaymentRequirements) bool { seen = req; return false }}
	if _, err := s.Pay(context.Background(), required); err != ErrDeclined || seen.Amount != "10000" {
		t.Fatalf("declined: %v %+v", err, seen)
	}
	s.Allow = func(ResourceInfo, PaymentRequirements) bool { return true }
	p, err := s.Pay(context.Background(), required)
	if err != nil {
		t.Fatal(err)
	}
	a := p.Payload.Authorization
	if a.ValidAfter != "0" || a.ValidBefore != "1060" {
		t.Fatalf("window %s..%s", a.ValidAfter, a.ValidBefore)
	}
	if err := Verify(required.Accepts[0], p); err != nil {
		t.Fatal(err)
	}
	if p.Resource == nil || p.Resource.URL != "https://x.test/a" {
		t.Fatalf("resource %+v", p.Resource)
	}
	// An offer it cannot sign is skipped, not mangled.
	odd := required
	odd.Accepts = []PaymentRequirements{{Scheme: "upto"}, {Scheme: "exact", Network: "solana:abc"}}
	if _, err := s.Pay(context.Background(), odd); err != ErrDeclined {
		t.Fatalf("unsignable: %v", err)
	}
}

func TestAtomic(t *testing.T) {
	good := []struct {
		in       string
		decimals int
		want     string
	}{
		{"0.01", 6, "10000"}, {"1", 6, "1000000"}, {"1.5", 6, "1500000"}, {"0.000001", 6, "1"},
		{" 2.50 ", 6, "2500000"}, {"5", 0, "5"}, {".5", 6, "500000"}, {"123456789012345678.123456", 6, "123456789012345678123456"},
	}
	for _, c := range good {
		if got, err := Atomic(c.in, c.decimals); err != nil || got != c.want {
			t.Errorf("Atomic(%q,%d) = %q, %v; want %q", c.in, c.decimals, got, err, c.want)
		}
	}
	for _, in := range []string{"", ".", "0", "0.0", "-1", "+1", "1e3", "1,5", "0.0000001", "abc", "1.2.3", "0x10",
		"115792089237316195423570985008687907853269984665640564039457584007913129639936"} {
		if got, err := Atomic(in, 6); err == nil {
			t.Errorf("Atomic(%q) = %q, want an error", in, got)
		}
	}
	if _, err := Atomic("1.5", 0); err == nil {
		t.Error("fraction at 0 decimals")
	}
}

func TestAssetValidate(t *testing.T) {
	if err := testAsset().Validate(); err != nil {
		t.Fatal(err)
	}
	bad := map[string]func(*Asset){
		"network":   func(a *Asset) { a.Network = "solana:abc" },
		"address":   func(a *Asset) { a.Address = "0x12" },
		"payTo":     func(a *Asset) { a.PayTo = "" },
		"name":      func(a *Asset) { a.Name = "" },
		"version":   func(a *Asset) { a.Version = "" },
		"decimals":  func(a *Asset) { a.Decimals = -1 },
		"timeout":   func(a *Asset) { a.MaxTimeoutSeconds = 0 },
		"no domain": func(a *Asset) { a.Name, a.Version = "", "" },
	}
	for name, f := range bad {
		a := testAsset()
		f(&a)
		if err := a.Validate(); err == nil {
			t.Errorf("%s: validated", name)
		}
	}
}

func TestTable(t *testing.T) {
	if _, err := NewTable(); err == nil {
		t.Fatal("empty table")
	}
	other := testAsset()
	other.Network, other.Address, other.Decimals = "eip155:8453", "0x00000000000000000000000000000000000b0b0b", 18
	if _, err := NewTable(testAsset(), Asset{}); err == nil {
		t.Fatal("invalid asset accepted")
	}
	tab, err := NewTable(testAsset(), other)
	if err != nil {
		t.Fatal(err)
	}
	if tab.Priced("GET /a") {
		t.Fatal("priced before set")
	}
	for _, bad := range []Price{{Amount: ""}, {Amount: "0"}, {Amount: "0.0000001"}, {Amount: "1", PayTo: "nope"}} {
		if err := tab.Set("GET /a", bad); err == nil {
			t.Errorf("%+v set", bad)
		}
	}
	if err := tab.Set(" ", Price{Amount: "1"}); err == nil {
		t.Error("unnamed resource")
	}
	if err := tab.Set("GET /a", Price{Amount: "0.01", Description: "a", MimeType: "application/json"}); err != nil {
		t.Fatal(err)
	}
	if err := tab.Set("GET /b", Price{Amount: "2", PayTo: "0x0000000000000000000000000000000000000002"}); err != nil {
		t.Fatal(err)
	}
	accepts, info, ok := tab.Quote("GET /a")
	if !ok || len(accepts) != 2 || info.Description != "a" || info.MimeType != "application/json" {
		t.Fatalf("%v %+v %+v", ok, accepts, info)
	}
	if accepts[0].Amount != "10000" || accepts[1].Amount != "10000000000000000" || accepts[0].Network != "eip155:84532" || accepts[1].Network != "eip155:8453" {
		t.Fatalf("%+v", accepts)
	}
	if b, _, _ := tab.Quote("GET /b"); b[0].PayTo != "0x0000000000000000000000000000000000000002" || b[0].Amount != "2000000" {
		t.Fatalf("%+v", b)
	}
	rows := tab.Listing()
	if len(rows) != 2 || rows[0].Resource != "GET /a" || rows[1].Resource != "GET /b" || rows[0].Price != "0.01" {
		t.Fatalf("%+v", rows)
	}
	tab.Remove("GET /a")
	if tab.Priced("GET /a") || len(tab.Listing()) != 1 {
		t.Fatal("remove")
	}
	if rows := (&Table{}).Listing(); rows == nil {
		t.Fatal("listing must never be nil")
	}
}
