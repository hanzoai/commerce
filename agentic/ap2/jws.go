// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ap2

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

// JWK is an EC P-256 public key in JWK form (types/jwk.json, RFC 7517). The schema forbids
// any other member, so a key that carries one, a private "d" included, is refused.
type JWK struct {
	Kty     string   `json:"kty"`
	Crv     string   `json:"crv,omitempty"`
	X       string   `json:"x,omitempty"`
	Y       string   `json:"y,omitempty"`
	Use     string   `json:"use,omitempty"`
	KeyOps  []string `json:"key_ops,omitempty"`
	Alg     string   `json:"alg,omitempty"`
	Kid     string   `json:"kid,omitempty"`
	X5u     string   `json:"x5u,omitempty"`
	X5c     []string `json:"x5c,omitempty"`
	X5t     string   `json:"x5t,omitempty"`
	X5tS256 string   `json:"x5t#S256,omitempty"`
}

// NewJWK states a P-256 public key as a JWK for signature verification.
func NewJWK(pub *ecdsa.PublicKey, kid string) JWK {
	pt, _ := pub.Bytes()
	return JWK{Kty: "EC", Crv: "P-256", X: b64.EncodeToString(pt[1:33]), Y: b64.EncodeToString(pt[33:65]), Use: "sig", Alg: "ES256", Kid: kid}
}

// decodeJWK reads a JWK from a decoded JSON value, refusing any member the schema does not
// list.
func decodeJWK(v any) (JWK, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return JWK{}, err
	}
	var k JWK
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&k); err != nil {
		return JWK{}, fmt.Errorf("%w: JWK: %v", ErrKey, err)
	}
	return k, nil
}

// Key is the public key the JWK names. It must be an on-curve P-256 key meant for signature
// verification with ES256.
func (k JWK) Key() (*ecdsa.PublicKey, error) {
	switch {
	case k.Kty != "EC" || k.Crv != "P-256":
		return nil, fmt.Errorf("%w: only EC P-256 keys are accepted", ErrKey)
	case k.Use != "" && k.Use != "sig":
		return nil, fmt.Errorf("%w: key use %q is not sig", ErrKey, k.Use)
	case k.Alg != "" && k.Alg != "ES256":
		return nil, fmt.Errorf("%w: key alg %q is not ES256", ErrKey, k.Alg)
	}
	if k.KeyOps != nil {
		ok := false
		for _, op := range k.KeyOps {
			ok = ok || op == "verify"
		}
		if !ok {
			return nil, fmt.Errorf("%w: key_ops lacks verify", ErrKey)
		}
	}
	x, errx := unb64(k.X)
	y, erry := unb64(k.Y)
	if errx != nil || erry != nil || len(x) != 32 || len(y) != 32 {
		return nil, fmt.Errorf("%w: x and y must be 32 bytes, base64url", ErrKey)
	}
	pub, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), append([]byte{4}, append(x, y...)...))
	if err != nil {
		return nil, fmt.Errorf("%w: point is not on P-256", ErrKey)
	}
	return pub, nil
}

// jwsHeader is the part of a JOSE header AP2 reads.
type jwsHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ,omitempty"`
	Kid string `json:"kid,omitempty"`
}

// jws is a parsed compact JWS: header and payload as sent, and the signature over them.
type jws struct {
	header  jwsHeader
	payload []byte
	input   string
	sig     []byte
}

// parseJWS reads a compact JWS. Only ES256 is accepted: it is the one algorithm AP2's key
// schema names, and every other, "none" included, is refused here and never reaches a key.
func parseJWS(compact string) (*jws, error) {
	p := strings.Split(compact, ".")
	if len(p) != 3 {
		return nil, fmt.Errorf("%w: a JWS has three parts", ErrMalformed)
	}
	head, _, err := segment(p[0])
	if err != nil {
		return nil, fmt.Errorf("%w: JWS header: %v", ErrMalformed, err)
	}
	if _, crit := head["crit"]; crit {
		return nil, fmt.Errorf("%w: crit header", ErrAlgorithm)
	}
	var j jws
	for name, dst := range map[string]*string{"alg": &j.header.Alg, "typ": &j.header.Typ, "kid": &j.header.Kid} {
		if v, ok := head[name]; ok {
			s, isStr := v.(string)
			if !isStr {
				return nil, fmt.Errorf("%w: header %s is not a string", ErrMalformed, name)
			}
			*dst = s
		}
	}
	if j.header.Alg != "ES256" {
		return nil, fmt.Errorf("%w: alg %q", ErrAlgorithm, j.header.Alg)
	}
	if j.payload, err = unb64(p[1]); err != nil {
		return nil, fmt.Errorf("%w: JWS payload: %v", ErrMalformed, err)
	}
	if j.sig, err = unb64(p[2]); err != nil {
		return nil, fmt.Errorf("%w: JWS signature: %v", ErrMalformed, err)
	}
	j.input = p[0] + "." + p[1]
	return &j, nil
}

// verify checks the ES256 signature (RFC 7518 section 3.4: r and s as 32 bytes each).
func (j *jws) verify(pub *ecdsa.PublicKey) error {
	if len(j.sig) != 64 {
		return fmt.Errorf("%w: ES256 signatures are 64 bytes", ErrSignature)
	}
	sum := sha256.Sum256([]byte(j.input))
	r, s := new(big.Int).SetBytes(j.sig[:32]), new(big.Int).SetBytes(j.sig[32:])
	if !ecdsa.Verify(pub, sum[:], r, s) {
		return fmt.Errorf("%w", ErrSignature)
	}
	return nil
}

// sign makes a compact ES256 JWS over payload.
func sign(h jwsHeader, payload []byte, key *ecdsa.PrivateKey) (string, error) {
	head, err := json.Marshal(h)
	if err != nil {
		return "", err
	}
	input := b64.EncodeToString(head) + "." + b64.EncodeToString(payload)
	sum := sha256.Sum256([]byte(input))
	der, err := ecdsa.SignASN1(rand.Reader, key, sum[:])
	if err != nil {
		return "", err
	}
	var rs struct{ R, S *big.Int }
	if _, err := asn1.Unmarshal(der, &rs); err != nil {
		return "", err
	}
	sig := make([]byte, 64)
	rs.R.FillBytes(sig[:32])
	rs.S.FillBytes(sig[32:])
	return input + "." + b64.EncodeToString(sig), nil
}

// Signer is a merchant's ES256 signing key.
type Signer struct {
	Key *ecdsa.PrivateKey
	Kid string
}

func (s Signer) check() error {
	switch {
	case s.Key == nil || s.Key.Curve != elliptic.P256():
		return errors.New("ap2: the signing key must be a P-256 key")
	case s.Kid == "":
		return errors.New("ap2: the signing key needs a kid")
	}
	return nil
}

// Sign makes a compact JWT of claims.
func (s Signer) Sign(claims any) (string, error) {
	raw, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	return sign(jwsHeader{Alg: "ES256", Typ: "JWT", Kid: s.Kid}, raw, s.Key)
}
