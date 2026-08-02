// Package idempotencykey is the guard on a money-moving request: refunds,
// captures, payouts, charges.
//
// One record per [Guard], whose STORAGE id is derived from the guard, so the
// key names a row rather than merely labelling one.
//
// IT IS A CLAIM, NOT A READ-THEN-WRITE. [Begin] takes the row with one store
// statement that succeeds for exactly one caller ([datastore.Datastore.Claim]),
// because the alternative — read, decide, write — lets two concurrent callers
// both observe "not started" and both move money, which is the duplicate
// disbursement this package exists to prevent. The winner performs the side
// effect; every other caller is told the key is taken and replays or refuses.
//
// Lifecycle of a guarded operation:
//
//	rec, replay, err := idempotencykey.Begin(db, idempotencykey.Guard{...})
//	  replay == true  ⇒ this key is already taken; return rec.Response when
//	                    Status==completed, else refuse — do NOT move money.
//	  replay == false ⇒ we own it; perform the side effect, then
//	                    idempotencykey.Complete(rec, response).
//	  err == ErrConflict ⇒ the key was used for a DIFFERENT request.
//
// A GUARD IS NEVER RECLAIMED ON A TIMER. An operation that crashed between
// [Begin] and [Complete] leaves a started guard, and a started guard refuses
// the retry: we do not know whether the money moved, and a retry that "assumes
// it did not" is a second disbursement. The refusal is loud (the caller gets a
// conflict, not a silent replay) and it is reversible by the operation itself —
// a caller that establishes NOTHING happened releases the guard by deleting it,
// which is the one legitimate way a key becomes free again.
package idempotencykey

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/orm"
)

func init() {
	orm.Register[IdempotencyKey]("idempotency-key", orm.WithStringKey[IdempotencyKey]())
}

// Status values.
const (
	StatusStarted   = "started"
	StatusCompleted = "completed"
)

// ErrConflict refuses a key that was already used for a DIFFERENT request.
//
// A key names ONE question. Answering a second question with the first one's
// answer is how a caller asks for a $10,000 payout and is told "created" with
// the receipt for a $1 one — so a request whose digest does not match the
// guard's is a caller mistake and is named as one, at every door, the same way.
var ErrConflict = errors.New("idempotencykey: this key was used for a different request")

// Guard names one guarded request.
//
// It is a VALUE and not three positional strings because the third one is
// OPTIONAL and changes what the guard means: with a digest the guard also
// answers "is this the same request?", and a digest that ends up in the key
// field would silently turn every retry into a new operation.
type Guard struct {
	// Scope namespaces the key to a resource kind and id ("refund:ord_123"), so
	// one key under two scopes never collides.
	Scope string
	// Key is the caller's idempotency key.
	Key string
	// Digest is a stable fingerprint of the REQUEST this key answers. When it is
	// set, a later request under the same key whose digest differs is refused
	// with [ErrConflict] instead of replaying the first request's answer. Empty
	// means the guard only de-dups and does not compare.
	Digest string
}

// ID is the stable storage id for this guard. Same guard → same id → the claim
// is a race between callers for one row.
//
// The digest is deliberately NOT in it: the id must be the same for a retry of
// the same key so the second caller FINDS the first's row and can be refused,
// which is exactly what a digest in the id would prevent.
func (g Guard) ID() string { return DeterministicID(g.Scope, g.Key) }

// IdempotencyKey records one guarded request. Response holds the JSON body
// returned on first success so a replay returns byte-identical output.
type IdempotencyKey struct {
	mixin.Model[IdempotencyKey]

	Scope string `json:"scope"`
	// IdemKey is the caller idempotency key. Named IdemKey (not Key) because a
	// field named Key would shadow the embedded Model[T].Key() method and break
	// the mixin.Entity interface. JSON stays "key" for the API.
	IdemKey string `json:"key"`
	// Digest is the fingerprint of the request this guard was taken for. It is
	// what makes the key mean "the same request" rather than "the same string".
	Digest   string `json:"digest,omitempty"`
	Status   string `json:"status" orm:"default:started"`
	Response string `json:"response,omitempty" datastore:",noindex"`

	// RecoveryPoint lets a caller record how far a multi-step side effect got,
	// so a retry can resume rather than restart. Optional.
	RecoveryPoint string `json:"recoveryPoint,omitempty"`
}

func (k *IdempotencyKey) Load(ps []datastore.Property) error {
	return datastore.LoadStruct(k, ps)
}

func (k *IdempotencyKey) Save() ([]datastore.Property, error) {
	return datastore.SaveStruct(k)
}

// Begin CLAIMS the guard for g.
//
// Returns replay=false with the record when this caller now owns the key: it
// performs the side effect and calls [Complete]. Returns replay=true with the
// stored record when the key is already taken — completed (return
// rec.Response) or still started (refuse; the first attempt's outcome is not
// known). Returns [ErrConflict] when the key is held for a different request.
//
// The claim reads the row back by its EXACT storage key (kind + deterministic
// id + namespace). It does NOT route that read through GetById: GetById decodes
// the id as a hashid, and a deterministic NON-hashid string ("idem_<hex>")
// decodes to a KIND-LESS key. The production Postgres backend's Get requires an
// exact kind match (db/postgres.go), so a kind-less lookup never finds the row
// this guard just wrote under kind "idempotency-key" — every retry then looks
// brand-new and the money move runs AGAIN. SQLite's Get has a kind-less
// fallback (db/sqlite.go), which is the ONLY reason this passed in tests and
// bit solely in production. A kind-qualified Get round-trips on both backends.
func Begin(db *datastore.Datastore, g Guard) (rec *IdempotencyKey, replay bool, err error) {
	id := g.ID()

	rec = New(db)
	rec.SetId(id)
	rec.Scope = g.Scope
	rec.IdemKey = g.Key
	rec.Digest = g.Digest
	rec.Status = StatusStarted

	mine, e := rec.Claim()
	if e != nil {
		return nil, false, e
	}
	if mine {
		return rec, false, nil
	}

	existing := New(db)
	if e := existing.Get(db.NewKey(existing.Kind(), id, 0, nil)); e != nil {
		// The claim said someone holds the key and the read cannot say who. That
		// is exactly the state in which proceeding costs a duplicate
		// disbursement, so it is an error and never a first attempt.
		if errors.Is(e, datastore.ErrNoSuchEntity) {
			return nil, false, errors.New("idempotencykey: the guard is held by a record that cannot be read")
		}
		return nil, false, e
	}
	if g.Digest != "" && existing.Digest != "" && existing.Digest != g.Digest {
		return existing, true, ErrConflict
	}
	return existing, true, nil
}

// DeterministicID derives the stable storage id for a (scope, key) pair.
func DeterministicID(scope, key string) string {
	sum := sha256.Sum256([]byte(scope + "\x00" + key))
	return "idem_" + hex.EncodeToString(sum[:16])
}

// Complete records the successful response so future replays return it, and
// flips Status to completed. Called after the guarded side effect succeeds.
//
// Re-pins the deterministic id and Puts so the write lands on the exact
// "started" row (the ON CONFLICT upsert transitions it in place). SetId+Put is
// the reliable in-place write for a string-key model.
func Complete(rec *IdempotencyKey, response string) error {
	rec.SetId(DeterministicID(rec.Scope, rec.IdemKey))
	rec.Status = StatusCompleted
	rec.Response = response
	return rec.Put()
}

// New returns an initialized IdempotencyKey bound to db.
func New(db *datastore.Datastore) *IdempotencyKey {
	k := new(IdempotencyKey)
	k.Init(db)
	return k
}

// Query returns a query for this kind, scoped to db's namespace.
func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("idempotency-key")
}
