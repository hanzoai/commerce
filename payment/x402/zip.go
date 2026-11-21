// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package x402

import (
	"net/http"

	"github.com/zap-proto/zip"
)

// Paywall is Middleware for a zip router. It must be applied before the routes it
// prices (app.Use, or a Group's Use). name maps a method and path to a Table key,
// as for Middleware; HEAD is looked up as GET, and an empty or unpriced key and every
// OPTIONS preflight pass through.
//
// A zip handler writes its answer without an http.ResponseWriter, so there is no
// first byte to settle ahead of: the payment settles when the handler has returned
// and answered below 400, before the response is sent, and PAYMENT-RESPONSE is set
// then. A handler that returns an error, or answers 4xx or 5xx, settles nothing; a
// payment that fails to settle replaces the answer with the 402 of spec 5.3. The
// payment stays valid for a retry whenever nothing settled.
func Paywall(g *Gate, name func(method, path string) string) zip.Handler {
	return func(c *zip.Ctx) error {
		key := name(lookupMethod(c.Method()), c.Path())
		if c.Method() == http.MethodOptions || !g.Table.Priced(key) {
			return c.Next()
		}
		fc := c.Fiber()
		adm, ref := g.Admit(c.Context(), key, fc.Scheme()+"://"+c.Host()+fc.OriginalURL(), c.Header(HeaderPaymentSignature))
		if ref != nil {
			return refuseZip(c, ref)
		}
		defer adm.Release()
		if err := c.Next(); err != nil {
			return err
		}
		if fc.Response().StatusCode() >= 400 {
			return nil
		}
		s, ref := adm.Settle(c.Context())
		if ref != nil {
			return refuseZip(c, ref)
		}
		if v, err := EncodeHeader(s); err == nil {
			c.SetHeader(HeaderPaymentResponse, v)
			exposeZip(c)
		}
		return nil
	}
}

// refuseZip answers a Refusal on a zip context.
func refuseZip(c *zip.Ctx, ref *Refusal) error {
	headers, err := ref.Headers()
	if err != nil {
		ref, headers = &Refusal{Status: http.StatusInternalServerError, Reason: ReasonSettleError}, nil
	}
	for k, v := range headers {
		c.SetHeader(k, v)
	}
	exposeZip(c)
	c.SetHeader("Cache-Control", "no-store") // a 402 must stay out of shared caches
	return c.JSON(ref.Status, map[string]any{"x402Version": Version, "error": ref.Reason})
}

// exposeZip merges the two response headers into Access-Control-Expose-Headers.
func exposeZip(c *zip.Ctx) {
	h := http.Header{}
	if v := c.Fiber().Response().Header.Peek("Access-Control-Expose-Headers"); len(v) > 0 {
		h.Add("Access-Control-Expose-Headers", string(v))
	}
	expose(h)
	c.SetHeader("Access-Control-Expose-Headers", joinHeader(h.Values("Access-Control-Expose-Headers")))
}

func joinHeader(vs []string) string {
	out := ""
	for i, v := range vs {
		if i > 0 {
			out += ", "
		}
		out += v
	}
	return out
}
