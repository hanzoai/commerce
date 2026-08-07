package billing

import (
	"net/http"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/depositledger"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
)

// DepositWatcherStatus reports what the crypto deposit rail is watching and how
// far it has scanned.
//
//	GET /_/commerce/deposits
//
// It exists because the answer used to be two fmt.Fprintf(os.Stderr, …) lines at
// boot, and a boot line is not an answer. Measured on the live pod: the watcher
// state is NOT in `kubectl logs` — the line is gone, and the one fact an
// operator needs before deciding whether a customer's deposit was lost ("is this
// rail armed, and for what?") could not be established from the process that
// holds it. That cost a wrong diagnosis. A fact worth printing once is a fact
// worth serving.
//
// SUPERADMIN ONLY — deliberately stricter than the sibling routes in this group
// (GET /_/commerce/providers allows an org admin too). The watch table is a
// PLATFORM fact, not a tenant one: it is a single process-wide table shared by
// every org, and it names the custody accounts and node providers the whole
// deployment's money moves through. An org admin has no tenant-scoped view of it
// to be given, so there is nothing here to scope and the answer is no.
//
// It is READ-ONLY, and that is a design constraint rather than an unfinished
// feature. Arming an asset is configuring CRYPTO_DEPOSIT_* and nothing else —
// one deliberate act, made through KMS, auditable there. An admin toggle would
// be a SECOND way to open a money path, reachable over HTTP, and the two would
// then be able to disagree about whether a chain is being watched. The gate's
// meaning is "something is watching this"; a button that claims it is watched
// without a watcher is exactly the lie this rail was built to stop telling.
func DepositWatcherStatus(c *zip.Ctx) error {
	// Anonymous is 401 — not signed in, never the authorization 403. Same
	// distinction the tenant-admin handlers draw, and a browser only
	// re-authenticates on 401.
	if !iammiddleware.IsIAMAuthenticated(c) {
		return c.JSON(http.StatusUnauthorized, map[string]any{"error": "authentication required"})
	}
	if !iammiddleware.GetIAMClaims(c).IsSuperAdmin() {
		return c.JSON(http.StatusForbidden, map[string]any{"error": "superadmin role required"})
	}
	return c.JSON(http.StatusOK, depositledger.Default().Status(c.Context()))
}
