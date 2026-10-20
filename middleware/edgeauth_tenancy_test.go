// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
)

// signer stands up a JWKS for one RSA key and installs the IAM client EdgeAuth
// verifies against, so the verified-token branch runs as it does at a standalone
// edge. It returns a function that signs claims as IAM would.
func signer(t *testing.T) func(jwt.MapClaims) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwks := map[string]any{"keys": []map[string]string{{
		"kty": "RSA", "kid": "k", "use": "sig", "alg": "RS256",
		"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	}}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	t.Cleanup(srv.Close)
	if err := iammiddleware.Init(&auth.IAMConfig{Issuer: "https://hanzo.id", ClientID: "hanzo-commerce", JwksURI: srv.URL}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = iammiddleware.Init(nil) })
	return func(c jwt.MapClaims) string {
		c["iss"] = "https://hanzo.id"
		c["aud"] = "hanzo-commerce"
		c["exp"] = time.Now().Add(time.Hour).Unix()
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, c)
		tok.Header["kid"] = "k"
		s, err := tok.SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
}

// person is a user token as IAM mints it: `owner` is the org of the application
// the person signed in through, and `orgs` the signed membership set, home first.
func person(owner string, orgs ...[2]string) jwt.MapClaims {
	set := []map[string]string{}
	for _, m := range orgs {
		set = append(set, map[string]string{"org": m[0], "role": m[1]})
	}
	return jwt.MapClaims{"sub": "u-1", "name": "bob", "tokenType": "access-token", "owner": owner, "orgs": set}
}

var tenancyHeaders = []string{"X-Org-Id", "X-User-Owner", "X-User-IsAdmin", "X-User-IsOrgAdmin"}

// A person whose own org is "bob" signs in through an application the "hanzo" org
// owns (the brand app every customer uses), so IAM stamps owner=hanzo: the
// APPLICATION's org. The request acts in bob's org, never hanzo's.
func TestRed_EdgeAuth_OrgIsTheSubjectsNotTheApplications(t *testing.T) {
	t.Setenv("COMMERCE_EDGE_AUTH", "true")
	sign := signer(t)
	req := httptest.NewRequest("GET", "/v1/billing/balance?user=bob&currency=usd", nil)
	req.Header.Set("Authorization", "Bearer "+sign(person("hanzo", [2]string{"bob", "owner"})))
	_, h := runEdgeAuth(t, req, tenancyHeaders...)
	if h["X-User-Owner"] != "bob" {
		t.Fatalf("X-User-Owner = %q, want bob", h["X-User-Owner"])
	}
	if h["X-Org-Id"] != "bob" {
		t.Fatalf("X-Org-Id = %q: the request acts in the org of the app the person signed in through, not the person's own (bob)", h["X-Org-Id"])
	}
	if h["X-User-IsOrgAdmin"] != "true" {
		t.Fatalf("X-User-IsOrgAdmin = %q: bob owns bob", h["X-User-IsOrgAdmin"])
	}
}

// The billing subject is pinned to the org the request acts in, in the query and in
// a write's body, so a person who signed in through the brand app reads and writes
// their own billing and never the brand's.
func TestEdgeAuth_BillingSubjectIsTheSubjectsOrg(t *testing.T) {
	t.Setenv("COMMERCE_EDGE_AUTH", "true")
	sign := signer(t)
	tok := sign(person("hanzo", [2]string{"bob", "owner"}))

	read := httptest.NewRequest("GET", "/v1/billing/balance?user=hanzo&currency=usd", nil)
	read.Header.Set("Authorization", "Bearer "+tok)
	var query string
	_, h := runEdgeAuthWith(t, read, func(q, _ string) { query = q }, "X-Org-Id")
	if h["X-Org-Id"] != "bob" {
		t.Fatalf("billing namespace X-Org-Id = %q, want bob", h["X-Org-Id"])
	}
	if q, _ := url.ParseQuery(query); q.Get("user") != "bob" {
		t.Fatalf("billing read subject user = %q, want bob (query %q)", q.Get("user"), query)
	}

	write := httptest.NewRequest("POST", "/v1/billing/methods", strings.NewReader(`{"customerId":"hanzo","brand":"visa"}`))
	write.Header.Set("Content-Type", "application/json")
	write.Header.Set("Authorization", "Bearer "+tok)
	var body string
	runEdgeAuthWith(t, write, func(_, b string) { body = b })
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("body %q: %v", body, err)
	}
	if m["customerId"] != "bob" {
		t.Fatalf("billing write subject customerId = %v, want bob", m["customerId"])
	}
}

// The org a request acts in is the selection the signed set admits, by the rule
// cloud's boundary applies (authz.Claims.EffectiveOrg): any org for a SuperAdmin,
// an org the set opens for anyone else, and otherwise the default. The `owner`
// claim decides none of it, and a program acts in its own application's org.
func TestEdgeAuth_TheOrgIsTheSignedSetsSelection(t *testing.T) {
	t.Setenv("COMMERCE_EDGE_AUTH", "true")
	sign := signer(t)

	program := jwt.MapClaims{"sub": "app-1", "tokenType": "access-token", "type": "application", "owner": "acme"}
	cases := []struct {
		name   string
		claims jwt.MapClaims
		path   string
		sel    string
		want   map[string]string
	}{
		{"default is the home", person("hanzo", [2]string{"bob", "owner"}, [2]string{"acme", "member"}), "/v1/order", "",
			map[string]string{"X-Org-Id": "bob", "X-User-Owner": "bob", "X-User-IsOrgAdmin": "true"}},
		{"a member selects an org of its set", person("hanzo", [2]string{"bob", "owner"}, [2]string{"acme", "member"}), "/v1/order", "acme",
			map[string]string{"X-Org-Id": "acme", "X-User-Owner": "bob"}},
		{"an org outside the set is not granted", person("hanzo", [2]string{"bob", "owner"}), "/v1/order", "victim",
			map[string]string{"X-Org-Id": "bob", "X-User-Owner": "bob", "X-User-IsOrgAdmin": "true"}},
		{"the app's org is not selectable by its users", person("hanzo", [2]string{"bob", "owner"}), "/v1/order", "hanzo",
			map[string]string{"X-Org-Id": "bob", "X-User-Owner": "bob", "X-User-IsOrgAdmin": "true"}},
		{"a directory customer acts in their personal org", person("hanzo", [2]string{"hanzo-id", "member"}, [2]string{"bob", "owner"}), "/v1/order", "",
			map[string]string{"X-Org-Id": "bob", "X-User-Owner": "hanzo-id", "X-User-IsOrgAdmin": "true"}},
		{"a plain member of a brand org selects none of it", person("hanzo", [2]string{"hanzo-id", "member"}, [2]string{"bob", "owner"}, [2]string{"hanzo", "member"}), "/v1/order", "hanzo",
			map[string]string{"X-Org-Id": "bob", "X-User-Owner": "hanzo-id", "X-User-IsOrgAdmin": "true"}},
		{"a membership of the admin org confers nothing", person("hanzo", [2]string{"bob", "owner"}, [2]string{"admin", "admin"}), "/v1/order", "admin",
			map[string]string{"X-Org-Id": "bob", "X-User-Owner": "bob", "X-User-IsOrgAdmin": "true"}},
		{"a SuperAdmin selects any org", person("admin", [2]string{"admin", "owner"}), "/v1/order", "acme",
			map[string]string{"X-Org-Id": "acme", "X-User-Owner": "admin", "X-User-IsAdmin": "true"}},
		{"a SuperAdmin's billing view follows ?org", person("admin", [2]string{"admin", "owner"}), "/v1/billing/balance?org=acme", "",
			map[string]string{"X-Org-Id": "acme", "X-User-Owner": "admin", "X-User-IsAdmin": "true"}},
		{"a malformed ?org selects nothing", person("admin", [2]string{"admin", "owner"}), "/v1/billing/balance?org=hanzo%2Fevil", "",
			map[string]string{"X-Org-Id": "admin", "X-User-Owner": "admin", "X-User-IsAdmin": "true", "X-User-IsOrgAdmin": "true"}},
		{"?org is not folded into another org's name", person("admin", [2]string{"admin", "owner"}), "/v1/billing/balance?org=Acme", "",
			map[string]string{"X-Org-Id": "admin", "X-User-Owner": "admin", "X-User-IsAdmin": "true", "X-User-IsOrgAdmin": "true"}},
		{"a member's ?org selects an org of its set", person("hanzo", [2]string{"bob", "owner"}, [2]string{"acme", "member"}), "/v1/billing/balance?org=acme", "",
			map[string]string{"X-Org-Id": "acme", "X-User-Owner": "bob"}},
		{"anyone else's ?org outside the set is ignored", person("hanzo", [2]string{"bob", "owner"}), "/v1/billing/balance?org=acme", "",
			map[string]string{"X-Org-Id": "bob", "X-User-Owner": "bob", "X-User-IsOrgAdmin": "true"}},
		{"an org whose owner claim is admin is not a SuperAdmin", person("admin", [2]string{"bob", "owner"}), "/v1/order", "acme",
			map[string]string{"X-Org-Id": "bob", "X-User-Owner": "bob", "X-User-IsOrgAdmin": "true"}},
		{"a program acts in its application's org and selects nothing", program, "/v1/order", "victim",
			map[string]string{"X-Org-Id": "acme", "X-User-Owner": "acme"}},
		{"a person token without a membership set acts in no org", jwt.MapClaims{"sub": "u-1", "tokenType": "access-token", "owner": "hanzo"}, "/v1/order", "hanzo",
			map[string]string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			req.Header.Set("Authorization", "Bearer "+sign(tc.claims))
			if tc.sel != "" {
				req.Header.Set("X-Org-Id", tc.sel)
			}
			var query string
			_, h := runEdgeAuthWith(t, req, func(q, _ string) { query = q }, tenancyHeaders...)
			for _, k := range tenancyHeaders {
				if h[k] != tc.want[k] {
					t.Errorf("%s = %q, want %q", k, h[k], tc.want[k])
				}
			}
			if q, _ := url.ParseQuery(query); q.Has("org") {
				t.Errorf("?org reached the handler: %q", query)
			}
		})
	}
}

// runEdgeAuthWith is runEdgeAuth that also hands the probe's view of the query
// string and body to see, so a test reads the subject the handler would bind.
func runEdgeAuthWith(t *testing.T, req *http.Request, see func(query, body string), captureHeaders ...string) (bool, map[string]string) {
	t.Helper()
	headers := map[string]string{}
	reached := false
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Use(EdgeAuth())
	app.Raw(zip.MethodAll, "/*", func(c *zip.Ctx) error {
		reached = true
		r := c.Fiber().Request()
		see(string(r.URI().QueryString()), string(r.Body()))
		for _, k := range captureHeaders {
			headers[k] = c.Header(k)
		}
		return c.NoContent(http.StatusOK)
	})
	if _, err := app.Test(req); err != nil {
		t.Fatalf("edgeauth request: %v", err)
	}
	return reached, headers
}
