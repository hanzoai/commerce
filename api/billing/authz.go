package billing

import (
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/permission"
)

// isPrivilegedBillingCaller reports whether the authenticated caller may act on
// billing records belonging to ANY subject within the resolved org — the
// service-to-service and administrative callers that legitimately operate on a
// user's behalf (cloud-api, the auto-recharge cron, an org owner, a platform
// admin). Any of three independent signals suffices:
//
//   - the Admin permission bit — set by middleware.TokenRequired for a verified
//     COMMERCE_SERVICE_TOKEN and for a legacy org admin access token, and by
//     iammiddleware.IAMTokenRequired for a gateway/EdgeAuth-minted org admin
//     (X-User-Permissions). This is the ONLY signal a stripped-identity service
//     token carries (EdgeAuth deletes its X-* headers), so it must be checked.
//   - the org-level IsAdmin claim (an org owner acting inside their own org); or
//   - a platform global admin (IsSuperAdmin() — owner=="admin" — the
//     "admin" org), who may retarget any org via EdgeAuth's ?org override.
//
// It reads c["permissions"] defensively (no MustGet) so a handler reached
// without a permission-setting middleware fails CLOSED (treated as
// non-privileged) rather than panicking.
func isPrivilegedBillingCaller(c *zip.Ctx) bool {
	if v := c.Locals("permissions"); v != nil {
		if f, ok := v.(bit.Field); ok && f.Has(permission.Admin) {
			return true
		}
	}
	claims := iammiddleware.GetIAMClaims(c)
	return claims.IsAdmin || claims.IsSuperAdmin()
}

// callerMayReachBillingSubject reports whether the authenticated caller is
// authorized to act on a per-user billing record whose owner is one of
// ownerIDs (its CustomerId / UserId — the fields that name whose money/instrument
// the record is). It is the intra-org companion to EdgeAuth's namespace +
// billing-subject lock: EdgeAuth already pins the namespace and the pinnable body
// keys (user/userId/customerId) to the caller's org, but a URL :id path-param
// (GetById handlers) and the paymentMethodId body field — neither of which
// EdgeAuth pins — can still NAME a different subject's record inside that same
// org. Cross-ORG is prevented by the namespace; this closes the residual
// INTRA-org gap.
//
// Privileged callers always pass. Everyone else passes only when one of ownerIDs
// equals their own billing subject — the PAYER, userBillingKey, which is the key
// saveCard stamps onto a card it vaults (its parameter is named `subject`) and the
// key Topup compares before charging one. Fail-closed: an empty subject, or no
// owner matching it, denies.
//
// It read the ORG SLUG until now, and that is not the key these records carry. A
// person in the signup org pays from "<org>/<user>", so every card they saved was
// looked up under an owner it was never written with: their own card answered 404
// on get, update, detach and set-default, while Topup — already on the payer —
// charged the same card happily. One record cannot have two owners. The payer is
// the one the writer used, so the reader uses it too.
func callerMayReachBillingSubject(c *zip.Ctx, ownerIDs ...string) bool {
	if isPrivilegedBillingCaller(c) {
		return true
	}
	subject := userBillingKey(c)
	if subject == "" {
		return false
	}
	for _, o := range ownerIDs {
		if strings.EqualFold(strings.TrimSpace(o), subject) {
			return true
		}
	}
	return false
}
