package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"
)

// A downstream subsystem sharing the /v1 prefix returns a TYPED HTTP error
// (zip.HTTPError). Commerce's ErrorHandlerJSON must NOT flatten it to 500 — it
// must preserve the real status (co-residence: platform/provisioning 401/403/404
// were being re-rendered as 500, discarding status). This is the regression gate.
func TestErrorHandlerJSON_PreservesTypedStatus(t *testing.T) {
	cases := []struct {
		name string
		err  *zip.HTTPError
	}{
		{"unauthorized", zip.ErrUnauthorized("X-Org-Id required")},
		{"forbidden", zip.ErrForbidden("stranger")},
		{"not found", zip.ErrNotFound("nope")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := zip.New(zip.Config{DisableStartupMessage: true})
			app.Use(ErrorHandlerJSON())
			app.Get("/x", func(c *zip.Ctx) error { return tc.err })

			resp, err := app.Fiber().Test(httptest.NewRequest(http.MethodGet, "/x", nil))
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != tc.err.Status {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("status = %d, want %d (typed error clobbered) body=%s",
					resp.StatusCode, tc.err.Status, body)
			}
		})
	}
}

// A genuinely UNTYPED downstream failure still gets commerce's friendly 500
// envelope — the generic-error behavior is unchanged.
func TestErrorHandlerJSON_UntypedIs500(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Use(ErrorHandlerJSON())
	app.Get("/boom", func(c *zip.Ctx) error { return io.ErrUnexpectedEOF })

	resp, err := app.Fiber().Test(httptest.NewRequest(http.MethodGet, "/boom", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("untyped error status = %d, want 500", resp.StatusCode)
	}
}
