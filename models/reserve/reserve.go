// Copyright © 2026 Hanzo AI. MIT License.

// Package reserve is the ACCOUNT of money a reserve is holding: the running
// total and the ledger that explains it.
//
// A reserve rate says what share of a move to hold back. It does not say how
// much is being held, when it was taken, which judgement took it, or what a
// release returns — and a haircut nobody accounts for is not a reserve, it is
// an arbitrary shortfall on a merchant's payout. This package is the account,
// in the org's own store.
//
// THE CONTROL DECLARES; THE RESERVE ACCOUNTS. control.Control carries the rate,
// the ceiling and the currency — it is a standing instruction and, as its own
// package doc says, "not a judgement and not a balance". The BALANCE lives
// here, on [Account], because a total that lives on the declaration is a total
// every op that can read or place a declaration can move. Here there is exactly
// one door that moves it and it is [Take].
//
// TWO ROWS, ONE ACT. [Account] is the total the ceiling is measured against —
// one read, on the money path, never a sum over a table. [Entry] is the
// evidence: what was taken, from which judgement, under which declaration. They
// are written INSIDE ONE STORE TRANSACTION, so the total and the ledger cannot
// disagree: they land together or neither lands. A release returns the total,
// so a total the ledger does not explain is money a merchant never gets back.
//
// THE MONEY MOVES WHERE THE MONEY MOVES. Nothing here is called by a judgement.
// A screen answers a question, and answering a question must not spend a
// ceiling: a reserve whose ceiling any authenticated caller can consume with a
// money-free screen is strictly worse than a reserve with no ceiling at all,
// because the ceiling then switches the reserve OFF. The one caller is the
// disbursement boundary, and a test pins that it is the only one.
//
// Rows are IDENTIFIED BY WHAT THEY RECORD, never minted fresh: a hold is named
// by the judgement that took it, a release by the declaration that returned it,
// an account by the declaration it belongs to. A retried movement finds its own
// row already there and moves nothing — a ledger that double-posts under retry
// is worse than no ledger.
package reserve

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/db"
	"github.com/hanzoai/commerce/models/control"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/orm"
)

func init() {
	orm.Register[Entry]("risk-reserve", orm.WithStringKey[Entry]())
	orm.Register[Account]("risk-account", orm.WithStringKey[Account]())
}

// Every model MUST satisfy mixin.Entity — see the note on control.Control.
var (
	_ mixin.Entity = (*Entry)(nil)
	_ mixin.Entity = (*Account)(nil)
)

// Max is the most entries any read of this kind materialises. The ledger grows
// with a merchant's payouts, so it is exactly the table a single unbounded read
// would eventually die on.
const Max = 500

// Unlimited is the headroom of a reserve that declares no ceiling. It is a
// value and not a nil so the clamp has one shape for both cases.
const Unlimited int64 = math.MaxInt64

// Account is what ONE reserve is holding right now: the running total its
// ceiling is measured against, and the amount a release returns.
type Account struct {
	mixin.Model[Account]

	// Control is the declaration this is the account OF.
	Control string `json:"control"`
	// Currency denominates Held, and is the currency the control declared.
	Currency currency.Type `json:"currency,omitempty"`
	// Held is the cumulative exact minor units this reserve is holding.
	Held int64 `json:"held"`
}

func (a *Account) Load(ps []datastore.Property) error { return datastore.LoadStruct(a, ps) }

func (a *Account) Save() ([]datastore.Property, error) { return datastore.SaveStruct(a) }

// NewAccount returns an account bound to db. Like [New] it sets no ancestor:
// the tenant boundary is db's namespace.
func NewAccount(db *datastore.Datastore) *Account {
	a := new(Account)
	a.Init(db)
	return a
}

// Entry is one side of one judgement's position in the account: what it has
// taken, or what it has given back.
//
// BOTH NUMBERS ONLY EVER GROW. A hold row's Held rises as money is withheld; a
// return row's Released rises as it is given back; neither is ever rewritten
// downwards, so the ledger reads as the history it is. What one judgement is
// holding RIGHT NOW is the difference, and the account is the same difference
// summed over everything:
//
//	Account.Held  ==  Σ Entry.Held  −  Σ Entry.Released
//
// That equality is the whole reason the two are written in one transaction. A
// release returns the account, so a total the ledger cannot explain is money a
// merchant never gets back, and a ledger the total does not match is a number
// nobody can act on.
type Entry struct {
	mixin.Model[Entry]

	SubjectKind string        `json:"subjectKind"`
	Subject     string        `json:"subject"`
	Currency    currency.Type `json:"currency"`

	// Held and Released are exact minor units of Currency.
	Held     int64 `json:"held,omitempty"`
	Released int64 `json:"released,omitempty"`

	// Control is the reserve that took or returned it, and Screen the judgement
	// that decided the move. Together they join a shortfall on a payout back to
	// the declaration that caused it.
	Control string `json:"control"`
	Screen  string `json:"screen,omitempty"`

	// Reference is the money object the hold came out of — the payout, the
	// transfer — so a merchant reconciling a short payment finds this row by the
	// id on its own statement.
	Reference string `json:"reference,omitempty"`
}

func (e *Entry) Load(ps []datastore.Property) error { return datastore.LoadStruct(e, ps) }

func (e *Entry) Save() ([]datastore.Property, error) { return datastore.SaveStruct(e) }

// New returns an entry bound to db. It sets no ancestor: an entry is named by
// what it records (a deterministic string id), and a string-keyed row is stored
// under that id alone — the tenant boundary is db's namespace, which every read
// and write here inherits and none can widen.
func New(db *datastore.Datastore) *Entry {
	e := new(Entry)
	e.Init(db)
	return e
}

// Query is every read on this kind. Like screen.Query it filters by NO
// ANCESTOR — an ancestor-filtered read loses a row the moment anything updates
// it, and the tenant boundary is the datastore's namespace either way. The full
// reasoning lives on screen.Query; it is one property of one ORM and it is
// written down once.
func Query(db *datastore.Datastore) datastore.Query { return db.Query("risk-reserve") }

// Cause names the money one movement came out of: whose it is, what it is
// denominated in, which judgement decided it and which money object it belongs
// to. It is a value and not five positional strings because that is how a
// subject ends up in a reference.
type Cause struct {
	SubjectKind string
	Subject     string
	Currency    currency.Type
	// Screen is the judgement this movement belongs to. It NAMES the hold: a
	// retried disbursement under one judgement re-posts one row.
	Screen string
	// Reference is the money object — the payout, the transfer.
	Reference string
}

// ErrAmount refuses a movement that records no money. A zero movement is not a
// ledger row, it is noise in the account a merchant has to read.
var ErrAmount = errors.New("reserve: a movement records a positive amount")

// ErrControl refuses a movement against something that is not a reserve.
var ErrControl = errors.New("reserve: only a reserve holds money")

// ErrCause refuses a hold that names no judgement. A hold that cannot be joined
// back to the screen that decided it is the shortfall-with-no-name this whole
// package exists to prevent.
var ErrCause = errors.New("reserve: a hold names the judgement that took it")

// ErrStore refuses a movement the store cannot make INDIVISIBLE.
//
// It is a refusal and never a fallback. Without one transaction the total and
// the ledger are two writes that can half-land, and a read-modify-write on the
// total loses increments under concurrency — a reserve quietly withholding past
// its own declared ceiling while the money it took goes unrecorded. A
// disbursement that cannot be accounted for does not happen.
var ErrStore = errors.New("reserve: the store cannot account for this indivisibly")

// id derives a row's STORAGE id from what it records, so the same movement
// posted twice lands on one row rather than doubling the account.
func id(prefix, cause string) string {
	sum := sha256.Sum256([]byte(prefix + "\x00" + cause))
	return prefix + "_" + hex.EncodeToString(sum[:16])
}

// Take withholds up to want from ONE move against the reserve c, and reports
// EXACTLY what it withheld.
//
// It is THE ONE DOOR to the account and it belongs to the disbursement
// boundary. The account moves where the money moves, for the amount the money
// moved, and the amount the caller may disburse is what this leaves behind.
//
// THE CEILING IS ENFORCED HERE AND NOWHERE ELSE. The clamp is computed INSIDE
// the transaction from the total the store holds at that instant, so N
// concurrent disbursements against a ceiling of C withhold C in total and not
// N×C: the losers are granted the headroom that is left, which may be nothing.
// A judgement's own clamp ([Headroom] read outside a transaction) is a forecast
// and is deliberately not authoritative.
//
// It is IDEMPOTENT ON THE JUDGEMENT. A retried disbursement under one screen
// finds its own entry already posted and reports what that entry says, moving
// nothing — because the screen is idempotent too and the two must agree about
// how many times one move happened.
func Take(ds *datastore.Datastore, c *control.Control, want int64, cause Cause) (int64, error) {
	if c == nil || c.Effect != control.Reserve || c.Ref() == "" {
		return 0, ErrControl
	}
	if want <= 0 {
		return 0, ErrAmount
	}
	if cause.Screen == "" {
		return 0, ErrCause
	}

	var granted int64
	err := transact(ds, c, cause.Screen, func(tx db.Transaction, k keys) error {
		granted = 0

		position, hold, back, err := position(ds, tx, k, c, cause)
		if err != nil {
			return err
		}
		// Already accounted for under this judgement: report what it is holding
		// and take nothing more.
		if position > 0 {
			granted = position
			return nil
		}

		account, err := load(ds, tx, k, c)
		if err != nil {
			return err
		}
		// The clamp the judgement forecast with, against the total the store
		// holds RIGHT HERE. This one decides.
		granted = Grant(want, room(c, account.Held))
		if granted <= 0 {
			return nil
		}

		account.Held += granted
		if err := put(tx, k.account, account); err != nil {
			return err
		}
		hold.Held += granted
		_ = back
		return put(tx, k.hold, hold)
	})
	if err != nil {
		return 0, err
	}
	return granted, nil
}

// Return gives back what ONE judgement's hold took, because the move it was
// taken from did not happen after all.
//
// It exists so the account can follow the money in BOTH directions from the one
// boundary that knows whether money left. A disbursement that fails after its
// share was withheld would otherwise leave the merchant short by that share
// forever, under a ceiling that believes it is that much fuller.
//
// Idempotent on the judgement, like [Take]: one hold is returned once.
func Return(ds *datastore.Datastore, c *control.Control, screen string) (int64, error) {
	if c == nil || c.Effect != control.Reserve || c.Ref() == "" {
		return 0, ErrControl
	}
	if screen == "" {
		return 0, ErrCause
	}

	var returned int64
	err := transact(ds, c, screen, func(tx db.Transaction, k keys) error {
		returned = 0

		position, _, back, err := position(ds, tx, k, c, Cause{Screen: screen})
		if err != nil || position <= 0 {
			return err // nothing outstanding under this judgement
		}

		account, err := load(ds, tx, k, c)
		if err != nil {
			return err
		}
		returned = position
		if returned > account.Held {
			returned = account.Held
		}
		account.Held -= returned
		if err := put(tx, k.account, account); err != nil {
			return err
		}
		// The hold STAYS: it is what this judgement took, and that happened. The
		// return is its own row and its own fact, so the ledger reads as history
		// and the two sum to what the judgement is now holding, which is nothing.
		back.Released += returned
		return put(tx, k.back, back)
	})
	if err != nil {
		return 0, err
	}
	return returned, nil
}

// Close returns EVERYTHING a reserve is holding, because the declaration that
// took it has been lifted. It reports what the account gave back.
//
// The release entry is named by the DECLARATION, so a retried lift re-posts the
// same row rather than returning the money twice on paper — and because the
// account is zeroed inside the same transaction, a second Close returns nothing
// rather than the same amount again.
func Close(ds *datastore.Datastore, c *control.Control) (int64, error) {
	if c == nil || c.Effect != control.Reserve || c.Ref() == "" {
		return 0, ErrControl
	}

	var returned int64
	err := transact(ds, c, "", func(tx db.Transaction, k keys) error {
		returned = 0

		account, err := load(ds, tx, k, c)
		if err != nil {
			return err
		}
		if account.Held <= 0 {
			return nil
		}
		returned = account.Held
		account.Held = 0
		if err := put(tx, k.account, account); err != nil {
			return err
		}

		e := New(ds)
		if err := tx.Get(k.release, e); err != nil {
			if !errors.Is(err, datastore.ErrNoSuchEntity) {
				return err
			}
			e = New(ds)
			e.SetId(k.releaseID)
			e.SubjectKind = c.SubjectKind
			e.Subject = c.Subject
			e.Currency = c.Currency
			e.Control = c.Ref()
		}
		e.Released += returned
		return put(tx, k.release, e)
	})
	if err != nil {
		return 0, err
	}
	return returned, nil
}

// Held is what one reserve is holding now: ONE read of ONE row, which is what
// the money path needs and what a ceiling is measured against. A reserve that
// never took anything holds nothing and says so without an error.
//
// It answers about ONE declaration. A page of them asks [Accounts] once — see
// why there.
func Held(ds *datastore.Datastore, c *control.Control) int64 {
	if ds == nil || c == nil || c.Effect != control.Reserve || c.Ref() == "" {
		return 0
	}
	a := NewAccount(ds)
	if a.Get(ds.NewKey(a.Kind(), id("account", c.Ref()), 0, nil)) != nil {
		return 0
	}
	return a.Held
}

// Accounts is what every reserve in this tenant is holding, keyed by the
// declaration it belongs to. ONE read for a whole page.
//
// A BOUND ON ROWS IS NOT A BOUND ON READS. A list renders up to control.Max
// declarations, and asking the store for each row's total turns one request
// into five hundred round trips against a store shared with every other tenant
// — the same request amplification a page limit is supposed to prevent, moved
// one layer down. There is one account per reserve, so this read is bounded by
// the same [Max] every other read here is.
//
// A declaration with no entry in the result is holding nothing.
func Accounts(ds *datastore.Datastore) map[string]int64 {
	out := map[string]int64{}
	if ds == nil {
		return out
	}
	iter := ds.Query("risk-account").Order("-CreatedAt").Limit(Max).Run()
	for {
		a := NewAccount(ds)
		if _, err := iter.Next(a); err != nil {
			break
		}
		out[a.Control] = a.Held
	}
	return out
}

// Headroom is how much more a reserve may still withhold before it reaches the
// ceiling it declared. A reserve that declares none has [Unlimited] headroom;
// one that has reached its ceiling has none.
//
// It is a FORECAST. The authoritative headroom is the one [Take] computes
// inside the transaction — this one is read outside any transaction so a
// judgement can tell a caller what a payout would withhold, and it may be stale
// by the time the money moves.
func Headroom(ds *datastore.Datastore, c *control.Control) int64 {
	if c == nil || !c.Bounded() {
		return Unlimited
	}
	return room(c, Held(ds, c))
}

// room is the ceiling less what is already held, never negative — a ceiling
// LOWERED below what a reserve is already holding leaves no room, it does not
// owe the merchant a negative withholding.
func room(c *control.Control, held int64) int64 {
	if !c.Bounded() {
		return Unlimited
	}
	if left := c.Cap - held; left > 0 {
		return left
	}
	return 0
}

// Grant is what a reserve may take of want under headroom: the smaller of the
// two, never negative.
//
// IT IS THE ONLY CLAMP. The judgement forecasts with it against [Headroom] and
// the disbursement decides with it against the total the store holds inside its
// own transaction — same arithmetic, two headrooms, one authority. Written
// twice it would be two arithmetics, and a forecast that disagrees with the
// decision by a cent is a payout a merchant cannot reconcile.
func Grant(want, headroom int64) int64 {
	if headroom <= 0 || want <= 0 {
		return 0
	}
	if headroom < want {
		return headroom
	}
	return want
}

// For reads a subject's reserve ledger, newest first, up to limit — and never
// more than [Max]. The datastore is already namespaced to one org, which is the
// tenant boundary.
func For(db *datastore.Datastore, subjectKind, subject string, limit int) []*Entry {
	if limit <= 0 || limit > Max {
		limit = Max
	}
	q := Query(db)
	if subjectKind != "" {
		q = q.Filter("SubjectKind=", subjectKind)
	}
	if subject != "" {
		q = q.Filter("Subject=", subject)
	}

	out := []*Entry{}
	iter := q.Order("-CreatedAt").Limit(limit).Run()
	for {
		e := New(db)
		if _, err := iter.Next(e); err != nil {
			break
		}
		out = append(out, e)
	}
	return out
}

// keys are the storage keys one movement touches, resolved once so the
// transaction body names rows and not strings.
type keys struct {
	account   db.Key
	hold      db.Key
	holdID    string
	back      db.Key
	backID    string
	release   db.Key
	releaseID string
}

// position is what ONE judgement is holding right now — what it took less what
// it gave back — together with the two rows that say so, ready to write.
//
// It is the idempotency predicate for both directions: a judgement already
// holding something takes nothing more, and one holding nothing returns
// nothing. Rows that do not exist yet come back zeroed and named, because a
// judgement that has never moved money holds exactly nothing.
// A return INHERITS ITS IDENTITY FROM THE HOLD IT REVERSES rather than from the
// caller: the two rows are one judgement's two sides, and a return that named a
// different subject — or none, because the caller giving money back does not
// have to know whose it was — would be money returned into a ledger the
// merchant's own reads cannot see.
func position(ds *datastore.Datastore, tx db.Transaction, k keys, c *control.Control, cause Cause) (int64, *Entry, *Entry, error) {
	read := func(key db.Key, rowID string, from Cause) (*Entry, error) {
		e := New(ds)
		if err := tx.Get(key, e); err != nil {
			if !errors.Is(err, datastore.ErrNoSuchEntity) {
				return nil, err
			}
			e = New(ds)
			e.SetId(rowID)
			e.SubjectKind = from.SubjectKind
			e.Subject = from.Subject
			e.Currency = from.Currency
			e.Control = c.Ref()
			e.Screen = from.Screen
			e.Reference = from.Reference
		}
		return e, nil
	}
	hold, err := read(k.hold, k.holdID, cause)
	if err != nil {
		return 0, nil, nil, err
	}
	back, err := read(k.back, k.backID, Cause{
		SubjectKind: hold.SubjectKind,
		Subject:     hold.Subject,
		Currency:    hold.Currency,
		Screen:      hold.Screen,
		Reference:   hold.Reference,
	})
	if err != nil {
		return 0, nil, nil, err
	}
	return hold.Held - back.Released, hold, back, nil
}

// transact runs fn inside ONE store transaction with the keys of the rows it
// may touch, and REFUSES when the store cannot give it one.
//
// It is the ONLY place this package writes. Everything fn touches is
// TRANSACTION-scoped: nothing inside may reach the store any other way, because
// the SQLite backend holds its single write lock for the whole body and a write
// that went around the transaction would wait on a lock its own caller holds.
func transact(ds *datastore.Datastore, c *control.Control, screen string, fn func(db.Transaction, keys) error) error {
	if ds == nil || ds.DB() == nil {
		return ErrStore
	}
	store := ds.DB()
	k := keys{
		account:   store.NewKey("risk-account", id("account", c.Ref()), 0, nil),
		releaseID: id("release", c.Ref()),
	}
	k.release = store.NewKey("risk-reserve", k.releaseID, 0, nil)
	if screen != "" {
		k.holdID = id("hold", screen)
		k.hold = store.NewKey("risk-reserve", k.holdID, 0, nil)
		k.backID = id("back", screen)
		k.back = store.NewKey("risk-reserve", k.backID, 0, nil)
	}
	return store.RunInTransaction(ds.Context, func(tx db.Transaction) error {
		return fn(tx, k)
	}, &db.TransactionOptions{Isolation: db.IsolationSerializable, MaxAttempts: attempts})
}

// attempts bounds how many times a store may retry the whole movement when it
// refuses to serialize two of them. It is small on purpose: a reserve under
// this much contention should refuse and be retried by the caller, not spin.
const attempts = 5

// load reads the account inside the transaction. An account that does not exist
// yet holds nothing and comes back ready to write.
func load(ds *datastore.Datastore, tx db.Transaction, k keys, c *control.Control) (*Account, error) {
	a := NewAccount(ds)
	if e := tx.Get(k.account, a); e != nil {
		if !errors.Is(e, datastore.ErrNoSuchEntity) {
			return nil, e
		}
		a = NewAccount(ds)
		a.SetId(id("account", c.Ref()))
		a.Control = c.Ref()
		a.Currency = c.Currency
	}
	return a, nil
}

// put writes one row through the transaction, stamping it exactly as the
// entity's own Put would so a transacted row and a put row are the same bytes.
func put(tx db.Transaction, k db.Key, e interface{ stamp(time.Time) }) error {
	e.stamp(time.Now())
	_, err := tx.Put(k, e)
	return err
}

func (a *Account) stamp(now time.Time) {
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	a.UpdatedAt = now
}

func (e *Entry) stamp(now time.Time) {
	if e.CreatedAt.IsZero() {
		e.CreatedAt = now
	}
	e.UpdatedAt = now
}
