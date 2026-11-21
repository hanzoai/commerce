// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Structured Field Values (RFC 8941) as UCP uses them: the UCP-Agent and
// Signature-Agent dictionaries (overview, "Platform Advertisement on Request";
// signatures, "UCP-Agent parsing rules"), the Signature-Input and Signature
// dictionaries (RFC 9421 section 4) and Content-Digest (RFC 9530). Only
// dictionaries are parsed at the top level; lists are not used by any UCP header.

// token is an RFC 8941 token (a bare, unquoted word).
type token string

// param is one parameter of an item or inner list. Value is int64, float64,
// string, token, []byte or bool.
type param struct {
	key   string
	value any
}

// item is an RFC 8941 item: a bare value and its parameters.
type item struct {
	value  any
	params []param
}

// inner is an RFC 8941 inner list with its parameters.
type inner struct {
	items  []item
	params []param
}

// member is one dictionary member. value is an item or an inner.
type member struct {
	key   string
	value any
}

// dict is an RFC 8941 dictionary in order of first appearance.
type dict []member

func (d dict) get(key string) (any, bool) {
	for _, m := range d {
		if m.key == key {
			return m.value, true
		}
	}
	return nil, false
}

var errSF = errors.New("malformed structured field")

type sfReader struct {
	s string
	i int
}

func (r *sfReader) done() bool { return r.i >= len(r.s) }

func (r *sfReader) peek() byte {
	if r.done() {
		return 0
	}
	return r.s[r.i]
}

func (r *sfReader) sp() {
	for !r.done() && r.s[r.i] == ' ' {
		r.i++
	}
}

func (r *sfReader) ows() {
	for !r.done() && (r.s[r.i] == ' ' || r.s[r.i] == '\t') {
		r.i++
	}
}

// parseDict parses a Dictionary Structured Field (RFC 8941 section 4.2.2).
func parseDict(s string) (dict, error) {
	r := &sfReader{s: s}
	r.sp()
	var out dict
	for !r.done() {
		key, err := r.key()
		if err != nil {
			return nil, err
		}
		var v any
		if r.peek() == '=' {
			r.i++
			v, err = r.itemOrInner()
			if err != nil {
				return nil, err
			}
		} else {
			ps, err := r.params()
			if err != nil {
				return nil, err
			}
			v = item{value: true, params: ps}
		}
		replaced := false
		for i := range out {
			if out[i].key == key {
				out[i].value, replaced = v, true
				break
			}
		}
		if !replaced {
			out = append(out, member{key: key, value: v})
		}
		r.ows()
		if r.done() {
			return out, nil
		}
		if r.peek() != ',' {
			return nil, fmt.Errorf("%w: expected comma at %d", errSF, r.i)
		}
		r.i++
		r.ows()
		if r.done() {
			return nil, fmt.Errorf("%w: trailing comma", errSF)
		}
	}
	return out, nil
}

func (r *sfReader) itemOrInner() (any, error) {
	if r.peek() == '(' {
		return r.innerList()
	}
	return r.item()
}

func (r *sfReader) innerList() (inner, error) {
	r.i++ // (
	var in inner
	for !r.done() {
		r.sp()
		if r.peek() == ')' {
			r.i++
			ps, err := r.params()
			if err != nil {
				return inner{}, err
			}
			in.params = ps
			return in, nil
		}
		it, err := r.item()
		if err != nil {
			return inner{}, err
		}
		in.items = append(in.items, it)
		if c := r.peek(); c != ' ' && c != ')' {
			return inner{}, fmt.Errorf("%w: inner list at %d", errSF, r.i)
		}
	}
	return inner{}, fmt.Errorf("%w: unterminated inner list", errSF)
}

func (r *sfReader) item() (item, error) {
	v, err := r.bare()
	if err != nil {
		return item{}, err
	}
	ps, err := r.params()
	if err != nil {
		return item{}, err
	}
	return item{value: v, params: ps}, nil
}

func (r *sfReader) params() ([]param, error) {
	var ps []param
	for r.peek() == ';' {
		r.i++
		r.sp()
		k, err := r.key()
		if err != nil {
			return nil, err
		}
		var v any = true
		if r.peek() == '=' {
			r.i++
			if v, err = r.bare(); err != nil {
				return nil, err
			}
		}
		replaced := false
		for i := range ps {
			if ps[i].key == k {
				ps[i].value, replaced = v, true
				break
			}
		}
		if !replaced {
			ps = append(ps, param{key: k, value: v})
		}
	}
	return ps, nil
}

func lcalpha(c byte) bool { return c >= 'a' && c <= 'z' }
func digit(c byte) bool   { return c >= '0' && c <= '9' }
func alpha(c byte) bool   { return lcalpha(c) || (c >= 'A' && c <= 'Z') }

func (r *sfReader) key() (string, error) {
	if c := r.peek(); !lcalpha(c) && c != '*' {
		return "", fmt.Errorf("%w: key at %d", errSF, r.i)
	}
	start := r.i
	for !r.done() {
		c := r.s[r.i]
		if !lcalpha(c) && !digit(c) && c != '_' && c != '-' && c != '.' && c != '*' {
			break
		}
		r.i++
	}
	return r.s[start:r.i], nil
}

func (r *sfReader) bare() (any, error) {
	switch c := r.peek(); {
	case c == '-' || digit(c):
		return r.number()
	case c == '"':
		return r.str()
	case alpha(c) || c == '*':
		return r.token()
	case c == ':':
		return r.bytes()
	case c == '?':
		return r.boolean()
	}
	return nil, fmt.Errorf("%w: item at %d", errSF, r.i)
}

func (r *sfReader) number() (any, error) {
	start := r.i
	if r.peek() == '-' {
		r.i++
	}
	if !digit(r.peek()) {
		return nil, fmt.Errorf("%w: number at %d", errSF, start)
	}
	dec := false
	ints, fracs := 0, 0
	for !r.done() {
		c := r.s[r.i]
		if digit(c) {
			if dec {
				fracs++
			} else {
				ints++
			}
		} else if c == '.' && !dec {
			dec = true
		} else {
			break
		}
		r.i++
		if (!dec && ints > 15) || (dec && (ints > 12 || fracs > 3)) {
			return nil, fmt.Errorf("%w: number too long", errSF)
		}
	}
	text := r.s[start:r.i]
	if dec {
		if fracs == 0 {
			return nil, fmt.Errorf("%w: decimal needs a fraction", errSF)
		}
		return strconv.ParseFloat(text, 64)
	}
	return strconv.ParseInt(text, 10, 64)
}

func (r *sfReader) str() (string, error) {
	r.i++ // opening quote
	var b strings.Builder
	for !r.done() {
		c := r.s[r.i]
		r.i++
		switch {
		case c == '\\':
			if r.done() {
				return "", fmt.Errorf("%w: dangling escape", errSF)
			}
			n := r.s[r.i]
			r.i++
			if n != '"' && n != '\\' {
				return "", fmt.Errorf("%w: bad escape", errSF)
			}
			b.WriteByte(n)
		case c == '"':
			return b.String(), nil
		case c < 0x20 || c > 0x7e:
			return "", fmt.Errorf("%w: string byte %#x", errSF, c)
		default:
			b.WriteByte(c)
		}
	}
	return "", fmt.Errorf("%w: unterminated string", errSF)
}

func tchar(c byte) bool {
	return alpha(c) || digit(c) || strings.IndexByte("!#$%&'*+-.^_`|~", c) >= 0
}

func (r *sfReader) token() (token, error) {
	start := r.i
	for !r.done() && (tchar(r.s[r.i]) || r.s[r.i] == ':' || r.s[r.i] == '/') {
		r.i++
	}
	return token(r.s[start:r.i]), nil
}

func (r *sfReader) bytes() ([]byte, error) {
	r.i++ // opening colon
	end := strings.IndexByte(r.s[r.i:], ':')
	if end < 0 {
		return nil, fmt.Errorf("%w: unterminated byte sequence", errSF)
	}
	text := r.s[r.i : r.i+end]
	r.i += end + 1
	b, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		b, err = base64.RawStdEncoding.DecodeString(text)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: byte sequence: %v", errSF, err)
	}
	return b, nil
}

func (r *sfReader) boolean() (bool, error) {
	r.i++ // ?
	switch r.peek() {
	case '1':
		r.i++
		return true, nil
	case '0':
		r.i++
		return false, nil
	}
	return false, fmt.Errorf("%w: boolean at %d", errSF, r.i)
}

// sfString serialises a string item value (RFC 8941 section 4.1.6).
func sfString(s string) (string, error) {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x20 || c > 0x7e {
			return "", fmt.Errorf("%w: string byte %#x", errSF, c)
		}
		if c == '"' || c == '\\' {
			b.WriteByte('\\')
		}
		b.WriteByte(c)
	}
	b.WriteByte('"')
	return b.String(), nil
}

func sfBare(v any) (string, error) {
	switch x := v.(type) {
	case int64:
		return strconv.FormatInt(x, 10), nil
	case float64:
		t := strconv.FormatFloat(x, 'f', 3, 64)
		t = strings.TrimRight(t, "0")
		if strings.HasSuffix(t, ".") {
			t += "0"
		}
		return t, nil
	case string:
		return sfString(x)
	case token:
		return string(x), nil
	case []byte:
		return ":" + base64.StdEncoding.EncodeToString(x) + ":", nil
	case bool:
		if x {
			return "?1", nil
		}
		return "?0", nil
	}
	return "", fmt.Errorf("%w: unsupported value %T", errSF, v)
}

func sfParams(ps []param) (string, error) {
	var b strings.Builder
	for _, p := range ps {
		b.WriteByte(';')
		b.WriteString(p.key)
		if v, ok := p.value.(bool); ok && v {
			continue
		}
		s, err := sfBare(p.value)
		if err != nil {
			return "", err
		}
		b.WriteByte('=')
		b.WriteString(s)
	}
	return b.String(), nil
}

func (it item) String() (string, error) {
	s, err := sfBare(it.value)
	if err != nil {
		return "", err
	}
	ps, err := sfParams(it.params)
	if err != nil {
		return "", err
	}
	return s + ps, nil
}

func (in inner) String() (string, error) {
	parts := make([]string, len(in.items))
	for i, it := range in.items {
		s, err := it.String()
		if err != nil {
			return "", err
		}
		parts[i] = s
	}
	ps, err := sfParams(in.params)
	if err != nil {
		return "", err
	}
	return "(" + strings.Join(parts, " ") + ")" + ps, nil
}

// stringOf serialises a dictionary member value (an item or an inner list).
func stringOf(v any) (string, error) {
	switch x := v.(type) {
	case item:
		return x.String()
	case inner:
		return x.String()
	}
	return "", fmt.Errorf("%w: unsupported member %T", errSF, v)
}
