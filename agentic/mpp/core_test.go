// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package mpp

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The three challenge ids of the working copy's HMAC-SHA256 test vectors (section
// 5.1.2.1.1.1): secret test-vector-secret, request {"amount":"1000000"}, no expires and
// no digest. The tests require each to appear in the specification file as well.
func TestBindVectors(t *testing.T) {
	secret := []byte("test-vector-secret")
	req, err := Encode(map[string]string{"amount": "1000000"})
	if err != nil {
		t.Fatal(err)
	}
	if req != "eyJhbW91bnQiOiIxMDAwMDAwIn0" {
		t.Fatalf("JCS request is %s", req)
	}
	opaque, err := Encode(map[string]string{"pi": "pi_123"})
	if err != nil || opaque != "eyJwaSI6InBpXzEyMyJ9" {
		t.Fatalf("JCS opaque is %s, %v", opaque, err)
	}
	base := Challenge{Realm: "api.example.com", Method: "tempo", Intent: "charge", Request: req}
	withHeader := base
	withHeader.Header = "Payment-Authorization"
	withBoth := withHeader
	withBoth.Opaque = opaque

	core := spec(t, "core.md")
	for _, c := range []struct {
		name string
		in   Challenge
		id   string
	}{
		{"legacy layout", base, "X6v1eo7fJ76gAxqY0xN9Jd__4lUyDDYmriryOM-5FO4"},
		{"header, no opaque", withHeader, "S91xi-OFGZPMs-j7GsX0FDpIkmCcZT1P9XyV58WNy_U"},
		{"header and opaque", withBoth, "CJ4X1O4aTDmS59hfdhnhBtxIQjWDOf0bcrhsswwMOW8"},
	} {
		if !strings.Contains(core, c.id) {
			t.Fatalf("%s: vector %s is not in testdata/core.md", c.name, c.id)
		}
		if got := Bind(secret, c.in); got != c.id {
			t.Errorf("%s: Bind = %s, want %s", c.name, got, c.id)
		}
		c.in.ID = c.id
		if !bound(secret, c.in) {
			t.Errorf("%s: a challenge with its own id is not bound", c.name)
		}
		c.in.ID = "A" + c.id[1:]
		if c.id[0] == 'A' {
			c.in.ID = "B" + c.id[1:]
		}
		if bound(secret, c.in) {
			t.Errorf("%s: a changed id is bound", c.name)
		}
	}
}

// Every parameter the binding covers changes the id when it changes; the description,
// which the draft excludes, does not.
func TestBindCoversTheParameters(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	base := Challenge{Realm: "r", Method: "m", Intent: "charge", Request: "cmVx", Expires: "2025-01-15T12:05:00Z", Digest: "sha-256=:AA==:", Opaque: "e30"}
	id := Bind(secret, base)
	for name, mod := range map[string]func(*Challenge){
		"realm":   func(c *Challenge) { c.Realm = "r2" },
		"method":  func(c *Challenge) { c.Method = "n" },
		"intent":  func(c *Challenge) { c.Intent = "session" },
		"request": func(c *Challenge) { c.Request = "cmVxMg" },
		"expires": func(c *Challenge) { c.Expires = "2025-01-15T12:06:00Z" },
		"digest":  func(c *Challenge) { c.Digest = "sha-256=:BB==:" },
		"opaque":  func(c *Challenge) { c.Opaque = "e30x" },
		"header":  func(c *Challenge) { c.Header = alternate },
	} {
		c := base
		mod(&c)
		if Bind(secret, c) == id {
			t.Errorf("changing %s does not change the id", name)
		}
	}
	c := base
	c.Description = "anything"
	if Bind(secret, c) != id {
		t.Error("the description is bound; the draft excludes it")
	}
	// (expires set, no digest) and (no expires, digest set) must differ (section 5.1.2.1.1).
	a, b := base, base
	a.Digest, b.Expires = "", ""
	a.Opaque, b.Opaque = "", ""
	b.Digest = a.Expires
	if Bind(secret, a) == Bind(secret, b) {
		t.Error("optional slots are not positional")
	}
}

// Every fully-formed challenge in the draft parses, formats back to the same challenge,
// and carries a request in JCS form. The ones whose request is elided with "..." are
// not challenges and are rejected.
func TestSpecChallenges(t *testing.T) {
	formed, elided := 0, 0
	for _, f := range []string{"core.md", "stripe-charge.md"} {
		for _, b := range fences(t, f) {
			if b.lang != "http" || !strings.HasPrefix(b.text, "HTTP/1.1 402") {
				continue
			}
			resp := readResponse(t, b.text)
			values := resp.Header.Values("WWW-Authenticate")
			found, err := Parse(values)
			if strings.Contains(b.text, `request="..."`) || strings.Contains(b.text, "WWW-Authenticate: Payment ...") {
				elided++
				if len(found) != 0 || err == nil {
					t.Errorf("%s:%d: an elided request was accepted: %v", f, b.line, found)
				}
				continue
			}
			formed++
			if err != nil || len(found) != len(values) {
				t.Fatalf("%s:%d: %d of %d challenges, %v", f, b.line, len(found), len(values), err)
			}
			for _, c := range found {
				again, err := Parse([]string{c.String()})
				if err != nil || len(again) != 1 || again[0] != c {
					t.Errorf("%s:%d: format does not round-trip: %v %v", f, b.line, again, err)
				}
				decoded, _ := raw.DecodeString(c.Request)
				var v any
				if err := json.Unmarshal(decoded, &v); err != nil {
					t.Fatalf("%s:%d: request is not JSON: %v", f, b.line, err)
				}
				canon, err := Canonical(v)
				if err != nil {
					t.Fatal(err)
				}
				if string(canon) != string(decoded) {
					t.Errorf("%s:%d: request is not JCS:\n have %s\n want %s", f, b.line, decoded, canon)
				}
			}
			// The body, when given, is problem details.
			if i := strings.Index(b.text, "\n\n"); i >= 0 && strings.Contains(b.text[i:], "{") {
				var p Problem
				strict(t, []byte(b.text[i:]), &p)
				if p.Status != 402 || !strings.HasPrefix(p.Type, base) {
					t.Errorf("%s:%d: problem %+v", f, b.line, p)
				}
			}
		}
	}
	if formed < 5 || elided < 5 {
		t.Fatalf("only %d formed and %d elided challenges were found in the specification", formed, elided)
	}
}

// Every credential in the draft parses to the object its decoded form shows, and that
// object decodes strictly into the wire types and encodes back to the same value.
func TestSpecCredentials(t *testing.T) {
	seen := 0
	for _, f := range []string{"core.md", "stripe-charge.md"} {
		blocks := fences(t, f)
		for i, b := range blocks {
			if b.lang != "http" {
				continue
			}
			var value string
			for _, line := range strings.Split(b.text, "\n") {
				for _, p := range []string{"Authorization: ", "Payment-Authorization: "} {
					if v, ok := strings.CutPrefix(line, p); ok && strings.HasPrefix(v, "Payment ") {
						value = strings.TrimSpace(v)
					}
				}
			}
			if value == "" || i+1 >= len(blocks) || blocks[i+1].lang != "json" {
				continue
			}
			seen++
			cred, err := ParseCredential(value)
			if err != nil {
				t.Fatalf("%s:%d: %v", f, b.line, err)
			}
			if err := cred.Challenge.Valid(); err != nil {
				t.Errorf("%s:%d: echoed challenge: %v", f, b.line, err)
			}
			want := []byte(blocks[i+1].text)
			got, err := json.Marshal(cred)
			if err != nil {
				t.Fatal(err)
			}
			sameJSON(t, got, want)
			var round Credential
			strict(t, want, &round)
			if cred2, err := ParseCredential(round.String()); err != nil || cred2.Challenge != round.Challenge {
				t.Errorf("%s:%d: encode does not round-trip: %v", f, b.line, err)
			}
		}
	}
	if seen < 4 {
		t.Fatalf("only %d credentials were found", seen)
	}
	// Appendix B.3 shows a credential as JSON alone; it decodes strictly and carries a source.
	for _, b := range fences(t, "core.md") {
		if b.lang == "json" && strings.Contains(b.text, `"source"`) {
			var c Credential
			strict(t, []byte(b.text), &c)
			if c.Source == "" || c.Challenge.Valid() != nil {
				t.Errorf("signed credential: %+v %v", c, c.Challenge.Valid())
			}
			if _, err := ParseCredential(c.String()); err != nil {
				t.Errorf("signed credential does not round-trip: %v", err)
			}
		}
	}
}

// The receipts the draft shows decode to the object beside them and encode back to it.
func TestSpecReceipts(t *testing.T) {
	blocks := fences(t, "stripe-charge.md")
	seen := 0
	for i, b := range blocks {
		if b.lang != "http" {
			continue
		}
		var value string
		for _, line := range strings.Split(b.text, "\n") {
			if v, ok := strings.CutPrefix(line, "Payment-Receipt: "); ok {
				value = strings.TrimSpace(v)
			}
		}
		if value == "" {
			continue
		}
		seen++
		r, err := ParseReceipt(value)
		if err != nil {
			t.Fatal(err)
		}
		got, _ := json.Marshal(r)
		sameJSON(t, got, []byte(blocks[i+1].text))
		if again, err := ParseReceipt(r.String()); err != nil || again.Reference != r.Reference {
			t.Errorf("receipt does not round-trip: %v", err)
		}
		if r.Method != "stripe" || !strings.HasPrefix(r.Reference, "pi_") {
			t.Errorf("receipt %+v", r)
		}
	}
	// Appendix B.1 of the core draft shows its receipt only encoded.
	for _, b := range fences(t, "core.md") {
		for _, line := range strings.Split(b.text, "\n") {
			if v, ok := strings.CutPrefix(line, "Payment-Receipt: "); ok {
				seen++
				r, err := ParseReceipt(strings.TrimSpace(v))
				if err != nil || r.Method != "invoice" || r.Reference != "inv_12345" || r.Timestamp != "2025-01-15T12:00:00Z" {
					t.Errorf("core receipt %+v %v", r, err)
				}
			}
		}
	}
	if seen != 2 {
		t.Fatalf("found %d receipts, want 2", seen)
	}
}

// A receipt keeps the members a method adds, and a method cannot shadow the four.
func TestReceiptFields(t *testing.T) {
	r := Receipt{Status: "success", Method: "stripe", Timestamp: "2025-01-15T12:04:32Z", Reference: "pi_1", Fields: map[string]any{"externalId": "e1", "status": "failure"}}
	got, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	sameJSON(t, got, []byte(`{"status":"success","method":"stripe","timestamp":"2025-01-15T12:04:32Z","reference":"pi_1","externalId":"e1"}`))
	back, err := ParseReceipt(r.String())
	if err != nil || back.Fields["externalId"] != "e1" {
		t.Fatalf("%+v %v", back, err)
	}
	if _, err := ParseReceipt(Receipt{Status: "failure", Timestamp: "2025-01-15T12:04:32Z"}.String()); err == nil {
		t.Error("a receipt that is not success was accepted")
	}
}

// The draft's own digest example: the SHA-256 of {"hello": "world"} (RFC 9530).
func TestDigest(t *testing.T) {
	const want = "sha-256=:X48E9qOokqqrvdts8nOJRJN3OWDUoyWxBf7kbu9DBPE=:"
	if !strings.Contains(spec(t, "core.md"), want) {
		t.Fatal("the digest example is not in testdata/core.md")
	}
	body := []byte(`{"hello": "world"}`)
	if got := digestOf(body); got != want {
		t.Fatalf("digestOf = %s", got)
	}
	if !digestMatches(want, body) || digestMatches(want, []byte(`{"hello": "world!"}`)) {
		t.Error("digestMatches")
	}
	for _, bad := range []string{"", "sha-256", "md5=:AA==:", "sha-256=:AAAA:", want[:len(want)-1]} {
		if digestMatches(bad, body) {
			t.Errorf("%q matched", bad)
		}
	}
}

// The error table is the draft's: the same codes with the same statuses, none missing.
func TestProblemTable(t *testing.T) {
	core := spec(t, "core.md")
	rows := regexp.MustCompile("(?m)^\\| `([a-z-]+)` \\| ([45][0-9]{2}) \\|").FindAllStringSubmatch(core[strings.Index(core, "## Error Codes"):], -1)
	if len(rows) != len(status) {
		t.Fatalf("the draft lists %d problem types, the package has %d", len(rows), len(status))
	}
	for _, r := range rows {
		n, _ := strconv.Atoi(r[2])
		if status[Code(r[1])] != n {
			t.Errorf("%s: status %d, draft says %d", r[1], status[Code(r[1])], n)
		}
		p := Refuse(Code(r[1]), "d")
		if p.Type != "https://paymentauth.org/problems/"+r[1] || p.Code() != Code(r[1]) || p.Status != n || p.Title == "" {
			t.Errorf("problem %+v", p)
		}
	}
	if p := Refuse("no-such-code", "d"); p.Code() != Internal {
		t.Errorf("an unknown code is %s", p.Code())
	}
	// The titles the draft's examples spell out.
	if title(Required) != "Payment Required" || title(Failed) != "Payment Verification Failed" || title(Invalid) != "Invalid Challenge" {
		t.Error("titles")
	}
}

// Section 5.1.1: an empty or missing id, a method that is not lowercase letters, and a
// request with padding are not challenges; unknown parameters are ignored.
func TestParseRejectsMalformed(t *testing.T) {
	good := `Payment id="a", realm="r", method="stripe", intent="charge", request="e30"`
	if cs, err := Parse([]string{good + `, future="x"`}); err != nil || len(cs) != 1 || cs[0].ID != "a" {
		t.Fatalf("a challenge with an unknown parameter: %v %v", cs, err)
	}
	for name, v := range map[string]string{
		"empty id":        `Payment id="", realm="r", method="stripe", intent="charge", request="e30"`,
		"missing id":      `Payment realm="r", method="stripe", intent="charge", request="e30"`,
		"missing realm":   `Payment id="a", method="stripe", intent="charge", request="e30"`,
		"upper method":    `Payment id="a", realm="r", method="Stripe", intent="charge", request="e30"`,
		"digit method":    `Payment id="a", realm="r", method="s3", intent="charge", request="e30"`,
		"bad intent":      `Payment id="a", realm="r", method="stripe", intent="char ge", request="e30"`,
		"padded request":  `Payment id="a", realm="r", method="stripe", intent="charge", request="e30="`,
		"bad expires":     `Payment id="a", realm="r", method="stripe", intent="charge", request="e30", expires="tomorrow"`,
		"other header":    `Payment id="a", realm="r", method="stripe", intent="charge", request="e30", header="X-Pay"`,
		"bad digest":      `Payment id="a", realm="r", method="stripe", intent="charge", request="e30", digest="sha-256=AA"`,
		"opaque not map":  `Payment id="a", realm="r", method="stripe", intent="charge", request="e30", opaque="WzFd"`,
		"opaque non-text": `Payment id="a", realm="r", method="stripe", intent="charge", request="e30", opaque="eyJhIjoxfQ"`,
		"duplicate id":    `Payment id="a", id="b", realm="r", method="stripe", intent="charge", request="e30"`,
		"unterminated":    `Payment id="a, realm="r`,
	} {
		if cs, err := Parse([]string{v}); len(cs) != 0 || err == nil {
			t.Errorf("%s: accepted %v", name, cs)
		}
	}
}

// Several challenges in one field value, and challenges of other schemes among them.
func TestParseSeveralSchemes(t *testing.T) {
	cs, err := Parse([]string{
		`Basic realm="x", Payment id="a", realm="r", method="stripe", intent="charge", request="e30", Bearer realm="y"`,
		`Negotiate abc==`,
		`payment ID="b", Realm="r", Method="tempo", Intent="charge", Request="e30"`,
	})
	if err != nil || len(cs) != 2 || cs[0].ID != "a" || cs[1].ID != "b" || cs[1].Method != "tempo" {
		t.Fatalf("%+v %v", cs, err)
	}
	if cs[0].Realm != "r" {
		t.Errorf("the Basic challenge's realm leaked into the Payment one: %q", cs[0].Realm)
	}
}

// A challenge that carries a quote or a backslash in its description survives, and a
// control character cannot end the header line.
func TestChallengeQuoting(t *testing.T) {
	c := Challenge{ID: "a", Realm: "r", Method: "stripe", Intent: "charge", Request: "e30", Description: "say \"hi\" \\ and\r\nX-Injected: 1"}
	s := c.String()
	if strings.ContainsAny(s, "\r\n") {
		t.Fatalf("control characters in %q", s)
	}
	cs, err := Parse([]string{s})
	if err != nil || len(cs) != 1 || cs[0].Description != "say \"hi\" \\ and  X-Injected: 1" {
		t.Fatalf("%+v %v", cs, err)
	}
}

// Credentials: another scheme is not a credential, a credential must be base64url
// without padding holding a challenge and a payload object.
func TestParseCredentialRejects(t *testing.T) {
	if _, err := ParseCredential("Bearer abc"); err != errNotPayment {
		t.Errorf("Bearer: %v", err)
	}
	enc := func(s string) string { return "Payment " + rawEncode(s) }
	for name, v := range map[string]string{
		"empty":           "Payment ",
		"padding":         "Payment e30=",
		"not base64url":   "Payment e+0",
		"not json":        enc("nope"),
		"array":           enc("[]"),
		"no challenge":    enc(`{"payload":{}}`),
		"no payload":      enc(`{"challenge":{"id":"a"}}`),
		"payload string":  enc(`{"challenge":{"id":"a"},"payload":"x"}`),
		"non-text member": enc(`{"challenge":{"id":1},"payload":{}}`),
	} {
		if _, err := ParseCredential(v); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// Unknown members are ignored (section 9.2.1).
	if c, err := ParseCredential(enc(`{"challenge":{"id":"a","future":1},"payload":{"x":1},"extra":true}`)); err != nil || c.Challenge.ID != "a" {
		t.Errorf("unknown members: %v", err)
	}
	// The scheme is case-insensitive and tolerates more than one space.
	if _, err := ParseCredential("payment   " + rawEncode(`{"challenge":{"id":"a"},"payload":{}}`)); err != nil {
		t.Errorf("scheme case or spacing: %v", err)
	}
}

func rawEncode(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
