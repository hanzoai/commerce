// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package mpp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// trip is what the server saw of one request.
type trip struct {
	accept, key, auth, payauth, body string
}

// e2e is a TLS server with a Gate in front of a resource, paid for by the stripe method
// through a recording Seller, and a client with a Wallet that issues tokens.
type e2e struct {
	t      *testing.T
	srv    *httptest.Server
	seller *seller
	clock  *clock
	hits   atomic.Int32
	tokens atomic.Int32
	mu     sync.Mutex
	seen   []trip
	issued []Limits
	max    int64
	wallet Wallet
}

func newE2E(t *testing.T, mods ...func(*Config)) *e2e {
	t.Helper()
	e := &e2e{t: t, seller: newSeller(), clock: &clock{at: epoch}, max: 10000}
	store := &memory{}
	cfg := Config{
		Realm: "api.example.com", Secret: testSecret, Store: store, Now: e.clock.now, Lifetime: 5 * time.Minute, Report: func(error) {},
		Methods: []Method{Stripe{Seller: e.seller, Store: store}},
		Price: func(*http.Request) (*Quote, error) {
			return &Quote{Amount: "5000", Currency: "usd", Description: "AI generation"}, nil
		},
	}
	for _, m := range mods {
		m(&cfg)
	}
	g, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	resource := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.hits.Add(1)
		b, _ := io.ReadAll(r.Body)
		_, _ = io.WriteString(w, "generated:"+string(b))
	})
	e.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(strings.NewReader(string(b)))
		e.mu.Lock()
		e.seen = append(e.seen, trip{r.Header.Get("Accept-Payment"), r.Header.Get("Idempotency-Key"), r.Header.Get("Authorization"), r.Header.Get("Payment-Authorization"), string(b)})
		e.mu.Unlock()
		g.Wrap(resource).ServeHTTP(w, r)
	}))
	t.Cleanup(e.srv.Close)
	e.wallet = Wallet{
		Allow: func(_ Challenge, l Limits) error {
			n, _ := strconv.ParseInt(l.Amount, 10, 64)
			if l.Currency != "usd" || n > e.max {
				return errors.New("over budget")
			}
			return nil
		},
		Issue: func(_ context.Context, l Limits) (string, error) {
			e.mu.Lock()
			e.issued = append(e.issued, l)
			e.mu.Unlock()
			return "spt_test_" + strconv.Itoa(int(e.tokens.Add(1))), nil
		},
	}
	return e
}

func (e *e2e) client(p Payer) *http.Client {
	return &http.Client{Transport: &Client{Base: e.srv.Client().Transport, Payer: p, Now: e.clock.now}}
}

func (e *e2e) trips() []trip {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]trip(nil), e.seen...)
}

func (e *e2e) tokensIssued() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.issued)
}

func read(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The client answers a 402 on its own and the resource is delivered with a receipt, and
// what the Seller was asked to charge is the challenge's amount (appendix B of the Stripe
// charge draft, end to end).
func TestClientPaysAChallenge(t *testing.T) {
	e := newE2E(t)
	resp, err := e.client(e.wallet).Get(e.srv.URL + "/api/generate")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || read(t, resp) != "generated:" {
		t.Fatalf("status %d", resp.StatusCode)
	}
	rcpt, err := ParseReceipt(resp.Header.Get("Payment-Receipt"))
	if err != nil || rcpt.Method != "stripe" || rcpt.Reference != "pi_1N4Zv32eZvKYlo2CPhVPkJlW" {
		t.Fatalf("receipt %+v %v", rcpt, err)
	}
	if resp.Header.Get("Cache-Control") != "private" {
		t.Errorf("Cache-Control %q", resp.Header.Get("Cache-Control"))
	}
	sales := e.seller.taken()
	if len(sales) != 1 || sales[0].Amount != "5000" || sales[0].Currency != "usd" || sales[0].Token != "spt_test_1" ||
		sales[0].Metadata["challenge_id"] == "" || sales[0].Key != sales[0].Challenge+"_spt_test_1" {
		t.Fatalf("sales %+v", sales)
	}
	if len(e.issued) != 1 || e.issued[0].Network != "profile_1MqDcVKA5fEO2tZvKQm9g8Yj" || e.issued[0].Amount != "5000" {
		t.Fatalf("limits %+v", e.issued)
	}
	// Two requests: the unpaid one declared what the client accepts, the paid one carried
	// the credential in Authorization.
	tr := e.trips()
	if len(tr) != 2 || tr[0].accept != "stripe/charge" || tr[0].auth != "" || !strings.HasPrefix(tr[1].auth, "Payment ") {
		t.Fatalf("trips %+v", tr)
	}
	cred, err := ParseCredential(tr[1].auth)
	if err != nil || cred.Challenge.Method != "stripe" || cred.Challenge.Field() != "Authorization" {
		t.Fatalf("credential %+v %v", cred, err)
	}
}

// A paid POST keeps its body across the retry, and a challenge that binds the body is
// answered with it (section 5.1.3).
func TestClientPostWithDigest(t *testing.T) {
	e := newE2E(t, func(c *Config) { c.Digest = 1 << 16 })
	resp, err := e.client(e.wallet).Post(e.srv.URL+"/api/generate", "application/json", strings.NewReader(`{"prompt":"hi"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := read(t, resp); resp.StatusCode != 200 || got != `generated:{"prompt":"hi"}` {
		t.Fatalf("status %d, body %q", resp.StatusCode, got)
	}
	tr := e.trips()
	if len(tr) != 2 || tr[0].body != tr[1].body || tr[1].body != `{"prompt":"hi"}` {
		t.Fatalf("trips %+v", tr)
	}
	cred, _ := ParseCredential(tr[1].auth)
	if cred.Challenge.Digest != digestOf([]byte(`{"prompt":"hi"}`)) {
		t.Errorf("digest %q", cred.Challenge.Digest)
	}
}

// What the client must not pay, it returns as the 402 it is: an amount the policy
// refuses, an expired challenge, a method it does not speak, a body it cannot send again,
// a field already carrying other authentication, and anything over unencrypted HTTP.
func TestClientDoesNotPay(t *testing.T) {
	t.Run("policy refuses", func(t *testing.T) {
		e := newE2E(t)
		e.max = 100
		_, err := e.client(e.wallet).Get(e.srv.URL + "/")
		if err == nil || !strings.Contains(err.Error(), "over budget") {
			t.Fatalf("%v", err)
		}
		if e.tokensIssued() != 0 || len(e.seller.taken()) != 0 {
			t.Fatal("paid")
		}
	})
	t.Run("expired", func(t *testing.T) {
		e := newE2E(t)
		cl := &http.Client{Transport: &Client{Base: e.srv.Client().Transport, Payer: e.wallet, Now: func() time.Time { return e.clock.now().Add(10 * time.Minute) }}}
		resp, err := cl.Get(e.srv.URL + "/")
		if err != nil || resp.StatusCode != 402 || e.tokensIssued() != 0 {
			t.Fatalf("%v %v", resp, err)
		}
	})
	t.Run("a method it does not speak", func(t *testing.T) {
		e := newE2E(t)
		resp, err := e.client(nopayer{}).Get(e.srv.URL + "/")
		if err != nil || resp.StatusCode != 402 {
			t.Fatalf("%v %v", resp, err)
		}
		if e.tokensIssued() != 0 {
			t.Fatal("paid")
		}
	})
	t.Run("a body it cannot send again", func(t *testing.T) {
		e := newE2E(t)
		req, _ := http.NewRequest("POST", e.srv.URL+"/", io.NopCloser(strings.NewReader("hi"))) // no GetBody
		resp, err := e.client(e.wallet).Do(req)
		if err != nil || resp.StatusCode != 402 || e.tokensIssued() != 0 {
			t.Fatalf("%v %v", resp, err)
		}
	})
	t.Run("authorization already in use", func(t *testing.T) {
		e := newE2E(t)
		wire, _ := Encode(request{Amount: "5000", Currency: "usd", MethodDetails: &details{NetworkID: "p", PaymentMethodTypes: []string{"card"}}})
		ch := Challenge{ID: "a", Realm: "r", Method: "stripe", Intent: Charge, Request: wire} // no header: Authorization
		stub := roundTrip(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 402, Header: http.Header{"Www-Authenticate": {ch.String()}}, Body: http.NoBody}, nil
		})
		req, _ := http.NewRequest("GET", e.srv.URL+"/", nil)
		req.Header.Set("Authorization", "Bearer abc")
		resp, err := (&Client{Base: stub, Payer: e.wallet}).RoundTrip(req)
		if err != nil || resp.StatusCode != 402 || e.tokensIssued() != 0 {
			t.Fatalf("%v %v", resp, err)
		}
	})
	t.Run("plain http", func(t *testing.T) {
		e := newE2E(t, func(c *Config) { c.Secure = func(*http.Request) bool { return true } })
		plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			e.srv.Config.Handler.ServeHTTP(w, r)
		}))
		defer plain.Close()
		resp, err := e.client(e.wallet).Get(plain.URL + "/")
		if err != nil || resp.StatusCode != 402 || e.tokensIssued() != 0 {
			t.Fatalf("%v %v", resp, err)
		}
	})
	t.Run("not a 402", func(t *testing.T) {
		e := newE2E(t, func(c *Config) { c.Price = func(*http.Request) (*Quote, error) { return nil, nil } })
		resp, err := e.client(e.wallet).Get(e.srv.URL + "/")
		if err != nil || resp.StatusCode != 200 || len(e.trips()) != 1 {
			t.Fatalf("%v %v", resp, err)
		}
	})
}

// nopayer answers nothing.
type nopayer struct{}

func (nopayer) Accepts() []Range { return []Range{{Method: "tempo", Intent: Charge, Weight: 1000}} }
func (nopayer) Pay(context.Context, Challenge, json.RawMessage) (*Answer, error) {
	return nil, errors.New("not reached")
}

// When the request carries other authentication in Authorization the server selects
// Payment-Authorization, the client pays in it, and both credentials go together
// (sections 4.4.3, 5.2).
func TestClientAlternateField(t *testing.T) {
	e := newE2E(t)
	req, _ := http.NewRequest("GET", e.srv.URL+"/", nil)
	req.Header.Set("Authorization", "Bearer abc")
	resp, err := e.client(e.wallet).Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("%v %v", resp, err)
	}
	tr := e.trips()
	if len(tr) != 2 || tr[1].auth != "Bearer abc" || !strings.HasPrefix(tr[1].payauth, "Payment ") {
		t.Fatalf("trips %+v", tr)
	}
	cred, _ := ParseCredential(tr[1].payauth)
	if cred.Challenge.Header != alternate {
		t.Errorf("the credential does not echo header: %+v", cred.Challenge)
	}
	if resp.Header.Get("Cache-Control") != "private" {
		t.Errorf("Cache-Control %q", resp.Header.Get("Cache-Control"))
	}
}

// Several challenges: the client answers the one its Payer ranks first, with one credential.
func TestClientChoosesAMethod(t *testing.T) {
	inv := &invoice{}
	e := newE2E(t, func(c *Config) { c.Methods = append([]Method{inv}, c.Methods...) })
	// The server lists invoice first; the wallet speaks stripe only. The request states no
	// preference of its own, so the server offers both and the client chooses.
	req, _ := http.NewRequest("GET", e.srv.URL+"/", nil)
	req.Header.Set("Accept-Payment", "*/*")
	resp, err := e.client(e.wallet).Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("%v %v", resp, err)
	}
	if inv.count() != 0 || len(e.seller.taken()) != 1 {
		t.Fatalf("paid by invoice %d, by stripe %d", inv.count(), len(e.seller.taken()))
	}
	tr := e.trips()
	cred, _ := ParseCredential(tr[len(tr)-1].auth)
	if cred.Challenge.Method != "stripe" {
		t.Fatalf("answered %s", cred.Challenge.Method)
	}
}

// A refusal the server gives to the paid request is returned as it is, and one request
// pays at most once: the client does not loop.
func TestClientPaysOnce(t *testing.T) {
	e := newE2E(t)
	e.seller.err = Refuse(Failed, "card declined")
	resp, err := e.client(e.wallet).Get(e.srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	p := problem(t, resp)
	if resp.StatusCode != 402 || codeOf(&p) != Failed || e.tokensIssued() != 1 || len(e.trips()) != 2 {
		t.Fatalf("%d %+v issued %d trips %d", resp.StatusCode, p, e.tokensIssued(), len(e.trips()))
	}
	if e.hits.Load() != 0 {
		t.Fatal("delivered after a refusal")
	}
}

// roundTrip is a transport that answers by a function.
type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
