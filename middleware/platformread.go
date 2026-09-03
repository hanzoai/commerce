// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package middleware

import (
	"errors"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/util/json/http"
)

// MayReadPlatform is THE single predicate for "may this caller READ cross-org
// PLATFORM (god-view) data" — the business metrics, vendor costs, and any other
// aggregate that spans every tenant. It admits exactly the principals a
// platform god-view trusts, and fails closed for everyone else:
//
// a Hanzo PLATFORM SuperAdmin — auth.IAMClaims.IsSuperAdmin(): home owner=="admin",
// the reserved admin org, from the gateway/EdgeAuth X-User-Owner header or the
// validated JWT `owner`. The platform's own scheduled work reaches this the same
// way, as an application IAM minted under the admin org (client_credentials), so
// the one predicate covers a person and a machine and there is no second door.
//
// Two admissions this used to have are gone. A shared COMMERCE_SERVICE_TOKEN,
// compared against a bearer here, was commerce authenticating callers on its own
// with a secret IAM had never seen. And "the Admin permission bit with no IAM
// Subject", which was meant to spot that token structurally, admitted exactly the
// thing it was argued not to — a legacy per-org access token holding Admin has
// no Subject either — so an org owner's old token could read the fleet's revenue.
//
// It deliberately does NOT admit the org-level Admin bit (permission.Admin) held
// by an org OWNER: the gateway mints permission.Admin from a tenant's own
// IsAdmin (edgeauth.permsHeader), so gating cross-org reads on that bit would let
// ANY org owner read the whole fleet's revenue. Only a SuperAdmin may. This is
// the same org-admin-vs-global-admin anti-conflation
// PlatformOnly/MayMintMoney enforce for the money-MINT side.
//
// Reads c["permissions"] without MustGet so a handler mounted without the token
// gate fails closed (false) rather than panicking. Fail-closed: none present → false.
func MayReadPlatform(c *zip.Ctx) bool {
	claims := iammiddleware.GetIAMClaims(c) // non-nil by contract
	if claims.IsSuperAdmin() {
		return true
	}
	return false
}

// RequirePlatformAdmin is MayReadPlatform as a route guard: it admits the same
// principal and writes the 403 for everyone else, so a handler that calls it and
// returns on false has already answered.
func RequirePlatformAdmin(c *zip.Ctx) bool {
	if MayReadPlatform(c) {
		return true
	}
	_ = http.Fail(c, 403,
		"This operation requires platform-administrator credentials.",
		errors.New("cross-org god-view: caller is not a platform admin"))
	return false
}
