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
// (overview, "Identity Resolution Algorithm"). identity is the URL that
// supplied the keys. refresh asks for a fresh copy rather than a cached one,
// which the verifier does once when a key id is unknown.
type Keys interface {
	Keys(ctx context.Context, r *http.Request, refresh bool) (identity string, keys []Key, err error)
}

// Verifier verifies RFC 9421 signatures on requests.
type Verifier struct {
	Keys Keys
	// Authority returns the authority the signer addressed, for a gateway that
	// rewrites Host. The default is the Host header.
	Authority func(*http.Request) string
	// Window, when positive, is the oldest age a signature's created time may
	// have, and makes created mandatory. Replay protection is otherwise the
	// business-layer Idempotency-Key; created is optional on default UCP
	// signatures and enforcing a window is application-defined (signatures,
	// "Replay Protection"; RFC 9421 section 3.2.1).
	//
	// With Window zero a signature that carries no expires parameter never
	// expires: a captured request verifies for as long as its key is published,
	// and what stops its replay is the Idempotency-Key, which a business keeps
	// for a bounded time (Keep). A business that cannot accept that sets Window
	// to no more than the time it keeps idempotency records.
	Window time.Duration
	// Skew is the clock difference tolerated on created and expires.
	Skew time.Duration
	// Now is the clock; it defaults to time.Now.
	Now func() time.Time
}

// Signed is a verified signature: the signer is the URL that supplied the
// verifying key.
type Signed struct {
	Label   string
	KeyID   string
	Tag     string
	Created time.Time
	// Identity is the profile URL that supplied the key (the UCP-Agent URL).
	Identity string
}

func (v *Verifier) now() time.Time {
	if v.Now != nil {
		return v.Now()
	}
	return time.Now()
}

// skip is the reason a candidate signature does not authenticate a message.
type skip struct {
	code string
	why  string
}

func (s *skip) Error() string { return s.code + ": " + s.why }

func skipf(code, format string, args ...any) *skip {
	return &skip{code: code, why: fmt.Sprintf(format, args...)}
}

func (s *skip) asError(keyid string) *Error {
	e := fail(s.code, "%s", s.why)
	if s.code == CodeSignatureInvalid && keyid != "" {
		e.Content = fmt.Sprintf("Request signature verification failed for key kid=%s: %s", keyid, s.why)
	}
	return e
}

// Request verifies the signatures of a request whose body has been read. A
// request is authenticated when at least one signature verifies; it is rejected
// only when every signature is skipped or fails (overview, "Identity
// Resolution Algorithm"; signatures, "REST Request Verification").
func (v *Verifier) Request(ctx context.Context, r *http.Request, body []byte) (*Signed, error) {
	inputs := strings.Join(r.Header.Values("Signature-Input"), ", ")
	sigs := strings.Join(r.Header.Values("Signature"), ", ")
	if inputs == "" || sigs == "" {
		return nil, fail(CodeSignatureMissing, "the request carries no Signature-Input and Signature")
	}
	cands, err := parseSigInput(inputs)
	if err != nil {
		return nil, fail(CodeSignatureInvalid, "Signature-Input: %v", err)
	}
	sd, err := parseDict(sigs)
	if err != nil {
		return nil, fail(CodeSignatureInvalid, "Signature: %v", err)
	}
	if len(cands) == 0 {
		return nil, fail(CodeSignatureMissing, "Signature-Input names no signature")
	}
	var first *Error
	for _, c := range cands {
		raw, ok := sigBytes(sd, c.label)
		if !ok {
			continue
		}
		got, sk := v.candidate(ctx, r, body, c, raw)
		if sk == nil {
			return got, nil
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

// candidate verifies one signature; a non-nil skip says why it does not
// authenticate the request.
func (v *Verifier) candidate(ctx context.Context, r *http.Request, body []byte, c sigInput, sig []byte) (*Signed, *skip) {
	keyid, ok := c.str("keyid")
	if !ok || keyid == "" {
		return nil, skipf(CodeKeyNotFound, "the signature has no keyid")
	}
	tag, hasTag := c.str("tag")
	if hasTag && tag != "web-bot-auth" {
		return nil, skipf(CodeSignatureInvalid, "signatures tagged %q are not UCP signatures", tag)
	}

	identity, key, sk := v.key(ctx, r, keyid)
	if sk != nil {
		return nil, sk
	}
	if tag == "web-bot-auth" {
		tp, err := key.Thumbprint()
		if err != nil || tp != keyid {
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

	// Covered-component enforcement, in every regime: the request target, the
	// body when present, and each integrity-relevant header the request carries.
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
	for _, name := range required {
		if !c.covers(name) {
			return nil, skipf(CodeSignatureInvalid, "the signature does not cover %s", name)
		}
	}

	if c.covers("content-digest") {
		if err := checkDigest(r.Header.Get("Content-Digest"), body); err != nil {
			return nil, skipf(CodeDigestMismatch, "%v", err)
		}
	}
	created, sk := v.fresh(c, tag)
	if sk != nil {
		return nil, sk
	}

	authority := ""
	if v.Authority != nil {
		authority = v.Authority(r)
	}
	base, err := c.base(requestMessage(r, authority))
	if err != nil {
		return nil, skipf(CodeSignatureInvalid, "%v", err)
	}
	if err := verifyRaw(alg, pub, base, sig); err != nil {
		return nil, skipf(CodeSignatureInvalid, "%v", err)
	}
	return &Signed{Label: c.label, KeyID: keyid, Tag: tag, Created: created, Identity: identity}, nil
}

// key finds the verification key, refreshing the key set once on an unknown
// key id (overview, "Fetching", rule 6).
func (v *Verifier) key(ctx context.Context, r *http.Request, keyid string) (string, Key, *skip) {
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

// fresh enforces the created and expires parameters. expires is always
// honoured; Window makes created mandatory and bounds its age; a Web Bot Auth
// signature must carry both (signatures, "WBA Interop", item 5).
func (v *Verifier) fresh(c sigInput, tag string) (time.Time, *skip) {
	now := v.now()
	cr, hasCreated, err := c.intParam("created")
	if err != nil {
		return time.Time{}, skipf(CodeSignatureInvalid, "%v", err)
	}
	ex, hasExpires, err := c.intParam("expires")
	if err != nil {
		return time.Time{}, skipf(CodeSignatureInvalid, "%v", err)
	}
	if tag == "web-bot-auth" && (!hasCreated || !hasExpires) {
		return time.Time{}, skipf(CodeSignatureInvalid, "a web-bot-auth signature must carry created and expires")
	}
	if v.Window > 0 && !hasCreated {
		return time.Time{}, skipf(CodeSignatureInvalid, "the signature carries no created time")
	}
	var created time.Time
	if hasCreated {
		created = time.Unix(cr, 0)
		if created.After(now.Add(v.Skew)) {
			return time.Time{}, skipf(CodeSignatureInvalid, "the signature was created in the future")
		}
		if v.Window > 0 && now.Sub(created) > v.Window+v.Skew {
			return time.Time{}, skipf(CodeSignatureInvalid, "the signature is older than %s", v.Window)
		}
	}
	if hasExpires && now.After(time.Unix(ex, 0).Add(v.Skew)) {
		return time.Time{}, skipf(CodeSignatureInvalid, "the signature has expired")
	}
	return created, nil
}

// verifyRaw verifies a raw signature: ECDSA r||s of fixed width, or Ed25519.
func verifyRaw(alg string, pub any, base, sig []byte) error {
	switch alg {
	case EdDSA:
		k, ok := pub.(ed25519.PublicKey)
		if !ok || len(sig) != ed25519.SignatureSize || !ed25519.Verify(k, base, sig) {
			return errors.New("signature does not verify")
		}
		return nil
	case ES256, ES384:
		k, ok := pub.(*ecdsa.PublicKey)
		size := 32
		var digest []byte
		if alg == ES384 {
			size = 48
			d := sha512.Sum384(base)
			digest = d[:]
		} else {
			d := sha256.Sum256(base)
			digest = d[:]
		}
		if !ok || len(sig) != 2*size {
			return errors.New("signature does not verify")
		}
		r, s := new(big.Int).SetBytes(sig[:size]), new(big.Int).SetBytes(sig[size:])
		if !ecdsa.Verify(k, digest, r, s) {
			return errors.New("signature does not verify")
		}
		return nil
	}
	return errors.New("algorithm is not supported")
}

// Response verifies the signature of a response with the signer's published
// keys (signatures, "REST Response Verification"): @status is covered, and so
// are content-digest and content-type when there is a body.
func (v *Verifier) Response(status int, h http.Header, body []byte, keys []Key) (*Signed, error) {
	inputs := strings.Join(h.Values("Signature-Input"), ", ")
	sigs := strings.Join(h.Values("Signature"), ", ")
	if inputs == "" || sigs == "" {
		return nil, fail(CodeSignatureMissing, "the response carries no Signature-Input and Signature")
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
		raw, ok := sigBytes(sd, c.label)
		if !ok {
			continue
		}
		got, sk := v.responseCandidate(status, h, body, keys, c, raw)
		if sk == nil {
			return got, nil
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

func (v *Verifier) responseCandidate(status int, h http.Header, body []byte, keys []Key, c sigInput, sig []byte) (*Signed, *skip) {
	keyid, _ := c.str("keyid")
	var key *Key
	for i := range keys {
		if keys[i].KID == keyid && keys[i].Usable() {
			key = &keys[i]
			break
		}
	}
	if key == nil {
		return nil, skipf(CodeKeyNotFound, "key %q is not in the signer's published key set", keyid)
	}
	alg := key.Algorithm()
	pub, err := key.Public()
	if alg == "" || err != nil {
		return nil, skipf(CodeAlgorithmUnsupported, "key %q is not supported", keyid)
	}
	required := []string{"@status"}
	if len(body) > 0 {
		required = append(required, "content-digest", "content-type")
	}
	for _, name := range required {
		if !c.covers(name) {
			return nil, skipf(CodeSignatureInvalid, "the signature does not cover %s", name)
		}
	}
	if c.covers("content-digest") {
		if err := checkDigest(h.Get("Content-Digest"), body); err != nil {
			return nil, skipf(CodeDigestMismatch, "%v", err)
		}
	}
	created, sk := v.fresh(c, "")
	if sk != nil {
		return nil, sk
	}
	base, err := c.base(&message{response: true, status: status, header: h})
	if err != nil {
		return nil, skipf(CodeSignatureInvalid, "%v", err)
	}
	if err := verifyRaw(alg, pub, base, sig); err != nil {
		return nil, skipf(CodeSignatureInvalid, "%v", err)
	}
	return &Signed{Label: c.label, KeyID: keyid, Created: created}, nil
}
