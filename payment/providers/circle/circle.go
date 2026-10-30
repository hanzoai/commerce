// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

// Package circle is commerce's client for Circle Gateway: a chain-abstracted
// USDC balance keyed by depositor address, and batched settlement of EIP-3009
// authorizations against it (Gateway Nanopayments, the x402 "exact" scheme).
//
// Every call here is PERMISSIONLESS. Gateway takes no API key for info,
// balances, deposits, estimate, transfer reads, or x402 verify/settle/transfers;
// the only keyed surface is webhook subscription management, which this
// package does not use because no Gateway webhook reports the settlement of a
// batched authorization (see LLM.md, "Circle Gateway").
//
// Specs: Circle's gateway.yaml and gateway-nanopayments.yaml OpenAPI documents,
// and the @circle-fin/x402-batching SDK for the EIP-712 domain of a batched
// authorization (eip712.go).
package circle

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Network is which Gateway a deployment talks to. Mainnet moves real USDC;
// testnet moves faucet USDC that is worth nothing.
type Network string

const (
	Mainnet Network = "mainnet"
	Testnet Network = "testnet"
)

// Base is the Gateway API origin for the network.
func (n Network) Base() string {
	if n == Mainnet {
		return "https://gateway-api.circle.com"
	}
	return "https://gateway-api-testnet.circle.com"
}

// timeout bounds every Gateway call. A settle that has not answered in this
// long is treated as an UNKNOWN outcome by the caller, never as a refusal.
const timeout = 20 * time.Second

// limit bounds a response body. Gateway's largest answer (x402/supported) is
// a few KiB; a body past this is not Gateway's and is refused unread.
const limit = 1 << 20

// Client calls one Gateway origin.
type Client struct {
	base string
	http *http.Client
}

// New returns a client for the Gateway at base (Network.Base in production,
// an httptest server in tests). Redirects are refused: Gateway answers in
// place, and following one would send a signed authorization to a host the
// operator never named.
func New(base string) *Client {
	return &Client{
		base: strings.TrimRight(base, "/"),
		http: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// Error is a Gateway answer that was not a success: a non-2xx status, or a
// 2xx whose body could not be read as the documented shape.
//
// Status is the HTTP status; Code and Message are Gateway's own error body
// ({code, message}) when it sent one.
type Error struct {
	Status  int
	Code    int
	Message string
}

func (e *Error) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("circle gateway: HTTP %d", e.Status)
	}
	return fmt.Sprintf("circle gateway: HTTP %d: %s", e.Status, e.Message)
}

// Refused reports whether Gateway answered definitively that it did not
// process the request: a 4xx other than a timeout (408) or a throttle (429).
// Every other failure — a transport error, a timeout, a 5xx, an unreadable
// 2xx — leaves the outcome UNKNOWN, and a caller that moves money on the
// answer must treat it that way.
func Refused(err error) bool {
	var e *Error
	if !errors.As(err, &e) {
		return false
	}
	return e.Status >= 400 && e.Status < 500 && e.Status != http.StatusRequestTimeout && e.Status != http.StatusTooManyRequests
}

// do sends one request and decodes a 2xx body into out.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("circle gateway: encode %s %s: %w", method, path, err)
		}
		body = bytes.NewReader(b)
	}
	u := c.base + path
	if len(query) > 0 {
		// A CAIP-2 network ("eip155:8453") travels with its colon literal, as
		// Circle's own SDK sends it; ':' is legal in a query (RFC 3986 pchar).
		u += "?" + strings.ReplaceAll(query.Encode(), "%3A", ":")
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return fmt.Errorf("circle gateway: %s %s: %w", method, path, err)
	}
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("circle gateway: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return fmt.Errorf("circle gateway: read %s %s: %w", method, path, err)
	}
	if len(raw) > limit {
		return &Error{Status: resp.StatusCode, Message: "response exceeds 1 MiB"}
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		e := &Error{Status: resp.StatusCode}
		var body struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		if json.Unmarshal(raw, &body) == nil && body.Message != "" {
			e.Code, e.Message = body.Code, body.Message
		} else {
			e.Message = snippet(raw)
		}
		return e
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return &Error{Status: resp.StatusCode, Message: "unreadable response: " + err.Error()}
	}
	return nil
}

// snippet is the start of a body Gateway sent without its error shape, for a
// log line. Gateway's bodies carry nothing of ours, so this leaks nothing.
func snippet(raw []byte) string {
	s := strings.TrimSpace(string(raw))
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}
