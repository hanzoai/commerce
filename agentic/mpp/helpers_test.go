// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package mpp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func stringsReader(s string) io.Reader { return strings.NewReader(s) }

// epoch is the instant the draft's examples are set at: their challenges expire five
// minutes later, at 2025-01-15T12:05:00Z.
var epoch = time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

// clock is a clock the tests move.
type clock struct {
	mu sync.Mutex
	at time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(d)
}

// block is one fenced code block of a specification file.
type block struct {
	lang string
	text string
	line int
}

// spec reads a verbatim copy of a specification file from testdata.
func spec(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// fences returns the ~~~ fenced blocks of a specification, in order.
func fences(t *testing.T, name string) []block {
	t.Helper()
	var out []block
	var cur *block
	for i, line := range strings.Split(spec(t, name), "\n") {
		switch {
		case cur == nil && strings.HasPrefix(line, "~~~"):
			cur = &block{lang: strings.TrimSpace(strings.TrimPrefix(line, "~~~")), line: i + 1}
		case cur != nil && strings.TrimSpace(line) == "~~~":
			out = append(out, *cur)
			cur = nil
		case cur != nil:
			cur.text += line + "\n"
		}
	}
	return out
}

// readResponse parses an HTTP response written out in a specification.
func readResponse(t *testing.T, text string) *http.Response {
	t.Helper()
	r, err := http.ReadResponse(bufio.NewReader(strings.NewReader(terminated(text))), nil)
	if err != nil {
		t.Fatalf("response: %v\n%s", err, text)
	}
	return r
}

// readRequest parses an HTTP request written out in a specification.
func readRequest(t *testing.T, text string) *http.Request {
	t.Helper()
	r, err := http.ReadRequest(bufio.NewReader(strings.NewReader(terminated(text))))
	if err != nil {
		t.Fatalf("request: %v\n%s", err, text)
	}
	return r
}

// terminated ends the header section of a message a specification shows without the
// blank line that closes it.
func terminated(text string) string {
	if !strings.Contains(text, "\n\n") {
		return text + "\n"
	}
	return text
}

// sameJSON requires two JSON documents to hold the same value.
func sameJSON(t *testing.T, got, want []byte) {
	t.Helper()
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatalf("got is not JSON: %v\n%s", err, got)
	}
	if err := json.Unmarshal(want, &w); err != nil {
		t.Fatalf("want is not JSON: %v\n%s", err, want)
	}
	gb, _ := json.Marshal(g)
	wb, _ := json.Marshal(w)
	if string(gb) != string(wb) {
		t.Fatalf("JSON differs\n got  %s\n want %s", gb, wb)
	}
}

// strict decodes JSON refusing any member the type does not name, so a specification
// example the wire types fail to model is caught.
func strict(t *testing.T, b []byte, v any) {
	t.Helper()
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		t.Fatalf("strict decode: %v\n%s", err, b)
	}
}

// invoice is the method of the draft's one-time charge example (appendix B.1): an
// invoice whose preimage is the proof. It settles by comparing the preimage.
type invoice struct {
	// name is the method identifier; "invoice" when empty.
	name    string
	mu      sync.Mutex
	settled int
	// crash makes Settle fail with an error that is not a refusal, an outcome not known.
	crash bool
}

const preimage = "0xabc123..."

func (i *invoice) Name() string {
	if i.name != "" {
		return i.name
	}
	return "invoice"
}

func (*invoice) Propose(_ context.Context, _ Caller, q Quote) (*Proposal, error) {
	req := map[string]string{"amount": q.Amount, "currency": q.Currency, "invoice": "inv_12345"}
	if q.External != "" {
		req["externalId"] = q.External
	}
	return &Proposal{Request: req}, nil
}

func (*invoice) Verify(_ context.Context, c Claim) error {
	var p struct {
		Preimage string `json:"preimage"`
	}
	if err := json.Unmarshal(c.Payload, &p); err != nil || p.Preimage == "" {
		return Refuse(Payload, "The payload needs a preimage.")
	}
	return nil
}

func (i *invoice) Settle(_ context.Context, c Claim) (*Receipt, error) {
	var p struct {
		Preimage string `json:"preimage"`
	}
	_ = json.Unmarshal(c.Payload, &p)
	if p.Preimage != preimage {
		return nil, Refuse(Failed, "Invalid payment proof.")
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.crash {
		return nil, context.DeadlineExceeded
	}
	i.settled++
	return &Receipt{Reference: "inv_12345"}, nil
}

func (i *invoice) count() int {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.settled
}

// memory is a Store held in one process.
type memory struct {
	mu    sync.Mutex
	taken map[string]bool
	saved map[string][]byte
}

func (m *memory) Take(_ context.Context, key string, _ time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.taken == nil {
		m.taken = map[string]bool{}
	}
	if m.taken[key] {
		return false, nil
	}
	m.taken[key] = true
	return true, nil
}

func (m *memory) Drop(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.taken, key)
	return nil
}

func (m *memory) Save(_ context.Context, key string, value []byte, _ time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saved == nil {
		m.saved = map[string][]byte{}
	}
	m.saved[key] = append([]byte(nil), value...)
	return nil
}

func (m *memory) Load(_ context.Context, key string) ([]byte, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.saved[key]
	return append([]byte(nil), v...), ok, nil
}

// codeOf is the problem type's code, "" when it is not one of the draft's.
func codeOf(p *Problem) Code {
	c, _ := strings.CutPrefix(p.Type, base)
	return Code(c)
}
