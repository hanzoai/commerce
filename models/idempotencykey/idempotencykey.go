// Package idempotencykey is a reusable idempotency guard for money-moving HTTP
// requests (refunds, captures, payouts). It stores one record per
// (scope, key) whose STORAGE id is deterministic, so a duplicate submit
// collapses onto the same row via the backend ON CONFLICT(id, kind, namespace)
// upsert — independent of datastore.RunInTransaction, which is a no-op on this
// backend and provides no isolation.
//
// Lifecycle of a guarded operation:
//
//	rec, replay, err := idempotencykey.Begin(db, scope, key)
//	  replay == true  ⇒ this exact request already ran; return rec.Response,
//	                    do NOT move money again.
//	  replay == false ⇒ we recorded the in-flight marker first; perform the
//	                    side effect, then idempotencykey.Complete(rec, response).
//
// Begin's writes are swaps (mixin.Model.Swap): two callers racing on a
// first-ever key, in two goroutines or two processes over one store, cannot both
// proceed — one inserts the guard and the other finds it and replays. Callers
// still pass the same key to the gateway (Stripe and Square honor one), which
// de-dups the money move should a guard re-claimed as stale race its original.
package idempotencykey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/orm"
)

func init() {
	orm.Register[IdempotencyKey]("idempotency-key", orm.WithStringKey[IdempotencyKey]())
}

// nowFn is the clock, a seam so tests can simulate elapsed time for the
// stale-guard recovery path without sleeping.
var nowFn = time.Now

// Status values.
const (
	StatusStarted   = "started"
	StatusCompleted = "completed"
)

// StartedTTL bounds how long a "started" guard is treated as a live in-flight
// operation. A money op (refund/capture) completes in seconds; a "started" guard
// older than this is presumed CRASHED (the process died between Begin and
// Complete) and is recoverable — Begin re-claims it and lets the caller retry.
// This is only safe because the guarded money move ALSO carries the same
// deterministic gateway idempotency key, so the gateway de-dupes the retry (no
// double charge). Without that gateway key a stale guard must stay fail-closed.
const StartedTTL = 5 * time.Minute

// IdempotencyKey records one guarded request. Scope namespaces the key to a
// resource kind + id (e.g. "refund:ord_123") so the same key under different
// scopes never collides. Response holds the JSON body returned on first success
// so a replay returns byte-identical output.
type IdempotencyKey struct {
	mixin.Model[IdempotencyKey]

	Scope string `json:"scope"`
	// IdemKey is the caller idempotency key. Named IdemKey (not Key) because a
	// field named Key would shadow the embedded Model[T].Key() method and break
	// the mixin.Entity interface. JSON stays "key" for the API.
	IdemKey  string `json:"key"`
	Status   string `json:"status" orm:"default:started"`
	Response string `json:"response,omitempty" datastore:",noindex"`

	// RecoveryPoint lets a caller record how far a multi-step side effect got,
	// so a retry can resume rather than restart. Optional.
	RecoveryPoint string `json:"recoveryPoint,omitempty"`

	// Facts is a digest of the request the key was first used for, so the same
	// key sent with another request is told apart from a retry. Optional.
	Facts string `json:"facts,omitempty"`

	// Holder names whoever holds the guard as a lease (Hold), so only its holder
	// renews or releases it.
	Holder string `json:"holder,omitempty"`
}

func (k *IdempotencyKey) Load(ps []datastore.Property) error {
	return datastore.LoadStruct(k, ps)
}

func (k *IdempotencyKey) Save() ([]datastore.Property, error) {
	return datastore.SaveStruct(k)
}

// Begin looks up (or records) the guard for (scope, key).
//
// Returns replay=true with the stored record when the key was already seen
// (whether still in-flight or completed) — the caller must NOT repeat the side
// effect and should return rec.Response if Status==completed. Returns
// replay=false with a freshly-created "started" marker when this is the first
// sighting; the caller performs the side effect then calls Complete.
func Begin(db *datastore.Datastore, scope, key string) (rec *IdempotencyKey, replay bool, err error) {
	id := DeterministicID(scope, key)
	// Every write here is a swap (mixin.Model.Swap), so the guard holds across
	// processes as well as goroutines: of two first sightings, one inserts the row
	// and the other finds it there and replays; of two recoveries of a stale
	// guard, one re-claims it and the other reads it again, fresh, and replays.
	for range mixin.Rounds() {
		// Read it by its EXACT storage key (kind + deterministic id + namespace). Do
		// NOT route this read through GetById: GetById decodes the id as a hashid,
		// and a deterministic NON-hashid string ("idem_<hex>") decodes to a KIND-LESS
		// key. The production Postgres backend's Get requires an exact kind match
		// (db/postgres.go), so a kind-less lookup never finds the row this guard
		// wrote under kind "idempotency-key" — every retry then looks brand-new and
		// the money move runs AGAIN (double charge). A kind-qualified Get
		// round-trips on both backends.
		existing := New(db)
		guardKey := db.NewKey(existing.Kind(), id, 0, nil)
		e := existing.Get(guardKey)
		switch {
		case e == nil && (existing.Status == StatusCompleted || !existing.Recoverable()):
			// Completed → a replay (return the stored response). Started and fresh →
			// a genuine concurrent in-flight op: replay (the caller 409s), never a
			// second money move alongside it.
			return existing, true, nil
		case e == nil:
			// Started and STALE → the original crashed between Begin and Complete.
			// Re-claim it and let the caller RETRY: the money move carries the same
			// deterministic gateway key, so the gateway de-dupes if the original
			// had in fact reached it.
			existing.SetId(id)
			existing.Status = StatusStarted
			ok, err := existing.Swap()
			if err != nil {
				return nil, false, err
			}
			if ok {
				return existing, false, nil
			}
			continue
		case !errors.Is(e, datastore.ErrNoSuchEntity):
			return nil, false, e
		}

		rec = New(db)
		rec.SetId(id)
		rec.Scope = scope
		rec.IdemKey = key
		rec.Status = StatusStarted
		ok, err := rec.Swap()
		if err != nil {
			return nil, false, err
		}
		if ok {
			return rec, false, nil
		}
	}
	return nil, false, fmt.Errorf("idempotency key %s changed under %d reads in a row", id, mixin.Swaps)
}

// DeterministicID derives the stable storage id for a (scope, key) pair. Same
// inputs → same id → the second concurrent Begin finds the first's row.
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

// Mark records how far a guarded operation got (RecoveryPoint) on its
// still-started guard, with the facts it is for, so a retry that re-claims the
// guard after a crash resumes from there rather than starting over.
func Mark(rec *IdempotencyKey, point string) error {
	rec.SetId(DeterministicID(rec.Scope, rec.IdemKey))
	rec.RecoveryPoint = point
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

// Recoverable reports whether a "started" guard is stale enough to presume its
// originator crashed — i.e. safe to re-claim and retry. A completed guard is
// never recoverable (it's a replay); a fresh started guard is a live in-flight
// op (fail-closed, caller 409s).
func (k *IdempotencyKey) Recoverable() bool {
	return k.Status == StatusStarted && nowFn().Sub(k.UpdatedAt) >= StartedTTL
}

// Lease is a lock held under one (scope, key) guard: taken atomically by Begin,
// renewed while its holder works so it never lapses under a slow holder, and
// released only by its holder, so a release can never take another's lease.
type Lease struct {
	db    *datastore.Datastore
	id    string
	token string
	since time.Time
	stop  chan struct{}
	wg    sync.WaitGroup
}

// MaxHold is the longest a lease is renewed: a holder still working after it
// (hung, or looping) stops renewing, and its lease lapses StartedTTL later, so
// another holder takes over. What a lease guards is idempotent per key, so a
// holder that outlived its lease writes nothing twice.
const MaxHold = 3 * StartedTTL

// Hold takes the lease for (scope, key). It answers taken=false, with nothing
// held, when a live holder has it; a lease whose holder stopped renewing for
// StartedTTL is taken over, as Begin re-claims a crashed guard.
func Hold(db *datastore.Datastore, scope, key string) (*Lease, bool, error) {
	rec, replay, err := Begin(db, scope, key)
	if err != nil || replay {
		return nil, false, err
	}
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		_ = rec.Delete()
		return nil, false, err
	}
	l := &Lease{db: db, id: DeterministicID(scope, key), token: hex.EncodeToString(b[:]), since: nowFn(), stop: make(chan struct{})}
	rec.Holder = l.token
	if ok, err := rec.Swap(); err != nil || !ok {
		return nil, false, err
	}
	l.wg.Add(1)
	go l.renew()
	return l, true, nil
}

// renew keeps the lease fresh while it is held: every third of StartedTTL it is
// written again, while it is still this holder's, for at most MaxHold.
func (l *Lease) renew() {
	defer l.wg.Done()
	t := time.NewTicker(StartedTTL / 3)
	defer t.Stop()
	for {
		select {
		case <-l.stop:
			return
		case <-t.C:
			if !l.refresh() {
				return
			}
		}
	}
}

// refresh writes the lease again while it is this holder's, and answers false,
// writing nothing, once it has been held for MaxHold.
func (l *Lease) refresh() bool {
	if nowFn().Sub(l.since) >= MaxHold {
		return false
	}
	if rec, ok := l.mine(); ok {
		_, _ = rec.Swap()
	}
	return true
}

// mine reads the lease and reports whether this holder still holds it.
func (l *Lease) mine() (*IdempotencyKey, bool) {
	rec := New(l.db)
	if err := rec.Get(l.db.NewKey(rec.Kind(), l.id, 0, nil)); err != nil {
		return nil, false
	}
	return rec, rec.Holder == l.token
}

// Release gives the lease up, if it is still this holder's. Releasing twice is
// releasing once.
func (l *Lease) Release() {
	if l == nil {
		return
	}
	select {
	case <-l.stop:
		return
	default:
		close(l.stop)
	}
	l.wg.Wait()
	if rec, ok := l.mine(); ok {
		_ = rec.Delete()
	}
}
