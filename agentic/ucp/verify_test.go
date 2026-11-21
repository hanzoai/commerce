// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/base64"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

// keyset is a Keys that serves a fixed key set.
type keyset struct {
	identity  string
	keys      []Key
	err       error
	refreshes int
}

func (k *keyset) Keys(_ context.Context, _ *http.Request, refresh bool) (string, []Key, error) {
	if refresh {
		k.refreshes++
	}
	return k.identity, k.keys, k.err
}

const body = `{"line_items":[{"item":{"id":"item_123"},"quantity":2}]}`

// signedRequest builds a request as a platform sends it and signs it.
func signedRequest(t *testing.T, s *Signer, o Options, mutate ...func(*http.Request)) *http.Request {
	t.Helper()
	r := httptest.NewRequest("POST", "https://merchant.example/ucp/v1/checkout-sessions", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("UCP-Agent", `profile="https://platform.example/.well-known/ucp"`)
	r.Header.Set("Idempotency-Key", goodKey)
	for _, m := range mutate {
		m(r)
	}
	if err := s.SignRequest(r, []byte(body), o); err != nil {
		t.Fatal(err)
	}
	return r
}

// signedWith signs a request with exactly the signature parameters given, which
// Signer never writes (a missing created, a decimal expires).
func signedWith(t *testing.T, s *Signer, params []param) *http.Request {
	t.Helper()
	r := httptest.NewRequest("POST", "https://merchant.example/ucp/v1/checkout-sessions", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("UCP-Agent", `profile="https://platform.example/.well-known/ucp"`)
	r.Header.Set("Idempotency-Key", goodKey)
	r.Header.Set("Content-Digest", ContentDigest([]byte(body)))
	si := sigInput{label: "sig1", params: params}
	for _, c := range []string{"@method", "@authority", "@path", "ucp-agent", "idempotency-key", "content-digest", "content-type"} {
		si.items = append(si.items, covered{name: c})
	}
	base, err := si.base(requestMessage(r, ""))
	if err != nil {
		t.Fatal(err)
	}
	sig, err := s.sign(base)
	if err != nil {
		t.Fatal(err)
	}
	line, _ := si.paramsLine()
	r.Header.Set("Signature-Input", "sig1="+line)
	r.Header.Set("Signature", "sig1=:"+base64.StdEncoding.EncodeToString(sig)+":")
	return r
}

func verifier(keys ...Key) (*Verifier, *keyset) {
	ks := &keyset{identity: "https://platform.example/.well-known/ucp", keys: keys}
	return &Verifier{Keys: ks, clock: func() time.Time { return t0 }}, ks
}

func clock(s *Signer, at time.Time) *Signer { s.clock = func() time.Time { return at }; return s }

func mustReject(t *testing.T, err error, code string, status int) {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("error = %v; want a UCP error %s", err, code)
	}
	if e.Code != code || e.Status != status {
		t.Fatalf("error = %s (%d): %s; want %s (%d)", e.Code, e.Status, e.Content, code, status)
	}
}

func TestSignedRequestsVerifyForEveryAlgorithm(t *testing.T) {
	for _, alg := range []string{ES256, ES384, EdDSA} {
		t.Run(alg, func(t *testing.T) {
			s, pub := newKeyPair(t, alg)
			clock(s, t0)
			v, _ := verifier(pub)
			r := signedRequest(t, s, Options{})
			got, err := v.Request(context.Background(), r, []byte(body))
			if err != nil {
				t.Fatal(err)
			}
			if got.KeyID != s.KeyID || got.Label != "sig1" || got.Identity != "https://platform.example/.well-known/ucp" {
				t.Fatalf("signed = %+v", got)
			}
			// The headers are the ones the specification names.
			in := r.Header.Get("Signature-Input")
			for _, c := range []string{`"@method"`, `"@authority"`, `"@path"`, `"ucp-agent"`, `"idempotency-key"`, `"content-digest"`, `"content-type"`} {
				if !strings.Contains(in, c) {
					t.Errorf("Signature-Input %s does not cover %s", in, c)
				}
			}
			if strings.Contains(in, "alg=") {
				t.Error("alg must not be in Signature-Input")
			}
		})
	}
}

func TestQueryAndGetRequestsAreCovered(t *testing.T) {
	s, pub := newKeyPair(t, ES256)
	clock(s, t0)
	v, _ := verifier(pub)
	r := httptest.NewRequest("GET", "https://merchant.example/ucp/v1/checkout-sessions/chk_1?expand=totals", nil)
	r.Header.Set("UCP-Agent", `profile="https://platform.example/.well-known/ucp"`)
	if err := s.SignRequest(r, nil, Options{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Header.Get("Signature-Input"), `"@query"`) {
		t.Fatal("@query is required when there is a query string")
	}
	if _, err := v.Request(context.Background(), r, nil); err != nil {
		t.Fatal(err)
	}
	r.URL.RawQuery = "expand=other"
	_, err := v.Request(context.Background(), r, nil)
	mustReject(t, err, CodeSignatureInvalid, 401)
}

func TestVerificationRejects(t *testing.T) {
	good, goodPub := newKeyPair(t, ES256)
	clock(good, t0)
	_, otherPub := newKeyPair(t, ES256) // same kid, another key
	otherPub.KID = good.KeyID

	type tc struct {
		name   string
		build  func(t *testing.T) *http.Request
		keys   []Key
		window time.Duration
		code   string
		status int
		body   string // the body the server read; defaults to the signed one
	}
	signed := func(o Options, m ...func(*http.Request)) func(*testing.T) *http.Request {
		return func(t *testing.T) *http.Request { return signedRequest(t, good, o, m...) }
	}
	cases := []tc{
		{name: "tampered body", build: signed(Options{}), keys: []Key{goodPub}, code: CodeDigestMismatch, status: 400,
			body: `{"line_items":[{"item":{"id":"item_123"},"quantity":200}]}`},
		{name: "tampered body with a recomputed digest", build: signed(Options{}), keys: []Key{goodPub}, code: CodeSignatureInvalid, status: 401,
			body: `{"line_items":[{"item":{"id":"item_123"},"quantity":200}]}`},
		{name: "wrong key", build: signed(Options{}), keys: []Key{otherPub}, code: CodeSignatureInvalid, status: 401},
		{name: "unknown key id", build: signed(Options{}), keys: []Key{{KID: "someone-else", KTY: goodPub.KTY, CRV: goodPub.CRV, X: goodPub.X, Y: goodPub.Y}}, code: CodeKeyNotFound, status: 401},
		{name: "no keys published", build: signed(Options{}), keys: nil, code: CodeKeyNotFound, status: 401},
		{name: "encryption key", build: signed(Options{}), keys: []Key{func() Key { k := goodPub; k.Use = "enc"; return k }()}, code: CodeKeyNotFound, status: 401},
		{name: "key without verify", build: signed(Options{}), keys: []Key{func() Key { k := goodPub; k.KeyOps = []string{"sign"}; return k }()}, code: CodeKeyNotFound, status: 401},
		{name: "unsupported key type", build: signed(Options{}), keys: []Key{{KID: good.KeyID, KTY: "RSA", X: "AQAB"}}, code: CodeAlgorithmUnsupported, status: 400},
		{name: "stale created", build: func(t *testing.T) *http.Request {
			return signedRequest(t, clock(&Signer{KeyID: good.KeyID, Key: good.Key}, t0.Add(-10*time.Minute)), Options{})
		}, keys: []Key{goodPub}, window: 5 * time.Minute, code: CodeSignatureInvalid, status: 401},
		{name: "created in the future", build: func(t *testing.T) *http.Request {
			return signedRequest(t, clock(&Signer{KeyID: good.KeyID, Key: good.Key}, t0.Add(time.Hour)), Options{})
		}, keys: []Key{goodPub}, code: CodeSignatureInvalid, status: 401},
		{name: "expired", build: func(t *testing.T) *http.Request {
			return signedRequest(t, clock(&Signer{KeyID: good.KeyID, Key: good.Key}, t0.Add(-2*time.Hour)), Options{Expires: time.Hour})
		}, keys: []Key{goodPub}, code: CodeSignatureInvalid, status: 401},
		{name: "a window needs created", build: func(t *testing.T) *http.Request { return signedWith(t, good, []param{{"keyid", good.KeyID}}) },
			keys: []Key{goodPub}, window: time.Minute, code: CodeSignatureInvalid, status: 401},
		{name: "unknown tag", build: signed(Options{Tag: "something-else"}), keys: []Key{goodPub}, code: CodeSignatureInvalid, status: 401},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := c.build(t)
			v, ks := verifier(c.keys...)
			v.Window = c.window
			b := body
			if c.body != "" {
				b = c.body
			}
			if c.name == "tampered body with a recomputed digest" {
				r.Header.Set("Content-Digest", ContentDigest([]byte(b)))
			}
			_, err := v.Request(context.Background(), r, []byte(b))
			mustReject(t, err, c.code, c.status)
			if c.code == CodeKeyNotFound && ks.refreshes != 1 {
				t.Fatalf("an unknown key id forces one refresh of the key set; got %d", ks.refreshes)
			}
		})
	}
}

func TestMessageChangedAfterSigningIsRejected(t *testing.T) {
	s, pub := newKeyPair(t, ES256)
	clock(s, t0)
	v, _ := verifier(pub)
	for name, mutate := range map[string]func(*http.Request){
		"method":          func(r *http.Request) { r.Method = "PUT" },
		"path":            func(r *http.Request) { r.URL.Path = "/ucp/v1/checkout-sessions/other" },
		"authority":       func(r *http.Request) { r.Host = "evil.example" },
		"idempotency key": func(r *http.Request) { r.Header.Set("Idempotency-Key", "6f1c1d5e-0f0a-4c55-9b61-9a1f1f9c9d10") },
		"ucp-agent":       func(r *http.Request) { r.Header.Set("UCP-Agent", `profile="https://attacker.example/profile"`) },
		"content type":    func(r *http.Request) { r.Header.Set("Content-Type", "application/json; charset=utf-16") },
	} {
		t.Run(name, func(t *testing.T) {
			r := signedRequest(t, s, Options{})
			mutate(r)
			_, err := v.Request(context.Background(), r, []byte(body))
			mustReject(t, err, CodeSignatureInvalid, 401)
		})
	}
}

func TestMissingSignatureHeadersAreSignatureMissing(t *testing.T) {
	s, pub := newKeyPair(t, ES256)
	clock(s, t0)
	v, _ := verifier(pub)
	r := signedRequest(t, s, Options{})
	r.Header.Del("Signature")
	_, err := v.Request(context.Background(), r, []byte(body))
	mustReject(t, err, CodeSignatureMissing, 401)
	r = signedRequest(t, s, Options{})
	r.Header.Del("Signature-Input")
	r.Header.Del("Signature")
	_, err = v.Request(context.Background(), r, []byte(body))
	mustReject(t, err, CodeSignatureMissing, 401)
	r = signedRequest(t, s, Options{})
	r.Header.Set("Signature-Input", "(((")
	_, err = v.Request(context.Background(), r, []byte(body))
	mustReject(t, err, CodeSignatureInvalid, 401)
}

// A signature must cover the target, the body, and every integrity-relevant
// header the request carries; one that does not is treated as unsigned.
func TestCoverageIsEnforced(t *testing.T) {
	s, pub := newKeyPair(t, ES256)
	clock(s, t0)
	v, _ := verifier(pub)
	full := []string{"@method", "@authority", "@path", "ucp-agent", "idempotency-key", "content-digest", "content-type"}
	for _, drop := range full {
		t.Run("without "+drop, func(t *testing.T) {
			r := httptest.NewRequest("POST", "https://merchant.example/ucp/v1/checkout-sessions", strings.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("UCP-Agent", `profile="https://platform.example/.well-known/ucp"`)
			r.Header.Set("Idempotency-Key", goodKey)
			r.Header.Set("Content-Digest", ContentDigest([]byte(body)))
			var items []covered
			for _, c := range full {
				if c != drop {
					items = append(items, covered{name: c})
				}
			}
			if err := s.emit(r.Header, requestMessage(r, ""), items, Options{}); err != nil {
				t.Fatal(err)
			}
			_, err := v.Request(context.Background(), r, []byte(body))
			mustReject(t, err, CodeSignatureInvalid, 401)
		})
	}
	t.Run("a header added after signing without coverage", func(t *testing.T) {
		r := httptest.NewRequest("POST", "https://merchant.example/ucp/v1/checkout-sessions", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if err := s.SignRequest(r, []byte(body), Options{}); err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Idempotency-Key", goodKey)
		_, err := v.Request(context.Background(), r, []byte(body))
		mustReject(t, err, CodeSignatureInvalid, 401)
	})
	t.Run("webhook headers", func(t *testing.T) {
		r := httptest.NewRequest("POST", "https://platform.example/webhooks", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Webhook-Id", "evt_1")
		r.Header.Set("Webhook-Timestamp", "1737000000")
		if err := s.SignRequest(r, []byte(body), Options{}); err != nil {
			t.Fatal(err)
		}
		if _, err := v.Request(context.Background(), r, []byte(body)); err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Webhook-Timestamp", "1737000001") // replay with another event time
		_, err := v.Request(context.Background(), r, []byte(body))
		mustReject(t, err, CodeSignatureInvalid, 401)
	})
}

func TestEcdsaSignaturesMustBeRawNotDER(t *testing.T) {
	s, pub := newKeyPair(t, ES256)
	clock(s, t0)
	v, _ := verifier(pub)
	r := signedRequest(t, s, Options{})
	raw, _, _ := strings.Cut(strings.TrimPrefix(r.Header.Get("Signature"), "sig1=:"), ":")
	sig, _ := base64.StdEncoding.DecodeString(raw)
	der, _ := asn1.Marshal(struct{ R, S *big.Int }{new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])})
	r.Header.Set("Signature", "sig1=:"+base64.StdEncoding.EncodeToString(der)+":")
	_, err := v.Request(context.Background(), r, []byte(body))
	mustReject(t, err, CodeSignatureInvalid, 401)
}

func TestRawEcdsaLengthIsFixed(t *testing.T) {
	for alg, size := range map[string]int{ES256: 64, ES384: 96} {
		s, _ := newKeyPair(t, alg)
		sig, err := s.sign([]byte("base"))
		if err != nil || len(sig) != size {
			t.Fatalf("%s signature is %d bytes (%v); want %d", alg, len(sig), err, size)
		}
	}
	s, _ := newKeyPair(t, EdDSA)
	if sig, _ := s.sign([]byte("base")); len(sig) != 64 {
		t.Fatalf("Ed25519 signature is %d bytes", len(sig))
	}
}

func TestMultipleSignaturesAreAuthenticatedWhenOneVerifies(t *testing.T) {
	good, goodPub := newKeyPair(t, ES256)
	clock(good, t0)
	bad, _ := newKeyPair(t, ES256)
	bad.KeyID = "unknown-key"
	clock(bad, t0)
	v, _ := verifier(goodPub)

	r := signedRequest(t, bad, Options{Label: "sigbad"})
	badIn, badSig := r.Header.Get("Signature-Input"), r.Header.Get("Signature")
	if err := good.SignRequest(r, []byte(body), Options{Label: "siggood"}); err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Signature-Input", badIn+", "+r.Header.Get("Signature-Input"))
	r.Header.Set("Signature", badSig+", "+r.Header.Get("Signature"))
	got, err := v.Request(context.Background(), r, []byte(body))
	if err != nil || got.Label != "siggood" {
		t.Fatalf("signed = %+v, %v", got, err)
	}
	// Only when every signature fails is the request rejected.
	r = signedRequest(t, bad, Options{})
	_, err = v.Request(context.Background(), r, []byte(body))
	mustReject(t, err, CodeKeyNotFound, 401)
}

// Web Bot Auth shape: Ed25519, the RFC 7638 thumbprint as keyid, created and
// expires, tag, and the Signature-Agent header covered as a dictionary member.
func TestWebBotAuthShape(t *testing.T) {
	s := &Signer{KeyID: rfcKID, Key: rfcKey(t), clock: func() time.Time { return t0 }}
	pub, _ := KeyOf(rfcKID, s.Key.Public())
	v, _ := verifier(pub)
	wba := func(m ...func(*http.Request)) *http.Request {
		return signedRequest(t, s, Options{Expires: time.Hour, Tag: "web-bot-auth"},
			append([]func(*http.Request){func(r *http.Request) {
				r.Header.Set("Signature-Agent", `sig1="https://platform.example/.well-known/ucp";type=jwks_uri`)
			}}, m...)...)
	}
	r := wba()
	in := r.Header.Get("Signature-Input")
	for _, want := range []string{`"signature-agent";key="sig1"`, `keyid="` + rfcKID + `"`, `tag="web-bot-auth"`, "created=", "expires="} {
		if !strings.Contains(in, want) {
			t.Errorf("Signature-Input %s lacks %s", in, want)
		}
	}
	got, err := v.Request(context.Background(), r, []byte(body))
	if err != nil || got.Tag != "web-bot-auth" {
		t.Fatalf("signed = %+v, %v", got, err)
	}

	// Signature-Agent changed after signing.
	r = wba()
	r.Header.Set("Signature-Agent", `sig1="https://attacker.example/.well-known/ucp";type=jwks_uri`)
	_, err = v.Request(context.Background(), r, []byte(body))
	mustReject(t, err, CodeSignatureInvalid, 401)

	// A keyid that is not the thumbprint of its key.
	liar := &Signer{KeyID: "not-the-thumbprint", Key: s.Key, clock: s.clock}
	lp, _ := KeyOf("not-the-thumbprint", liar.Key.Public())
	lv, _ := verifier(lp)
	r = httptest.NewRequest("POST", "https://merchant.example/ucp/v1/checkout-sessions", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("UCP-Agent", `profile="https://platform.example/.well-known/ucp"`)
	r.Header.Set("Signature-Agent", `sig1="https://platform.example/.well-known/ucp";type=jwks_uri`)
	items := []covered{{name: "@method"}, {name: "@authority"}, {name: "@path"}, {name: "signature-agent", params: []param{{"key", "sig1"}}}, {name: "ucp-agent"}, {name: "content-digest"}, {name: "content-type"}}
	r.Header.Set("Content-Digest", ContentDigest([]byte(body)))
	if err := liar.emit(r.Header, requestMessage(r, ""), items, Options{Expires: time.Hour, Tag: "web-bot-auth"}); err != nil {
		t.Fatal(err)
	}
	_, err = lv.Request(context.Background(), r, []byte(body))
	mustReject(t, err, CodeSignatureInvalid, 401)

	// The signer refuses to produce a non-conforming web-bot-auth signature.
	for name, o := range map[string]Options{"no expires": {Tag: "web-bot-auth"}} {
		r := httptest.NewRequest("POST", "https://merchant.example/x", strings.NewReader(body))
		r.Header.Set("Signature-Agent", `sig1="https://platform.example/.well-known/ucp";type=jwks_uri`)
		if err := s.SignRequest(r, []byte(body), o); err == nil {
			t.Errorf("%s: signed", name)
		}
	}
	r = httptest.NewRequest("POST", "https://merchant.example/x", strings.NewReader(body))
	if err := s.SignRequest(r, []byte(body), Options{Tag: "web-bot-auth", Expires: time.Hour}); err == nil {
		t.Error("web-bot-auth without Signature-Agent was signed")
	}
	bad := &Signer{KeyID: "kid", Key: s.Key}
	r = httptest.NewRequest("POST", "https://merchant.example/x", strings.NewReader(body))
	r.Header.Set("Signature-Agent", `sig1="https://platform.example/.well-known/ucp";type=jwks_uri`)
	if err := bad.SignRequest(r, []byte(body), Options{Tag: "web-bot-auth", Expires: time.Hour}); err == nil {
		t.Error("web-bot-auth with a keyid that is not the thumbprint was signed")
	}

	// Missing expires on the wire.
	r = httptest.NewRequest("POST", "https://merchant.example/ucp/v1/checkout-sessions", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("UCP-Agent", `profile="https://platform.example/.well-known/ucp"`)
	r.Header.Set("Signature-Agent", `sig1="https://platform.example/.well-known/ucp";type=jwks_uri`)
	r.Header.Set("Content-Digest", ContentDigest([]byte(body)))
	if err := s.emit(r.Header, requestMessage(r, ""), items, Options{Tag: "web-bot-auth"}); err != nil {
		t.Fatal(err)
	}
	_, err = v.Request(context.Background(), r, []byte(body))
	mustReject(t, err, CodeSignatureInvalid, 401)
}

func TestAuthorityOverrideServesAGateway(t *testing.T) {
	s, pub := newKeyPair(t, ES256)
	clock(s, t0)
	v, _ := verifier(pub)
	v.Authority = func(*http.Request) string { return "merchant.example" }
	r := signedRequest(t, s, Options{})
	r.Host = "10.1.2.3:8080" // the host the gateway forwards to
	if _, err := v.Request(context.Background(), r, []byte(body)); err != nil {
		t.Fatal(err)
	}
	v.Authority = nil
	_, err := v.Request(context.Background(), r, []byte(body))
	mustReject(t, err, CodeSignatureInvalid, 401)
}

func TestDefaultPortIsNotPartOfTheAuthority(t *testing.T) {
	m := &message{scheme: "https", authority: "Merchant.Example:443"}
	if got := m.authorityValue(); got != "merchant.example" {
		t.Fatalf("authority = %q", got)
	}
	m = &message{scheme: "https", authority: "merchant.example:8443"}
	if got := m.authorityValue(); got != "merchant.example:8443" {
		t.Fatalf("authority = %q", got)
	}
}

func TestResponsesAreSignedAndVerified(t *testing.T) {
	for _, alg := range []string{ES256, ES384, EdDSA} {
		t.Run(alg, func(t *testing.T) {
			s, pub := newKeyPair(t, alg)
			clock(s, t0)
			v, _ := verifier()
			payload := []byte(`{"id":"chk_123","status":"completed","order":{"id":"ord_456"}}`)
			h := http.Header{"Content-Type": {"application/json"}}
			if err := s.SignResponse(h, 200, payload, Options{}); err != nil {
				t.Fatal(err)
			}
			if in := h.Get("Signature-Input"); !strings.Contains(in, `("@status" "content-digest" "content-type")`) || !strings.Contains(in, "created=") {
				t.Fatalf("Signature-Input = %s", in)
			}
			if _, err := v.Response(200, h, payload, []Key{pub}); err != nil {
				t.Fatal(err)
			}
			_, err := v.Response(200, h, []byte(`{"id":"chk_123","status":"completed","order":{"id":"ord_999"}}`), []Key{pub})
			mustReject(t, err, CodeDigestMismatch, 400)
			_, err = v.Response(500, h, payload, []Key{pub}) // another status
			mustReject(t, err, CodeSignatureInvalid, 401)
			_, err = v.Response(200, h, payload, nil)
			mustReject(t, err, CodeKeyNotFound, 401)
			k2, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			wrong, _ := KeyOf(s.KeyID, k2.Public())
			_, err = v.Response(200, h, payload, []Key{wrong})
			mustReject(t, err, CodeSignatureInvalid, 401)
			h2 := http.Header{"Content-Type": {"application/json"}}
			_, err = v.Response(200, h2, payload, []Key{pub})
			mustReject(t, err, CodeSignatureMissing, 401)
		})
	}
}

func TestResponseSignatureMustCoverTheBody(t *testing.T) {
	s, pub := newKeyPair(t, ES256)
	clock(s, t0)
	v, _ := verifier()
	payload := []byte(`{"id":"chk_1"}`)
	h := http.Header{"Content-Type": {"application/json"}, "Content-Digest": {ContentDigest(payload)}}
	if err := s.emit(h, &message{response: true, status: 200, header: h}, []covered{{name: "@status"}}, Options{}); err != nil {
		t.Fatal(err)
	}
	_, err := v.Response(200, h, []byte(`{"id":"chk_2"}`), []Key{pub})
	mustReject(t, err, CodeSignatureInvalid, 401)
}

func TestAgentKeysResolveByTheUCPAgentProfile(t *testing.T) {
	s, pub := newKeyPair(t, ES256)
	clock(s, t0)
	pr := platform(Version, CapCheckout)
	pr.Keys = []Key{pub}
	ps := &peers{m: map[string]*Profile{"https://platform.example/.well-known/ucp": pr}}
	v := &Verifier{Keys: AgentKeys{Profiles: ps}, clock: func() time.Time { return t0 }}
	r := signedRequest(t, s, Options{})
	got, err := v.Request(context.Background(), r, []byte(body))
	if err != nil || got.Identity != "https://platform.example/.well-known/ucp" {
		t.Fatalf("signed = %+v, %v", got, err)
	}
	// An agent profile that cannot be fetched is profile_unreachable.
	r = signedRequest(t, s, Options{}, func(r *http.Request) { r.Header.Set("UCP-Agent", `profile="https://nowhere.example/.well-known/ucp"`) })
	_, err = v.Request(context.Background(), r, []byte(body))
	mustReject(t, err, CodeProfileUnreachable, 424)
	// A request with no UCP-Agent has no key to resolve.
	r = signedRequest(t, s, Options{}, func(r *http.Request) { r.Header.Del("UCP-Agent") })
	_, err = v.Request(context.Background(), r, []byte(body))
	mustReject(t, err, CodeInvalidProfileURL, 400)
}

func TestThumbprintOfAnECKey(t *testing.T) {
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	pub, _ := KeyOf("k", k.Public())
	tp, err := pub.Thumbprint()
	if err != nil {
		t.Fatal(err)
	}
	canon := `{"crv":"P-256","kty":"EC","x":"` + pub.X + `","y":"` + pub.Y + `"}`
	sum := sha256.Sum256([]byte(canon))
	if tp != base64.RawURLEncoding.EncodeToString(sum[:]) {
		t.Fatal("RFC 7638 canonical members are crv, kty, x, y")
	}
}

// created and expires are integers (RFC 9421 section 2.3). A value that is not
// one, such as expires=100.0, must not make the signature immortal.
func TestNonIntegerCreatedAndExpiresAreRefused(t *testing.T) {
	s, pub := newKeyPair(t, ES256)
	clock(s, t0)
	v, _ := verifier(pub)
	for name, params := range map[string][]param{
		"expires as a decimal": {{"created", t0.Unix()}, {"expires", float64(100)}, {"keyid", s.KeyID}},
		"created as a decimal": {{"created", float64(t0.Unix()) + 0.5}, {"keyid", s.KeyID}},
		"expires as a string":  {{"created", t0.Unix()}, {"expires", "soon"}, {"keyid", s.KeyID}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := v.Request(context.Background(), signedWith(t, s, params), []byte(body))
			mustReject(t, err, CodeSignatureInvalid, 401)
		})
	}
}
