// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

// Key is a public JSON Web Key (RFC 7517) as a profile publishes it in keys[]
// (overview, "Profile Structure"; schemas/profile.json, jwk_public_key). The
// vocabulary is open: a key of a type this package does not know is carried and
// never fails the profile, it is only unusable for verification (signatures,
// "Signature Algorithms").
type Key struct {
	KID    string   `json:"kid"`
	KTY    string   `json:"kty"`
	CRV    string   `json:"crv,omitempty"`
	X      string   `json:"x,omitempty"`
	Y      string   `json:"y,omitempty"`
	Alg    string   `json:"alg,omitempty"`
	Use    string   `json:"use,omitempty"`
	KeyOps []string `json:"key_ops,omitempty"`

	// Extra holds the members the schema permits beyond these ("Additional
	// public JWK members are permitted").
	Extra map[string]json.RawMessage `json:"-"`
}

func (k *Key) UnmarshalJSON(b []byte) error {
	type plain Key
	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	ex, err := extras(b, reflect.TypeOf(p))
	if err != nil {
		return err
	}
	*k = Key(p)
	k.Extra = ex
	return nil
}

func (k Key) MarshalJSON() ([]byte, error) {
	type plain Key
	return merge(plain(k), k.Extra)
}

// Algorithms the package verifies and signs (signatures, "Signature Algorithms").
const (
	ES256 = "ES256"
	ES384 = "ES384"
	EdDSA = "EdDSA"
)

// privateMembers are the JWK members that carry private key material; a
// profile publishes public keys only (schemas/profile.json).
var privateMembers = []string{"d", "p", "q", "dp", "dq", "qi", "oth", "k"}

// check validates a key as the profile schema does: kid and kty are required,
// EC keys carry crv, x and y, OKP keys carry crv and x, a stated alg matches
// its curve, and no private member is present.
func (k Key) check() error {
	if k.KID == "" || k.KTY == "" {
		return errors.New("key needs kid and kty")
	}
	for _, m := range privateMembers {
		if _, ok := k.Extra[m]; ok {
			return fmt.Errorf("key %q carries private member %q", k.KID, m)
		}
	}
	switch k.KTY {
	case "EC":
		if k.CRV == "" || k.X == "" || k.Y == "" {
			return fmt.Errorf("EC key %q needs crv, x and y", k.KID)
		}
	case "OKP":
		if k.CRV == "" || k.X == "" {
			return fmt.Errorf("OKP key %q needs crv and x", k.KID)
		}
	}
	want := map[string]string{"P-256": ES256, "P-384": ES384, "Ed25519": EdDSA}[k.CRV]
	if want != "" && k.Alg != "" && k.Alg != want {
		return fmt.Errorf("key %q: alg %q does not match crv %q", k.KID, k.Alg, k.CRV)
	}
	return nil
}

// Usable reports whether the key may verify a signature: use "enc" and a
// key_ops without "verify" exclude it (RFC 7517 sections 4.2 and 4.3;
// overview, "Identity Resolution Algorithm", step 4).
func (k Key) Usable() bool {
	if k.Use == "enc" {
		return false
	}
	if k.KeyOps != nil {
		for _, op := range k.KeyOps {
			if op == "verify" {
				return true
			}
		}
		return false
	}
	return true
}

func b64(s string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(s) }

// Algorithm returns the JWA algorithm the key's kty and crv imply, or "" when
// the key is of a type this package does not support.
func (k Key) Algorithm() string {
	switch {
	case k.KTY == "EC" && k.CRV == "P-256":
		return ES256
	case k.KTY == "EC" && k.CRV == "P-384":
		return ES384
	case k.KTY == "OKP" && k.CRV == "Ed25519":
		return EdDSA
	}
	return ""
}

// Public builds the verification key: *ecdsa.PublicKey for P-256 and P-384,
// ed25519.PublicKey for Ed25519.
func (k Key) Public() (crypto.PublicKey, error) {
	x, err := b64(k.X)
	if err != nil {
		return nil, fmt.Errorf("key %q: x: %w", k.KID, err)
	}
	switch k.Algorithm() {
	case ES256, ES384:
		curve, size := elliptic.P256(), 32
		if k.CRV == "P-384" {
			curve, size = elliptic.P384(), 48
		}
		y, err := b64(k.Y)
		if err != nil {
			return nil, fmt.Errorf("key %q: y: %w", k.KID, err)
		}
		if len(x) != size || len(y) != size {
			return nil, fmt.Errorf("key %q: coordinates must be %d bytes", k.KID, size)
		}
		pt := append([]byte{4}, append(x, y...)...)
		return ecdsa.ParseUncompressedPublicKey(curve, pt)
	case EdDSA:
		if len(x) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("key %q: Ed25519 key must be %d bytes", k.KID, ed25519.PublicKeySize)
		}
		return ed25519.PublicKey(x), nil
	}
	return nil, fmt.Errorf("key %q: kty %q crv %q is not supported", k.KID, k.KTY, k.CRV)
}

// Thumbprint is the RFC 7638 SHA-256 JWK thumbprint, base64url without padding:
// the digest of the key's required members in lexicographic order. A signature
// that opts into Web Bot Auth carries it as keyid (signatures, "WBA Interop").
func (k Key) Thumbprint() (string, error) {
	var s string
	switch k.KTY {
	case "EC":
		s = fmt.Sprintf(`{"crv":%q,"kty":"EC","x":%q,"y":%q}`, k.CRV, k.X, k.Y)
	case "OKP":
		s = fmt.Sprintf(`{"crv":%q,"kty":"OKP","x":%q}`, k.CRV, k.X)
	default:
		return "", fmt.Errorf("key %q: no thumbprint for kty %q", k.KID, k.KTY)
	}
	sum := sha256.Sum256([]byte(s))
	return base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

// KeyOf returns the public JWK for a private or public key, with kid set. It
// serves a signer that publishes its own key.
func KeyOf(kid string, pub crypto.PublicKey) (Key, error) {
	switch p := pub.(type) {
	case *ecdsa.PublicKey:
		crv, alg, size := "P-256", ES256, 32
		if p.Curve == elliptic.P384() {
			crv, alg, size = "P-384", ES384, 48
		} else if p.Curve != elliptic.P256() {
			return Key{}, errors.New("only P-256 and P-384 are supported")
		}
		raw, err := p.Bytes()
		if err != nil {
			return Key{}, err
		}
		return Key{KID: kid, KTY: "EC", CRV: crv, Alg: alg, Use: "sig",
			X: base64.RawURLEncoding.EncodeToString(raw[1 : 1+size]),
			Y: base64.RawURLEncoding.EncodeToString(raw[1+size:])}, nil
	case ed25519.PublicKey:
		return Key{KID: kid, KTY: "OKP", CRV: "Ed25519", Alg: EdDSA, Use: "sig",
			X: base64.RawURLEncoding.EncodeToString(p)}, nil
	}
	return Key{}, fmt.Errorf("unsupported key type %T", pub)
}
