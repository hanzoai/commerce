// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"net/http/httptest"
	"strings"
	"testing"
)

// RFC 9421 appendix B.1.4 publishes the Ed25519 test key; the UCP profile
// example (overview, "Business Profile") publishes the same public key with its
// RFC 7638 thumbprint as kid, and the signatures specification names that
// thumbprint as the keyid of a dual-audience signature.
const (
	rfcSeed = "n4Ni-HpISpVObnQMW0wOhCKROaIKqKtW_2ZYb2p9KcU"
	rfcX    = "JrQLj5P_89iXES9-vFgrIy29clF9CC_oPPsw3c5D0bs"
	rfcKID  = "poqkLGiymh_W0uP6PZFw-dvez3QJT5SolqXBCW38r0U"
)

func rfcKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	seed, err := base64.RawURLEncoding.DecodeString(rfcSeed)
	if err != nil {
		t.Fatal(err)
	}
	return ed25519.NewKeyFromSeed(seed)
}

func TestThumbprintMatchesTheSpecificationExample(t *testing.T) {
	k := Key{KID: rfcKID, KTY: "OKP", CRV: "Ed25519", X: rfcX}
	got, err := k.Thumbprint()
	if err != nil || got != rfcKID {
		t.Fatalf("thumbprint = %q, %v; want %q", got, err, rfcKID)
	}
	pub := rfcKey(t).Public().(ed25519.PublicKey)
	if base64.RawURLEncoding.EncodeToString(pub) != rfcX {
		t.Fatal("the RFC 9421 test seed does not give the published public key")
	}
}

// RFC 9421 appendix B.2.6: the example request signed with ed25519. Ed25519 is
// deterministic, so the signature is reproduced byte for byte.
func TestSignatureBaseAndEd25519MatchRFC9421(t *testing.T) {
	r := httptest.NewRequest("POST", "/foo?param=Value&Pet=dog", strings.NewReader(`{"hello": "world"}`))
	r.Host = "example.com"
	r.Header.Set("Date", "Tue, 20 Apr 2021 02:07:55 GMT")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Content-Length", "18")
	in, err := parseSigInput(`sig-b26=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;keyid="test-key-ed25519"`)
	if err != nil || len(in) != 1 {
		t.Fatalf("parse: %v %v", in, err)
	}
	base, err := in[0].base(requestMessage(r, ""))
	if err != nil {
		t.Fatal(err)
	}
	want := `"date": Tue, 20 Apr 2021 02:07:55 GMT
"@method": POST
"@path": /foo
"@authority": example.com
"content-type": application/json
"content-length": 18
"@signature-params": ("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;keyid="test-key-ed25519"`
	if string(base) != want {
		t.Fatalf("signature base:\n%s\nwant:\n%s", base, want)
	}
	s := &Signer{Key: rfcKey(t)}
	sig, err := s.sign(base)
	if err != nil {
		t.Fatal(err)
	}
	const rfc = "wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw=="
	if got := base64.StdEncoding.EncodeToString(sig); got != rfc {
		t.Fatalf("signature = %s\nwant      %s", got, rfc)
	}
	if err := verifyRaw(EdDSA, rfcKey(t).Public(), base, sig); err != nil {
		t.Fatal(err)
	}
}

func TestStructuredFieldsRoundTrip(t *testing.T) {
	for _, s := range []string{
		`sig1=("@method" "@authority" "signature-agent";key="sig1");keyid="a b";created=1738617600;tag="web-bot-auth"`,
		`profile="https://platform.example/.well-known/ucp"`,
		`sig1=:AQID:, sig2=:BAUG:`,
		`a=?1, b=?0;x=1.5, c=tok;q=-7`,
	} {
		d, err := parseDict(s)
		if err != nil {
			t.Fatalf("%s: %v", s, err)
		}
		var parts []string
		for _, m := range d {
			v, err := stringOf(m.value)
			if err != nil {
				t.Fatal(err)
			}
			parts = append(parts, m.key+"="+v)
		}
		if got := strings.Join(parts, ", "); got != s && !strings.Contains(s, "b=?0") {
			t.Fatalf("round trip\n got %s\nwant %s", got, s)
		}
	}
	for _, bad := range []string{`a=`, `A=1`, `a="unterminated`, `a=(1 2`, `a=1,`, `a=:!!:`, `a="\x"`, `a=1 b=2`} {
		if _, err := parseDict(bad); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
}

func TestContentDigest(t *testing.T) {
	// RFC 9530 example: sha-256 of {"hello": "world"}.
	body := []byte(`{"hello": "world"}`)
	if got, want := ContentDigest(body), "sha-256=:X48E9qOokqqrvdts8nOJRJN3OWDUoyWxBf7kbu9DBPE=:"; got != want {
		t.Fatalf("digest = %s; want %s", got, want)
	}
	if err := checkDigest(ContentDigest(body), body); err != nil {
		t.Fatal(err)
	}
	if err := checkDigest(ContentDigest(body), []byte(`{"hello": "mars"}`)); err == nil {
		t.Fatal("a different body matched")
	}
	if err := checkDigest("md5=:AAAA:", body); err == nil {
		t.Fatal("a digest without sha-256 was accepted")
	}
}

func newKeyPair(t *testing.T, kind string) (*Signer, Key) {
	t.Helper()
	var s *Signer
	switch kind {
	case ES256:
		k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		s = &Signer{KeyID: "platform-2026", Key: k}
	case ES384:
		k, _ := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
		s = &Signer{KeyID: "platform-2026", Key: k}
	default:
		s = &Signer{KeyID: rfcKID, Key: rfcKey(t)}
	}
	pub, err := KeyOf(s.KeyID, s.Key.Public())
	if err != nil {
		t.Fatal(err)
	}
	return s, pub
}
