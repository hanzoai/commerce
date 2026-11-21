// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
)

// ContentDigest is the Content-Digest header value for body: the sha-256 of the
// raw bytes, never canonicalised (RFC 9530; signatures, "Headers"). Proxies must
// not re-serialise JSON, since any change to the bytes breaks verification.
func ContentDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha-256=:" + base64.StdEncoding.EncodeToString(sum[:]) + ":"
}

var errDigest = errors.New("content digest does not match the body")

// checkDigest verifies a Content-Digest header against the raw body. The
// header must carry a sha-256 member (the algorithm UCP mandates); a sha-512
// member is checked as well when present. Comparison is constant time.
func checkDigest(header string, body []byte) error {
	d, err := parseDict(header)
	if err != nil {
		return err
	}
	if _, ok := d.get("sha-256"); !ok {
		return errors.New("Content-Digest has no sha-256 member")
	}
	checked := 0
	for _, c := range []struct {
		name string
		sum  func([]byte) []byte
	}{
		{"sha-256", func(b []byte) []byte { s := sha256.Sum256(b); return s[:] }},
		{"sha-512", func(b []byte) []byte { s := sha512.Sum512(b); return s[:] }},
	} {
		v, ok := d.get(c.name)
		if !ok {
			continue
		}
		it, isItem := v.(item)
		raw, isBytes := it.value.([]byte)
		if !isItem || !isBytes {
			return fmt.Errorf("Content-Digest %s is not a byte sequence", c.name)
		}
		if subtle.ConstantTimeCompare(raw, c.sum(body)) != 1 {
			return errDigest
		}
		checked++
	}
	if checked == 0 {
		return errors.New("Content-Digest has no verifiable member")
	}
	return nil
}
