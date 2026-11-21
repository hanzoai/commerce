// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ap2

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
)

// b64 is base64url without padding (RFC 4648 section 5), refusing non-canonical encodings.
var b64 = base64.RawURLEncoding.Strict()

// parse reads one JSON value. Unlike encoding/json it refuses a repeated object member and
// trailing data, so the bytes a signature covers have one reading. Numbers stay json.Number.
func parse(data []byte) (any, error) {
	if !json.Valid(data) { // also bounds the nesting
		return nil, errors.New("not JSON")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := value(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("data after the JSON value")
	}
	return v, nil
}

func value(dec *json.Decoder) (any, error) {
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
			if m[k], err = value(dec); err != nil {
				return nil, err
			}
		}
		_, err := dec.Token()
		return m, err
	case '[':
		a := []any{}
		for dec.More() {
			v, err := value(dec)
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

// unb64 decodes canonical base64url. encoding/base64 skips a carriage return or a line feed
// anywhere in its input, even in Strict mode, so a segment could be re-spelled without changing
// what it decodes to; every byte here must be in the alphabet.
func unb64(s string) ([]byte, error) {
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return nil, fmt.Errorf("byte %#x is not base64url", c)
		}
	}
	return b64.DecodeString(s)
}

// segment decodes one base64url JWT segment as a JSON object.
func segment(s string) (map[string]any, []byte, error) {
	raw, err := unb64(s)
	if err != nil {
		return nil, nil, err
	}
	m, err := object(raw)
	return m, raw, err
}

// digest is base64url(SHA-256(ASCII(s))): the hash AP2 names when a token's `_sd_alg` is absent
// or sha-256, the only one this package accepts.
func digest(s string) string {
	h := sha256.Sum256([]byte(s))
	return b64.EncodeToString(h[:])
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
