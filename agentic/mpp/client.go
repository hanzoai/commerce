// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package mpp

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Answer is what a Payer returns for a challenge: the payer's identifier, optionally
// (a DID is recommended, section 5.2), and the method's proof, which is JSON-encoded
// into the credential's payload.
type Answer struct {
	Source  string
	Payload any
}

// Payer is the client's side of a payment method: it can say which challenges it
// answers, and pays one.
type Payer interface {
	// Accepts lists the method and intent ranges the payer answers, with weights. They
	// are sent as Accept-Payment (section 7.4) and rank the challenges a server offers.
	Accepts() []Range
	// Pay authorizes one challenge, whose request is decoded JSON. It must verify the
	// amount, the recipient, the currency and the validity window before paying, and
	// must not rely on the description (section 11.6): a payer that pays what a server
	// asks is the loss the draft warns of. An error refuses the payment.
	Pay(ctx context.Context, c Challenge, request json.RawMessage) (*Answer, error)
}

// Client is an http.RoundTripper that answers a 402 with a Payment credential.
//
// A request that comes back 402 with a Payment challenge the Payer answers is sent once
// more with a credential, in the field the challenge selects. It never pays over
// unencrypted HTTP (section 11.2), never answers a challenge that has expired, never
// sends more than one credential, and sends the request again only when its body can
// be read again. Any other response is returned as it is.
type Client struct {
	// Base is the transport that carries the requests; http.DefaultTransport when nil.
	Base  http.RoundTripper
	Payer Payer
	// Now is the clock; time.Now when nil.
	Now func() time.Time
}

func (c *Client) base() http.RoundTripper {
	if c.Base != nil {
		return c.Base
	}
	return http.DefaultTransport
}

func (c *Client) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// RoundTrip implements http.RoundTripper.
func (c *Client) RoundTrip(req *http.Request) (*http.Response, error) {
	first := req.Clone(req.Context())
	if len(first.Header.Values(accepting)) == 0 {
		if ranges := c.Payer.Accepts(); len(ranges) > 0 {
			parts := make([]string, len(ranges))
			for i, r := range ranges {
				parts[i] = r.String()
			}
			first.Header.Set(accepting, strings.Join(parts, ", "))
		}
	}
	if mutating(req.Method) && first.Header.Get(idempotency) == "" {
		// The same key rides the paid retry, so a response lost after paying can be
		// asked for again without paying twice (section 11.4).
		var b [16]byte
		if _, err := rand.Read(b[:]); err != nil {
			return nil, err
		}
		first.Header.Set(idempotency, base64.RawURLEncoding.EncodeToString(b[:]))
	}
	resp, err := c.base().RoundTrip(first)
	if err != nil || resp.StatusCode != http.StatusPaymentRequired {
		return resp, err
	}
	paid, err := c.pay(req, first, resp)
	if err != nil {
		_ = resp.Body.Close()
		return nil, err
	}
	if paid == nil {
		return resp, nil
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	_ = resp.Body.Close()
	return c.base().RoundTrip(paid)
}

// mutating says the method may change state, so a retry wants an Idempotency-Key.
func mutating(method string) bool {
	switch method {
	case "", http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return false
	}
	return true
}

// pay builds the request that answers resp, or nil when the challenge cannot or should
// not be answered.
func (c *Client) pay(orig, first *http.Request, resp *http.Response) (*http.Request, error) {
	if orig.URL.Scheme != "https" {
		return nil, nil // credentials never travel over unencrypted HTTP
	}
	found, _ := Parse(resp.Header.Values(authenticate))
	now := c.now()
	var open []Challenge
	for _, ch := range found {
		if at, ok := ch.Expiry(); ok && !now.Before(at) {
			continue // clients must not answer an expired challenge (section 5.1.2)
		}
		open = append(open, ch)
	}
	offers := make([][2]string, len(open))
	for i, ch := range open {
		offers[i] = [2]string{ch.Method, ch.Intent}
	}
	order, err := rank(offers, c.Payer.Accepts())
	if err != nil {
		return nil, nil
	}
	ch := open[order[0]]

	retry := first.Clone(first.Context())
	if orig.Body != nil && orig.Body != http.NoBody {
		if orig.GetBody == nil {
			return nil, nil // the body cannot be sent again
		}
		body, err := orig.GetBody()
		if err != nil {
			return nil, err
		}
		retry.Body = body
	}
	if ch.Digest != "" {
		var sent []byte
		if retry.Body != nil && retry.Body != http.NoBody {
			body, err := orig.GetBody()
			if err != nil {
				return nil, err
			}
			sent, err = io.ReadAll(body)
			_ = body.Close()
			if err != nil {
				return nil, err
			}
		}
		if !digestMatches(ch.Digest, sent) {
			return nil, nil // the credential must go with the body the challenge names
		}
	}
	field := ch.Field()
	if retry.Header.Get(field) != "" {
		return nil, nil // the field is taken by another authentication; no other may carry it
	}

	request, _ := raw.DecodeString(ch.Request)
	ans, err := c.Payer.Pay(first.Context(), ch, request)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(ans.Payload)
	if err != nil {
		return nil, fmt.Errorf("mpp: payload: %w", err)
	}
	retry.Header.Set(field, Credential{Challenge: ch, Source: ans.Source, Payload: payload}.String())
	return retry, nil
}
