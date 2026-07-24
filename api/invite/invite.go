// Package invite exposes the commerce paywall's invite-code HTTP surface:
//
//	POST /v1/commerce/invite/redeem  {code}   — the caller's org claims a code
//	POST /v1/commerce/invite         {code,note?} — platform-admin mints a code
//
// The engine (billing/invite) is the referrals code→org primitive reimplemented
// natively on commerce's datastore. Redeem binds the code to the VALIDATED
// caller's org (first-touch, idempotent); mint is platform-superadmin only.
package invite

import (
	"errors"

	"github.com/zap-proto/zip"

	invitemodel "github.com/hanzoai/commerce/billing/invite"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
)

// Route mounts the invite surface under /v1/commerce/invite. args carry the
// bundle's tokenRequired (identity resolution); redeem needs an authenticated org
// and mint additionally enforces platform-superadmin inside the handler.
func Route(r zip.Router, args ...zip.Handler) {
	g := r.Group("/commerce/invite")
	g.Post("/redeem", append(append([]zip.Handler{}, args...), Redeem)...)
	g.Post("", append(append([]zip.Handler{}, args...), Mint)...)
}

type request struct {
	Code string `json:"code"`
	Note string `json:"note,omitempty"`
}

// Redeem marks the caller's org access-granted by claiming an invite code.
// Idempotent: a repeat redeem by the same org replays; a code already claimed by
// another org is refused.
func Redeem(c *zip.Ctx) error {
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return http.Fail(c, 401, "authentication required", errors.New("no organization"))
	}

	var req request
	if err := json.DecodeBytes(c.Body(), &req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}
	if invitemodel.Normalize(req.Code) == "" {
		return http.Fail(c, 400, "code is required", nil)
	}

	db := invitemodel.SystemDB(c.Context())
	inv, redeemed, err := invitemodel.Redeem(db, req.Code, org.Name)
	switch {
	case errors.Is(err, invitemodel.ErrUnknownCode):
		return http.Fail(c, 404, "unknown invite code", err)
	case errors.Is(err, invitemodel.ErrAlreadyRedeemed):
		return http.Fail(c, 409, "invite code already redeemed by another org", err)
	case err != nil:
		return http.Fail(c, 500, "failed to redeem invite", err)
	}

	status := 200
	if redeemed {
		status = 201
	}
	return http.Render(c, status, map[string]any{
		"code":     inv.Code,
		"org":      inv.Org,
		"redeemed": inv.Redeemed,
	})
}

// Mint creates an invite code. Platform-superadmin only: invite codes grant
// subscription-free access, so an org-level admin must never mint one.
func Mint(c *zip.Ctx) error {
	claims := iammiddleware.GetIAMClaims(c)
	if !middleware.IsServiceToken(c) && (claims == nil || !claims.IsSuperAdmin()) {
		return http.Fail(c, 403, "platform admin required to mint an invite code", errors.New("not a global admin"))
	}

	var req request
	if err := json.DecodeBytes(c.Body(), &req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}
	if invitemodel.Normalize(req.Code) == "" {
		return http.Fail(c, 400, "code is required", nil)
	}

	db := invitemodel.SystemDB(c.Context())
	inv, err := invitemodel.Mint(db, req.Code, req.Note)
	if err != nil {
		return http.Fail(c, 500, "failed to mint invite", err)
	}
	return http.Render(c, 201, map[string]any{
		"code":      inv.Code,
		"note":      inv.Note,
		"redeemed":  inv.Redeemed,
		"createdAt": inv.CreatedAt,
	})
}
