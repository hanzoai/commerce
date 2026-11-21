// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package x402

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// books is a Facilitator that settles on its own ledger: it refuses a nonce it
// has already settled, the way a token contract refuses one on chain.
type books struct {
	mu       sync.Mutex
	spent    map[string]bool
	verified int
	settled  []PaymentRequirements
	failWith string // Settle answers this reason when set
	down     bool   // neither call can answer
}

func (b *books) Verify(_ context.Context, pay PaymentPayload, _ PaymentRequirements) (VerifyResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.down {
		return VerifyResponse{}, errors.New("unreachable")
	}
	b.verified++
	if b.spent[pay.Payload.Authorization.Nonce] {
		return VerifyResponse{IsValid: false, InvalidReason: ReasonInvalidTransaction, Payer: pay.Payload.Authorization.From}, nil
	}
	return VerifyResponse{IsValid: true, Payer: pay.Payload.Authorization.From}, nil
}

func (b *books) Settle(_ context.Context, pay PaymentPayload, offered PaymentRequirements) (SettlementResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.down {
		return SettlementResponse{}, errors.New("unreachable")
	}
	a := pay.Payload.Authorization
	if b.failWith != "" || b.spent[a.Nonce] {
		reason := b.failWith
		if reason == "" {
			reason = ReasonInvalidTransaction
		}
		return SettlementResponse{ErrorReason: reason, Payer: a.From, Network: offered.Network}, nil
	}
	if b.spent == nil {
		b.spent = map[string]bool{}
	}
	b.spent[a.Nonce] = true
	b.settled = append(b.settled, offered)
	return SettlementResponse{Success: true, Payer: a.From, Transaction: "tx-" + a.Nonce[:10], Network: offered.Network, Amount: offered.Amount}, nil
}

type rig struct {
	books *books
	gate  Gate
	srv   *httptest.Server
	now   time.Time
	hits  int
}

// newRig serves two priced routes and one free one behind the middleware.
func newRig(t *testing.T, handler func(*rig) http.Handler, wrap ...func(*books) Facilitator) *rig {
	t.Helper()
	tab, err := NewTable(testAsset())
	if err != nil {
		t.Fatal(err)
	}
	for k, p := range map[string]Price{
		"GET /data":   {Amount: "0.01", Description: "premium data", MimeType: "application/json"},
		"POST /order": {Amount: "0.25"},
	} {
		if err := tab.Set(k, p); err != nil {
			t.Fatal(err)
		}
	}
	r := &rig{books: &books{}, now: time.Unix(1740672100, 0)}
	var fac Facilitator = r.books
	if len(wrap) > 0 {
		fac = wrap[0](r.books)
	}
	r.gate = Gate{Table: tab, Facilitator: fac, Now: func() time.Time { return r.now }}
	mw := Middleware(r.gate, func(q *http.Request) string { return q.Method + " " + q.URL.Path })
	r.srv = httptest.NewServer(mw(handler(r)))
	t.Cleanup(r.srv.Close)
	return r
}

func okHandler(r *rig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		r.hits++
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"data":"premium"}`)
	})
}

func (r *rig) client(t *testing.T, allow bool) *http.Client {
	t.Helper()
	return &http.Client{Transport: Transport{Payer: Signer{
		Key:   testKey(t),
		Now:   func() time.Time { return r.now },
		Allow: func(ResourceInfo, PaymentRequirements) bool { return allow },
	}}}
}

func get(t *testing.T, c *http.Client, url string, hdr map[string]string) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

func challengeOf(t *testing.T, resp *http.Response) PaymentRequired {
	t.Helper()
	var pr PaymentRequired
	if err := DecodeHeader(resp.Header.Get(HeaderPaymentRequired), &pr); err != nil {
		t.Fatalf("PAYMENT-REQUIRED: %v", err)
	}
	return pr
}

func TestUnpaidRequestIsChallenged(t *testing.T) {
	r := newRig(t, okHandler)
	resp, body := get(t, http.DefaultClient, r.srv.URL+"/data?x=1", nil)
	if resp.StatusCode != 402 || r.hits != 0 {
		t.Fatalf("status %d, handler hit %d times", resp.StatusCode, r.hits)
	}
	pr := challengeOf(t, resp)
	if pr.X402Version != 2 || pr.Error != "PAYMENT-SIGNATURE header is required" {
		t.Fatalf("%+v", pr)
	}
	if want := r.srv.URL + "/data?x=1"; pr.Resource.URL != want || pr.Resource.Description != "premium data" || pr.Resource.MimeType != "application/json" {
		t.Fatalf("resource %+v", pr.Resource)
	}
	if len(pr.Accepts) != 1 || pr.Accepts[0].Amount != "10000" || pr.Accepts[0].Scheme != "exact" || pr.Accepts[0].Extra.Name != "Test Dollar" {
		t.Fatalf("accepts %+v", pr.Accepts)
	}
	if resp.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("a 402 must not be cacheable")
	}
	if got := resp.Header.Values("Access-Control-Expose-Headers"); strings.Join(got, ",") != "PAYMENT-REQUIRED,PAYMENT-RESPONSE" {
		t.Fatalf("expose %v", got)
	}
	if strings.Contains(body, "premium") {
		t.Fatalf("leaked: %s", body)
	}
}

func TestPayingClientIsServedAndSettledOnce(t *testing.T) {
	r := newRig(t, okHandler)
	resp, body := get(t, r.client(t, true), r.srv.URL+"/data", nil)
	if resp.StatusCode != 200 || body != `{"data":"premium"}` {
		t.Fatalf("%d %s", resp.StatusCode, body)
	}
	var s SettlementResponse
	if err := DecodeHeader(resp.Header.Get(HeaderPaymentResponse), &s); err != nil {
		t.Fatal(err)
	}
	if !s.Success || s.Network != "eip155:84532" || !strings.EqualFold(s.Payer, refAddress) || s.Transaction == "" || s.Amount != "10000" {
		t.Fatalf("%+v", s)
	}
	if len(r.books.settled) != 1 || r.books.settled[0].Amount != "10000" || r.hits != 1 {
		t.Fatalf("settled %d, hits %d", len(r.books.settled), r.hits)
	}
}

func TestNeverPaysWithoutLeave(t *testing.T) {
	r := newRig(t, okHandler)
	resp, _ := get(t, r.client(t, false), r.srv.URL+"/data", nil)
	if resp.StatusCode != 402 || len(r.books.settled) != 0 || r.hits != 0 {
		t.Fatalf("%d settled %d hits %d", resp.StatusCode, len(r.books.settled), r.hits)
	}
	if challengeOf(t, resp).Accepts[0].Amount != "10000" {
		t.Fatal("the caller must still see the challenge")
	}
}

func TestPostBodyIsReplayedOnThePaidRetry(t *testing.T) {
	var got string
	r := newRig(t, func(*rig) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
			b, _ := io.ReadAll(q.Body)
			got = string(b)
			if a := FromContext(q.Context()); a == nil || !strings.EqualFold(a.Payer(), refAddress) || a.Requirements().Amount != "250000" {
				t.Errorf("admission %+v", a)
			}
			w.WriteHeader(http.StatusCreated)
		})
	})
	req, _ := http.NewRequest(http.MethodPost, r.srv.URL+"/order", bytes.NewReader([]byte(`{"sku":"a"}`)))
	resp, err := r.client(t, true).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 201 || got != `{"sku":"a"}` || resp.Header.Get(HeaderPaymentResponse) == "" {
		t.Fatalf("%d %q", resp.StatusCode, got)
	}
}

func TestFailedHandlerSettlesNothing(t *testing.T) {
	for _, code := range []int{400, 404, 500, 503} {
		r := newRig(t, func(*rig) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) { http.Error(w, "no", code) })
		})
		resp, _ := get(t, r.client(t, true), r.srv.URL+"/data", nil)
		if resp.StatusCode != code || resp.Header.Get(HeaderPaymentResponse) != "" || len(r.books.settled) != 0 {
			t.Errorf("%d: status %d, response header %q, settled %d", code, resp.StatusCode, resp.Header.Get(HeaderPaymentResponse), len(r.books.settled))
		}
	}
}

func TestAnEmptySuccessStillSettles(t *testing.T) {
	r := newRig(t, func(*rig) http.Handler { return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}) })
	resp, _ := get(t, r.client(t, true), r.srv.URL+"/data", nil)
	if resp.StatusCode != 200 || resp.Header.Get(HeaderPaymentResponse) == "" || len(r.books.settled) != 1 {
		t.Fatalf("%d %d", resp.StatusCode, len(r.books.settled))
	}
}

func TestFailedSettlementWithholdsTheResource(t *testing.T) {
	r := newRig(t, okHandler)
	r.books.failWith = ReasonInsufficientFunds
	resp, body := get(t, r.client(t, true), r.srv.URL+"/data", nil)
	if resp.StatusCode != 402 || strings.Contains(body, "premium") {
		t.Fatalf("%d %s", resp.StatusCode, body)
	}
	var s SettlementResponse
	if err := DecodeHeader(resp.Header.Get(HeaderPaymentResponse), &s); err != nil {
		t.Fatal(err)
	}
	if s.Success || s.ErrorReason != ReasonInsufficientFunds || s.Transaction != "" || s.Network != "eip155:84532" {
		t.Fatalf("%+v", s)
	}
}

// vanishing verifies and then cannot be reached to settle.
type vanishing struct{ *books }

func (vanishing) Settle(context.Context, PaymentPayload, PaymentRequirements) (SettlementResponse, error) {
	return SettlementResponse{}, errors.New("gone")
}

func TestUnreachableFacilitatorIsAServerError(t *testing.T) {
	r := newRig(t, okHandler)
	r.books.down = true
	resp, _ := get(t, r.client(t, true), r.srv.URL+"/data", nil)
	if resp.StatusCode != 500 || r.hits != 0 {
		t.Fatalf("verify down: %d hits %d", resp.StatusCode, r.hits)
	}
	// Verified, served, then the facilitator vanishes: the payment's fate is
	// unknown, so the resource is withheld and the answer is a server error.
	r2 := newRig(t, okHandler, func(b *books) Facilitator { return vanishing{b} })
	resp, body := get(t, r2.client(t, true), r2.srv.URL+"/data", nil)
	var s SettlementResponse
	if err := DecodeHeader(resp.Header.Get(HeaderPaymentResponse), &s); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 500 || strings.Contains(body, "premium") || s.Success || s.ErrorReason != ReasonSettleError {
		t.Fatalf("%d %+v", resp.StatusCode, s)
	}
}

func TestRefusedPaymentsAreChallengedAgainWithTheReason(t *testing.T) {
	r := newRig(t, okHandler)
	req := testRequirements()
	pay := func(mut func(*PaymentPayload), after, before uint64) string {
		p := signed(t, req, after, before)
		if mut != nil {
			mut(&p)
		}
		h, _ := EncodeHeader(p)
		return h
	}
	now := uint64(r.now.Unix())
	cases := []struct {
		name   string
		header string
		status int
		reason string
	}{
		{"garbage", "!!!", 400, ReasonInvalidPayload},
		{"empty object", "e30=", 400, ReasonInvalidPayload},
		{"expired", pay(nil, 0, now-1), 402, ReasonValidBefore},
		{"not yet valid", pay(nil, now+60, now+120), 402, ReasonValidAfter},
		{"wrong amount echo", pay(func(p *PaymentPayload) { p.Accepted.Amount = "1" }, 0, now+60), 402, ReasonValueMismatch},
		{"wrong amount signed", pay(func(p *PaymentPayload) { p.Payload.Authorization.Value = "1" }, 0, now+60), 402, ReasonValueMismatch},
		{"another payee", pay(func(p *PaymentPayload) { p.Payload.Authorization.To = "0x0000000000000000000000000000000000000001" }, 0, now+60), 402, ReasonRecipientMismatch},
		{"bad signature", pay(func(p *PaymentPayload) { p.Payload.Signature = "0x" + strings.Repeat("11", 65) }, 0, now+60), 402, ReasonSignature},
		{"version 1", pay(func(p *PaymentPayload) { p.X402Version = 1 }, 0, now+60), 402, ReasonInvalidVersion},
	}
	for _, c := range cases {
		resp, body := get(t, http.DefaultClient, r.srv.URL+"/data", map[string]string{HeaderPaymentSignature: c.header})
		if resp.StatusCode != c.status || strings.Contains(body, "premium") {
			t.Errorf("%s: status %d", c.name, resp.StatusCode)
			continue
		}
		if c.status == 402 {
			if got := challengeOf(t, resp).Error; got != c.reason {
				t.Errorf("%s: error %q, want %q", c.name, got, c.reason)
			}
		}
	}
	if r.hits != 0 || len(r.books.settled) != 0 {
		t.Fatalf("a refused payment ran the handler (%d) or settled (%d)", r.hits, len(r.books.settled))
	}
}

// A payment for one resource's terms does not buy another's.
func TestPaymentForAnotherPriceIsRefused(t *testing.T) {
	r := newRig(t, okHandler)
	cheap := testAsset().Requirements("1", "")
	h, _ := EncodeHeader(signed(t, cheap, 0, uint64(r.now.Unix())+60))
	resp, _ := get(t, http.DefaultClient, r.srv.URL+"/data", map[string]string{HeaderPaymentSignature: h})
	if resp.StatusCode != 402 || challengeOf(t, resp).Error != ReasonValueMismatch || r.hits != 0 {
		t.Fatalf("%d %q", resp.StatusCode, challengeOf(t, resp).Error)
	}
}

// The same signed payment, sent twice: the second is refused by the facilitator
// before the handler runs, as a token contract refuses a spent nonce.
func TestReplayedPaymentIsRefused(t *testing.T) {
	r := newRig(t, okHandler)
	h, _ := EncodeHeader(signed(t, testRequirements(), 0, uint64(r.now.Unix())+60))
	hdr := map[string]string{HeaderPaymentSignature: h}
	if resp, _ := get(t, http.DefaultClient, r.srv.URL+"/data", hdr); resp.StatusCode != 200 {
		t.Fatalf("first: %d", resp.StatusCode)
	}
	resp, body := get(t, http.DefaultClient, r.srv.URL+"/data", hdr)
	if resp.StatusCode != 402 || challengeOf(t, resp).Error != ReasonInvalidTransaction || strings.Contains(body, "premium") || r.hits != 1 {
		t.Fatalf("second: %d hits %d", resp.StatusCode, r.hits)
	}
}

func TestUnpricedAndPreflightPassThrough(t *testing.T) {
	r := newRig(t, okHandler)
	if resp, body := get(t, http.DefaultClient, r.srv.URL+"/free", nil); resp.StatusCode != 200 || body == "" || resp.Header.Get(HeaderPaymentRequired) != "" {
		t.Fatalf("free route: %d", resp.StatusCode)
	}
	req, _ := http.NewRequest(http.MethodOptions, r.srv.URL+"/data", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 || r.books.verified != 0 {
		t.Fatalf("preflight: %d", resp.StatusCode)
	}
}

func TestAskingTheGateForAnUnpricedResourceRefuses(t *testing.T) {
	r := newRig(t, okHandler)
	adm, ref := r.gate.Admit(context.Background(), "GET /nope", "u", "x")
	if adm != nil || ref == nil || ref.Status != 500 {
		t.Fatalf("%v %v", adm, ref)
	}
}

func TestStreamingKeepsStreaming(t *testing.T) {
	released := make(chan struct{})
	r := newRig(t, func(*rig) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
			io.WriteString(w, "one\n")
			http.NewResponseController(w).Flush()
			<-released
			io.WriteString(w, "two\n")
		})
	})
	req, _ := http.NewRequest(http.MethodGet, r.srv.URL+"/data", nil)
	resp, err := r.client(t, true).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	buf := make([]byte, 4)
	if _, err := io.ReadFull(resp.Body, buf); err != nil || string(buf) != "one\n" {
		t.Fatalf("first chunk %q %v", buf, err)
	}
	if resp.Header.Get(HeaderPaymentResponse) == "" {
		t.Fatal("PAYMENT-RESPONSE must precede the first byte")
	}
	close(released)
	rest, _ := io.ReadAll(resp.Body)
	if string(rest) != "two\n" {
		t.Fatalf("rest %q", rest)
	}
}

func TestTransportLeavesOtherChallengesAlone(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		calls++
		w.WriteHeader(402) // a 402 with no x402 challenge
	}))
	defer srv.Close()
	c := &http.Client{Transport: Transport{Payer: Signer{Key: testKey(t), Allow: func(ResourceInfo, PaymentRequirements) bool { return true }}}}
	resp, _ := get(t, c, srv.URL, nil)
	if resp.StatusCode != 402 || calls != 1 {
		t.Fatalf("%d after %d calls", resp.StatusCode, calls)
	}
}

func TestTransportPaysOnlyOnce(t *testing.T) {
	var calls int
	var req PaymentRequirements = testRequirements()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		calls++
		h, _ := EncodeHeader(PaymentRequired{X402Version: 2, Resource: ResourceInfo{URL: "u"}, Accepts: []PaymentRequirements{req}})
		w.Header().Set(HeaderPaymentRequired, h)
		w.WriteHeader(402) // refuses every payment
	}))
	defer srv.Close()
	c := &http.Client{Transport: Transport{Payer: Signer{Key: testKey(t), Allow: func(ResourceInfo, PaymentRequirements) bool { return true }}}}
	resp, _ := get(t, c, srv.URL, nil)
	if resp.StatusCode != 402 || calls != 2 {
		t.Fatalf("%d after %d calls", resp.StatusCode, calls)
	}
}

// Remote speaks the facilitator API of spec section 7.
func TestRemoteFacilitator(t *testing.T) {
	req := testRequirements()
	pay := signed(t, req, 0, 2000000000)
	var seen []string
	mux := http.NewServeMux()
	mux.HandleFunc("POST /verify", func(w http.ResponseWriter, q *http.Request) {
		seen = append(seen, "verify "+q.Header.Get("Authorization"))
		var in struct {
			X402Version         int
			PaymentPayload      PaymentPayload
			PaymentRequirements PaymentRequirements
		}
		if err := json.NewDecoder(q.Body).Decode(&in); err != nil || in.X402Version != 2 || in.PaymentRequirements.Amount != "10000" || in.PaymentPayload.Payload.Signature != pay.Payload.Signature {
			t.Errorf("verify body: %v %+v", err, in)
		}
		json.NewEncoder(w).Encode(VerifyResponse{IsValid: true, Payer: in.PaymentPayload.Payload.Authorization.From})
	})
	mux.HandleFunc("POST /settle", func(w http.ResponseWriter, q *http.Request) {
		seen = append(seen, "settle")
		w.WriteHeader(402) // a refusal is still an answer
		json.NewEncoder(w).Encode(SettlementResponse{ErrorReason: ReasonInsufficientFunds, Network: req.Network})
	})
	mux.HandleFunc("GET /supported", func(w http.ResponseWriter, q *http.Request) {
		io.WriteString(w, `{"kinds":[{"x402Version":2,"scheme":"exact","network":"eip155:84532"}],"extensions":[],"signers":{"eip155:*":["0x1234567890abcdef1234567890abcdef12345678"]}}`)
	})
	mux.HandleFunc("POST /broken", func(w http.ResponseWriter, q *http.Request) { http.Error(w, "boom", 502) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	f := Remote{URL: srv.URL + "/", Authorize: func(r *http.Request, path string) error {
		r.Header.Set("Authorization", "Bearer k:"+path)
		return nil
	}}
	v, err := f.Verify(context.Background(), pay, req)
	if err != nil || !v.IsValid || !strings.EqualFold(v.Payer, refAddress) {
		t.Fatalf("%+v %v", v, err)
	}
	s, err := f.Settle(context.Background(), pay, req)
	if err != nil || s.Success || s.ErrorReason != ReasonInsufficientFunds {
		t.Fatalf("%+v %v", s, err)
	}
	sup, err := f.Supported(context.Background())
	if err != nil || len(sup.Kinds) != 1 || sup.Kinds[0].Network != "eip155:84532" || sup.Signers["eip155:*"][0] == "" {
		t.Fatalf("%+v %v", sup, err)
	}
	if strings.Join(seen, "|") != "verify Bearer k:/verify|settle" {
		t.Fatalf("calls %v", seen)
	}
	// A facilitator that answers with something else is an error, never a yes.
	var out VerifyResponse
	if err := (Remote{URL: srv.URL}).call(context.Background(), http.MethodPost, "/broken", nil, &out); err == nil || out.IsValid {
		t.Fatalf("broken facilitator: %v", err)
	}
	if _, err := (Remote{URL: "http://127.0.0.1:1"}).Verify(context.Background(), pay, req); err == nil {
		t.Fatal("unreachable facilitator answered")
	}
	// Remote is a Facilitator: the gate takes it.
	var _ Facilitator = f
}
