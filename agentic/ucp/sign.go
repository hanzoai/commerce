// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/asn1"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"time"
)

// Signer signs UCP messages with one key (signatures, "REST Request Signing"
// and "REST Response Signing"). Key is an *ecdsa.PrivateKey on P-256 or P-384,
// or an ed25519.PrivateKey; the algorithm follows from the key, and alg is not
// written into Signature-Input ("Usage guidance").
type Signer struct {
	KeyID string
	Key   crypto.Signer
	clock func() time.Time
}

// Options shape one signature.
type Options struct {
	// Label is the signature label, "sig1" by default.
	Label string
	// Expires, when positive, adds an expires parameter that far after created.
	Expires time.Duration
	// Tag, when set, is the tag parameter; "web-bot-auth" opts into the Web Bot
	// Auth shape, which also needs a Signature-Agent header on the request,
	// Expires, and a KeyID equal to the key's RFC 7638 thumbprint.
	Tag string
}

func (s *Signer) now() time.Time {
	if s.clock != nil {
		return s.clock()
	}
	return time.Now()
}

func (o Options) label() string {
	if o.Label == "" {
		return "sig1"
	}
	return o.Label
}

// algorithm is the JWA algorithm of the signer's key.
func (s *Signer) algorithm() (string, error) {
	pub, err := KeyOf(s.KeyID, s.Key.Public())
	if err != nil {
		return "", err
	}
	return pub.Algorithm(), nil
}

// sign signs the signature base with the key's algorithm. ECDSA signatures are
// the fixed-width r||s encoding of RFC 9421 section 3.3.4 and 3.3.5, not DER;
// Ed25519 signatures are the 64 bytes of RFC 8032 section 5.1.6.
func (s *Signer) sign(base []byte) ([]byte, error) {
	alg, err := s.algorithm()
	if err != nil {
		return nil, err
	}
	switch alg {
	case EdDSA:
		return s.Key.Sign(rand.Reader, base, crypto.Hash(0))
	case ES256:
		d := sha256.Sum256(base)
		return rawECDSA(s.Key, d[:], crypto.SHA256, 32)
	default:
		d := sha512.Sum384(base)
		return rawECDSA(s.Key, d[:], crypto.SHA384, 48)
	}
}

func rawECDSA(k crypto.Signer, digest []byte, h crypto.Hash, size int) ([]byte, error) {
	der, err := k.Sign(rand.Reader, digest, h)
	if err != nil {
		return nil, err
	}
	var sig struct{ R, S *big.Int }
	if _, err := asn1.Unmarshal(der, &sig); err != nil {
		return nil, err
	}
	out := make([]byte, 2*size)
	sig.R.FillBytes(out[:size])
	sig.S.FillBytes(out[size:])
	return out, nil
}

// params returns the signature parameters: created, expires, keyid, tag.
func (s *Signer) params(o Options) []param {
	now := s.now()
	ps := []param{{"created", now.Unix()}}
	if o.Expires > 0 {
		ps = append(ps, param{"expires", now.Add(o.Expires).Unix()})
	}
	ps = append(ps, param{"keyid", s.KeyID})
	if o.Tag != "" {
		ps = append(ps, param{"tag", o.Tag})
	}
	return ps
}

// emit signs and writes Signature-Input and Signature into h.
func (s *Signer) emit(h http.Header, m *message, items []covered, o Options) error {
	si := sigInput{label: o.label(), items: items, params: s.params(o)}
	base, err := si.base(m)
	if err != nil {
		return err
	}
	sig, err := s.sign(base)
	if err != nil {
		return err
	}
	line, err := si.paramsLine()
	if err != nil {
		return err
	}
	h.Set("Signature-Input", si.label+"="+line)
	h.Set("Signature", si.label+"=:"+base64.StdEncoding.EncodeToString(sig)+":")
	return nil
}

// SignRequest signs a request and sets Content-Digest (when body is not empty),
// Signature-Input and Signature on it. The covered components follow
// sign_rest_request: @method, @authority, @path, @query when present, then
// ucp-agent, signature-agent, idempotency-key, webhook-id and webhook-timestamp
// when those headers are set, then content-digest and content-type when there
// is a body. The signature of a request that has a Signature-Agent header
// covers it as signature-agent;key="<label>".
func (s *Signer) SignRequest(r *http.Request, body []byte, o Options) error {
	if o.Tag == "web-bot-auth" {
		if err := s.webBotAuth(r, o); err != nil {
			return err
		}
	}
	if len(body) > 0 {
		r.Header.Set("Content-Digest", ContentDigest(body))
		if r.Header.Get("Content-Type") == "" {
			r.Header.Set("Content-Type", "application/json")
		}
	}
	items := []covered{{name: "@method"}, {name: "@authority"}, {name: "@path"}}
	m := requestMessage(r, "")
	if m.hasQuery {
		items = append(items, covered{name: "@query"})
	}
	if r.Header.Get("Signature-Agent") != "" {
		items = append(items, covered{name: "signature-agent", params: []param{{"key", o.label()}}})
	}
	for _, h := range []string{"ucp-agent", "idempotency-key", "webhook-id", "webhook-timestamp"} {
		if r.Header.Get(h) != "" {
			items = append(items, covered{name: h})
		}
	}
	if len(body) > 0 {
		items = append(items, covered{name: "content-digest"}, covered{name: "content-type"})
	}
	return s.emit(r.Header, m, items, o)
}

// webBotAuth checks the signer meets the opt-in requirements that the signer
// controls (signatures, "WBA Interop", items 2 to 5).
func (s *Signer) webBotAuth(r *http.Request, o Options) error {
	if r.Header.Get("Signature-Agent") == "" {
		return errors.New("a web-bot-auth signature needs a Signature-Agent header")
	}
	if o.Expires <= 0 {
		return errors.New("a web-bot-auth signature needs expires")
	}
	pub, err := KeyOf(s.KeyID, s.Key.Public())
	if err != nil {
		return err
	}
	tp, err := pub.Thumbprint()
	if err != nil {
		return err
	}
	if tp != s.KeyID {
		return fmt.Errorf("a web-bot-auth keyid must be the key thumbprint %s", tp)
	}
	return nil
}

// SignResponse signs a response's status and body: @status, then
// content-digest and content-type when there is a body, with created and the
// key id (signatures, "REST Response Signing"). It sets Content-Digest,
// Signature-Input and Signature in h; Content-Type must already be set when
// there is a body.
func (s *Signer) SignResponse(h http.Header, status int, body []byte, o Options) error {
	items := []covered{{name: "@status"}}
	if len(body) > 0 {
		h.Set("Content-Digest", ContentDigest(body))
		if h.Get("Content-Type") == "" {
			h.Set("Content-Type", "application/json")
		}
		items = append(items, covered{name: "content-digest"}, covered{name: "content-type"})
	}
	return s.emit(h, &message{response: true, status: status, header: h}, items, o)
}
