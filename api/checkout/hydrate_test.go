// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package checkout

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/secrets"
)

// tenancyStore answers orgs/<org>/… only to a call acting for <org>, as cloud's
// KMS does.
type tenancyStore map[string]string

func (s tenancyStore) GetSecret(ctx context.Context, ref string) ([]byte, error) {
	rest, ok := strings.CutPrefix(ref, "orgs/")
	if !ok {
		return nil, errors.New("not a tenant ref")
	}
	if org, _, _ := strings.Cut(rest, "/"); zip.CallerOf(ctx).Org != org {
		return nil, errors.New("refused")
	}
	if v, ok := s[ref]; ok {
		return []byte(v), nil
	}
	return nil, errors.New("no such secret")
}

// A raw checkout handler loads the merchant org's own credentials: the request's
// caller reaches the host's KMS through Forward, where the handler's Context
// carries none.
func TestAuthedOrgLoadsTheMerchantsOwnCredentials(t *testing.T) {
	prev := secrets.Get()
	secrets.Set(tenancyStore{"orgs/acme/square/SQUARE_SANDBOX_ACCESS_TOKEN": "sb-acme"})
	t.Cleanup(func() { secrets.Set(prev) })

	var got *organization.Organization
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Raw(http.MethodGet, "/probe", func(c *zip.Ctx) error {
		c.Locals("organization", &organization.Organization{Name: "acme"})
		got, _ = authedOrg(c)
		return c.JSON(http.StatusOK, map[string]bool{"ok": true})
	})
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.Header.Set(zip.HeaderOrg, "acme")
	req.Header.Set(zip.HeaderUser, "alice")
	if _, err := app.Test(req); err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Square.Sandbox.AccessToken != "sb-acme" {
		t.Fatalf("the merchant's own sandbox token did not load: %+v", got)
	}
}
