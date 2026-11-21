// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ap2

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
)

// Bounds on what a verifier reads from the wire.
const (
	maxPresentation = 1 << 20
	maxDisclosures  = 512
	maxDepth        = 32
)

// b64 is base64url without padding (RFC 4648 section 5), refusing non-canonical encodings.
var b64 = base64.RawURLEncoding.Strict()

// parse reads one JSON value. Unlike encoding/json it refuses a repeated object member and
// trailing data, so the bytes a signature covers have one reading. Numbers stay json.Number.
func parse(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := value(dec, 0)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("data after the JSON value")
	}
	return v, nil
}

func value(dec *json.Decoder, depth int) (any, error) {
	if depth > maxDepth {
		return nil, errors.New("JSON nested too deep")
	}
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	d, ok := tok.(json.Delim)
	if !ok {
		return tok, nil
	}
	switch d {
	case '{':
		m := map[string]any{}
		for dec.More() {
			kt, err := dec.Token()
			if err != nil {
				return nil, err
			}
			k, _ := kt.(string)
			if _, dup := m[k]; dup {
				return nil, fmt.Errorf("repeated member %q", k)
			}
			if m[k], err = value(dec, depth+1); err != nil {
				return nil, err
			}
		}
		_, err := dec.Token()
		return m, err
	case '[':
		a := []any{}
		for dec.More() {
			v, err := value(dec, depth+1)
			if err != nil {
				return nil, err
			}
			a = append(a, v)
		}
		_, err := dec.Token()
		return a, err
	}
	return nil, fmt.Errorf("unexpected %q", d)
}

// object is parse for a value that must be a JSON object.
func object(data []byte) (map[string]any, error) {
	v, err := parse(data)
	if err != nil {
		return nil, err
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("not a JSON object")
	}
	return m, nil
}

// segment decodes one base64url JWT segment as a JSON object.
func segment(s string) (map[string]any, []byte, error) {
	raw, err := b64.DecodeString(s)
	if err != nil {
		return nil, nil, err
	}
	m, err := object(raw)
	return m, raw, err
}

// digest is base64url(hash(ASCII(s))) under an SD-JWT `_sd_alg` name (RFC 9901 section 4.1).
// An empty name is sha-256, the default.
func digest(alg, s string) (string, error) {
	var sum []byte
	switch alg {
	case "", "sha-256":
		h := sha256.Sum256([]byte(s))
		sum = h[:]
	case "sha-384":
		h := sha512.Sum384([]byte(s))
		sum = h[:]
	case "sha-512":
		h := sha512.Sum512([]byte(s))
		sum = h[:]
	default:
		return "", fmt.Errorf("%w: _sd_alg %q", ErrAlgorithm, alg)
	}
	return b64.EncodeToString(sum), nil
}

// integer reads a JSON number that must be a whole number.
func integer(v any) (int64, bool) {
	n, ok := v.(json.Number)
	if !ok {
		return 0, false
	}
	i, err := strconv.ParseInt(n.String(), 10, 64)
	return i, err == nil
}

// equal says two decoded JSON values are the same value.
func equal(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}
