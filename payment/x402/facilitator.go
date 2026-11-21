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

// Remote is a Facilitator served over HTTP: POST {URL}/verify and POST
// {URL}/settle, as section 7 defines them.
type Remote struct {
	// URL is the facilitator's base URL, without a trailing slash.
	URL string
	// Client is the HTTP client; nil means http.DefaultClient. Calls made through a
	// Gate are bounded by the requirement's maxTimeoutSeconds.
	Client *http.Client
	// Authorize adds whatever credential the facilitator requires to a request;
	// nil adds none. It is given the path being called ("/verify", "/settle").
	Authorize func(r *http.Request, path string) error
}

// body is the request of POST /verify and POST /settle (spec 7.1, 7.2).
type body struct {
	X402Version         int                 `json:"x402Version"`
	PaymentPayload      PaymentPayload      `json:"paymentPayload"`
	PaymentRequirements PaymentRequirements `json:"paymentRequirements"`
}

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

// call sends one request and decodes the answer. Anything but a 2xx is an error:
// a refusal of a payment is a 200 with isValid or success false (spec 7.1, 7.2).
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
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("x402: facilitator %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("x402: facilitator %s: status %d", path, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("x402: facilitator %s: %w", path, err)
	}
	return nil
}
