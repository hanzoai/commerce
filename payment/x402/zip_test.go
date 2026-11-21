// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package x402

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zap-proto/zip"
)

// zipRig serves routes behind Paywall on a zip app.
type zipRig struct {
	books *books
	app   *zip.App
	now   time.Time
	ran   int
}

func newZipRig(t *testing.T) *zipRig {
	t.Helper()
	tab, err := NewTable(testAsset())
	if err != nil {
		t.Fatal(err)
	}
	if err := tab.Set("GET /data", Price{Amount: "0.01", Description: "premium data", MimeType: "application/json"}); err != nil {
		t.Fatal(err)
	}
	r := &zipRig{books: &books{}, now: time.Unix(1740672100, 0)}
	g := &Gate{Table: tab, Facilitator: r.books, Now: func() time.Time { return r.now }}
	r.app = zip.New(zip.Config{})
	r.app.Use(Paywall(g, func(method, path string) string { return method + " " + path }))
	r.app.Raw(http.MethodGet, "/data", func(c *zip.Ctx) error {
		r.ran++
		return c.JSON(200, map[string]string{"data": "premium"})
	})
	r.app.Raw(http.MethodGet, "/broken", func(c *zip.Ctx) error { return c.JSON(500, map[string]string{"error": "no"}) })
	r.app.Raw(http.MethodGet, "/free", func(c *zip.Ctx) error { return c.JSON(200, map[string]string{"free": "yes"}) })
	return r
}

func (r *zipRig) do(t *testing.T, method, path string, hdr map[string]string) (*http.Response, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := r.app.Test(req, zip.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

func (r *zipRig) paid(t *testing.T) map[string]string {
	t.Helper()
	h, err := EncodeHeader(signed(t, testRequirements(), 0, uint64(r.now.Unix())+60))
	if err != nil {
		t.Fatal(err)
	}
	return map[string]string{HeaderPaymentSignature: h}
}

func TestPaywallChallengesServesAndSettlesOnZip(t *testing.T) {
	r := newZipRig(t)
	resp, body := r.do(t, http.MethodGet, "/data", nil)
	if resp.StatusCode != 402 || r.ran != 0 || strings.Contains(body, "premium") {
		t.Fatalf("unpaid: %d ran %d", resp.StatusCode, r.ran)
	}
	var pr PaymentRequired
	if err := DecodeHeader(resp.Header.Get(HeaderPaymentRequired), &pr); err != nil || pr.Accepts[0].Amount != "10000" || pr.Resource.Description != "premium data" {
		t.Fatalf("challenge %v %+v", err, pr)
	}

	resp, body = r.do(t, http.MethodGet, "/data", r.paid(t))
	if resp.StatusCode != 200 || !strings.Contains(body, "premium") || r.ran != 1 || len(r.books.settled) != 1 {
		t.Fatalf("paid: %d %s ran %d settled %d", resp.StatusCode, body, r.ran, len(r.books.settled))
	}
	var s SettlementResponse
	if err := DecodeHeader(resp.Header.Get(HeaderPaymentResponse), &s); err != nil || !s.Success || s.Amount != "10000" {
		t.Fatalf("receipt %v %+v", err, s)
	}
}

func TestPaywallSettlesNothingWhenTheHandlerFails(t *testing.T) {
	r := newZipRig(t)
	tab := r.app
	_ = tab
	// /broken is priced too, and answers 500.
	g := &Gate{Facilitator: r.books, Now: func() time.Time { return r.now }}
	g.Table, _ = NewTable(testAsset())
	_ = g.Table.Set("GET /broken", Price{Amount: "0.01"})
	app := zip.New(zip.Config{})
	app.Use(Paywall(g, func(method, path string) string { return method + " " + path }))
	app.Raw(http.MethodGet, "/broken", func(c *zip.Ctx) error { return c.JSON(500, map[string]string{"error": "no"}) })
	app.Raw(http.MethodGet, "/refused", func(c *zip.Ctx) error { return zip.ErrNotFound("gone") })
	_ = g.Table.Set("GET /refused", Price{Amount: "0.01"})
	for _, path := range []string{"/broken", "/refused"} {
		h, _ := EncodeHeader(signed(t, testRequirements(), 0, uint64(r.now.Unix())+60))
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set(HeaderPaymentSignature, h)
		resp, err := app.Test(req, zip.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode < 400 || resp.Header.Get(HeaderPaymentResponse) != "" || len(r.books.settled) != 0 {
			t.Fatalf("%s: status %d settled %d", path, resp.StatusCode, len(r.books.settled))
		}
	}
}

func TestPaywallFailedSettlementReplacesTheAnswer(t *testing.T) {
	r := newZipRig(t)
	r.books.failWith = ReasonInsufficientFunds
	resp, body := r.do(t, http.MethodGet, "/data", r.paid(t))
	var s SettlementResponse
	if err := DecodeHeader(resp.Header.Get(HeaderPaymentResponse), &s); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 402 || strings.Contains(body, "premium") || s.Success || s.ErrorReason != ReasonInsufficientFunds {
		t.Fatalf("%d %s %+v", resp.StatusCode, body, s)
	}
}

func TestPaywallFreeRoutesAndHEAD(t *testing.T) {
	r := newZipRig(t)
	if resp, body := r.do(t, http.MethodGet, "/free", nil); resp.StatusCode != 200 || !strings.Contains(body, "yes") {
		t.Fatalf("free: %d", resp.StatusCode)
	}
	// HEAD reaches the GET handler, so it is priced as GET: no free ride.
	resp, _ := r.do(t, http.MethodHead, "/data", nil)
	if resp.StatusCode != 402 || r.ran != 0 {
		t.Fatalf("HEAD: %d, handler ran %d", resp.StatusCode, r.ran)
	}
}

// Two requests carrying one payment at once: the second is refused while the
// first is in flight, so one payment cannot run the resource twice.
func TestConcurrentUseOfOnePaymentIsRefused(t *testing.T) {
	r := newRig(t, okHandler)
	h, _ := EncodeHeader(signed(t, testRequirements(), 0, uint64(r.now.Unix())+60))
	adm, ref := r.gate.Admit(t.Context(), "GET /data", "u", h)
	if ref != nil {
		t.Fatalf("first: %+v", ref)
	}
	if _, ref := r.gate.Admit(t.Context(), "GET /data", "u", h); ref == nil || ref.Status != 402 || ref.Reason != ReasonInvalidTransaction {
		t.Fatalf("second while the first is in flight: %+v", ref)
	}
	adm.Release() // the resource failed: the payment moved nothing and is usable again
	adm.Release() // idempotent
	adm2, ref := r.gate.Admit(t.Context(), "GET /data", "u", h)
	if ref != nil {
		t.Fatalf("after release: %+v", ref)
	}
	if _, ref := adm2.Settle(t.Context()); ref != nil {
		t.Fatalf("settle: %+v", ref)
	}
	if _, ref := r.gate.Admit(t.Context(), "GET /data", "u", h); ref == nil {
		t.Fatal("a settled payment was admitted again")
	}
}

func TestHEADIsPricedAsGETOnNetHTTP(t *testing.T) {
	r := newRig(t, okHandler)
	req, _ := http.NewRequest(http.MethodHead, r.srv.URL+"/data", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 402 || r.hits != 0 {
		t.Fatalf("HEAD: %d hits %d", resp.StatusCode, r.hits)
	}
}

// A facilitator that panics must not leave the payment claimed: the next request
// with the same payment is judged on its merits, not refused for ever.
type panicking struct{ *books }

func (panicking) Verify(context.Context, PaymentPayload, PaymentRequirements) (VerifyResponse, error) {
	panic("facilitator exploded")
}

func TestAPanickingFacilitatorLeavesNoClaim(t *testing.T) {
	r := newRig(t, okHandler)
	h, _ := EncodeHeader(signed(t, testRequirements(), 0, uint64(r.now.Unix())+60))
	good := r.gate.Facilitator
	r.gate.Facilitator = panicking{r.books}
	func() {
		defer func() { _ = recover() }()
		r.gate.Admit(t.Context(), "GET /data", "u", h)
	}()
	r.gate.Facilitator = good
	if adm, ref := r.gate.Admit(t.Context(), "GET /data", "u", h); ref != nil {
		t.Fatalf("claim leaked: %+v", ref)
	} else {
		adm.Release()
	}
}
