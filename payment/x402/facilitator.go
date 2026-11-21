// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package x402

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Facilitator verifies and settles payments (spec section 7). A resource server
// delegates the chain, or the ledger, to one: Remote reaches any facilitator
// that serves the HTTP API of section 7, and a host that settles on its own
// books implements this interface directly.
//
// A refusal of the payment is an answer, not an error: Verify returns
// IsValid false with a reason, Settle returns Success false with a reason. The
// returned error is for a facilitator that could not answer at all.
type Facilitator interface {
	// Verify checks the payment without committing it. It MUST NOT move funds
	// or write state (spec 7.1).
	Verify(ctx context.Context, pay PaymentPayload, offered PaymentRequirements) (VerifyResponse, error)
	// Settle commits the payment (spec 7.2). A payment settles at most once.
	Settle(ctx context.Context, pay PaymentPayload, offered PaymentRequirements) (SettlementResponse, error)
}

// Remote is a Facilitator served over HTTP: POST {URL}/verify, POST
// {URL}/settle and GET {URL}/supported, as section 7 defines them.
type Remote struct {
	// URL is the facilitator's base URL, without a trailing slash.
	URL string
	// Client is the HTTP client; nil means one with a 30 second timeout.
	Client *http.Client
	// Authorize adds whatever credential the facilitator requires to a request;
	// nil adds none. It is given the path being called ("/verify", "/settle",
	// "/supported").
	Authorize func(r *http.Request, path string) error
}

// body is the request of POST /verify and POST /settle (spec 7.1, 7.2).
type body struct {
	X402Version         int                 `json:"x402Version"`
	PaymentPayload      PaymentPayload      `json:"paymentPayload"`
	PaymentRequirements PaymentRequirements `json:"paymentRequirements"`
}

// maxBody bounds what is read from a facilitator, so a hostile or broken one
// cannot exhaust memory.
const maxBody = 1 << 20

func (f Remote) Verify(ctx context.Context, pay PaymentPayload, offered PaymentRequirements) (VerifyResponse, error) {
	var out VerifyResponse
	if err := f.call(ctx, http.MethodPost, "/verify", body{Version, pay, offered}, &out); err != nil {
		return VerifyResponse{}, err
	}
	return out, nil
}

func (f Remote) Settle(ctx context.Context, pay PaymentPayload, offered PaymentRequirements) (SettlementResponse, error) {
	var out SettlementResponse
	if err := f.call(ctx, http.MethodPost, "/settle", body{Version, pay, offered}, &out); err != nil {
		return SettlementResponse{}, err
	}
	return out, nil
}

// Supported lists the schemes and networks the facilitator settles (spec 7.3).
func (f Remote) Supported(ctx context.Context) (SupportedResponse, error) {
	var out SupportedResponse
	if err := f.call(ctx, http.MethodGet, "/supported", nil, &out); err != nil {
		return SupportedResponse{}, err
	}
	return out, nil
}

// call sends one request and decodes the answer. A facilitator may refuse a
// payment with a 4xx whose body is still the section 7 answer, so a non-2xx
// body that decodes into out is an answer; one that does not is an error.
func (f Remote) call(ctx context.Context, method, path string, in, out any) error {
	var rd io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("x402: facilitator %s: %w", path, err)
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(f.URL, "/")+path, rd)
	if err != nil {
		return fmt.Errorf("x402: facilitator %s: %w", path, err)
	}
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if f.Authorize != nil {
		if err := f.Authorize(req, path); err != nil {
			return fmt.Errorf("x402: facilitator %s: authorize: %w", path, err)
		}
	}
	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("x402: facilitator %s: %w", path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return fmt.Errorf("x402: facilitator %s: %w", path, err)
	}
	if err := json.Unmarshal(raw, out); err != nil || (resp.StatusCode >= 300 && !answered(raw)) {
		return fmt.Errorf("x402: facilitator %s: status %d: %s", path, resp.StatusCode, snippet(raw))
	}
	return nil
}

// answered reports whether a non-2xx body is a section 7 answer: it carries
// isValid or success.
func answered(raw []byte) bool {
	var probe struct {
		IsValid *bool `json:"isValid"`
		Success *bool `json:"success"`
	}
	return json.Unmarshal(raw, &probe) == nil && (probe.IsValid != nil || probe.Success != nil)
}

func snippet(b []byte) string {
	const n = 200
	if len(b) > n {
		b = b[:n]
	}
	return strings.TrimSpace(string(b))
}
