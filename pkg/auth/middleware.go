// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

// Package auth wires gateway-supplied identity headers (X-Org-Id,
// X-User-Id, X-User-Email) into request context. commerced does not
// validate JWTs itself — hanzoai/gateway already did that. The
// middleware is the trust boundary: in production, only the gateway
// can reach commerced, and COMMERCED_REQUIRE_IDENTITY=true rejects any
// request without identity headers.
package auth

import (
	"context"
	"net/http"

	"github.com/zap-proto/zip"
)

// Header names — vendor-free X-* convention (see /Users/z/work/hanzo/CLAUDE.md).
const (
	HeaderOrgID     = "X-Org-Id"
	HeaderUserID    = "X-User-Id"
	HeaderUserEmail = "X-User-Email"
)

type ctxKey int

const (
	ctxKeyOrgID ctxKey = iota
	ctxKeyUserID
	ctxKeyUserEmail
)

// RequireIdentity reads identity headers and attaches them to ctx.
// When require=true and no headers are present, responds 401. When
// require=false, missing headers yield empty ctx values — the
// embedded/dev path.
func RequireIdentity(require bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			org := r.Header.Get(HeaderOrgID)
			user := r.Header.Get(HeaderUserID)
			email := r.Header.Get(HeaderUserEmail)

			if require && org == "" && user == "" {
				http.Error(w, `{"error":"identity required","code":401}`, http.StatusUnauthorized)
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, ctxKeyOrgID, org)
			ctx = context.WithValue(ctx, ctxKeyUserID, user)
			ctx = context.WithValue(ctx, ctxKeyUserEmail, email)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Identity returns a zip middleware that mirrors RequireIdentity. Used by
// server.go to gate the /v1/commerce and /_/commerce groups.
func Identity(require bool) zip.Handler {
	return func(c *zip.Ctx) error {
		org := c.Header(HeaderOrgID)
		user := c.Header(HeaderUserID)
		email := c.Header(HeaderUserEmail)

		if require && org == "" && user == "" {
			return c.JSON(http.StatusUnauthorized, map[string]any{
				"error": "identity required",
				"code":  401,
			})
		}

		ctx := c.Context()
		ctx = context.WithValue(ctx, ctxKeyOrgID, org)
		ctx = context.WithValue(ctx, ctxKeyUserID, user)
		ctx = context.WithValue(ctx, ctxKeyUserEmail, email)
		c.Fiber().SetContext(ctx)

		// Mirror onto request locals for legacy handlers that read from c.Locals.
		if org != "" {
			c.Locals("iam_org", org)
			c.Locals("iam_authenticated", true)
		}
		if user != "" {
			c.Locals("iam_user_id", user)
		}
		if email != "" {
			c.Locals("iam_email", email)
		}
		return c.Next()
	}
}

// OrgID returns the org id attached by RequireIdentity, or "".
func OrgID(ctx context.Context) string { return strFromCtx(ctx, ctxKeyOrgID) }

// UserID returns the user id attached by RequireIdentity, or "".
func UserID(ctx context.Context) string { return strFromCtx(ctx, ctxKeyUserID) }

// UserEmail returns the user email attached by RequireIdentity, or "".
func UserEmail(ctx context.Context) string { return strFromCtx(ctx, ctxKeyUserEmail) }

func strFromCtx(ctx context.Context, k ctxKey) string {
	if v, ok := ctx.Value(k).(string); ok {
		return v
	}
	return ""
}
