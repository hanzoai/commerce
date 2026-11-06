package form

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"
)

// TestAFormTakesSubmissionsAndNoSubscriptions pins the retirement: a list
// subscription is cloud's POST /v1/marketing/subscribe, so the form routes
// answer submissions and nothing at /subscribe.
func TestAFormTakesSubmissionsAndNoSubscriptions(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	Route(app.Group(""))
	routes := map[string]bool{}
	for _, r := range app.Declaration().Routes {
		routes[r.Method+" "+r.Pattern] = true
	}
	for _, want := range []string{"POST /form/:formid/submit", "POST /mailinglist/:formid/submit"} {
		if !routes[want] {
			t.Errorf("%s is not routed", want)
		}
	}
	for _, gone := range []string{"/form/x/subscribe", "/mailinglist/x/subscribe"} {
		res, err := app.Test(httptest.NewRequest(http.MethodPost, gone, nil))
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != http.StatusNotFound && res.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("POST %s = %d, want it gone", gone, res.StatusCode)
		}
	}
}
