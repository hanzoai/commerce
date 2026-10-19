// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The standalone reader speaks the KMS REST surface as one machine identity: the
// org's ref orgs/<org>/<path>/<name> is GET /v1/kms/secrets/<path>/<name> with the
// identity's bearer, and a ref under any other org never leaves the process.
func TestOrgReaderReadsItsOwnOrg(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/iam/oauth/token":
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "expires_in": 3600})
		default:
			got = append(got, r.URL.Path+"?"+r.URL.RawQuery)
			if r.Header.Get("Authorization") != "Bearer tok" {
				http.Error(w, "no bearer", http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"secret": map[string]string{"value": "LOC-ACME"}})
		}
	}))
	defer srv.Close()
	t.Setenv("KMS_URL", srv.URL)
	t.Setenv("HANZO_IAM_URL", srv.URL)
	t.Setenv("KMS_CLIENT_ID", "acme-commerce")
	t.Setenv("KMS_CLIENT_SECRET", "s")
	t.Setenv("KMS_ORG", "acme")

	rd, err := secretReader()
	if err != nil || rd == nil {
		t.Fatalf("secretReader: %v", err)
	}
	v, err := rd.GetSecret(context.Background(), "orgs/acme/square/SQUARE_SANDBOX_LOCATION_ID")
	if err != nil || string(v) != "LOC-ACME" {
		t.Fatalf("read = %q, %v", v, err)
	}
	if len(got) != 1 || got[0] != "/v1/kms/secrets/square/SQUARE_SANDBOX_LOCATION_ID?env=default" {
		t.Fatalf("asked %v, want the org-relative REST address in the default env", got)
	}
	if _, err := rd.GetSecret(context.Background(), "orgs/other/square/SQUARE_SANDBOX_LOCATION_ID"); err == nil {
		t.Fatal("a ref under another org was read")
	}
	if len(got) != 1 {
		t.Fatalf("a foreign ref reached the server: %v", got)
	}
}

func TestSecretReaderConfiguration(t *testing.T) {
	t.Setenv("KMS_URL", "")
	if rd, err := secretReader(); rd != nil || err != nil {
		t.Fatalf("no KMS_URL: reader %v err %v, want none and no error", rd, err)
	}
	t.Setenv("KMS_URL", "https://kms.example.test")
	t.Setenv("KMS_ORG", "")
	if _, err := secretReader(); err == nil {
		t.Fatal("KMS_URL without KMS_ORG booted")
	}
	t.Setenv("KMS_ORG", "acme")
	t.Setenv("KMS_CLIENT_ID", "")
	if _, err := secretReader(); err == nil {
		t.Fatal("KMS_URL without a client identity booted")
	}
}
