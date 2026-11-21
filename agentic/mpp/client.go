// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package mpp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Answer is a Payer's answer to a challenge: the payer's identifier (section 5.2) and the
// method's proof, JSON-encoded as the credential's payload.
type Answer struct {
	Source  string
	Payload any
}

// Payer is the client side of a payment method.
type Payer interface {
	// Accepts lists the method and intent ranges the payer answers. They are sent as
	// Accept-Payment and rank the challenges offered (section 7.4).
	Accepts() []Range
	// Pay authorizes one challenge, given its decoded request. It verifies the amount,
	// recipient, currency and validity window, never the description (section 11.6); an
	// error refuses the payment.
	Pay(ctx context.Context, c Challenge, request json.RawMessage) (*Answer, error)
}

// Client is an http.RoundTripper that answers a 402 with a Payment credential: it sends the
// request once more with one credential, in the field the challenge selects. It does not
// pay over unencrypted HTTP (section 11.2), answer an expired challenge (section 5.1.2),
// or retry a body it cannot read again; any other response is returned as it is.
type Client struct {
	// Base carries the requests; http.DefaultTransport when nil.
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
		var ranges []string
		for _, r := range c.Payer.Accepts() {
			ranges = append(ranges, r.String())
		}
		if len(ranges) > 0 {
			first.Header.Set(accepting, strings.Join(ranges, ", "))
		}
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
	_ = resp.Body.Close()
	return c.base().RoundTrip(paid)
}

// pay builds the request that answers resp, or nil when the challenge is not to be answered.
func (c *Client) pay(orig, first *http.Request, resp *http.Response) (*http.Request, error) {
	if orig.URL.Scheme != "https" {
		return nil, nil
	}
	found, _ := Parse(resp.Header.Values(authenticate))
	now := c.now()
	var open []Challenge
	for _, ch := range found {
		if at, ok := ch.Expiry(); !ok || now.Before(at) {
			open = append(open, ch)
		}
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
	var sent []byte
	if orig.Body != nil && orig.Body != http.NoBody {
		if orig.GetBody == nil {
			return nil, nil
		}
		var err error
		if retry.Body, err = orig.GetBody(); err != nil {
			return nil, err
		}
		if ch.Digest != "" {
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
	}
	if ch.Digest != "" && !digestMatches(ch.Digest, sent) {
		return nil, nil // the credential goes with the body the challenge names (section 5.1.3)
	}
	field := ch.Field()
	if retry.Header.Get(field) != "" {
		return nil, nil // the field is taken by other authentication
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
