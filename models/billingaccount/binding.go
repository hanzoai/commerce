package billingaccount

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/orm"
)

// BindingKind is the datastore kind for a holder→account binding.
const BindingKind = "billing-binding"

func init() {
	orm.Register[Binding](BindingKind, orm.WithStringKey[Binding]())
}

// Binding maps a holder (a user, org, or project) to a billing account with a
// priority. A holder's bindings, ordered by ascending Priority (lower is charged
// first), are the ordered chain a debit walks — falling to the next account when
// one cannot cover the debit or its card is declined. A holder may bind many
// accounts (redundancy, personal overflow); an account may be bound by many
// holders.
//
// The row id is DETERMINISTIC in (HolderKind, HolderId, AccountId), so re-binding
// the same pair upserts (e.g. to change Priority) rather than forking a duplicate
// — the property the idempotent backfill relies on.
type Binding struct {
	mixin.Model[Binding]

	HolderKind string `json:"holderKind"`
	HolderId   string `json:"holderId"`
	AccountId  string `json:"accountId"`
	Priority   int    `json:"priority"`
}

func (b *Binding) Load(ps []datastore.Property) error  { return datastore.LoadStruct(b, ps) }
func (b *Binding) Save() ([]datastore.Property, error) { return datastore.SaveStruct(b) }

// NewBinding returns an initialized Binding bound to db.
func NewBinding(db *datastore.Datastore) *Binding {
	b := new(Binding)
	b.Init(db)
	return b
}

// BindingQuery returns a datastore query over bindings in db's namespace.
func BindingQuery(db *datastore.Datastore) datastore.Query { return db.Query(BindingKind) }

// BindingID derives the stable storage id for a (holderKind, holderId, accountId)
// triple. Same inputs → same id → a re-bind upserts onto the one row via the
// storage ON CONFLICT(id, kind, namespace), so the backfill is idempotent.
func BindingID(holderKind, holderId, accountId string) string {
	sum := sha256.Sum256([]byte(holderKind + "\x00" + holderId + "\x00" + accountId))
	return "bnd_" + hex.EncodeToString(sum[:16])
}

// Bind idempotently records that holder (holderKind, holderId) draws on accountId
// at the given priority, in db's namespace. Re-binding the same pair updates the
// priority in place (deterministic id + upsert). It returns the persisted Binding.
func Bind(db *datastore.Datastore, holderKind, holderId, accountId string, priority int) (*Binding, error) {
	b := NewBinding(db)
	b.SetId(BindingID(holderKind, holderId, accountId))
	b.HolderKind = holderKind
	b.HolderId = holderId
	b.AccountId = accountId
	b.Priority = priority
	if err := b.Put(); err != nil {
		return nil, err
	}
	return b, nil
}

// ForHolder returns holder (holderKind, holderId)'s bindings in db's namespace,
// ascending by Priority (the charge order). Ties break on AccountId so the order
// is TOTAL and deterministic. An empty holderId yields no rows. Ordering is done
// in Go, not via a datastore Order clause, so the money-critical charge order is
// identical on every backend (SQLite/Postgres/in-memory).
func ForHolder(db *datastore.Datastore, holderKind, holderId string) ([]*Binding, error) {
	if holderId == "" {
		return nil, nil
	}
	out := make([]*Binding, 0)
	_, err := BindingQuery(db).
		Filter("HolderKind=", holderKind).
		Filter("HolderId=", holderId).
		GetAll(&out)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority < out[j].Priority
		}
		return out[i].AccountId < out[j].AccountId
	})
	return out, nil
}
