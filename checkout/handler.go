// SPA handler for the hosted checkout. Serves the embedded Vite bundle at
// "/". Follows the admin/embed.go pattern:
//   - path with file extension that exists in the embed → serve with
//     long-cache immutable headers (hashed filenames from Vite)
//   - anything else → serve index.html (client-side router takes over),
//     no-cache so a deploy rolls out without stale page fragments
package checkout

import (
	"io/fs"
	"mime"
	"strings"

	"github.com/zap-proto/zip"
)

// SPAHandler returns the zip.Handler that serves the embedded checkout
// SPA. prefix is typically "" (mounted at root) or "/pay" if the SPA
// needs to live at a subpath. Unknown extension-less paths fall through
// to index.html so TanStack Router can render them.
//
// The handler is Host-agnostic — tenant branding is fetched at runtime by
// the SPA via GET /checkout/v1/tenant. This keeps the embed identical
// across all tenants and the binary itself reproducible.
func SPAHandler(prefix string) zip.Handler {
	root := UISub()

	return func(c *zip.Ctx) error {
		p := strings.TrimPrefix(c.Path(), prefix)
		p = strings.TrimPrefix(p, "/")

		// Asset request: has a file extension and no further slash after
		// the extension. Hashed filenames from Vite → immutable cache.
		if i := strings.LastIndexByte(p, '.'); i >= 0 && !strings.Contains(p[i:], "/") {
			if b, err := fs.ReadFile(root, p); err == nil {
				c.SetHeader("Cache-Control", "public, max-age=31536000, immutable")
				if ct := mime.TypeByExtension(p[i:]); ct != "" {
					c.SetHeader("Content-Type", ct)
				}
				return c.Bytes(200, b)
			}
		}

		// SPA fallback — always the freshest index.html so deploys
		// invalidate immediately without users holding a stale shell.
		c.SetHeader("Cache-Control", "no-cache")
		// Defense-in-depth browser hardening for the checkout surface:
		//   - framing: clickjacking the Square payment form would be
		//     catastrophic, so deny all.
		//   - MIME: lock so a crafted upload can't be interpreted as
		//     script.
		//   - referrer: strict origin cross-origin means the backend
		//     never sees the full URL including ?return= in logs.
		c.SetHeader("X-Frame-Options", "DENY")
		c.SetHeader("X-Content-Type-Options", "nosniff")
		c.SetHeader("Referrer-Policy", "strict-origin-when-cross-origin")

		idx, err := fs.ReadFile(root, "index.html")
		if err != nil {
			c.SetHeader("Content-Type", "text/plain; charset=utf-8")
			return c.String(503, "checkout SPA not built")
		}
		c.SetHeader("Content-Type", "text/html; charset=utf-8")
		return c.Bytes(200, idx)
	}
}
