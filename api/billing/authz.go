package billing

import (
	"strings"

	"github.com/hanzoai/account"
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
//   - a SuperAdmin (IsSuperAdmin() — owner=="admin" — the
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
// equals their own per-org billing subject — the org slug returned by
// orgBillingKey, which is exactly the key EdgeAuth pins those records under
// (== claims.Owner == org.Name == namespace). Fail-closed: an empty subject, or
// no owner matching it, denies.
func callerMayReachBillingSubject(c *zip.Ctx, ownerIDs ...string) bool {
	if isPrivilegedBillingCaller(c) {
		return true
	}
	subject := orgBillingKey(c)
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

// mayActForOrg reports whether the caller may act for the org's own account: a
// privileged billing caller (service, platform SuperAdmin) or an admin of the org
// the request acts in.
func mayActForOrg(c *zip.Ctx) bool {
	return isPrivilegedBillingCaller(c) || iammiddleware.ActsAsOrgAdmin(c)
}

// mayChangeWallet reports whether the caller may change what the wallet it pays
// from pays with — save, change or remove a card, or charge one on file: anyone
// who may act for the org, and a member whose wallet is their own person account
// in the org (the shared signup org's members). A member billed from the org's
// pool may not, because the org's cards are its admins' to change; nor may a
// credential billed from any other account it does not own, such as a project.
func mayChangeWallet(c *zip.Ctx) bool {
	if mayActForOrg(c) {
		return true
	}
	own := userBillingKey(c)
	return own != "" && own == account.Person(orgBillingKey(c), c.Header("X-User-Id")).Subject()
}

// askAnAdmin is what a member is told when an act is the org's admins' to take.
func askAnAdmin(c *zip.Ctx, act string) string {
	return "Only an admin of " + orgBillingKey(c) + " can " + act + ". Ask an admin to do it."
}
