package checkout

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/store"
)

// Which rails a deployment offers is now a thing an admin can change, so these
// pin the two ways changing it could go wrong: writing someone else's tenant,
// and destroying the credentials of the rails the call was not about.

// putRail runs PUT /_/commerce/providers/{name} against a seeded tenant and
// returns the status, the body, and the store so a caller can read back what was
// actually persisted — the response is a projection, and a projection cannot
// prove what happened to the fields it strips.
func putRail(t *testing.T, claims *auth.IAMClaims, name, body string) (int, string, *store.Tenant) {
	t.Helper()
	s := newHandlerStore(t)
	tenant := seedTenant(t, s, "maxpower", "pay.maxpower.test")
	router := newRouterWithClaims(s, claims)
	req := httptest.NewRequest(http.MethodPut, "/_/commerce/providers/"+name, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	code, respBody, _ := doReq(t, router, req)
	stored, err := s.Tenants.FindByID(tenant.ID)
	if err != nil {
		t.Fatalf("read back tenant: %v", err)
	}
	return code, string(respBody), stored
}

// TestSetProviderEnabled_TurnsARailOn is the acceptance criterion: a rail the
// tenant did not carry can be switched on, and the pay SPA reads exactly this
// list to decide what to offer.
func TestSetProviderEnabled_TurnsARailOn(t *testing.T) {
	claims := &auth.IAMClaims{Owner: "maxpower", IsAdmin: true}
	claims.Subject = "dave"
	code, body, got := putRail(t, claims, "crypto", `{"enabled":true}`)
	if code != http.StatusOK {
		t.Fatalf("enable crypto: status=%d want 200; body=%s", code, body)
	}
	if !railEnabled(got.Providers, "crypto") {
		t.Fatalf("crypto did not persist as enabled: %+v", got.Providers)
	}
	if !strings.Contains(body, `"crypto"`) {
		t.Fatalf("the response must show the new list, got %s", body)
	}
}

// TestSetProviderEnabled_PreservesOtherRailsCredentials is the reason this verb
// takes ONE rail. A Provider carries the KMS path naming where its credentials
// live, and the read projection strips it; a whole-list PUT built from what an
// admin UI can see would write every rail back with an empty path and silently
// disconnect all of them from their secrets. Toggling crypto must leave square's
// path exactly as it was.
func TestSetProviderEnabled_PreservesOtherRailsCredentials(t *testing.T) {
	claims := &auth.IAMClaims{Owner: "maxpower", IsAdmin: true}
	claims.Subject = "dave"
	_, _, got := putRail(t, claims, "crypto", `{"enabled":true}`)
	for _, p := range got.Providers {
		if strings.EqualFold(p.Name, "square") {
			if p.KMSPath != "kms/commerce/maxpower/square" {
				t.Fatalf("square lost its credential path: %q", p.KMSPath)
			}
			if !p.Enabled {
				t.Fatal("square was disabled by a call about crypto")
			}
			return
		}
	}
	t.Fatal("square disappeared from the provider list")
}

// TestSetProviderEnabled_Unauthenticated401 — not signed in is 401, so a browser
// re-authenticates instead of reporting a permission error.
func TestSetProviderEnabled_Unauthenticated401(t *testing.T) {
	code, body, _ := putRail(t, nil, "crypto", `{"enabled":true}`)
	if code != http.StatusUnauthorized {
		t.Fatalf("anonymous: status=%d want 401; body=%s", code, body)
	}
}

// TestSetProviderEnabled_NonAdmin403 — authenticated but with no admin signal.
func TestSetProviderEnabled_NonAdmin403(t *testing.T) {
	claims := &auth.IAMClaims{Owner: "maxpower"}
	claims.Subject = "nobody"
	code, body, _ := putRail(t, claims, "crypto", `{"enabled":true}`)
	if code != http.StatusForbidden {
		t.Fatalf("plain user: status=%d want 403; body=%s", code, body)
	}
}

// TestSetProviderEnabled_UnknownRailRefused — a name nothing downstream reads
// would be config that reports success and changes nothing a customer can see.
func TestSetProviderEnabled_UnknownRailRefused(t *testing.T) {
	claims := &auth.IAMClaims{Owner: "maxpower", IsAdmin: true}
	claims.Subject = "dave"
	code, body, got := putRail(t, claims, "dogecoin-direct", `{"enabled":true}`)
	if code != http.StatusBadRequest {
		t.Fatalf("unknown rail: status=%d want 400; body=%s", code, body)
	}
	for _, p := range got.Providers {
		if strings.EqualFold(p.Name, "dogecoin-direct") {
			t.Fatal("a refused rail must not be persisted")
		}
	}
}

// TestSetProviderEnabled_MissingFieldRefused — `false` is a real instruction, so
// an absent `enabled` cannot be read as one. Binding into a bool would make a
// malformed body silently disable a rail.
func TestSetProviderEnabled_MissingFieldRefused(t *testing.T) {
	claims := &auth.IAMClaims{Owner: "maxpower", IsAdmin: true}
	claims.Subject = "dave"
	code, body, got := putRail(t, claims, "square", `{}`)
	if code != http.StatusBadRequest {
		t.Fatalf("missing enabled: status=%d want 400; body=%s", code, body)
	}
	if !railEnabled(got.Providers, "square") {
		t.Fatal("a malformed body must not disable a live rail")
	}
}

// TestSetProviderEnabled_TurnsARailOff — the other direction, and idempotent.
func TestSetProviderEnabled_TurnsARailOff(t *testing.T) {
	claims := &auth.IAMClaims{Owner: "maxpower", IsAdmin: true}
	claims.Subject = "dave"
	code, body, got := putRail(t, claims, "square", `{"enabled":false}`)
	if code != http.StatusOK {
		t.Fatalf("disable square: status=%d want 200; body=%s", code, body)
	}
	if railEnabled(got.Providers, "square") {
		t.Fatal("square is still enabled")
	}
}

func railEnabled(providers []store.Provider, name string) bool {
	for _, p := range providers {
		if strings.EqualFold(p.Name, name) {
			return p.Enabled
		}
	}
	return false
}
