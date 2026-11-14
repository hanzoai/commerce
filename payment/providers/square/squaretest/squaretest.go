// Package squaretest stands in for Square's locations endpoint, so a test of what
// Square is offered for describes the MERCHANT'S location (its country, currency and
// capabilities) the way Square does.
package squaretest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Location serves GET /v2/locations/{id} for one active location in country and
// currency, holding the given capabilities, and returns the server's address for
// square.Config.Endpoint. Pass no capabilities for a location that cannot take cards.
func Location(t *testing.T, country, currency string, capabilities ...string) string {
	t.Helper()
	caps := make([]string, len(capabilities))
	for i, c := range capabilities {
		caps[i] = fmt.Sprintf("%q", c)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v2/locations" {
			fmt.Fprintf(w, `{"locations":[{"id":"sq-loc","name":"Test","country":%q,"currency":%q,"status":"ACTIVE","capabilities":[%s]}]}`, country, currency, strings.Join(caps, ","))
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/v2/locations/") {
			http.NotFound(w, r)
			return
		}
		fmt.Fprintf(w, `{"location":{"id":%q,"country":%q,"currency":%q,"status":"ACTIVE","capabilities":[%s]}}`,
			strings.TrimPrefix(r.URL.Path, "/v2/locations/"), country, currency, strings.Join(caps, ","))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// Cards is Location with card processing.
func Cards(t *testing.T, country, currency string) string {
	t.Helper()
	return Location(t, country, currency, "CREDIT_CARD_PROCESSING")
}

// Everywhere makes every Square API origin answer the locations lookup with one
// active location in country and currency holding card processing, for tests that
// build their providers from an org's own credentials and have no Endpoint to name.
// Any other request is passed on unchanged.
func Everywhere(t *testing.T, country, currency string) {
	t.Helper()
	prev := http.DefaultTransport
	http.DefaultTransport = roundTrip(func(r *http.Request) (*http.Response, error) {
		if (r.URL.Host == "connect.squareup.com" || r.URL.Host == "connect.squareupsandbox.com") && strings.HasPrefix(r.URL.Path, "/v2/locations/") {
			rec := httptest.NewRecorder()
			rec.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(rec, `{"location":{"id":%q,"country":%q,"currency":%q,"status":"ACTIVE","capabilities":["CREDIT_CARD_PROCESSING"]}}`,
				strings.TrimPrefix(r.URL.Path, "/v2/locations/"), country, currency)
			return rec.Result(), nil
		}
		return prev.RoundTrip(r)
	})
	t.Cleanup(func() { http.DefaultTransport = prev })
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
