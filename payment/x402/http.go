// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package x402

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type admissionKey struct{}

// FromContext returns the Admission of a request the middleware let through,
// for a handler that wants the payer. It is nil for a request that was not
// priced.
func FromContext(ctx context.Context) *Admission {
	a, _ := ctx.Value(admissionKey{}).(*Admission)
	return a
}

// Middleware charges for the resources of g.Table around any http.Handler. name
// maps a request to its Table key (a route pattern, a method and path: whatever
// the table was filled with); a request whose key is not priced, and every
// OPTIONS preflight, passes through untouched.
//
// A priced request without a payment is answered 402 with the challenge on
// PAYMENT-REQUIRED. One with a payment is verified (Gate.Admit) before the
// handler runs. The payment settles when the handler first commits a success
// status, before its first byte reaches the client, so the response carries
// PAYMENT-RESPONSE; a handler that answers 4xx or 5xx settles nothing, and a
// payment that fails to settle replaces the response with the 402 of spec 5.3.
// Nothing is buffered, so a streaming handler stays streaming; a handler that
// fails after committing a success status has been paid for what it sent.
func Middleware(g Gate, name func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := name(r)
			if r.Method == http.MethodOptions || !g.Table.Priced(key) {
				next.ServeHTTP(w, r)
				return
			}
			adm, ref := g.Admit(r.Context(), key, requestURL(r), r.Header.Get(HeaderPaymentSignature))
			if ref != nil {
				writeRefusal(w, ref)
				return
			}
			pw := &paywall{ResponseWriter: w, r: r, adm: adm}
			next.ServeHTTP(pw, r.WithContext(context.WithValue(r.Context(), admissionKey{}, adm)))
			if !pw.started {
				pw.WriteHeader(http.StatusOK)
			}
		})
	}
}

// paywall holds the response back only until the status is known.
type paywall struct {
	http.ResponseWriter
	r       *http.Request
	adm     *Admission
	started bool
	denied  bool
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (p *paywall) Unwrap() http.ResponseWriter { return p.ResponseWriter }

func (p *paywall) WriteHeader(code int) {
	if code >= 100 && code < 200 && code != http.StatusSwitchingProtocols {
		p.ResponseWriter.WriteHeader(code) // informational: not the answer
		return
	}
	if p.started {
		return
	}
	p.started = true
	if code >= 400 {
		p.ResponseWriter.WriteHeader(code)
		return
	}
	s, ref := p.adm.Settle(p.r.Context())
	if ref != nil {
		p.denied = true
		p.ResponseWriter.Header().Del("Content-Length")
		writeRefusal(p.ResponseWriter, ref)
		return
	}
	if v, err := EncodeHeader(s); err == nil {
		p.ResponseWriter.Header().Set(HeaderPaymentResponse, v)
		expose(p.ResponseWriter.Header())
	}
	p.ResponseWriter.WriteHeader(code)
}

func (p *paywall) Write(b []byte) (int, error) {
	if !p.started {
		p.WriteHeader(http.StatusOK)
	}
	if p.denied {
		return len(b), nil // the response was replaced; the handler's bytes are dropped
	}
	return p.ResponseWriter.Write(b)
}

func (p *paywall) Flush() {
	if !p.started {
		p.WriteHeader(http.StatusOK)
	}
	if f, ok := p.ResponseWriter.(http.Flusher); ok && !p.denied {
		f.Flush()
	}
}

// writeRefusal answers a Refusal: the status, the challenge and settlement
// headers, and a small JSON body (the body is an implementation concern,
// spec transports-v2/http.md).
func writeRefusal(w http.ResponseWriter, ref *Refusal) {
	h := w.Header()
	headers, err := ref.Headers()
	if err != nil {
		ref = &Refusal{Status: http.StatusInternalServerError, Reason: ReasonSettleError}
		headers = nil
	}
	for k, v := range headers {
		h.Set(k, v)
	}
	expose(h)
	h.Set("Content-Type", "application/json")
	h.Set("Cache-Control", "no-store") // a 402 must stay out of shared caches
	w.WriteHeader(ref.Status)
	_ = json.NewEncoder(w).Encode(map[string]any{"x402Version": Version, "error": ref.Reason})
}

// expose names the two response headers in Access-Control-Expose-Headers,
// merged with what is already there, so a browser can read them
// (transports-v2/http.md, browser clients).
func expose(h http.Header) {
	have := h.Values("Access-Control-Expose-Headers")
	for _, name := range []string{HeaderPaymentRequired, HeaderPaymentResponse} {
		found := false
		for _, v := range have {
			for _, part := range strings.Split(v, ",") {
				found = found || strings.EqualFold(strings.TrimSpace(part), name)
			}
		}
		if !found {
			h.Add("Access-Control-Expose-Headers", name)
		}
	}
}

// requestURL is the public URL the client asked for: https when the connection
// is TLS or a proxy says it was.
func requestURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host + r.URL.RequestURI()
}

// Transport is the client's side as an http.RoundTripper: a 402 that carries a
// PAYMENT-REQUIRED challenge is answered by Payer and the request is sent again
// once with PAYMENT-SIGNATURE. A Payer that declines, a body that cannot be
// replayed (no GetBody) or a second 402 hands back the response it has, so the
// caller sees the 402 and its challenge.
type Transport struct {
	Base  http.RoundTripper // nil means http.DefaultTransport
	Payer Payer
}

func (t Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	resp, err := base.RoundTrip(req)
	if err != nil || resp.StatusCode != http.StatusPaymentRequired {
		return resp, err
	}
	header := resp.Header.Get(HeaderPaymentRequired)
	if header == "" || req.Header.Get(HeaderPaymentSignature) != "" {
		return resp, nil
	}
	var required PaymentRequired
	if DecodeHeader(header, &required) != nil || required.X402Version != Version {
		return resp, nil
	}
	retry := req.Clone(req.Context())
	if req.Body != nil && req.Body != http.NoBody {
		if req.GetBody == nil {
			return resp, nil
		}
		if retry.Body, err = req.GetBody(); err != nil {
			return resp, nil
		}
	}
	pay, err := t.Payer.Pay(req.Context(), required)
	if err != nil {
		return resp, nil
	}
	signed, err := EncodeHeader(pay)
	if err != nil {
		return resp, nil
	}
	retry.Header.Set(HeaderPaymentSignature, signed)
	resp.Body.Close()
	return base.RoundTrip(retry)
}
