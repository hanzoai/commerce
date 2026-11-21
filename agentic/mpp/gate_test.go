// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package mpp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var testSecret = []byte("0123456789abcdef0123456789abcdef")

// rig is a Gate in front of a counting resource, with the draft's example price: 1000
// of usd, paid by the invoice method, at a clock set to the draft's examples.
type rig struct {
	t        *testing.T
	cfg      Config
	store    *Memory
	clock    *clock
	invoice  *invoice
	hits     atomic.Int32
	serve    func(http.ResponseWriter, *http.Request)
	mu       sync.Mutex
	reported []error
	h        http.Handler
}

func newRig(t *testing.T, mods ...func(*Config)) *rig {
	t.Helper()
	r := &rig{t: t, clock: &clock{at: epoch}, invoice: &invoice{}}
	r.store = &Memory{Now: r.clock.now}
	r.cfg = Config{
		Realm: "api.example.com", Secret: testSecret, Store: r.store, Methods: []Method{r.invoice},
		Price: func(*http.Request) (*Quote, error) { return &Quote{Amount: "1000", Currency: "usd"}, nil },
		Now:   r.clock.now,
		Report: func(err error) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.reported = append(r.reported, err)
		},
	}
	for _, m := range mods {
		m(&r.cfg)
	}
	g, err := New(r.cfg)
	if err != nil {
		t.Fatal(err)
	}
	r.h = g.Wrap(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.hits.Add(1)
		if r.serve != nil {
			r.serve(w, req)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data": "..."}`)
	}))
	return r
}

func (r *rig) reports() []error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]error(nil), r.reported...)
}

// do sends a request to the Gate over TLS and returns the recorded response.
func (r *rig) do(method, path string, hdr http.Header, body string) *http.Response {
	r.t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, "https://api.example.com"+path, rd)
	for k, v := range hdr {
		req.Header[k] = v
	}
	rec := httptest.NewRecorder()
	r.h.ServeHTTP(rec, req)
	return rec.Result()
}

// challenge asks for a resource and returns the challenges offered.
func (r *rig) challenge(method, path string, hdr http.Header, body string) []Challenge {
	r.t.Helper()
	resp := r.do(method, path, hdr, body)
	if resp.StatusCode != 402 {
		r.t.Fatalf("status %d, want 402", resp.StatusCode)
	}
	cs, err := Parse(resp.Header.Values("WWW-Authenticate"))
	if err != nil || len(cs) == 0 {
		r.t.Fatalf("no challenge: %v", err)
	}
	return cs
}

func credential(c Challenge, payload string) Credential {
	return Credential{Challenge: c, Payload: json.RawMessage(payload)}
}

const proof = `{"preimage":"0xabc123..."}`

func auth(v string) http.Header { return http.Header{"Authorization": {v}} }

func problem(t *testing.T, resp *http.Response) Problem {
	t.Helper()
	var p Problem
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		t.Fatalf("problem body: %v", err)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("problem content type %q", ct)
	}
	return p
}

// wantProblem requires a 402 carrying the problem and a fresh challenge, and never a
// receipt, and that the response is not cacheable (sections 4.2, 5.3.1 and 11.10).
func wantProblem(t *testing.T, resp *http.Response, status int, code Code) {
	t.Helper()
	if resp.StatusCode != status {
		t.Fatalf("status %d, want %d", resp.StatusCode, status)
	}
	if p := problem(t, resp); p.Code() != code || p.Status != status {
		t.Fatalf("problem %+v, want %s", p, code)
	}
	if status == 402 {
		if cs, err := Parse(resp.Header.Values("WWW-Authenticate")); err != nil || len(cs) == 0 {
			t.Fatalf("a 402 with no fresh challenge: %v", err)
		}
	}
	if resp.Header.Get("Payment-Receipt") != "" {
		t.Fatal("a receipt on an error response")
	}
	if resp.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control %q", resp.Header.Get("Cache-Control"))
	}
}

// Appendix B.1 of the draft, end to end: the challenge the Gate issues is the draft's but
// for the id it chose, the credential the draft shows is accepted once the id is the one
// issued, and the receipt is the draft's.
func TestGateOneTimeCharge(t *testing.T) {
	var challengeBlock, credentialBlock, successBlock block
	for _, b := range fences(t, "core.md") {
		switch {
		case b.lang == "http" && strings.Contains(b.text, `id="qB3wErTyU7iOpAsD9fGhJk"`) && strings.HasPrefix(b.text, "HTTP/1.1 402"):
			challengeBlock = b
		case b.lang == "http" && strings.Contains(b.text, "Authorization: Payment eyJjaGFsbGVuZ2UiOnsiZXhwaXJlcyI6"):
			credentialBlock = b
		case b.lang == "http" && strings.HasPrefix(b.text, "HTTP/1.1 200 OK") && strings.Contains(b.text, "Payment-Receipt: eyJzdGF0dXMiOiJzdWNjZXNzIiwibWV0aG9kIjoiaW52b2ljZSI"):
			successBlock = b
		}
	}
	if challengeBlock.text == "" || credentialBlock.text == "" || successBlock.text == "" {
		t.Fatal("appendix B.1 blocks not found")
	}
	r := newRig(t)

	// Step 1 and 2: the request the draft shows, the challenge it shows back.
	wantResp := readResponse(t, challengeBlock.text)
	wantChallenges, err := Parse(wantResp.Header.Values("WWW-Authenticate"))
	if err != nil || len(wantChallenges) != 1 {
		t.Fatal(err)
	}
	resp := r.do("GET", "/resource", nil, "")
	if resp.StatusCode != 402 || resp.Header.Get("Cache-Control") != wantResp.Header.Get("Cache-Control") ||
		resp.Header.Get("Content-Type") != wantResp.Header.Get("Content-Type") {
		t.Fatalf("response %d %v", resp.StatusCode, resp.Header)
	}
	got, err := Parse(resp.Header.Values("WWW-Authenticate"))
	if err != nil || len(got) != 1 {
		t.Fatal(err)
	}
	// The id is the server's choice: the binding of the other parameters under its secret.
	if got[0].ID != Bind(testSecret, got[0]) {
		t.Error("the challenge id is not the HMAC binding")
	}
	// The id and the opaque nonce that makes it unique are the server's choice.
	nonce, err := Opaque(got[0].Opaque)
	if err != nil || len(nonce) != 1 || len(nonce["nonce"]) != 22 {
		t.Fatalf("opaque %q: %v %v", got[0].Opaque, nonce, err)
	}
	want := wantChallenges[0]
	want.ID, want.Opaque = got[0].ID, got[0].Opaque
	if got[0] != want {
		t.Errorf("challenge differs from the draft's beyond its id\n got  %+v\n want %+v", got[0], want)
	}
	var body, wantBody Problem
	_ = json.NewDecoder(resp.Body).Decode(&body)
	_ = json.NewDecoder(strings.NewReader(challengeBlock.text[strings.Index(challengeBlock.text, "\n\n"):])).Decode(&wantBody)
	wantBody.Challenge = got[0].ID // the extension member carries the server's id
	if body != wantBody {
		t.Errorf("problem body\n got  %+v\n want %+v", body, wantBody)
	}

	// Step 4 and 5: the credential the draft shows, with the issued id.
	value := ""
	for _, line := range strings.Split(credentialBlock.text, "\n") {
		if v, ok := strings.CutPrefix(line, "Authorization: "); ok {
			value = v
		}
	}
	cred, err := ParseCredential(value)
	if err != nil {
		t.Fatal(err)
	}
	// The draft's credential echoes the draft's challenge, which has neither the id nor the
	// opaque nonce the server chose; a client echoes both.
	cred.Challenge.ID, cred.Challenge.Opaque = got[0].ID, got[0].Opaque
	resp = r.do("GET", "/resource", auth(cred.String()), "")
	if resp.StatusCode != 200 {
		t.Fatalf("paid request: %d %v", resp.StatusCode, problem(t, resp))
	}
	wantSuccess := readResponse(t, successBlock.text)
	if resp.Header.Get("Cache-Control") != wantSuccess.Header.Get("Cache-Control") {
		t.Errorf("Cache-Control %q", resp.Header.Get("Cache-Control"))
	}
	rcpt, err := ParseReceipt(resp.Header.Get("Payment-Receipt"))
	wantRcpt, werr := ParseReceipt(wantSuccess.Header.Get("Payment-Receipt"))
	if err != nil || werr != nil || rcpt.String() != wantRcpt.String() {
		t.Fatalf("receipt %+v %v, want %+v", rcpt, err, wantRcpt)
	}
	if b, _ := io.ReadAll(resp.Body); string(b) != `{"data": "..."}` {
		t.Errorf("body %q", b)
	}
	if r.hits.Load() != 1 || r.invoice.count() != 1 {
		t.Fatalf("hits %d, settled %d", r.hits.Load(), r.invoice.count())
	}

	// The same credential is not accepted twice (section 11.3).
	wantProblem(t, r.do("GET", "/resource", auth(cred.String()), ""), 402, Invalid)
	if r.hits.Load() != 1 || r.invoice.count() != 1 {
		t.Fatalf("after the replay: hits %d, settled %d", r.hits.Load(), r.invoice.count())
	}
}

// The unpaid request changes nothing (section 11.4): no settlement, no resource, and the
// 402 carries a challenge, problem details and Cache-Control: no-store.
func TestGateUnpaid(t *testing.T) {
	r := newRig(t)
	resp := r.do("GET", "/resource", nil, "")
	wantProblem(t, resp, 402, Required)
	if r.hits.Load() != 0 || r.invoice.count() != 0 {
		t.Fatal("an unpaid request reached the resource")
	}
	// Another scheme's credential is not a Payment credential.
	wantProblem(t, r.do("GET", "/resource", auth("Bearer abc"), ""), 402, Required)
}

func TestGateRefusesBadCredentials(t *testing.T) {
	r := newRig(t)
	ch := r.challenge("GET", "/resource", nil, "")[0]
	good := credential(ch, proof)

	mutate := func(f func(*Challenge)) string {
		c := ch
		f(&c)
		return credential(c, proof).String()
	}
	req2, _ := Encode(map[string]string{"amount": "1", "currency": "usd", "invoice": "inv_12345"})
	cases := []struct {
		name   string
		header string
		status int
		code   Code
	}{
		{"not base64url", "Payment !!!", 402, Malformed},
		{"not json", "Payment " + rawEncode("nope"), 402, Malformed},
		{"empty id", mutate(func(c *Challenge) { c.ID = "" }), 402, Malformed},
		{"unknown id", mutate(func(c *Challenge) { c.ID = "AAAA" }), 402, Invalid},
		{"amount lowered", mutate(func(c *Challenge) { c.Request = req2 }), 402, Invalid},
		{"realm changed", mutate(func(c *Challenge) { c.Realm = "evil.example.com" }), 402, Invalid},
		{"method changed", mutate(func(c *Challenge) { c.Method = "other" }), 402, Invalid},
		{"expiry extended", mutate(func(c *Challenge) { c.Expires = "2099-01-01T00:00:00Z" }), 402, Invalid},
		{"expiry removed", mutate(func(c *Challenge) { c.Expires = "" }), 402, Invalid},
		{"intent changed", mutate(func(c *Challenge) { c.Intent = "session" }), 402, Invalid},
		{"header added", mutate(func(c *Challenge) { c.Header = alternate }), 402, Invalid},
		{"opaque added", mutate(func(c *Challenge) { c.Opaque = rawEncode(`{"a":"b"}`) }), 402, Invalid},
		{"digest added", mutate(func(c *Challenge) { c.Digest = digestOf([]byte("x")) }), 402, Invalid},
		{"payload not an object", "Payment " + rawEncode(`{"challenge":{"id":"x"},"payload":[]}`), 402, Malformed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wantProblem(t, r.do("GET", "/resource", auth(c.header), ""), c.status, c.code)
		})
	}
	if r.hits.Load() != 0 || r.invoice.count() != 0 {
		t.Fatal("a bad credential reached the resource or settled")
	}
	// None of those spent the good one.
	if resp := r.do("GET", "/resource", auth(good.String()), ""); resp.StatusCode != 200 {
		t.Fatalf("the good credential: %d", resp.StatusCode)
	}
}

func TestGateOversizedCredential(t *testing.T) {
	r := newRig(t, func(c *Config) { c.Limit = 4096 })
	big := "Payment " + strings.Repeat("A", 5000)
	wantProblem(t, r.do("GET", "/resource", auth(big), ""), 402, Malformed)
}

// Section 5.1.2: a client must not submit for an expired challenge, and the server refuses it.
func TestGateExpired(t *testing.T) {
	r := newRig(t)
	ch := r.challenge("GET", "/resource", nil, "")[0]
	if ch.Expires != "2025-01-15T12:05:00Z" {
		t.Fatalf("expires %s", ch.Expires)
	}
	r.clock.advance(5 * time.Minute)
	wantProblem(t, r.do("GET", "/resource", auth(credential(ch, proof).String()), ""), 402, Expired)
	if r.invoice.count() != 0 {
		t.Fatal("an expired challenge settled")
	}
}

// Section 5.2: the credential travels in the field the challenge selects, and in no other.
func TestGateField(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		r := newRig(t)
		ch := r.challenge("GET", "/resource", nil, "")[0]
		if ch.Header != "" {
			t.Fatalf("header %q", ch.Header)
		}
		c := credential(ch, proof).String()
		wantProblem(t, r.do("GET", "/resource", http.Header{"Payment-Authorization": {c}}, ""), 402, Malformed)
		if resp := r.do("GET", "/resource", auth(c), ""); resp.StatusCode != 200 {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})
	t.Run("alternate", func(t *testing.T) {
		r := newRig(t, func(c *Config) { c.Alternate = true })
		ch := r.challenge("GET", "/resource", nil, "")[0]
		if ch.Header != alternate {
			t.Fatalf("header %q", ch.Header)
		}
		c := credential(ch, proof).String()
		wantProblem(t, r.do("GET", "/resource", auth(c), ""), 402, Malformed)
		// The ordinary authentication stays in Authorization beside the payment.
		h := http.Header{"Authorization": {"Bearer mF_9.B5f-4.1JqM"}, "Payment-Authorization": {c}}
		resp := r.do("GET", "/resource", h, "")
		if resp.StatusCode != 200 {
			t.Fatalf("status %d", resp.StatusCode)
		}
		// Section 11.10: this field has no shared-cache protection of its own.
		if cc := resp.Header.Get("Cache-Control"); cc != "private" {
			t.Errorf("Cache-Control %q", cc)
		}
	})
	t.Run("alternate error responses are not cacheable", func(t *testing.T) {
		r := newRig(t, func(c *Config) { c.Alternate = true })
		ch := r.challenge("GET", "/resource", nil, "")[0]
		r.serve = func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(500) }
		resp := r.do("GET", "/resource", http.Header{"Payment-Authorization": {credential(ch, proof).String()}}, "")
		if resp.StatusCode != 500 || resp.Header.Get("Cache-Control") != "private" || resp.Header.Get("Payment-Receipt") != "" {
			t.Fatalf("%d %v", resp.StatusCode, resp.Header)
		}
	})
}

// Appendix B.4: more than one Payment credential is a bad request.
func TestGateRefusesTwoCredentials(t *testing.T) {
	r := newRig(t)
	ch := r.challenge("GET", "/resource", nil, "")[0]
	c := credential(ch, proof).String()
	h := http.Header{"Authorization": {c}, "Payment-Authorization": {c}}
	wantProblem(t, r.do("GET", "/resource", h, ""), 400, Bad)
	h = http.Header{"Authorization": {c, c}}
	wantProblem(t, r.do("GET", "/resource", h, ""), 400, Bad)
	if r.invoice.count() != 0 {
		t.Fatal("settled")
	}
}

// Sections 11.3 and 11.5: concurrent requests with one credential settle once.
func TestGateConcurrentCredential(t *testing.T) {
	r := newRig(t)
	ch := r.challenge("GET", "/resource", nil, "")[0]
	c := credential(ch, proof).String()
	var wg sync.WaitGroup
	var ok atomic.Int32
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if resp := r.do("GET", "/resource", auth(c), ""); resp.StatusCode == 200 {
				ok.Add(1)
			}
		}()
	}
	wg.Wait()
	if ok.Load() != 1 || r.invoice.count() != 1 || r.hits.Load() != 1 {
		t.Fatalf("%d successes, %d settlements, %d deliveries", ok.Load(), r.invoice.count(), r.hits.Load())
	}
}

// A refusal before anything is attempted frees the credential to be corrected; a refusal
// after the attempt, or an outcome not known, spends it.
func TestGateRefusals(t *testing.T) {
	t.Run("verify refusal frees the credential", func(t *testing.T) {
		r := newRig(t)
		ch := r.challenge("GET", "/resource", nil, "")[0]
		wantProblem(t, r.do("GET", "/resource", auth(credential(ch, `{"other":1}`).String()), ""), 402, Payload)
		if resp := r.do("GET", "/resource", auth(credential(ch, proof).String()), ""); resp.StatusCode != 200 {
			t.Fatalf("the corrected credential: %d", resp.StatusCode)
		}
	})
	t.Run("settle refusal spends it and answers with a fresh challenge", func(t *testing.T) {
		r := newRig(t)
		ch := r.challenge("GET", "/resource", nil, "")[0]
		wantProblem(t, r.do("GET", "/resource", auth(credential(ch, `{"preimage":"wrong"}`).String()), ""), 402, Failed)
		wantProblem(t, r.do("GET", "/resource", auth(credential(ch, proof).String()), ""), 402, Invalid)
		if r.invoice.count() != 0 || r.hits.Load() != 0 {
			t.Fatal("delivered")
		}
	})
	t.Run("unknown outcome is not attempted twice", func(t *testing.T) {
		r := newRig(t)
		r.invoice.crash = true
		ch := r.challenge("GET", "/resource", nil, "")[0]
		c := auth(credential(ch, proof).String())
		wantProblem(t, r.do("GET", "/resource", c, ""), 500, Internal)
		if len(r.reports()) != 1 {
			t.Fatalf("reports %v", r.reports())
		}
		r.invoice.crash = false
		wantProblem(t, r.do("GET", "/resource", c, ""), 402, Invalid)
		if r.invoice.count() != 0 || r.hits.Load() != 0 {
			t.Fatal("a credential whose outcome was not known was attempted again")
		}
	})
}

// A method cannot choose the amount; the host's price is the only one (section 11.6).
func TestGateMethodCannotChoosePrice(t *testing.T) {
	r := newRig(t, func(c *Config) { c.Methods = []Method{&greedy{invoice: &invoice{}}} })
	resp := r.do("GET", "/resource", nil, "")
	wantProblem(t, resp, 500, Internal)
	if len(r.reports()) != 1 {
		t.Fatal("not reported")
	}
}

type greedy struct{ *invoice }

func (g *greedy) Propose(_ context.Context, _ Caller, q Quote) (*Proposal, error) {
	return &Proposal{Request: map[string]string{"amount": "9999", "currency": q.Currency}}, nil
}

// The price at redemption is the price now: a challenge issued for one price is not good
// for a request that costs another, nor for another external reference.
func TestGatePriceAtRedemption(t *testing.T) {
	var amount atomic.Value
	amount.Store("1000")
	r := newRig(t, func(c *Config) {
		c.Price = func(req *http.Request) (*Quote, error) {
			return &Quote{Amount: amount.Load().(string), Currency: "usd", External: req.URL.Path}, nil
		}
	})
	ch := r.challenge("GET", "/a", nil, "")[0]
	c := credential(ch, proof).String()
	wantProblem(t, r.do("GET", "/b", auth(c), ""), 402, Invalid) // another external reference
	amount.Store("2000")
	wantProblem(t, r.do("GET", "/a", auth(c), ""), 402, Insufficient)
	amount.Store("500")
	wantProblem(t, r.do("GET", "/a", auth(c), ""), 402, Invalid) // cheaper now: ask again
	amount.Store("1000")
	if resp := r.do("GET", "/a", auth(c), ""); resp.StatusCode != 200 {
		t.Fatalf("the credential at its own price: %d", resp.StatusCode)
	}
}

// Section 5.1.3: with digests on, the paid request must carry the body the challenge
// was issued for, and the resource still reads it.
func TestGateDigest(t *testing.T) {
	r := newRig(t, func(c *Config) { c.Digest = 1024 })
	var read string
	r.serve = func(w http.ResponseWriter, req *http.Request) {
		b, _ := io.ReadAll(req.Body)
		read = string(b)
	}
	body := `{"hello": "world"}`
	ch := r.challenge("POST", "/resource", nil, body)[0]
	if ch.Digest != digestOf([]byte(body)) {
		t.Fatalf("digest %s", ch.Digest)
	}
	c := auth(credential(ch, proof).String())
	wantProblem(t, r.do("POST", "/resource", c, `{"hello": "world!"}`), 402, Failed)
	wantProblem(t, r.do("POST", "/resource", c, ""), 402, Failed)
	if r.hits.Load() != 0 {
		t.Fatal("a tampered body reached the resource")
	}
	// Failing the digest spent nothing: the right body goes through, and is readable.
	if resp := r.do("POST", "/resource", c, body); resp.StatusCode != 200 || read != body {
		t.Fatalf("status %d, body read %q", resp.StatusCode, read)
	}
	// A body too large to bind is refused rather than left unbound.
	resp := r.do("POST", "/resource", nil, strings.Repeat("x", 2000))
	if resp.StatusCode != 413 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	// A challenge issued for no body does not cover a body.
	empty := r.challenge("POST", "/resource", nil, "")[0]
	if empty.Digest != "" {
		t.Fatal("a digest of no body")
	}
	wantProblem(t, r.do("POST", "/resource", auth(credential(empty, proof).String()), body), 402, Failed)
}

// Sections 4.2 and 4.4.3: authentication and policy come before payment, with no
// challenge and no settlement.
func TestGateHostCallbacks(t *testing.T) {
	for name, c := range map[string]struct {
		err    error
		status int
	}{"unauthorized": {ErrUnauthorized, 401}, "denied": {ErrDenied, 403}} {
		for where, mod := range map[string]func(*Config){
			"caller": func(cfg *Config) { cfg.Caller = func(*http.Request) (Caller, error) { return Caller{}, c.err } },
			"price":  func(cfg *Config) { cfg.Price = func(*http.Request) (*Quote, error) { return nil, c.err } },
		} {
			t.Run(name+" from "+where, func(t *testing.T) {
				r := newRig(t, mod)
				resp := r.do("GET", "/resource", nil, "")
				if resp.StatusCode != c.status || len(resp.Header.Values("WWW-Authenticate")) != 0 {
					t.Fatalf("%d %v", resp.StatusCode, resp.Header)
				}
				if r.hits.Load() != 0 || r.invoice.count() != 0 {
					t.Fatal("reached")
				}
				// A credential changes nothing: authentication fails first, so it is not consumed.
				resp = r.do("GET", "/resource", auth("Payment "+rawEncode(`{"challenge":{"id":"a"},"payload":{}}`)), "")
				if resp.StatusCode != c.status {
					t.Fatalf("with a credential: %d", resp.StatusCode)
				}
			})
		}
	}
	t.Run("other errors are 500 and reported", func(t *testing.T) {
		r := newRig(t, func(c *Config) {
			c.Price = func(*http.Request) (*Quote, error) { return nil, io.ErrUnexpectedEOF }
		})
		wantProblem(t, r.do("GET", "/resource", nil, ""), 500, Internal)
		if len(r.reports()) != 1 {
			t.Fatal("not reported")
		}
	})
	t.Run("a free request passes", func(t *testing.T) {
		r := newRig(t, func(c *Config) { c.Price = func(*http.Request) (*Quote, error) { return nil, nil } })
		if resp := r.do("GET", "/resource", nil, ""); resp.StatusCode != 200 || resp.Header.Get("Payment-Receipt") != "" {
			t.Fatalf("%d", resp.StatusCode)
		}
	})
	t.Run("an invalid quote is the host's error", func(t *testing.T) {
		r := newRig(t, func(c *Config) {
			c.Price = func(*http.Request) (*Quote, error) { return &Quote{Amount: "01", Currency: "usd"}, nil }
		})
		wantProblem(t, r.do("GET", "/resource", nil, ""), 500, Internal)
	})
	t.Run("the caller reaches the method", func(t *testing.T) {
		seen := make(chan Caller, 1)
		m := &spy{invoice: &invoice{}, seen: seen}
		r := newRig(t, func(c *Config) {
			c.Methods = []Method{m}
			c.Caller = func(req *http.Request) (Caller, error) {
				return Caller{Org: "acme", Subject: req.Header.Get("X-Test-Subject")}, nil
			}
		})
		r.challenge("GET", "/resource", http.Header{"X-Test-Subject": {"agent-1"}}, "")
		if got := <-seen; got != (Caller{Org: "acme", Subject: "agent-1"}) {
			t.Fatalf("caller %+v", got)
		}
	})
}

type spy struct {
	*invoice
	seen chan Caller
}

func (s *spy) Propose(ctx context.Context, c Caller, q Quote) (*Proposal, error) {
	select {
	case s.seen <- c:
	default:
	}
	return s.invoice.Propose(ctx, c, q)
}

// Section 11.2: a challenge is never issued over unencrypted HTTP.
func TestGateRequiresTLS(t *testing.T) {
	r := newRig(t)
	req := httptest.NewRequest("GET", "http://api.example.com/resource", nil)
	rec := httptest.NewRecorder()
	r.h.ServeHTTP(rec, req)
	resp := rec.Result()
	if resp.StatusCode != 400 || len(resp.Header.Values("WWW-Authenticate")) != 0 {
		t.Fatalf("%d %v", resp.StatusCode, resp.Header)
	}
	// A host behind a TLS-terminating proxy states what it trusts.
	r2 := newRig(t, func(c *Config) {
		c.Secure = func(req *http.Request) bool { return req.Header.Get("X-Forwarded-Proto") == "https" }
	})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "http://api.example.com/resource", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	r2.h.ServeHTTP(rec, req)
	if rec.Code != 402 {
		t.Fatalf("status %d", rec.Code)
	}
}

// Section 11.2.2: a retired secret keeps verifying the challenges issued under it.
func TestGateSecretRotation(t *testing.T) {
	old := newRig(t)
	ch := old.challenge("GET", "/resource", nil, "")[0]
	c := auth(credential(ch, proof).String())
	rotated := []byte("rotated-secret-rotated-secret-rot")
	without := newRig(t, func(c *Config) { c.Secret = rotated })
	wantProblem(t, without.do("GET", "/resource", c, ""), 402, Invalid)
	with := newRig(t, func(c *Config) { c.Secret, c.Previous = rotated, [][]byte{testSecret} })
	if resp := with.do("GET", "/resource", c, ""); resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	// New challenges are bound under the new secret only.
	fresh := with.challenge("GET", "/resource", nil, "")[0]
	if fresh.ID != Bind(rotated, fresh) {
		t.Error("a challenge was issued under the retired secret")
	}
}

// A credential is good for the realm and the methods of the Gate that issued it.
func TestGateRealmAndMethod(t *testing.T) {
	a := newRig(t)
	ch := a.challenge("GET", "/resource", nil, "")[0]
	c := auth(credential(ch, proof).String())
	other := newRig(t, func(c *Config) { c.Realm = "other.example.com" })
	wantProblem(t, other.do("GET", "/resource", c, ""), 402, Invalid)
	fewer := newRig(t, func(c *Config) { c.Methods = []Method{&invoice{name: "other"}} })
	wantProblem(t, fewer.do("GET", "/resource", c, ""), 400, Unsupported)
}

// Section 7.4: Accept-Payment ranks the challenges, drops q=0, and is ignored when
// malformed or when it matches nothing.
func TestGateAcceptPayment(t *testing.T) {
	r := newRig(t, func(c *Config) {
		c.Methods = []Method{&invoice{name: "tempo"}, &invoice{name: "stripe"}, &invoice{name: "solana"}}
	})
	order := func(accept string) []string {
		h := http.Header{}
		if accept != "" {
			h.Set("Accept-Payment", accept)
		}
		var out []string
		for _, c := range r.challenge("GET", "/resource", h, "") {
			out = append(out, c.Method)
		}
		return out
	}
	eq := func(got []string, want string) {
		t.Helper()
		if strings.Join(got, ",") != want {
			t.Errorf("got %v, want %s", got, want)
		}
	}
	eq(order(""), "tempo,stripe,solana")
	eq(order("solana/charge, stripe/charge;q=0.5"), "solana,stripe")
	eq(order("tempo/charge;q=0, stripe/*"), "stripe")
	eq(order("*/charge;q=0.2, solana/charge"), "solana,tempo,stripe")
	eq(order("nonsense"), "tempo,stripe,solana")       // malformed: ignored
	eq(order("bitcoin/charge"), "tempo,stripe,solana") // no match: ignored
}

// No method can take the request: no 402, since no challenge can be made (section 4.4.2).
func TestGateNothingToOffer(t *testing.T) {
	r := newRig(t, func(c *Config) { c.Methods = []Method{skipper{&invoice{}}} })
	resp := r.do("GET", "/resource", nil, "")
	if resp.StatusCode != 403 || len(resp.Header.Values("WWW-Authenticate")) != 0 {
		t.Fatalf("%d %v", resp.StatusCode, resp.Header)
	}
}

type skipper struct{ *invoice }

func (skipper) Name() string { return "skip" }
func (skipper) Propose(context.Context, Caller, Quote) (*Proposal, error) {
	return nil, ErrSkip
}

// Section 5.3.1 and 11.10: a receipt only on a 2xx, never on an error; the response is
// uncacheable by shared caches whatever the resource said.
func TestGateReceiptOnSuccessOnly(t *testing.T) {
	r := newRig(t)
	r.serve = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=60")
		_, _ = io.WriteString(w, "ok")
	}
	ch := r.challenge("GET", "/resource", nil, "")[0]
	resp := r.do("GET", "/resource", auth(credential(ch, proof).String()), "")
	if resp.StatusCode != 200 || resp.Header.Get("Cache-Control") != "private" || resp.Header.Get("Payment-Receipt") == "" {
		t.Fatalf("%d %v", resp.StatusCode, resp.Header)
	}
	// A resource that asks for no-store keeps it.
	r2 := newRig(t)
	r2.serve = func(w http.ResponseWriter, _ *http.Request) { w.Header().Set("Cache-Control", "no-store") }
	ch = r2.challenge("GET", "/resource", nil, "")[0]
	if cc := r2.do("GET", "/resource", auth(credential(ch, proof).String()), "").Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control %q", cc)
	}
	// A resource that fails gets no receipt.
	r3 := newRig(t)
	r3.serve = func(w http.ResponseWriter, _ *http.Request) { http.NotFound(w, nil) }
	ch = r3.challenge("GET", "/resource", nil, "")[0]
	if resp := r3.do("GET", "/resource", auth(credential(ch, proof).String()), ""); resp.StatusCode != 404 || resp.Header.Get("Payment-Receipt") != "" {
		t.Fatalf("%d %v", resp.StatusCode, resp.Header)
	}
}

// Section 11.4: a retry of a paid request carrying the same Idempotency-Key and credential
// gets the response again, without paying and without running the resource a second time.
func TestGateIdempotentReplay(t *testing.T) {
	r := newRig(t, func(c *Config) { c.Replay = 1 << 20 })
	r.serve = func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("X-Run", "1")
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(201)
		_, _ = io.WriteString(w, "created")
	}
	ch := r.challenge("POST", "/resource", nil, "")[0]
	c := credential(ch, proof).String()
	hdr := func(key string) http.Header { return http.Header{"Authorization": {c}, "Idempotency-Key": {key}} }
	first := r.do("POST", "/resource", hdr("k1"), "")
	if first.StatusCode != 201 {
		t.Fatalf("first %d", first.StatusCode)
	}
	again := r.do("POST", "/resource", hdr("k1"), "")
	body, _ := io.ReadAll(again.Body)
	if again.StatusCode != 201 || string(body) != "created" || again.Header.Get("X-Run") != "1" ||
		again.Header.Get("Payment-Receipt") != first.Header.Get("Payment-Receipt") || again.Header.Get("Payment-Receipt") == "" {
		t.Fatalf("replay: %d %q %v", again.StatusCode, body, again.Header)
	}
	if r.hits.Load() != 1 || r.invoice.count() != 1 {
		t.Fatalf("hits %d, settled %d", r.hits.Load(), r.invoice.count())
	}
	// Another key, or none, is a second use of a spent credential.
	wantProblem(t, r.do("POST", "/resource", hdr("k2"), ""), 402, Invalid)
	wantProblem(t, r.do("POST", "/resource", http.Header{"Authorization": {c}}, ""), 402, Invalid)
}

// A paid request whose response failed is retried without a second charge.
func TestGateRetryAfterFailedDelivery(t *testing.T) {
	r := newRig(t)
	var calls atomic.Int32
	r.serve = func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			http.Error(w, "boom", 500)
			return
		}
		_, _ = io.WriteString(w, "ok")
	}
	ch := r.challenge("POST", "/resource", nil, "")[0]
	h := http.Header{"Authorization": {credential(ch, proof).String()}, "Idempotency-Key": {"k"}}
	if resp := r.do("POST", "/resource", h, ""); resp.StatusCode != 500 || resp.Header.Get("Payment-Receipt") != "" {
		t.Fatalf("first: %d %v", resp.StatusCode, resp.Header)
	}
	resp := r.do("POST", "/resource", h, "")
	if resp.StatusCode != 200 || resp.Header.Get("Payment-Receipt") == "" {
		t.Fatalf("retry: %d %v", resp.StatusCode, resp.Header)
	}
	if r.invoice.count() != 1 || r.hits.Load() != 2 {
		t.Fatalf("settled %d, hits %d", r.invoice.count(), r.hits.Load())
	}
	// Delivered now: the replay is of a settled payment with nothing kept (Replay is off),
	// and the resource is not run a third time.
	wantProblem(t, r.do("POST", "/resource", h, ""), 402, Invalid)
	if r.hits.Load() != 2 {
		t.Fatal("ran a third time")
	}
}

func TestNewRejectsBadConfig(t *testing.T) {
	ok := func() Config {
		return Config{Realm: "r", Secret: testSecret, Store: &Memory{}, Methods: []Method{&invoice{}},
			Price: func(*http.Request) (*Quote, error) { return nil, nil }}
	}
	if _, err := New(ok()); err != nil {
		t.Fatal(err)
	}
	for name, mod := range map[string]func(*Config){
		"no realm":         func(c *Config) { c.Realm = "" },
		"short secret":     func(c *Config) { c.Secret = []byte("short") },
		"short previous":   func(c *Config) { c.Previous = [][]byte{[]byte("short")} },
		"no store":         func(c *Config) { c.Store = nil },
		"no methods":       func(c *Config) { c.Methods = nil },
		"no price":         func(c *Config) { c.Price = nil },
		"negative life":    func(c *Config) { c.Lifetime = -1 },
		"negative digest":  func(c *Config) { c.Digest = -1 },
		"duplicate method": func(c *Config) { c.Methods = []Method{&invoice{}, &invoice{}} },
		"bad method name":  func(c *Config) { c.Methods = []Method{&invoice{name: "Stripe"}} },
	} {
		c := ok()
		mod(&c)
		if _, err := New(c); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// Streaming responses keep working through the receipt writer.
func TestGateFlushes(t *testing.T) {
	r := newRig(t)
	r.serve = func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "a")
		w.(http.Flusher).Flush()
		_, _ = io.WriteString(w, "b")
	}
	ch := r.challenge("GET", "/resource", nil, "")[0]
	resp := r.do("GET", "/resource", auth(credential(ch, proof).String()), "")
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || string(b) != "ab" || resp.Header.Get("Payment-Receipt") == "" {
		t.Fatalf("%d %q %v", resp.StatusCode, b, resp.Header)
	}
}

// Challenges are unique: two clients asking for the same thing in the same second get
// different ids, and each can pay its own (section 5.1.1).
func TestGateChallengesAreUnique(t *testing.T) {
	r := newRig(t)
	a := r.challenge("GET", "/resource", nil, "")[0]
	b := r.challenge("GET", "/resource", nil, "")[0]
	if a.ID == b.ID || a.Opaque == b.Opaque {
		t.Fatal("two challenges share an id")
	}
	for _, c := range []Challenge{a, b} {
		if resp := r.do("GET", "/resource", auth(credential(c, proof).String()), ""); resp.StatusCode != 200 {
			t.Fatalf("status %d", resp.StatusCode)
		}
	}
	if r.invoice.count() != 2 {
		t.Fatalf("settled %d", r.invoice.count())
	}
	// A method cannot use the member the gate keeps for itself.
	r2 := newRig(t, func(c *Config) { c.Methods = []Method{&nonced{invoice: &invoice{}}} })
	wantProblem(t, r2.do("GET", "/resource", nil, ""), 500, Internal)
}

type nonced struct{ *invoice }

func (n *nonced) Propose(ctx context.Context, c Caller, q Quote) (*Proposal, error) {
	p, err := n.invoice.Propose(ctx, c, q)
	if p != nil {
		p.Opaque = map[string]string{"nonce": "x"}
	}
	return p, err
}

// The resource never holds a Payment credential (section 11.2.1); other authentication
// reaches it.
func TestGateStripsCredentials(t *testing.T) {
	r := newRig(t, func(c *Config) { c.Alternate = true })
	var seen http.Header
	r.serve = func(w http.ResponseWriter, req *http.Request) { seen = req.Header.Clone() }
	ch := r.challenge("GET", "/resource", nil, "")[0]
	h := http.Header{"Authorization": {"Bearer mF_9.B5f-4.1JqM"}, "Payment-Authorization": {credential(ch, proof).String()}}
	if resp := r.do("GET", "/resource", h, ""); resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if seen.Get("Payment-Authorization") != "" || seen.Get("Authorization") != "Bearer mF_9.B5f-4.1JqM" {
		t.Fatalf("the resource saw %v", seen)
	}
	r2 := newRig(t)
	r2.serve = r.serve
	ch = r2.challenge("GET", "/resource", nil, "")[0]
	if resp := r2.do("GET", "/resource", auth(credential(ch, proof).String()), ""); resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if seen.Get("Authorization") != "" {
		t.Fatalf("the resource saw %v", seen)
	}
}

// Section 9.4: a challenge stays under 8 KiB, so a request that cannot fit is the host's error.
func TestGateChallengeSize(t *testing.T) {
	r := newRig(t, func(c *Config) {
		c.Price = func(*http.Request) (*Quote, error) {
			return &Quote{Amount: "1000", Currency: "usd", Description: strings.Repeat("x", 9000)}, nil
		}
	})
	wantProblem(t, r.do("GET", "/resource", nil, ""), 500, Internal)
	if _, err := New(Config{Realm: "r", Secret: testSecret, Store: &Memory{}, Methods: []Method{&invoice{}}, Limit: 1024,
		Price: func(*http.Request) (*Quote, error) { return nil, nil }}); err == nil {
		t.Error("a credential limit under 4096 was accepted")
	}
}
