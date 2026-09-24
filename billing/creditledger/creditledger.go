// Package creditledger is the injection seam for the ONE double-entry ledger
// commerce's prepaid money lives in: credits go in, balances are read, and
// payments are drawn out, all at one address.
//
// Commerce runs BOTH standalone and EMBEDDED in the cloud binary. When embedded,
// the AI spend-gate reads cloud's native ledger (a per-org finance ledger);
// commerce is a SEPARATE Go module and must NOT import cloud. So the host injects
// a ledger-backed CreditLedger through commerce.EmbedConfig.Ledger, and
// commerce's credit + balance handlers route to it — a credit then lands in the
// SAME per-org account the gate reads (one ledger, no split). When no ledger is
// injected (standalone dev/test), commerce falls back to its own datastore.
//
// The contract is compiler-enforced: cloud implements this interface (a ~40-line
// adapter over its finance ledger) and sets EmbedConfig.Ledger; commerce calls it.
package creditledger

import (
	"context"
	"errors"
	"sync"
	"time"
)

// CreditInput is one idempotent credit grant. The ledger is multi-currency, so
// Currency is explicit. ExpiresAt nil = no expiry.
//
// (Org, Subject) IS the address the credit lands at: Org names the ledger, Subject
// the account within it. Subject is OPTIONAL and empty means the org's pool account
// — the whole tenant shares one balance, which is what a tenant org wants and what
// this seam has always done. It exists because that is not universally true: in a
// shared signup org the members are strangers to each other, each spends from their
// own account, and a credit keyed on the org alone lands in a pool no gate reads —
// money that is neither lost nor spendable. A host that resolves per-member accounts
// names one here, from the SAME rule its gate resolves the payer with, so a credit
// and the spend it funds address one wallet by construction.
//
// Test names WHICH BOOKS the credit lands in, and it is the third component of the
// address rather than a mode: the host keeps sandbox money in a ledger of its own, so
// (Org, Subject, Test) is the whole answer to "where". It exists because a seam that
// could not say it forced every sandbox settlement to choose between two wrong
// answers — post it through here and unspendable sandbox money lands in the LIVE
// books, or bypass the seam entirely and there are two credit paths to keep in step.
// The zero value is live money, which is what every caller that never thought about
// it meant.
//
// It is also what makes the idempotency key honest. The host dedups on the key WITHIN
// one ledger, so two callers who resolve the same payment to different books both
// credit it — no error, no duplicate detected, the money simply lands twice. The key
// is only exactly-once if everyone who can credit one payment addresses one ledger,
// and Test is a component of that address.
type CreditInput struct {
	Org            string
	Subject        string
	Currency       string
	Reason         string
	Tag            string
	IdempotencyKey string
	AmountCents    int64
	Test           bool
	ExpiresAt      *time.Time
}

// DebitInput is one payment drawn from an account: (Org, Subject, Test) is the
// same address CreditInput names, so a debit spends the money a credit put there.
// Ref names the act the money pays for and is the idempotency key: the same Ref
// for the same amount debits at most once.
type DebitInput struct {
	Org         string
	Subject     string
	Currency    string
	Reason      string
	Ref         string
	AmountCents int64
	Test        bool
}

// ErrShort is a debit the account cannot cover in full. It moved nothing.
var ErrShort = errors.New("insufficient balance")

// CreditLedger is the ONE ledger commerce's prepaid money lives in when commerce is
// embedded. Every method MUST resolve the account the SAME way — an empty Subject is
// the org's pool everywhere — so a Credit is immediately visible to Balance, to
// Debit, and to the cloud AI gate that reads the same ledger account.
//
// THEY TAKE THE SAME ADDRESS, and that is the point of the shapes below. A credit
// lands at (Org, Subject, Test); a read or a debit that named fewer components than
// that could not name the account the credit went to, and the one it named instead
// was a different account that answers without complaining. Every verb spells the
// whole address or none does.
type CreditLedger interface {
	// Credit appends a BALANCED double-entry credit to the org's account and
	// returns the posting/tx id and the account's new available balance.
	// Idempotent on IdempotencyKey: the same key credits AT MOST once and returns
	// the original posting's id + balance (never a second grant).
	Credit(ctx context.Context, in CreditInput) (txID string, balanceCents int64, err error)
	// Balance returns the available balance in cents held at (org, subject) in
	// currency, from the sandbox books when test and the live ones otherwise —
	// byte-for-byte the address CreditInput names. An empty subject is the org's
	// pool, the same default Credit applies.
	Balance(ctx context.Context, org, subject, currency string, test bool) (availableCents int64, err error)
	// Debit draws a payment from the account, all or nothing: when the account
	// holds less than AmountCents it moves nothing and answers ErrShort. The check
	// and the posting are one transaction, so a concurrent spend cannot take the
	// account below zero between them. Idempotent on Ref: a replay answers the
	// original posting's id and moves nothing.
	Debit(ctx context.Context, in DebitInput) (txID string, balanceCents int64, err error)
}

var (
	mu       sync.RWMutex
	injected CreditLedger
)

// Set installs the host-injected ledger. commerce.Embed calls it from
// EmbedConfig.Ledger; passing nil clears it (→ commerce falls back to its own
// datastore). Process-wide, mirroring the host's finance.Publish/Current singleton.
func Set(l CreditLedger) {
	mu.Lock()
	injected = l
	mu.Unlock()
}

// Get returns the injected ledger, or nil when commerce runs standalone (datastore
// fallback). The credit + balance handlers call it on every request.
func Get() CreditLedger {
	mu.RLock()
	defer mu.RUnlock()
	return injected
}
