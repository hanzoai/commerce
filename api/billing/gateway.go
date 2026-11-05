// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package billing

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/gateway"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/models/organization"
)

// The USDC authorization top-up rail over HTTP. The rail itself — terms,
// checks, relay, confirmation, credit — is billing/gateway, and these handlers only resolve the
// caller and translate; cloud reaches the same functions over its internal
// plane, so the two transports answer one payment one way.
//
// Who is credited is the caller (payer, the shared account rule); nothing in a
// body names it.

// gatewayFail answers a rail refusal with its own status, sentence and
// reason. An error that is not a refusal is an internal failure: its text is
// logged and never sent.
func gatewayFail(c *zip.Ctx, err error) error {
	status, reason, msg := gateway.StatusOf(err), gateway.ReasonOf(err), err.Error()
	switch {
	case reason == "":
		slog.Error("gateway: request failed", "method", c.Method(), "path", c.Path(), "err", err)
		msg = "the payment could not be recorded; present the same payment again"
	case errors.Unwrap(err) != nil:
		slog.Warn("gateway: refused", "method", c.Method(), "path", c.Path(), "status", status, "reason", reason, "cause", errors.Unwrap(err))
	}
	return c.JSON(status, map[string]string{"error": msg, "reason": reason})
}

// caller is who a request credits: the org it acts in and the billing subject
// it pays from, both from the verified identity. Either is empty when the
// request carries no identity, and the rail refuses that.
func caller(c *zip.Ctx) (*organization.Organization, string) {
	org, _ := middleware.GetOrganizationOK(c)
	return org, userBillingKey(c)
}

// GetBillingGatewayTerms answers the payment terms for a USDC top-up: the
// treasury to pay, the networks and USDC contracts, the EIP-712 domain and
// types to sign, the limits, and a nonce issued to this caller with its salt.
//
//	GET /v1/billing/crypto/gateway
func GetBillingGatewayTerms(c *zip.Ctx) error {
	org, payer := caller(c)
	t, err := gateway.Default().Terms(c.Context(), org, payer)
	if err != nil {
		return gatewayFail(c, err)
	}
	return c.JSON(http.StatusOK, t)
}

// SubmitBillingGateway takes a signed USDC payment and answers what became of
// it: 200 with its status — pending (relayed; read it back), confirming (on
// chain, not yet deep enough) or succeeded (credited) — or a refusal with its
// reason. Cloud's typed route answers the same 200, so the
// two transports answer one payment one way.
//
//	POST /v1/billing/crypto/gateway   { network, salt, authorization, signature }
func SubmitBillingGateway(c *zip.Ctx) error {
	var sub gateway.Submission
	dec := json.NewDecoder(bytes.NewReader(c.Body()))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&sub); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body: " + err.Error(), "reason": gateway.ReasonPayload})
	}
	org, payer := caller(c)
	v, err := gateway.Default().Submit(c.Context(), org, payer, sub)
	if err != nil {
		return gatewayFail(c, err)
	}
	return c.JSON(http.StatusOK, v)
}

// GetBillingGateway reads one of the caller's USDC top-ups back.
//
//	GET /v1/billing/crypto/gateway/:id
func GetBillingGateway(c *zip.Ctx) error {
	org, payer := caller(c)
	v, err := gateway.Default().Get(c.Context(), org, payer, c.Param("id"))
	if err != nil {
		return gatewayFail(c, err)
	}
	return c.JSON(http.StatusOK, v)
}

// GatewayTreasury reports the rail's position — the treasury's USDC and the
// relayer's gas per network, and the payments still open.
//
//	GET /v1/commerce/treasury
//
// SUPERADMIN ONLY, decided here rather than by where it is mounted, for the
// reason DepositWatcherStatus gives: it is the deployment's money, not a
// tenant's, and no org admin has a view of it to be given. Read-only.
func GatewayTreasury(c *zip.Ctx) error {
	if !iammiddleware.IsIAMAuthenticated(c) {
		return c.JSON(http.StatusUnauthorized, map[string]any{"error": "authentication required"})
	}
	if !iammiddleware.GetIAMClaims(c).IsSuperAdmin() {
		return c.JSON(http.StatusForbidden, map[string]any{"error": "superadmin role required"})
	}
	t, err := gateway.Default().Treasury(c.Context())
	if err != nil {
		return gatewayFail(c, err)
	}
	return c.JSON(http.StatusOK, t)
}
