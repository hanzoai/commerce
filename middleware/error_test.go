// Copyright © 2026 Hanzo AI. MIT License.

package middleware

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/config"
)

// driveErrorRenderer runs the production JSON error middleware (ErrorHandlerJSON)
// in front of a handler, drives one GET /x, and returns exactly what a client of
// /v1/billing/* sees: the rendered HTTP status and body.
func driveErrorRenderer(t *testing.T, handler zip.Handler) (int, string) {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Use(ErrorHandlerJSON())
	app.Get("/x", handler)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

// returns wraps an error as a zip handler that returns it — the idiomatic way a
// commerce handler surfaces a typed rejection (e.g. billing's ErrForbidden).
func returns(err error) zip.Handler {
	return func(c *zip.Ctx) error { return err }
}

// TestErrorHandler_TypedErrorStatusTaxonomy pins the contract: the error's KIND
// is a value, and the HTTP status is a pure function of that value derived in the
// ONE renderer. A typed *zip.HTTPError renders with the status it carries;
// a plain error — or a typed 500 — stays a generic, non-leaking 500.
//
// This is the regression guard for the defect where every returned error
// collapsed to 500, mislabeling a signed-out billing rejection (403) as an
// outage.
func TestErrorHandler_TypedErrorStatusTaxonomy(t *testing.T) {
	// Pin production posture so the generic-500 body is deterministic: dev mode
	// echoes the error text, production hides it (the leak-safety property below).
	prev := config.IsDevelopment
	config.IsDevelopment = false
	t.Cleanup(func() { config.IsDevelopment = prev })

	cases := []struct {
		name       string
		handler    zip.Handler
		wantStatus int
		bodyHas    string // substring that MUST be present
		bodyHasNot string // substring that MUST be absent (leak-safety)
	}{
		{
			name:       "ErrForbidden -> 403, safe message surfaced",
			handler:    returns(zip.ErrForbidden("sign in to view billing")),
			wantStatus: http.StatusForbidden,
			bodyHas:    "sign in to view billing",
		},
		{
			name:       "ErrUnauthorized -> 401",
			handler:    returns(zip.ErrUnauthorized("no access token provided")),
			wantStatus: http.StatusUnauthorized,
			bodyHas:    "no access token provided",
		},
		{
			name:       "ErrNotFound -> 404",
			handler:    returns(zip.ErrNotFound("no such invoice")),
			wantStatus: http.StatusNotFound,
			bodyHas:    "no such invoice",
		},
		{
			name:       "Errorf(409) -> any typed status is honored",
			handler:    returns(zip.Errorf(http.StatusConflict, "already exists")),
			wantStatus: http.StatusConflict,
			bodyHas:    "already exists",
		},
		{
			name:       "wrapped ErrForbidden -> 403 (errors.As unwraps)",
			handler:    returns(fmt.Errorf("billing gate: %w", zip.ErrForbidden("sign in"))),
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "plain error -> 500, internals hidden",
			handler:    returns(errors.New("dial tcp: connection refused to secret-db:5432")),
			wantStatus: http.StatusInternalServerError,
			bodyHas:    "Unable to process request",
			bodyHasNot: "secret-db",
		},
		{
			name:       "typed 500 stays a generic 500 (no leak)",
			handler:    returns(zip.Errorf(http.StatusInternalServerError, "nil map write in ledger")),
			wantStatus: http.StatusInternalServerError,
			bodyHas:    "Unable to process request",
			bodyHasNot: "ledger",
		},
		{
			name: "happy path (handler renders, no error) -> untouched",
			handler: func(c *zip.Ctx) error {
				return c.Bytes(http.StatusOK, []byte(`{"balance":12345}`))
			},
			wantStatus: http.StatusOK,
			bodyHas:    `"balance":12345`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := driveErrorRenderer(t, tc.handler)
			if status != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", status, tc.wantStatus, body)
			}
			if tc.bodyHas != "" && !strings.Contains(body, tc.bodyHas) {
				t.Fatalf("body = %q, want it to contain %q", body, tc.bodyHas)
			}
			if tc.bodyHasNot != "" && strings.Contains(body, tc.bodyHasNot) {
				t.Fatalf("body = %q leaked %q — internal detail must not reach the client", body, tc.bodyHasNot)
			}
		})
	}
}
