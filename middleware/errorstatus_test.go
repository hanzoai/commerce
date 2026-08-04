package middleware

import (
	"net/http"
	"testing"

	"github.com/zap-proto/zip"
)

func do(t *testing.T, app *zip.App, target string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest("GET", target, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := app.Fiber().Test(req)
	if err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 512)
	n, _ := res.Body.Read(b)
	_ = res.Body.Close()
	return res, string(b[:n])
}

// A handler that returns an error has already said what it means: zip.ErrForbidden
// carries 403. The middleware must not flatten that to 500 — doing so is what made
// every unauthenticated /v1 read answer "server error" instead of "not allowed",
// and what let the status board show 12 broken services as green.
func TestReturnedErrorKeepsItsStatus(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	store := app.Group("/v1/store")
	store.Use(ErrorHandlerJSON())
	store.Get("/x", func(c *zip.Ctx) error {
		return zip.ErrForbidden("X-Org-Id required")
	})

	res, body := do(t, app, "/v1/store/x")
	if res.StatusCode != 403 {
		t.Fatalf("want 403, got %d: %s", res.StatusCode, body)
	}
	t.Logf("403 preserved: %s", body)
}

// A panic carries no status, so 500 is the honest answer and the process must live.
func TestPanicRecoversAs500(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	store := app.Group("/v1/store")
	store.Use(ErrorHandlerJSON())
	store.Get("/boom", func(c *zip.Ctx) error { panic("boom") })

	res, body := do(t, app, "/v1/store/boom")
	if res.StatusCode != 500 {
		t.Fatalf("want 500, got %d: %s", res.StatusCode, body)
	}
	t.Logf("panic recovered as 500: %s", body)
}
