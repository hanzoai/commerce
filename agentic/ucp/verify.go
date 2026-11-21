// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"
)

// Keys resolves the verification keys of whoever signed a request: the default
// UCP lookup reads the keys[] of the profile the UCP-Agent header names
// (overview, "Identity Resolution Algorithm"). identity is the URL that supplied
// the keys; refresh asks for a fresh copy, which the verifier does once when a
// key id is unknown.
type Keys interface {
	Keys(ctx context.Context, r *http.Request, refresh bool) (identity string, keys []Key, err error)
}

// Verifier verifies RFC 9421 signatures (signatures, "REST Request
// Verification" and "REST Response Verification").
type Verifier struct {
	Keys Keys
	// Authority returns the authority the signer addressed, for a gateway that
	// rewrites Host; the default is the Host header.
	Authority func(*http.Request) string
	// Window, when positive, is the oldest age a created time may have, and makes
	// created mandatory. Created is optional on default UCP signatures and
	// enforcing a window is application-defined (RFC 9421 section 3.2.1).
	// With Window zero a signature without expires never expires; only the
	// Idempotency-Key, kept for Keep, stops a replay.
	Window time.Duration
	// Skew is the clock difference tolerated on created and expires.
	Skew  time.Duration
	clock func() time.Time
}

// Signed is a verified signature; Identity is the profile URL that supplied the
// verifying key.
type Signed struct {
	Label    string
	KeyID    string
	Tag      string
	Identity string
}

func (v *Verifier) now() time.Time {
	if v.clock != nil {
		return v.clock()
	}
	return time.Now()
}

// skip is why a candidate signature does not authenticate a message.
type skip struct {
	code string
	why  string
}

func skipf(code, format string, args ...any) *skip {
	return &skip{code: code, why: fmt.Sprintf(format, args...)}
}

func (s *skip) asError(keyid string) *Error {
	e := fail(s.code, "%s", s.why)
	if s.code == CodeSignatureInvalid && keyid != "" {
		e.Content = fmt.Sprintf("Signature verification failed for key kid=%s: %s", keyid, s.why)
	}
	return e
}

// Request verifies the signatures of a request whose body has been read. It is
// authenticated when one signature verifies and rejected when every signature is
// skipped or fails (overview, "Identity Resolution Algorithm").
func (v *Verifier) Request(ctx context.Context, r *http.Request, body []byte) (*Signed, error) {
	required := []string{"@method", "@authority", "@path"}
	if r.URL.RawQuery != "" || r.URL.ForceQuery {
		required = append(required, "@query")
	}
	if len(body) > 0 {
		required = append(required, "content-digest", "content-type")
	}
	for _, h := range []string{"Idempotency-Key", "UCP-Agent", "Webhook-Id", "Webhook-Timestamp", "Signature-Agent"} {
		if r.Header.Get(h) != "" {
			required = append(required, strings.ToLower(h))
		}
	}
	authority := ""
	if v.Authority != nil {
		authority = v.Authority(r)
	}
	m := requestMessage(r, authority)
	lookup := func(keyid string) (string, Key, *skip) { return v.lookup(ctx, r, keyid) }
	return v.authenticate(m, body, required, lookup)
}

// Response verifies the signatures of a response with the signer's published
// keys; the status, and the body when there is one, must be covered.
func (v *Verifier) Response(status int, h http.Header, body []byte, keys []Key) (*Signed, error) {
	required := []string{"@status"}
	if len(body) > 0 {
		required = append(required, "content-digest", "content-type")
	}
	lookup := func(keyid string) (string, Key, *skip) {
		for _, k := range keys {
			if k.KID == keyid && k.Usable() {
				return "", k, nil
			}
		}
		return "", Key{}, skipf(CodeKeyNotFound, "key %q is not in the signer's published key set", keyid)
	}
	return v.authenticate(&message{response: true, status: status, header: h}, body, required, lookup)
}

// authenticate tries each signature of the message in turn.
func (v *Verifier) authenticate(m *message, body []byte, required []string, lookup func(string) (string, Key, *skip)) (*Signed, error) {
	inputs := strings.Join(m.header.Values("Signature-Input"), ", ")
	sigs := strings.Join(m.header.Values("Signature"), ", ")
	if inputs == "" || sigs == "" {
		return nil, fail(CodeSignatureMissing, "no Signature-Input and Signature")
	}
	cands, err := parseSigInput(inputs)
	if err != nil {
		return nil, fail(CodeSignatureInvalid, "Signature-Input: %v", err)
	}
	sd, err := parseDict(sigs)
	if err != nil {
		return nil, fail(CodeSignatureInvalid, "Signature: %v", err)
	}
	var first *Error
	for _, c := range cands {
		sig, ok := sigBytes(sd, c.label)
		if !ok {
			continue
		}
		signed, sk := v.verify(m, body, c, sig, required, lookup)
		if sk == nil {
			return signed, nil
		}
		if first == nil {
			kid, _ := c.str("keyid")
			first = sk.asError(kid)
		}
	}
	if first == nil {
		return nil, fail(CodeSignatureMissing, "no Signature matches a Signature-Input label")
	}
	return nil, first
}

func sigBytes(d dict, label string) ([]byte, bool) {
	v, ok := d.get(label)
	if !ok {
		return nil, false
	}
	it, ok := v.(item)
	if !ok {
		return nil, false
	}
	b, ok := it.value.([]byte)
	return b, ok
}

// verify checks one signature: the key, the covered components, the body digest,
// the freshness parameters and the signature itself.
func (v *Verifier) verify(m *message, body []byte, c sigInput, sig []byte, required []string, lookup func(string) (string, Key, *skip)) (*Signed, *skip) {
	keyid, ok := c.str("keyid")
	if !ok || keyid == "" {
		return nil, skipf(CodeKeyNotFound, "the signature has no keyid")
	}
	tag, hasTag := c.str("tag")
	if hasTag && tag != "web-bot-auth" {
		return nil, skipf(CodeSignatureInvalid, "signatures tagged %q are not UCP signatures", tag)
	}
	identity, key, sk := lookup(keyid)
	if sk != nil {
		return nil, sk
	}
	if tag == "web-bot-auth" {
		if tp, err := key.Thumbprint(); err != nil || tp != keyid {
			return nil, skipf(CodeSignatureInvalid, "a web-bot-auth keyid must be the RFC 7638 thumbprint of its key")
		}
	}
	alg := key.Algorithm()
	if alg == "" {
		return nil, skipf(CodeAlgorithmUnsupported, "key %q (kty %s crv %s) is not supported", keyid, key.KTY, key.CRV)
	}
	pub, err := key.Public()
	if err != nil {
		return nil, skipf(CodeAlgorithmUnsupported, "%v", err)
	}
	for _, name := range required {
		if !c.covers(name) {
			return nil, skipf(CodeSignatureInvalid, "the signature does not cover %s", name)
		}
	}
	if c.covers("content-digest") {
		if err := checkDigest(m.header.Get("Content-Digest"), body); err != nil {
			return nil, skipf(CodeDigestMismatch, "%v", err)
		}
	}
	if sk := v.fresh(c, tag); sk != nil {
		return nil, sk
	}
	base, err := c.base(m)
	if err != nil {
		return nil, skipf(CodeSignatureInvalid, "%v", err)
	}
	if err := verifyRaw(alg, pub, base, sig); err != nil {
		return nil, skipf(CodeSignatureInvalid, "%v", err)
	}
	return &Signed{Label: c.label, KeyID: keyid, Tag: tag, Identity: identity}, nil
}

// lookup finds the verification key, refreshing the key set once on an unknown
// key id (overview, "Fetching", rule 6).
func (v *Verifier) lookup(ctx context.Context, r *http.Request, keyid string) (string, Key, *skip) {
	for _, refresh := range []bool{false, true} {
		identity, keys, err := v.Keys.Keys(ctx, r, refresh)
		if err != nil {
			var e *Error
			if errors.As(err, &e) {
				return "", Key{}, &skip{code: e.Code, why: e.Content}
			}
			return "", Key{}, skipf(CodeProfileUnreachable, "%v", err)
		}
		for _, k := range keys {
			if k.KID == keyid && k.Usable() {
				return identity, k, nil
			}
		}
	}
	return "", Key{}, skipf(CodeKeyNotFound, "key %q is not in the signer's published key set", keyid)
}

// fresh enforces created and expires: expires is always honoured, Window bounds
// the age of created and makes it mandatory, and a Web Bot Auth signature must
// carry both ("WBA Interop", item 5).
func (v *Verifier) fresh(c sigInput, tag string) *skip {
	now := v.now()
	cr, hasCreated, err := c.intParam("created")
	if err != nil {
		return skipf(CodeSignatureInvalid, "%v", err)
	}
	ex, hasExpires, err := c.intParam("expires")
	if err != nil {
		return skipf(CodeSignatureInvalid, "%v", err)
	}
	if tag == "web-bot-auth" && (!hasCreated || !hasExpires) {
		return skipf(CodeSignatureInvalid, "a web-bot-auth signature must carry created and expires")
	}
	if v.Window > 0 && !hasCreated {
		return skipf(CodeSignatureInvalid, "the signature carries no created time")
	}
	if hasCreated {
		created := time.Unix(cr, 0)
		if created.After(now.Add(v.Skew)) {
			return skipf(CodeSignatureInvalid, "the signature was created in the future")
		}
		if v.Window > 0 && now.Sub(created) > v.Window+v.Skew {
			return skipf(CodeSignatureInvalid, "the signature is older than %s", v.Window)
		}
	}
	if hasExpires && now.After(time.Unix(ex, 0).Add(v.Skew)) {
		return skipf(CodeSignatureInvalid, "the signature has expired")
	}
	return nil
}

// verifyRaw verifies a raw signature: ECDSA r||s of fixed width, or Ed25519.
func verifyRaw(alg string, pub any, base, sig []byte) error {
	errBad := errors.New("signature does not verify")
	switch alg {
	case EdDSA:
		k, ok := pub.(ed25519.PublicKey)
		if !ok || len(sig) != ed25519.SignatureSize || !ed25519.Verify(k, base, sig) {
			return errBad
		}
	case ES256, ES384:
		k, ok := pub.(*ecdsa.PublicKey)
		size, digest := 32, sha256.Sum256(base)
		sum := digest[:]
		if alg == ES384 {
			d := sha512.Sum384(base)
			size, sum = 48, d[:]
		}
		if !ok || len(sig) != 2*size || !ecdsa.Verify(k, sum, new(big.Int).SetBytes(sig[:size]), new(big.Int).SetBytes(sig[size:])) {
			return errBad
		}
	default:
		return errors.New("algorithm is not supported")
	}
	return nil
}
