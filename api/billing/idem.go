package billing

import (
	"strconv"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/idempotencykey"
)

// idemBegin is the idempotency-guard seam, following the same convention as
// processorsForOrg: a package var ONLY so a test can drive the
// store-unavailable branch. Production is always idempotencykey.Begin.
//
// That branch is worth a seam because it is where the guard decides what to do
// when it cannot tell a first attempt from a retry — the single decision
// standing between a store blip and a duplicate charge on a real card. It is
// also unreachable by any other means in a test, which is precisely why it sat
// wrong (failing open) and unnoticed.
var idemBegin = func(db *datastore.Datastore, scope, key string) (*idempotencykey.IdempotencyKey, bool, error) {
	return idempotencykey.Begin(db, scope, key)
}

// idemWindowSec is the fallback idempotency WINDOW for a money move whose caller
// sent no X-Idempotency-Key: within the window a repeat of the SAME request
// de-dups (a lost-response retry), and after it a genuine later request of the
// same shape proceeds. Clients that need exact idempotency send the header,
// which is used verbatim and is never windowed.
const idemWindowSec = 900 // 15 minutes

// guardKey is the key a card money move guards itself on: the caller's
// X-Idempotency-Key when supplied, else a key derived from facts — the request
// details that stay STABLE across a retry — bucketed into a coarse time window.
//
// The window is what makes the fallback safe in both directions: a re-submit
// seconds later collapses onto the same key, so the card is charged once; a
// genuine repeat purchase minutes later gets a fresh key, so the guard is never
// a permanent per-request lock.
//
// facts must NEVER include a single-use nonce. A re-tokenized retry mints a new
// nonce, so a nonce-derived key changes exactly when it most needs to stay the
// same — which is the double charge this exists to prevent.
//
// ONE derivation, shared by every card money move (top-up, subscribe,
// auto-recharge), so they cannot drift into disagreeing answers to the same
// question.
func guardKey(c *zip.Ctx, facts string) string {
	if k := strings.TrimSpace(c.Header("X-Idempotency-Key")); k != "" {
		return k
	}
	return windowKey(facts)
}

// windowKey buckets stable request facts into the coarse idempotency window. It
// is guardKey's fallback, and the whole key for a money move with no client
// request behind it (the auto-recharge cron), where there is no header to read
// and reading one would be wrong — a single run-all request must not lend its
// key to every org it charges.
func windowKey(facts string) string {
	return facts + ":w" + strconv.FormatInt(time.Now().Unix()/idemWindowSec, 10)
}

// gatewayKey namespaces a guard key for the PAYMENT GATEWAY's own idempotency,
// so the provider de-dups the money move even when our local guard store is
// unavailable or two submits race — the definitive backstop, since it is
// enforced where the money actually moves.
//
// Scoped by operation and subject so a key can never collide across endpoints
// or tenants.
func gatewayKey(op, subject, guard string) string {
	return op + ":" + subject + ":" + guard
}
