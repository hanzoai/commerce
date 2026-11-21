// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package mpp

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	// Scheme is the authentication scheme (section 5).
	Scheme = "Payment"
	// Charge is the one-time payment intent (draft-payment-intent-charge-00).
	Charge = "charge"

	authorization = "Authorization"
	alternate     = "Payment-Authorization" // section 5.2
	receipted     = "Payment-Receipt"       // section 5.3
	accepting     = "Accept-Payment"        // section 7.4
	authenticate  = "WWW-Authenticate"
	idempotency   = "Idempotency-Key" // section 11.4
)

// Challenge is a Payment challenge (section 5.1) and its echo in a credential (section
// 5.2). Request and Opaque are base64url text as sent.
type Challenge struct {
	ID          string `json:"id"`
	Realm       string `json:"realm"`
	Method      string `json:"method"`
	Intent      string `json:"intent"`
	Request     string `json:"request"`
	Description string `json:"description,omitempty"`
	Opaque      string `json:"opaque,omitempty"`
	Digest      string `json:"digest,omitempty"`
	Expires     string `json:"expires,omitempty"`
	Header      string `json:"header,omitempty"`
}

var (
	methodShape = regexp.MustCompile(`^[a-z]+$`)
	intentShape = regexp.MustCompile(`^[A-Za-z0-9-]+$`)
	digestShape = regexp.MustCompile(`^[a-z0-9-]+=:[A-Za-z0-9+/]*={0,2}:$`)

	raw = base64.RawURLEncoding.Strict()
)

// Field is the field the credential travels in: Authorization, or Payment-Authorization
// when header is set (section 5.2).
func (c Challenge) Field() string {
	if c.Header != "" {
		return alternate
	}
	return authorization
}

// Expiry is the expires parameter; ok is false when absent or malformed.
func (c Challenge) Expiry() (at time.Time, ok bool) {
	if c.Expires == "" {
		return time.Time{}, false
	}
	at, err := time.Parse(time.RFC3339, c.Expires)
	return at, err == nil
}

// Valid checks the form of each parameter (sections 5.1, 6.1, 7.1).
func (c Challenge) Valid() error {
	switch {
	case c.ID == "":
		return errors.New("id is missing or empty")
	case c.Realm == "":
		return errors.New("realm is missing")
	case !methodShape.MatchString(c.Method):
		return fmt.Errorf("method %q is not lowercase ASCII letters", c.Method)
	case !intentShape.MatchString(c.Intent):
		return fmt.Errorf("intent %q is not a token of letters, digits and hyphens", c.Intent)
	case c.Request == "":
		return errors.New("request is missing")
	}
	if _, err := raw.DecodeString(c.Request); err != nil {
		return fmt.Errorf("request is not base64url without padding: %w", err)
	}
	if c.Expires != "" {
		if _, err := time.Parse(time.RFC3339, c.Expires); err != nil {
			return fmt.Errorf("expires is not an RFC 3339 date-time: %w", err)
		}
	}
	if c.Header != "" && c.Header != alternate {
		return fmt.Errorf("header %q is not %s", c.Header, alternate)
	}
	if c.Digest != "" && !digestShape.MatchString(c.Digest) {
		return fmt.Errorf("digest %q is not an RFC 9530 sha-256 or sha-512 value", c.Digest)
	}
	if c.Opaque != "" {
		if _, err := Opaque(c.Opaque); err != nil {
			return err
		}
	}
	return nil
}

// Opaque decodes an opaque parameter: base64url JSON, a flat object of strings (section 5.1.2).
func Opaque(s string) (map[string]string, error) {
	b, err := raw.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("opaque is not base64url without padding: %w", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil || m == nil {
		return nil, errors.New("opaque is not a JSON object")
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		var s string
		if len(v) == 0 || v[0] != '"' || json.Unmarshal(v, &s) != nil {
			return nil, fmt.Errorf("opaque member %q is not a string", k)
		}
		out[k] = s
	}
	return out, nil
}

// Encode is the wire form of a request or opaque value: JCS, then base64url without
// padding (section 5.1.1).
func Encode(v any) (string, error) {
	b, err := canonical(v)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Bind is the challenge id: HMAC-SHA256 over realm, method, intent, request, expires,
// digest, [header,] opaque joined by "|", an absent parameter being empty and the header
// slot present only with the header parameter, base64url without padding (section
// 5.1.2.1.1, in the working copy's slot order).
func Bind(secret []byte, c Challenge) string {
	slots := []string{c.Realm, c.Method, c.Intent, c.Request, c.Expires, c.Digest}
	if c.Header != "" {
		slots = append(slots, c.Header)
	}
	slots = append(slots, c.Opaque)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(strings.Join(slots, "|")))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// bound says, in constant time, whether the id is the binding of the other parameters.
func bound(secret []byte, c Challenge) bool {
	return subtle.ConstantTimeCompare([]byte(Bind(secret, c)), []byte(c.ID)) == 1
}

// String is the challenge as the value of a WWW-Authenticate header.
func (c Challenge) String() string {
	var b strings.Builder
	b.WriteString(Scheme)
	sep := " "
	put := func(name, v string) {
		if v == "" && name != "realm" {
			return
		}
		b.WriteString(sep)
		sep = ", "
		b.WriteString(name + "=" + quote(v))
	}
	put("id", c.ID)
	put("realm", c.Realm)
	put("method", c.Method)
	put("intent", c.Intent)
	put("request", c.Request)
	put("expires", c.Expires)
	put("digest", c.Digest)
	put("header", c.Header)
	put("opaque", c.Opaque)
	put("description", c.Description)
	return b.String()
}

// quote is an RFC 9110 quoted-string; control characters become spaces.
func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r < 0x20 || r == 0x7f:
			b.WriteByte(' ')
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// Parse reads the Payment challenges in WWW-Authenticate values, skipping other schemes.
// found holds the well-formed ones in order; err joins the reasons for the rest, which a
// client treats as unsupported (sections 5.1.1, 5.2). Unknown parameters are ignored.
func Parse(values []string) (found []Challenge, err error) {
	var bad []error
	for _, v := range values {
		for _, r := range scan(v) {
			if !strings.EqualFold(r.scheme, Scheme) {
				continue
			}
			c, e := r.challenge()
			if e == nil {
				e = c.Valid()
			}
			if e != nil {
				bad = append(bad, e)
				continue
			}
			found = append(found, c)
		}
	}
	return found, errors.Join(bad...)
}

// group is a scanned challenge: a scheme and its auth-params.
type group struct {
	scheme string
	params map[string]string
	err    error
}

func (g group) challenge() (Challenge, error) {
	if g.err != nil {
		return Challenge{}, g.err
	}
	p := g.params
	return Challenge{
		ID: p["id"], Realm: p["realm"], Method: p["method"], Intent: p["intent"], Request: p["request"],
		Description: p["description"], Opaque: p["opaque"], Digest: p["digest"], Expires: p["expires"], Header: p["header"],
	}, nil
}

func tchar(c byte) bool {
	switch {
	case c >= '0' && c <= '9', c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		return true
	}
	return strings.IndexByte("!#$%&'*+-.^_`|~", c) >= 0
}

// scan splits a field value into challenges (RFC 9110 section 11.6.1). A parameter name
// given twice rejects its challenge.
func scan(s string) []group {
	var out []group
	i, comma := 0, true
	for i < len(s) {
		switch s[i] {
		case ' ', '\t':
			i++
			continue
		case ',':
			comma = true
			i++
			continue
		}
		start := i
		for i < len(s) && tchar(s[i]) {
			i++
		}
		if i == start {
			i++
			continue
		}
		tok := s[start:i]
		j := i
		for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
			j++
		}
		if j < len(s) && s[j] == '=' && len(out) > 0 {
			j++
			for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
				j++
			}
			var val string
			quoted := j < len(s) && s[j] == '"'
			if quoted {
				var ok bool
				val, j, ok = unquote(s, j)
				if !ok {
					out[len(out)-1].err = errors.New("unterminated quoted-string")
					return out
				}
			} else {
				k := j
				for k < len(s) && tchar(s[k]) {
					k++
				}
				val, j = s[j:k], k
				if val == "" { // token68 padding
					for j < len(s) && s[j] == '=' {
						j++
					}
					i, comma = j, false
					continue
				}
			}
			g := &out[len(out)-1]
			name := strings.ToLower(tok)
			if _, dup := g.params[name]; dup {
				g.err = fmt.Errorf("parameter %s is given twice", name)
			}
			g.params[name] = val
			i, comma = j, false
			continue
		}
		if comma {
			out = append(out, group{scheme: tok, params: map[string]string{}})
			comma = false
		}
		i = j
		if j < len(s) && s[j] == '=' { // its padding
			for i < len(s) && s[i] == '=' {
				i++
			}
		}
	}
	return out
}

// unquote reads the quoted-string at s[i], returning its value and the index after it.
func unquote(s string, i int) (string, int, bool) {
	var b strings.Builder
	for i++; i < len(s); i++ {
		switch s[i] {
		case '"':
			return b.String(), i + 1, true
		case '\\':
			i++
			if i >= len(s) {
				return "", i, false
			}
			b.WriteByte(s[i])
		default:
			b.WriteByte(s[i])
		}
	}
	return "", i, false
}

// digestOf is the RFC 9530 sha-256 value of a body.
func digestOf(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha-256=:" + base64.StdEncoding.EncodeToString(sum[:]) + ":"
}

// digestMatches says, in constant time, whether body has the digest want (sha-256 or
// sha-512, RFC 9530).
func digestMatches(want string, body []byte) bool {
	algo, rest, ok := strings.Cut(want, "=:")
	if !ok || !strings.HasSuffix(rest, ":") {
		return false
	}
	var sum []byte
	switch algo {
	case "sha-256":
		s := sha256.Sum256(body)
		sum = s[:]
	case "sha-512":
		s := sha512.Sum512(body)
		sum = s[:]
	default:
		return false
	}
	have := algo + "=:" + base64.StdEncoding.EncodeToString(sum) + ":"
	return subtle.ConstantTimeCompare([]byte(have), []byte(want)) == 1
}
