// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// RFC 9421 HTTP Message Signatures as UCP profiles them (docs/specification/
// signatures.md): the covered components, the signature base and its
// parameters. Signing is in sign.go and verification in verify.go.

// covered is one covered component: a lower-case field name or a derived
// component such as @method, with its parameters (RFC 9421 section 2.1).
type covered struct {
	name   string
	params []param
}

func (c covered) id() (string, error) {
	s, err := sfString(c.name)
	if err != nil {
		return "", err
	}
	ps, err := sfParams(c.params)
	if err != nil {
		return "", err
	}
	return s + ps, nil
}

// sigInput is one member of a Signature-Input dictionary: the covered
// components and the signature parameters in the order they were written.
type sigInput struct {
	label  string
	items  []covered
	params []param
}

// parseSigInput parses a Signature-Input field into its members. A member
// that is not an inner list of strings is not a signature and is left out.
func parseSigInput(h string) ([]sigInput, error) {
	d, err := parseDict(h)
	if err != nil {
		return nil, err
	}
	var out []sigInput
	for _, m := range d {
		in, ok := m.value.(inner)
		if !ok {
			continue
		}
		si := sigInput{label: m.key, params: in.params}
		valid := true
		for _, it := range in.items {
			name, ok := it.value.(string)
			if !ok {
				valid = false
				break
			}
			si.items = append(si.items, covered{name: name, params: it.params})
		}
		if valid {
			out = append(out, si)
		}
	}
	return out, nil
}

// paramsLine is the @signature-params value: the covered component list with the
// signature parameters (RFC 9421 section 2.3).
func (s sigInput) paramsLine() (string, error) {
	in := inner{params: s.params}
	for _, c := range s.items {
		in.items = append(in.items, item{value: c.name, params: c.params})
	}
	return in.String()
}

// intParam reads an integer signature parameter. present reports whether the
// parameter appears; a parameter that is not an integer is an error, so that a
// value such as expires=100.0 cannot be skipped as if it were absent.
func (s sigInput) intParam(key string) (n int64, present bool, err error) {
	for _, p := range s.params {
		if p.key != key {
			continue
		}
		v, ok := p.value.(int64)
		if !ok {
			return 0, true, fmt.Errorf("signature parameter %s must be an integer", key)
		}
		return v, true, nil
	}
	return 0, false, nil
}

func (s sigInput) str(key string) (string, bool) {
	for _, p := range s.params {
		if p.key == key {
			v, ok := p.value.(string)
			return v, ok
		}
	}
	return "", false
}

// covers reports whether a component, whatever its parameters, is covered.
func (s sigInput) covers(name string) bool {
	for _, c := range s.items {
		if c.name == name {
			return true
		}
	}
	return false
}

// message is the HTTP message a signature covers.
type message struct {
	response  bool
	method    string
	scheme    string
	authority string
	path      string
	query     string
	hasQuery  bool
	status    int
	header    http.Header
}

var errComponent = errors.New("unsupported covered component")

// requestMessage describes a request as received by a server: authority names
// the host the signer addressed (the Host header unless a gateway rewrote it).
func requestMessage(r *http.Request, authority string) *message {
	scheme := r.URL.Scheme
	if scheme == "" {
		scheme = "http"
		if r.TLS != nil {
			scheme = "https"
		}
	}
	if authority == "" {
		authority = r.Host
	}
	if authority == "" {
		authority = r.URL.Host
	}
	return &message{
		method: r.Method, scheme: scheme, authority: authority,
		path: r.URL.EscapedPath(), query: r.URL.RawQuery, hasQuery: r.URL.RawQuery != "" || r.URL.ForceQuery,
		header: r.Header,
	}
}

func (m *message) authorityValue() string {
	a := strings.ToLower(m.authority)
	host, port, err := splitHostPort(a)
	if err != nil {
		return a
	}
	if (m.scheme == "https" && port == "443") || (m.scheme == "http" && port == "80") {
		return host
	}
	return a
}

// splitHostPort splits an authority that may omit the port.
func splitHostPort(a string) (host, port string, err error) {
	u, err := url.Parse("//" + a)
	if err != nil {
		return "", "", err
	}
	port = u.Port()
	host = u.Host
	if port != "" {
		host = strings.TrimSuffix(host, ":"+port)
	}
	return host, port, nil
}

func (m *message) pathValue() string {
	if m.path == "" {
		return "/"
	}
	return m.path
}

// value is the signature base value of a covered component (RFC 9421 section 2).
func (m *message) value(c covered) (string, error) {
	if strings.HasPrefix(c.name, "@") {
		if len(c.params) > 0 {
			return "", fmt.Errorf("%w: %s with parameters", errComponent, c.name)
		}
		switch c.name {
		case "@method":
			if m.response {
				return "", fmt.Errorf("%w: @method on a response", errComponent)
			}
			return m.method, nil
		case "@authority":
			return m.authorityValue(), nil
		case "@scheme":
			return m.scheme, nil
		case "@path":
			return m.pathValue(), nil
		case "@query":
			return "?" + m.query, nil
		case "@request-target":
			t := m.pathValue()
			if m.hasQuery {
				t += "?" + m.query
			}
			return t, nil
		case "@target-uri":
			t := m.scheme + "://" + m.authorityValue() + m.pathValue()
			if m.hasQuery {
				t += "?" + m.query
			}
			return t, nil
		case "@status":
			if !m.response {
				return "", fmt.Errorf("%w: @status on a request", errComponent)
			}
			return strconv.Itoa(m.status), nil
		}
		return "", fmt.Errorf("%w: %s", errComponent, c.name)
	}
	if c.name != strings.ToLower(c.name) {
		return "", fmt.Errorf("%w: field name %q is not lower case", errComponent, c.name)
	}
	vals := m.header.Values(c.name)
	if len(vals) == 0 {
		return "", fmt.Errorf("covered field %q is absent", c.name)
	}
	for i, v := range vals {
		vals[i] = strings.TrimSpace(v)
	}
	joined := strings.Join(vals, ", ")
	switch len(c.params) {
	case 0:
		return joined, nil
	case 1:
		if c.params[0].key == "key" {
			k, ok := c.params[0].value.(string)
			if !ok {
				return "", fmt.Errorf("%w: key parameter", errComponent)
			}
			d, err := parseDict(joined)
			if err != nil {
				return "", fmt.Errorf("covered field %q: %w", c.name, err)
			}
			v, ok := d.get(k)
			if !ok {
				return "", fmt.Errorf("covered field %q has no member %q", c.name, k)
			}
			return stringOf(v)
		}
	}
	return "", fmt.Errorf("%w: parameters on %q", errComponent, c.name)
}

// base builds the signature base (RFC 9421 section 2.5): one line per covered
// component, then the @signature-params line, joined by newlines.
func (s sigInput) base(m *message) ([]byte, error) {
	var b strings.Builder
	seen := map[string]bool{}
	for _, c := range s.items {
		id, err := c.id()
		if err != nil {
			return nil, err
		}
		if seen[id] {
			return nil, fmt.Errorf("covered component %s appears twice", id)
		}
		seen[id] = true
		v, err := m.value(c)
		if err != nil {
			return nil, err
		}
		b.WriteString(id)
		b.WriteString(": ")
		b.WriteString(v)
		b.WriteByte('\n')
	}
	line, err := s.paramsLine()
	if err != nil {
		return nil, err
	}
	b.WriteString(`"@signature-params": `)
	b.WriteString(line)
	return []byte(b.String()), nil
}
