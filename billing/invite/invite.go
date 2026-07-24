// Package invite is the commerce paywall's invite-code engine: a platform-minted
// code that grants ONE org subscription-free access to the commerce admin.
//
// It reimplements the cloud referrals code→org primitive (OrgForCode / Claim)
// natively on commerce's datastore — commerce never imports cloud. The design is
// the same one: a GLOBAL directory (the "system" namespace, like Organization /
// the platform catalog) so a code minted by the platform is redeemable by ANY
// org, keyed on the code with first-touch, idempotent binding.
//
// One Invite row per code. Its storage id is DETERMINISTIC on the normalized
// code (DeterministicID), so:
//
//   - Mint is idempotent: re-minting the same code resolves to the SAME row via
//     the storage ON CONFLICT(id,kind,namespace) upsert — never a duplicate.
//   - Redeem is first-touch: the first org to redeem a code claims it; a repeat
//     redeem by the SAME org is a no-op replay, and a redeem of an
//     already-claimed code by a DIFFERENT org is refused.
package invite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/val"
	"github.com/hanzoai/orm"
)

const kind = "commerce-invite"

// Namespace is the global directory the invite records live in — the same
// "system" namespace Organization / the platform catalog use, so a
// platform-minted code is visible to (and redeemable by) every tenant. Never a
// per-org namespace: the redeeming org is not known at mint time.
const Namespace = "system"

func init() { orm.Register[Invite](kind, orm.WithStringKey[Invite]()) }

// Sentinel errors mapped to HTTP status by the handlers.
var (
	// ErrUnknownCode — no invite exists for the presented code.
	ErrUnknownCode = errors.New("invite: unknown code")
	// ErrAlreadyRedeemed — the code was already claimed by a DIFFERENT org.
	ErrAlreadyRedeemed = errors.New("invite: code already redeemed by another org")
	// ErrEmptyCode / ErrEmptyOrg — malformed input.
	ErrEmptyCode = errors.New("invite: empty code")
	ErrEmptyOrg  = errors.New("invite: empty org")
)

// Invite is one code→org grant. Redeemed+Org record the first-touch binding.
type Invite struct {
	mixin.Model[Invite]

	// Code is the normalized (upper-cased, trimmed) invite code — also the
	// natural key the deterministic storage id is derived from.
	Code string `json:"code"`

	// Org is the org that redeemed the code (empty until redeemed). Once set it
	// is immutable — first-touch attribution.
	Org string `json:"org,omitempty"`

	// Note is optional free text captured at mint time (who/why), for audit.
	Note string `json:"note,omitempty"`

	// Redeemed latches true on the first successful redeem.
	Redeemed   bool      `json:"redeemed"`
	CreatedAt  time.Time `json:"createdAt"`
	RedeemedAt time.Time `json:"redeemedAt,omitempty"`
}

func (i *Invite) Load(ps []datastore.Property) error  { return datastore.LoadStruct(i, ps) }
func (i *Invite) Save() ([]datastore.Property, error) { return datastore.SaveStruct(i) }
func (i *Invite) Validator() *val.Validator           { return val.New() }

// New returns a datastore-wired Invite.
func New(db *datastore.Datastore) *Invite {
	i := new(Invite)
	i.Init(db)
	return i
}

// Query returns a datastore query for invites.
func Query(db *datastore.Datastore) datastore.Query { return db.Query(kind) }

// DeterministicID derives the stable storage id for a normalized code, so mint
// is idempotent and each code is a single row (ON CONFLICT dedup at storage).
func DeterministicID(code string) string {
	sum := sha256.Sum256([]byte("commerce-invite:" + code))
	return "inv_" + hex.EncodeToString(sum[:16])
}

// Normalize canonicalizes a code: trimmed + upper-cased so a code pasted in any
// case resolves. Kept in one place so mint and redeem agree.
func Normalize(code string) string { return strings.ToUpper(strings.TrimSpace(code)) }

// SystemDB builds the datastore scoped to the global invite directory. It uses
// datastore.New with the "system" namespace on the context (NOT NewNamespaced,
// which fail-closes to no DB for reserved namespaces) — identical to how the
// platform catalog reaches its system-namespace store.
func SystemDB(ctx context.Context) *datastore.Datastore {
	return datastore.New(nscontext.WithNamespace(ctx, Namespace))
}

// find loads the invite for a normalized code into a datastore-bound model (so a
// later Update persists in place), or reports not-found. It looks up by the
// indexed Code field rather than GetById: the deterministic string id does not
// round-trip through the hashid key decoder, so a code lookup is the robust path.
func find(db *datastore.Datastore, code string) (*Invite, bool, error) {
	inv := New(db)
	found, err := inv.Query().Filter("Code=", code).Get()
	if err != nil {
		return nil, false, err
	}
	return inv, found, nil
}

// Mint creates (or returns the existing) invite for a code. Idempotent on the
// normalized code: minting the same code twice returns the same row rather than a
// duplicate — the deterministic id makes concurrent first-mints collapse to ONE
// row via the storage ON CONFLICT(id,kind,namespace) upsert. db must be the SystemDB.
func Mint(db *datastore.Datastore, code, note string) (*Invite, error) {
	code = Normalize(code)
	if code == "" {
		return nil, ErrEmptyCode
	}

	if existing, found, err := find(db, code); err != nil {
		return nil, err
	} else if found {
		return existing, nil
	}

	inv := New(db)
	inv.SetId(DeterministicID(code)) // deterministic id ⇒ ON CONFLICT dedup at storage
	inv.Code = code
	inv.Note = note
	inv.CreatedAt = time.Now()
	if err := inv.Create(); err != nil {
		return nil, err
	}
	return inv, nil
}

// Redeem binds a code to org, first-touch. Returns (invite, redeemedNow, err):
//
//   - unknown code                       → ErrUnknownCode
//   - already redeemed by the SAME org   → (invite, false, nil)  — idempotent replay
//   - already redeemed by a DIFFERENT org → ErrAlreadyRedeemed
//   - fresh                              → (invite, true, nil)   — org bound
//
// db must be the SystemDB. org is the VALIDATED caller's org (never client-supplied).
func Redeem(db *datastore.Datastore, code, org string) (*Invite, bool, error) {
	code = Normalize(code)
	if code == "" {
		return nil, false, ErrEmptyCode
	}
	org = strings.TrimSpace(org)
	if org == "" {
		return nil, false, ErrEmptyOrg
	}

	inv, found, err := find(db, code)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, ErrUnknownCode
	}

	if inv.Redeemed {
		if inv.Org == org {
			return inv, false, nil // idempotent replay — same org
		}
		return nil, false, ErrAlreadyRedeemed
	}

	inv.Org = org
	inv.Redeemed = true
	inv.RedeemedAt = time.Now()
	if err := inv.Update(); err != nil {
		return nil, false, err
	}
	return inv, true, nil
}

// OrgRedeemed reports whether org holds a redeemed invite — the paywall's
// invite allow-path. db must be the SystemDB. A single-field query (Org=) then an
// in-memory Redeemed check avoids a two-equality-filter composite index (which is
// not guaranteed across datastore backends — see the same rule in billing/trial).
func OrgRedeemed(db *datastore.Datastore, org string) (bool, error) {
	org = strings.TrimSpace(org)
	if org == "" {
		return false, nil
	}
	invs := make([]*Invite, 0, 2)
	if _, err := Query(db).Filter("Org=", org).GetAll(&invs); err != nil {
		return false, err
	}
	for _, inv := range invs {
		if inv.Redeemed {
			return true, nil
		}
	}
	return false, nil
}
