package middleware

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/mintauth"
)

// RequestContext extracts the standard Go context from the HTTP request
// and stores it in the request-scoped locals for downstream handlers.
//
// It also marks the stored context mint-gated (mintauth.WithGate): every inbound
// request is a potential untrusted principal, so any spendable-balance mint that
// flows from it must carry mint authorization or the ledger sink refuses it. This
// backs up the primary gate in Organization.Namespaced for the rare handler that
// builds a datastore straight from c.Context(). Authorization
// (PlatformOnly / settled payment / server-fixed grant) rides on top.
func RequestContext() zip.Handler {
	return func(c *zip.Ctx) error {
		// SetContext, not a locals key: after this line c.Context() IS the
		// gated request context for every later middleware and handler — one
		// context per request, set once at the boundary.
		c.SetContext(mintauth.WithGate(c.Context()))
		return c.Next()
	}
}

// Automatically get the Host header so we can decide what to do with a given
// request.
func AddHost() zip.Handler {
	return func(c *zip.Ctx) error {
		c.Locals("host", c.Fiber().Host())
		return c.Next()
	}
}

func LiveReload() zip.Handler {
	return func(c *zip.Ctx) error {
		err := c.Next()
		// If 200 and text/html content-type inject bebop script for live reload
		resp := c.Fiber().Response()
		if resp.StatusCode() == 200 && string(resp.Header.ContentType()) == "text/html; charset=utf-8" {
			resp.AppendBodyString(`<!-- Live reload via bebop -->
<script src="http://localhost:8090/bebop.min.js"></script>
<script>try {(new Bebop({port: 8090})).connect()} catch(err) {}</script>`)
		}
		return err
	}
}
