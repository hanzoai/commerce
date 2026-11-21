// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ap2

import (
	"fmt"
	"strings"
)

// token is one SD-JWT (RFC 9901): an issuer-signed JWT and the disclosures sent with it.
type token struct {
	jws   *jws
	jwt   string
	discs []string
	// claims is the signed payload, disclosures not yet applied.
	claims map[string]any
}

// text is the token as it is hashed: the JWT, each disclosure, and a final "~" (RFC 9901 4.2.3).
func (t *token) text() string {
	if len(t.discs) == 0 {
		return t.jwt + "~"
	}
	return t.jwt + "~" + strings.Join(t.discs, "~") + "~"
}

// hash is the token's hash, as `sd_hash` and a receipt's `reference` are calculated.
func (t *token) hash() string { return digest(t.text()) }

// id names the mandate the token carries: the hash of what the issuer signed. It leaves out the
// signature (a signed JWT has more than one valid signature) and the disclosures (the holder
// chooses which to show), so a holder cannot present one mandate as another.
func (t *token) id() string { return digest(t.jws.input) }

func newToken(jwt string, discs []string) (*token, error) {
	j, err := parseJWS(jwt)
	if err != nil {
		return nil, err
	}
	claims, err := object(j.payload)
	if err != nil {
		return nil, fmt.Errorf("%w: JWT payload: %v", ErrMalformed, err)
	}
	if alg, ok := claims["_sd_alg"]; ok && alg != "sha-256" {
		return nil, fmt.Errorf("%w: _sd_alg %v", ErrAlgorithm, alg)
	}
	return &token{jws: j, jwt: jwt, discs: discs, claims: claims}, nil
}

// presentation is an issuer-signed token and, when the agent signs the closed mandate, the
// key-bound token that follows it, joined by an empty component: "~~" (Delegate SD-JWT).
type presentation struct{ tokens []*token }

// parsePresentation splits a presentation into its tokens; AP2 uses no trailing KB-JWT.
func parsePresentation(s string) (*presentation, *Error) {
	parts := strings.Split(s, "~")
	if len(parts) < 2 || parts[len(parts)-1] != "" {
		return nil, credential(ErrMalformed, "a presentation ends with ~")
	}
	parts = parts[:len(parts)-1]
	var p presentation
	for i := 0; i < len(parts); {
		jwt := parts[i]
		if jwt == "" {
			return nil, credential(ErrMalformed, "a token starts with its JWT")
		}
		var discs []string
		for i++; i < len(parts) && parts[i] != ""; i++ {
			discs = append(discs, parts[i])
		}
		if i < len(parts) {
			i++ // the empty component that joins this token to the next
			if i == len(parts) {
				return nil, credential(ErrMalformed, "a join with no token after it")
			}
		}
		if len(p.tokens) == 2 {
			return nil, credential(ErrChain, "delegation between agents is outside AP2 v0.2")
		}
		t, err := newToken(jwt, discs)
		if err != nil {
			return nil, &Error{Code: InvalidCredential, Err: err}
		}
		p.tokens = append(p.tokens, t)
	}
	return &p, nil
}

// last is the token that carries the closed mandate.
func (p *presentation) last() *token { return p.tokens[len(p.tokens)-1] }

// reference is what a receipt's `reference` names: the hash of the closed mandate's final
// SD-JWT, disclosures included (specification.md "Dispute", agent_authorization.md "Action
// Authorization").
func (p *presentation) reference() string { return p.last().hash() }

// disclosure is a decoded SD-JWT disclosure: [salt, value] for an array element, or
// [salt, name, value] for an object member.
type disclosure struct {
	parts []any
	used  bool
}

// resolve applies the token's disclosures to its payload (RFC 9901 section 7.1): each
// digest in an `_sd` array or an array element `{"...": digest}` that matches a disclosure is
// replaced by its value, one that matches none is dropped. A disclosure no digest names, a
// digest named twice, and a claim that would overwrite another are refusals.
func (t *token) resolve() (map[string]any, *Error) {
	r := &resolver{byDigest: map[string]*disclosure{}, seen: map[string]bool{}}
	for _, d := range t.discs {
		dg := digest(d)
		raw, err := unb64(d)
		if err != nil {
			return nil, credential(ErrDisclosure, "not base64url: %v", err)
		}
		v, err := parse(raw)
		if err != nil {
			return nil, credential(ErrDisclosure, "%v", err)
		}
		parts, ok := v.([]any)
		if !ok || len(parts) < 2 || len(parts) > 3 {
			return nil, credential(ErrDisclosure, "a disclosure is a JSON array of 2 or 3 elements")
		}
		if salt, ok := parts[0].(string); !ok || salt == "" {
			return nil, credential(ErrDisclosure, "a disclosure starts with its salt")
		}
		if _, dup := r.byDigest[dg]; dup {
			return nil, credential(ErrDisclosure, "a disclosure is sent twice")
		}
		r.byDigest[dg] = &disclosure{parts: parts}
	}
	out, err := r.object(t.claims, scope{})
	if err != nil {
		return nil, err
	}
	if r.withheld {
		return nil, unresolved("a constraint, or a member of one, was withheld: only the elements of an allowed list may be")
	}
	for _, d := range r.byDigest {
		if !d.used {
			return nil, credential(ErrDisclosure, "a disclosure no digest refers to")
		}
	}
	delete(out, "_sd_alg")
	return out, nil
}

type resolver struct {
	byDigest map[string]*disclosure
	seen     map[string]bool
	// withheld is set when a digest inside a mandate's constraints names no disclosure and is
	// not an element of a list the schema makes selectively disclosable. Dropping it would drop
	// a constraint, or a limit of one, and so widen what the mandate allows.
	withheld bool
}

// scope says where in a payload resolution is: inside constraints, and the member that holds the
// array being resolved.
type scope struct {
	constraints bool
	key         string
}

// discloses says digests withheld here are the holder's to withhold: the elements of the
// allowed and acceptable_items lists (open_checkout_mandate.json, open_payment_mandate.json,
// x-selectively-disclosable-array), and anything outside constraints.
func (s scope) discloses(member bool) bool {
	return !s.constraints || (!member && (s.key == "allowed" || s.key == "acceptable_items"))
}

// claim takes the disclosure a digest names, if one was sent.
func (r *resolver) claim(dg string, width int) (*disclosure, *Error) {
	if r.seen[dg] {
		return nil, credential(ErrDisclosure, "a digest appears twice")
	}
	r.seen[dg] = true
	d, ok := r.byDigest[dg]
	if !ok {
		return nil, nil // withheld, or a decoy
	}
	if len(d.parts) != width {
		return nil, credential(ErrDisclosure, "a disclosure of the wrong kind for where its digest sits")
	}
	d.used = true
	return d, nil
}

func (r *resolver) object(m map[string]any, sc scope) (map[string]any, *Error) {
	out := make(map[string]any, len(m))
	for k, v := range m {
		if k == "_sd" {
			continue
		}
		rv, err := r.value(v, scope{constraints: sc.constraints || k == "constraints", key: k})
		if err != nil {
			return nil, err
		}
		out[k] = rv
	}
	raw, ok := m["_sd"]
	if !ok {
		return out, nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, credential(ErrDisclosure, "_sd is not an array")
	}
	for _, e := range list {
		dg, ok := e.(string)
		if !ok {
			return nil, credential(ErrDisclosure, "_sd holds a non-string")
		}
		d, err := r.claim(dg, 3)
		if err != nil {
			return nil, err
		}
		if d == nil {
			r.withheld = r.withheld || !sc.discloses(true)
			continue
		}
		name, ok := d.parts[1].(string)
		if !ok || name == "_sd" || name == "..." {
			return nil, credential(ErrDisclosure, "a disclosure names the claim %q", name)
		}
		if _, dup := out[name]; dup {
			return nil, credential(ErrDisclosure, "a disclosure overwrites the claim %q", name)
		}
		if out[name], err = r.value(d.parts[2], scope{constraints: sc.constraints || name == "constraints", key: name}); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *resolver) value(v any, sc scope) (any, *Error) {
	switch t := v.(type) {
	case map[string]any:
		return r.object(t, sc)
	case []any:
		out := make([]any, 0, len(t))
		for _, e := range t {
			if m, ok := e.(map[string]any); ok {
				if raw, elem := m["..."]; elem {
					dg, ok := raw.(string)
					if !ok || len(m) != 1 {
						return nil, credential(ErrDisclosure, `an array element "..." holds only a digest`)
					}
					d, err := r.claim(dg, 2)
					if err != nil {
						return nil, err
					}
					if d == nil {
						r.withheld = r.withheld || !sc.discloses(false)
						continue
					}
					rv, err := r.value(d.parts[1], sc)
					if err != nil {
						return nil, err
					}
					out = append(out, rv)
					continue
				}
			}
			rv, err := r.value(e, sc)
			if err != nil {
				return nil, err
			}
			out = append(out, rv)
		}
		return out, nil
	}
	return v, nil
}
