// Package middleware provides HTTP middleware for the Commerce API.
//
// This file implements Cloudflare-aware HTTP cache control middleware.
// Routes served via api.hanzo.ai sit behind CF; correct Cache-Control
// headers are the only lever we have to control what CF caches.
//
// Strategy:
//   - All authenticated routes: Cache-Control: private, no-store
//     (CF must not cache these — they carry per-user data)
//   - Public read-only routes (billing plans, product catalog): Cache-Control: public
//     with a TTL appropriate to how often the data changes.
//   - All mutation routes (POST/PUT/PATCH/DELETE): Cache-Control: no-store
//     regardless of the route's other classification.
//
// CF Cache-Tag headers allow targeted cache purging when data changes.
// Add tags in individual handlers via SetCFCacheTags(c, "plans", "org:xyz").
package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/zap-proto/zip"
)

// CachePublic returns middleware that sets public cache headers with the given TTL.
//
// CF caches for ttl seconds (s-maxage). Browsers cache for ttl/2 seconds
// to ensure fresh content at browser re-visits. stale-while-revalidate
// allows CF to serve stale content while fetching fresh in background.
//
// Mutations (POST/PUT/PATCH/DELETE) are always no-store regardless.
func CachePublic(ttl int) zip.Handler {
	browserTTL := ttl / 2
	if browserTTL < 30 {
		browserTTL = 30
	}
	cc := fmt.Sprintf("public, max-age=%d, s-maxage=%d, stale-while-revalidate=60", browserTTL, ttl)
	cdnCC := fmt.Sprintf("max-age=%d", ttl)

	return func(c *zip.Ctx) error {
		switch c.Method() {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			c.SetHeader("Cache-Control", "no-store")
		default:
			c.SetHeader("Cache-Control", cc)
			c.SetHeader("CDN-Cache-Control", cdnCC)
			c.SetHeader("Vary", "Accept-Encoding")
		}
		return c.Next()
	}
}

// CachePublicTTL is CachePublic accepting a time.Duration.
func CachePublicTTL(ttl time.Duration) zip.Handler {
	return CachePublic(int(ttl.Seconds()))
}

// CachePrivate sets Cache-Control: private, no-store.
// Use on all authenticated per-user or per-org routes.
// CF will not cache these responses.
func CachePrivate() zip.Handler {
	return func(c *zip.Ctx) error {
		c.SetHeader("Cache-Control", "private, no-store")
		return c.Next()
	}
}

// CacheNoStore disables all caching unconditionally.
// Use on auth flows, checkout, and payment callbacks.
func CacheNoStore() zip.Handler {
	return func(c *zip.Ctx) error {
		c.SetHeader("Cache-Control", "no-store, no-cache, must-revalidate")
		c.SetHeader("Pragma", "no-cache")
		return c.Next()
	}
}

// SetCFCacheTags adds Cloudflare Cache-Tag header values to the response.
// Tags are used for targeted cache purging (e.g. purge all "plans" entries).
// Multiple calls accumulate; tags are comma-joined as CF requires.
//
// Example: SetCFCacheTags(c, "plans", "org:hanzo")
func SetCFCacheTags(c *zip.Ctx, tags ...string) {
	if len(tags) == 0 {
		return
	}
	if existing := string(c.Fiber().Response().Header.Peek("Cache-Tag")); existing != "" {
		tags = append([]string{existing}, tags...)
	}
	c.SetHeader("Cache-Tag", strings.Join(tags, ","))
}

// CFCacheTags returns middleware that sets Cache-Tag header(s).
// Use on route groups whose entries should be purgeable as a unit.
func CFCacheTags(tags ...string) zip.Handler {
	header := strings.Join(tags, ",")
	return func(c *zip.Ctx) error {
		// Accumulate with any previously set tags
		if existing := string(c.Fiber().Response().Header.Peek("Cache-Tag")); existing != "" {
			c.SetHeader("Cache-Tag", existing+","+header)
		} else {
			c.SetHeader("Cache-Tag", header)
		}
		return c.Next()
	}
}
