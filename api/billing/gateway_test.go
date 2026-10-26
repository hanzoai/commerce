// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package billing

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/gateway"
	"github.com/hanzoai/commerce/payment/providers/circle"
)

// The rail's logic is proven in billing/gateway; these pin the TRANSPORT: who
// the caller is, how a body is read, and that a refusal reaches the wire with
// its own status and reason.

func gatewayReply(t *testing.T, r *http.Response) map[string]string {
	t.Helper()
	raw, _ := io.ReadAll(r.Body)
	out := map[string]string{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("body %s: %v", raw, err)
	}
	return out
}

func TestGatewayRoutesRefuseAnonymousCallers(t *testing.T) {
	for _, tc := range []struct {
		method, pattern, path string
		h                     zip.Handler
	}{
		{http.MethodGet, "/v1/billing/crypto/gateway", "/v1/billing/crypto/gateway", GetBillingGatewayTerms},
		{http.MethodPost, "/v1/billing/crypto/gateway", "/v1/billing/crypto/gateway", SubmitBillingGateway},
		{http.MethodGet, "/v1/billing/crypto/gateway/:id", "/v1/billing/crypto/gateway/rem_00", GetBillingGateway},
	} {
		body := "{}"
		if tc.method == http.MethodGet {
			body = ""
		}
		resp := driveSeeded(nil, tc.pattern, httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(body)), tc.h)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s anonymous = %d, want 401", tc.method, tc.path, resp.StatusCode)
		}
	}
}

func TestGatewaySubmitReadsTheBodyStrictly(t *testing.T) {
	for _, body := range []string{
		`{"network":"eip155:8453","paymentPayload":{}}`, // an x402 envelope is not this body
		`not json`,
		``,
	} {
		resp := driveSeeded(nil, "/v1/billing/crypto/gateway",
			httptest.NewRequest(http.MethodPost, "/v1/billing/crypto/gateway", bytes.NewBufferString(body)), SubmitBillingGateway)
		if resp.StatusCode != http.StatusBadRequest || gatewayReply(t, resp)["reason"] != gateway.ReasonPayload {
			t.Errorf("body %q answered %d", body, resp.StatusCode)
		}
	}
}

func TestGatewayFailCarriesStatusAndReasonAndHidesInternals(t *testing.T) {
	refusal := func(err error) (int, map[string]string) {
		resp := driveSeeded(nil, "/x", httptest.NewRequest(http.MethodGet, "/x", nil), func(c *zip.Ctx) error { return gatewayFail(c, err) })
		return resp.StatusCode, gatewayReply(t, resp)
	}
	shut := gateway.New(circle.Config{}, nil)
	org := moneyOrg("acme")
	_, err := shut.Terms(t.Context(), org, "acme")
	if status, body := refusal(err); status != 503 || body["reason"] != gateway.ReasonClosed || body["error"] == "" {
		t.Fatalf("unarmed: %d %v", status, body)
	}
	if status, body := refusal(errors.New("db: secret detail")); status != 500 || body["error"] == "db: secret detail" || body["reason"] != "" {
		t.Fatalf("internal: %d %v — the cause must not reach the caller", status, body)
	}
}

func TestGatewayTreasuryIsSuperAdminOnly(t *testing.T) {
	resp := driveSeeded(nil, "/v1/commerce/treasury", httptest.NewRequest(http.MethodGet, "/v1/commerce/treasury", nil), GatewayTreasury)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous treasury = %d, want 401", resp.StatusCode)
	}
}

// Either rail alone is a menu; neither is the 503. And the deposit rail's own
// answer is untouched when the Gateway rail offers nothing.
func TestMenuJoinsTheTwoRails(t *testing.T) {
	down := errors.New("custody unreachable")
	if _, err := menu(nil, down, nil); err != down {
		t.Fatalf("no rail at all: %v, want the custody error", err)
	}
	m, err := menu(nil, down, []string{"eip155:8453"})
	if err != nil || len(m.Chains) != 0 || len(m.Tokens) != 0 || len(m.Assets) != 0 || len(m.Gateway) != 1 {
		t.Fatalf("gateway only: %+v %v", m, err)
	}
	m, err = menu([]CryptoAsset{{Chain: "base", Token: "usdc"}}, nil, nil)
	if err != nil || m.Gateway == nil || len(m.Gateway) != 0 || m.Chains[0] != "base" {
		t.Fatalf("deposit rail only: %+v %v", m, err)
	}
	b, _ := json.Marshal(m)
	if string(b) != `{"chains":["base"],"tokens":["usdc"],"assets":[{"chain":"base","token":"usdc"}],"gateway":[]}` {
		t.Fatalf("marshalled %s — every list is a list, never null", b)
	}
}
