// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package acp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// The signature of the webhook specification: Merchant-Signature is t=<unix_seconds>,v1=<64_hex>,
// the HMAC-SHA256 of timestamp + "." + raw body under the shared secret.

// This vector was computed outside Go with Python's hmac and hashlib.
const (
	vecSecret = "whsec_test_secret"
	vecBody   = `{"type":"order_create","data":{"type":"order","id":"ord_123","checkout_session_id":"checkout_session_123","permalink_url":"https://example.com/orders/123","status":"created"}}`
	vecSig    = "t=1709123456,v1=338efc7c9418016590773fec50cdde35f0cee854777ea940add20d93175b19b5"
)

var vecTime = time.Unix(1709123456, 0)

func TestSignMatchesAnIndependentHMAC(t *testing.T) {
	if got := Sign([]byte(vecSecret), vecTime, []byte(vecBody)); got != vecSig {
		t.Fatalf("Sign = %s; want %s", got, vecSig)
	}
}

func TestVerifyAcceptsTheSignatureAndRefusesEverythingElse(t *testing.T) {
	secret, body := []byte(vecSecret), []byte(vecBody)
	if err := Verify(secret, vecSig, body, vecTime, Tolerance); err != nil {
		t.Fatal(err)
	}
	// Hex is accepted in either case (the header pattern is [a-fA-F0-9]{64}).
	upper := "t=1709123456,v1=" + strings.ToUpper(vecSig[len("t=1709123456,v1="):])
	if err := Verify(secret, upper, body, vecTime, Tolerance); err != nil {
		t.Fatalf("uppercase hex: %v", err)
	}
	for name, tc := range map[string]struct {
		secret []byte
		header string
		body   string
		now    time.Time
		want   error
	}{
		"no header":            {secret, "", vecBody, vecTime, ErrMissing},
		"the wrong secret":     {[]byte("another"), vecSig, vecBody, vecTime, ErrMismatch},
		"a tampered body":      {secret, vecSig, strings.Replace(vecBody, "created", "shipped", 1), vecTime, ErrMismatch},
		"a tampered timestamp": {secret, strings.Replace(vecSig, "1709123456", "1709123457", 1), vecBody, vecTime, ErrMismatch},
		"a flipped bit":        {secret, vecSig[:len(vecSig)-1] + "6", vecBody, vecTime, ErrMismatch},
		"no t":                 {secret, "v1=338efc7c9418016590773fec50cdde35f0cee854777ea940add20d93175b19b5", vecBody, vecTime, ErrMalformed},
		"no v1":                {secret, "t=1709123456", vecBody, vecTime, ErrMalformed},
		"v1 first":             {secret, "v1=338efc7c9418016590773fec50cdde35f0cee854777ea940add20d93175b19b5,t=1709123456", vecBody, vecTime, ErrMalformed},
		"a short digest":       {secret, "t=1709123456,v1=338efc", vecBody, vecTime, ErrMalformed},
		"a long digest":        {secret, vecSig + "00", vecBody, vecTime, ErrMalformed},
		"a non-hex digest":     {secret, "t=1709123456,v1=" + strings.Repeat("g", 64), vecBody, vecTime, ErrMalformed},
		"a non-numeric t":      {secret, "t=abc,v1=" + strings.Repeat("a", 64), vecBody, vecTime, ErrMalformed},
		"an empty t":           {secret, "t=,v1=" + strings.Repeat("a", 64), vecBody, vecTime, ErrMalformed},
		"a signed t":           {secret, "t=-1,v1=" + strings.Repeat("a", 64), vecBody, vecTime, ErrMalformed},
		"a third field":        {secret, vecSig + ",v0=ab", vecBody, vecTime, ErrMalformed},
		"a replay after 301s":  {secret, vecSig, vecBody, vecTime.Add(301 * time.Second), ErrExpired},
		"a clock 301s behind":  {secret, vecSig, vecBody, vecTime.Add(-301 * time.Second), ErrExpired},
	} {
		if err := Verify(tc.secret, tc.header, []byte(tc.body), tc.now, Tolerance); !errors.Is(err, tc.want) {
			t.Errorf("%s: %v; want %v", name, err, tc.want)
		}
	}
	// The tolerance is inclusive.
	for _, d := range []time.Duration{300 * time.Second, -300 * time.Second} {
		if err := Verify(secret, vecSig, body, vecTime.Add(d), Tolerance); err != nil {
			t.Errorf("at %v: %v", d, err)
		}
	}
}

func TestEveryValueSignedIsAcceptedByTheSpecsHeaderPattern(t *testing.T) {
	sig := Sign([]byte("k"), time.Now(), []byte("{}"))
	// ^t=\d+,v1=[a-fA-F0-9]{64}$
	rest, ok := strings.CutPrefix(sig, "t=")
	if !ok {
		t.Fatal(sig)
	}
	ts, mac, ok := strings.Cut(rest, ",v1=")
	if !ok || strings.Trim(ts, "0123456789") != "" || len(mac) != 64 || strings.Trim(mac, "0123456789abcdef") != "" {
		t.Fatalf("signature %q does not match the header pattern", sig)
	}
}

// receiver is a webhook receiver of the shape the specification describes: it verifies
// Merchant-Signature, answers 200 {"received": true}, and records what it was sent.
type receiver struct {
	secret  []byte
	now     func() time.Time
	status  int
	reply   string
	count   atomic.Int32
	body    []byte
	headers http.Header
	verr    error
}

func (rc *receiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rc.count.Add(1)
	rc.body, _ = io.ReadAll(r.Body)
	rc.headers = r.Header.Clone()
	rc.verr = Verify(rc.secret, r.Header.Get(SignatureHeader), rc.body, rc.now(), Tolerance)
	w.Header().Set("Content-Type", "application/json")
	if rc.status != 0 {
		w.WriteHeader(rc.status)
		io.WriteString(w, rc.reply)
		return
	}
	io.WriteString(w, `{"received":true,"request_id":"`+r.Header.Get("Request-Id")+`"}`)
}

func order() Order {
	return Order{Type: "order", ID: "ord_123", CheckoutSessionID: "checkout_session_123", PermalinkURL: "https://example.com/orders/123", Status: "created"}
}

func TestSendPostsASignedEventTheReceiverVerifies(t *testing.T) {
	rc := &receiver{secret: []byte("whsec_x"), now: time.Now}
	srv := httptest.NewTLSServer(rc)
	defer srv.Close()
	w, err := NewWebhook(srv.URL, []byte("whsec_x"), srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Send(context.Background(), Event{Type: EventOrderCreate, Data: order()}); err != nil {
		t.Fatal(err)
	}
	if rc.verr != nil {
		t.Fatalf("the receiver could not verify the signature: %v", rc.verr)
	}
	if rc.headers.Get("Content-Type") != "application/json" || rc.headers.Get("Request-Id") == "" {
		t.Fatalf("headers = %v", rc.headers)
	}
	if _, err := time.Parse(time.RFC3339, rc.headers.Get("Timestamp")); err != nil {
		t.Fatalf("Timestamp = %q", rc.headers.Get("Timestamp"))
	}
	var ev Event
	if err := json.Unmarshal(rc.body, &ev); err != nil || !equal(ev, Event{Type: EventOrderCreate, Data: order()}) {
		t.Fatalf("body = %s (%v)", rc.body, err)
	}
	loadSchema(t).valid(t, "Order", mustJSON(ev.Data))
	// A receiver with another secret refuses the signature.
	rc2 := &receiver{secret: []byte("whsec_other"), now: time.Now}
	srv2 := httptest.NewTLSServer(rc2)
	defer srv2.Close()
	w2, _ := NewWebhook(srv2.URL, []byte("whsec_x"), srv2.Client())
	_ = w2.Send(context.Background(), Event{Type: EventOrderCreate, Data: order()})
	if !errors.Is(rc2.verr, ErrMismatch) {
		t.Fatalf("verr = %v", rc2.verr)
	}
}

func TestSendDeliversTheSpecsWebhookExamplesUnchanged(t *testing.T) {
	b, err := os.ReadFile("testdata/webhook_events.json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	rc := &receiver{secret: []byte("whsec_x"), now: time.Now}
	srv := httptest.NewTLSServer(rc)
	defer srv.Close()
	w, _ := NewWebhook(srv.URL, []byte("whsec_x"), srv.Client())
	for name, raw := range m {
		var ev Event
		if err := json.Unmarshal(raw, &ev); err != nil {
			t.Fatal(err)
		}
		if err := w.Send(context.Background(), ev); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var want, got any
		_ = json.Unmarshal(raw, &want)
		_ = json.Unmarshal(rc.body, &got)
		if rc.verr != nil || !equal(want, got) {
			t.Errorf("%s: verr=%v\nwant %s\n got %s", name, rc.verr, raw, rc.body)
		}
	}
}

func TestSendReportsWhatTheReceiverRefusedAndDoesNotRetry(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		reply  string
		want   int
		code   string
	}{
		{"a bad signature", 401, `{"type":"invalid_request","code":"invalid_signature","message":"Webhook signature verification failed."}`, 401, "invalid_signature"},
		{"a bad payload", 400, `{"type":"invalid_request","code":"missing_required_field","message":"no","param":"$.data"}`, 400, "missing_required_field"},
		{"rate limiting", 429, `{"type":"invalid_request","code":"rate_limited","message":"slow down"}`, 429, "rate_limited"},
		{"a server error", 500, `{"type":"processing_error","code":"internal_error","message":"boom"}`, 500, "internal_error"},
		{"an error that is not JSON", 502, `<html>bad gateway</html>`, 502, ""},
		{"a 200 that does not confirm", 200, `{"received":false}`, 200, "not_received"},
		{"a 200 that is not JSON", 200, `ok`, 200, "not_received"},
		{"a redirect", 307, ``, 307, ""},
	} {
		rc := &receiver{secret: []byte("s"), now: time.Now, status: tc.status, reply: tc.reply}
		srv := httptest.NewTLSServer(rc)
		w, _ := NewWebhook(srv.URL, []byte("s"), srv.Client())
		err := w.Send(context.Background(), Event{Type: EventOrderUpdate, Data: order()})
		var se *SendError
		if !errors.As(err, &se) || se.Status != tc.want || se.Body.Code != tc.code {
			t.Errorf("%s: %v", tc.name, err)
		}
		if rc.count.Load() != 1 {
			t.Errorf("%s: %d attempts; the spec defines no retry", tc.name, rc.count.Load())
		}
		srv.Close()
	}
}

func TestSendRefusesAnEventTheReceiverCouldNotUse(t *testing.T) {
	rc := &receiver{secret: []byte("s"), now: time.Now}
	srv := httptest.NewTLSServer(rc)
	defer srv.Close()
	w, _ := NewWebhook(srv.URL, []byte("s"), srv.Client())
	for name, ev := range map[string]Event{
		"no type":                Event{Data: order()},
		"no order id":            {Type: EventOrderCreate, Data: func() Order { o := order(); o.ID = ""; return o }()},
		"no discriminator":       {Type: EventOrderCreate, Data: func() Order { o := order(); o.Type = ""; return o }()},
		"no checkout session id": {Type: EventOrderCreate, Data: func() Order { o := order(); o.CheckoutSessionID = ""; return o }()},
		"no permalink":           {Type: EventOrderCreate, Data: func() Order { o := order(); o.PermalinkURL = ""; return o }()},
		"no status":              {Type: EventOrderCreate, Data: func() Order { o := order(); o.Status = ""; return o }()},
	} {
		if err := w.Send(context.Background(), ev); err == nil {
			t.Errorf("%s: sent", name)
		}
	}
	if rc.count.Load() != 0 {
		t.Fatal("an unusable event was sent")
	}
}

func TestNewWebhookRefusesWhatCouldNotBeSentSafely(t *testing.T) {
	c := &http.Client{}
	for name, f := range map[string]func() error{
		"plain http": func() error { _, err := NewWebhook("http://agent.example/hook", []byte("s"), c); return err },
		"no host":    func() error { _, err := NewWebhook("https:///hook", []byte("s"), c); return err },
		"relative":   func() error { _, err := NewWebhook("/hook", []byte("s"), c); return err },
		"no secret":  func() error { _, err := NewWebhook("https://agent.example/hook", nil, c); return err },
		"no client":  func() error { _, err := NewWebhook("https://agent.example/hook", []byte("s"), nil); return err },
	} {
		if f() == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := NewWebhook("https://agent.example/hook", []byte("s"), c); err != nil {
		t.Fatal(err)
	}
	// The caller's client is not changed.
	if c.CheckRedirect != nil {
		t.Fatal("NewWebhook changed the caller's client")
	}
}

func TestCompletionDeliversThroughARealWebhook(t *testing.T) {
	rc := &receiver{secret: []byte("whsec_x"), now: time.Now}
	srv := httptest.NewTLSServer(rc)
	defer srv.Close()
	wh, err := NewWebhook(srv.URL, []byte("whsec_x"), srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	r := newRig(t, func(c *Config) { c.Events = wh })
	id := r.create()
	if w := r.post("/checkout_sessions/"+id+"/complete", completeBody); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if rc.verr != nil || rc.count.Load() != 1 {
		t.Fatalf("verr=%v count=%d", rc.verr, rc.count.Load())
	}
	var ev Event
	if err := json.Unmarshal(rc.body, &ev); err != nil || ev.Type != EventOrderCreate || ev.Data.CheckoutSessionID != id {
		t.Fatalf("event = %s", rc.body)
	}
}
