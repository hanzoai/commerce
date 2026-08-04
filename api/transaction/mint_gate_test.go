// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package transaction

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/test/ae"
)

// invokeCreate drives the transaction Create handler with a real org + datastore
// context and a caller identity, then returns the response + its body. It
// reproduces the production reach of POST /v1/transaction (adminRequired route
// middleware + handler); the identity closure seeds whichever principal the case
// models via the request locals, exactly as the auth chain does.
func invokeCreate(org *organization.Organization, ctx context.Context, identity func(*zip.Ctx), body string) (*http.Response, string) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Post("/v1/transaction", func(c *zip.Ctx) error {
		c.Locals("organization", org)
		c.SetContext(ctx)
		identity(c)
		return c.Next()
	}, Create)

	req := httptest.NewRequest(http.MethodPost, "/v1/transaction", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		panic(err)
	}
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

// orgAdmin is the C1 adversary: an org OWNER (org-level isAdmin, permissions
// Admin|Live) who is NOT a platform global admin and NOT the internal service
// token. This is exactly the principal that must be able to move its own funds
// (withdraw/transfer) but NOT mint spendable balance.
func orgAdmin(c *zip.Ctx) {
	c.Locals("permissions", bit.Field(permission.Admin|permission.Live))
	c.Locals("iam_authenticated", true)
	c.Locals("iam_claims", &auth.IAMClaims{Owner: "acme", IsAdmin: true})
}

// mintPrincipal is the internal service token (cloud-api → commerce): carries the
// Admin permission AND the verified service-token marker → MayMintMoney true.
func mintPrincipal(c *zip.Ctx) {
	c.Locals("permissions", bit.Field(permission.Admin|permission.Live))
	c.Locals("service_token_verified", true)
}

func moneyOrg(name string) *organization.Organization {
	o := &organization.Organization{}
	o.Name = name
	o.Live = true
	return o
}

// TestCreate_OrgAdminDepositToIAMUser_Denied is C1-b: the exact PoC — an org
// admin POSTing a $1,000,000 deposit into the gateway-spendable IAM-user wallet
// must be REFUSED (403), NOT minted. Before the fix this returned 201.
func TestCreate_OrgAdminDepositToIAMUser_Denied(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("acme")

	body := `{"type":"deposit","destinationId":"acme/attacker","destinationKind":"iam-user","amount":100000000,"currency":"usd"}`
	resp, respBody := invokeCreate(org, ctx, orgAdmin, body)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("org-admin $1M iam-user deposit: status=%d body=%s, want 403 (no self-mint)", resp.StatusCode, respBody)
	}
}

// TestCreate_OrgAdminIAMUserTransfer_Denied — the forged-Transfer variant into
// the gateway wallet is likewise refused for an org admin.
func TestCreate_OrgAdminIAMUserTransfer_Denied(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("acme")

	body := `{"type":"transfer","sourceId":"x","sourceKind":"x","destinationId":"acme/attacker","destinationKind":"iam-user","amount":5000,"currency":"usd"}`
	resp, respBody := invokeCreate(org, ctx, orgAdmin, body)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("org-admin iam-user transfer: status=%d body=%s, want 403", resp.StatusCode, respBody)
	}
}

// TestCreate_MintPrincipalDeposit_Allowed — control: the internal service token
// (the legitimate cloud-api money path) still mints (201). The gate narrows WHO
// may mint; it does not break the authorized path.
func TestCreate_MintPrincipalDeposit_Allowed(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("acme")

	body := `{"type":"deposit","destinationId":"acme/user","destinationKind":"iam-user","amount":5000,"currency":"usd"}`
	resp, respBody := invokeCreate(org, ctx, mintPrincipal, body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("service-token iam-user deposit: status=%d body=%s, want 201 (authorized mint path intact)", resp.StatusCode, respBody)
	}
}

// TestCreate_OrgAdminWithdraw_Allowed — control: a Withdraw (moving the org's OWN
// funds out) is NOT a mint, so an org admin is still allowed (the gate is precise,
// not a blanket lock on the endpoint).
func TestCreate_OrgAdminWithdraw_Allowed(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("acme")

	// Seed a balance to withdraw from (ungated internal write — allowed).
	seed := `{"type":"deposit","destinationId":"acme/spender","destinationKind":"iam-user","amount":5000,"currency":"usd"}`
	if resp, respBody := invokeCreate(org, ctx, mintPrincipal, seed); resp.StatusCode != http.StatusCreated {
		t.Fatalf("seed deposit: status=%d body=%s, want 201", resp.StatusCode, respBody)
	}

	body := `{"type":"withdraw","sourceId":"acme/spender","sourceKind":"iam-user","amount":1000,"currency":"usd"}`
	resp, respBody := invokeCreate(org, ctx, orgAdmin, body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("org-admin withdraw of own funds: status=%d body=%s, want 201 (not a mint)", resp.StatusCode, respBody)
	}
}
