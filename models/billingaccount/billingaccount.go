// Package billingaccount is the top-level, tenant-independent money-of-record
// primitive. A BillingAccount is the unit balance is recorded against; entities
// (users, orgs, projects) BIND to accounts (Binding) so a debit walks an ordered
// chain and falls to the next account when one cannot cover it.
//
// The account's stable string Id IS its ledger subject key: the append-only
// commerce ledger keys every row on transaction.SourceId/DestinationId, and a
// vaulted card keys on paymentmethod.CustomerId — so an account's money and its
// instruments are exactly the rows carrying its id. Balance is the derived sum
// over that ledger (models/transaction/util), never a mutable field here. This
// entity carries only administrative metadata (who administers it, display name,
// currency, upstream processor handles); it never stores an amount.
package billingaccount

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/orm"
)

// Kind is the datastore kind for a billing account.
const Kind = "billing-account"

func init() {
	orm.Register[BillingAccount](Kind, orm.WithStringKey[BillingAccount]())
}

// Holder/owner kinds — the ONE vocabulary shared by a binding's HolderKind and an
// account's OwnerKind, so the two can never name entity classes differently.
const (
	HolderUser    = "user"
	HolderOrg     = "org"
	HolderProject = "project"
)

// BillingAccount is one account money is recorded against. Its Id is a free-form
// stable string that doubles as the ledger subject key. Accounts minted fresh
// take an opaque NewAccountID ("acct_…"); an account backfilled for an existing
// tenant instead ADOPTS that tenant's current subject string as its id, so the
// append-only ledger and vaulted cards carry forward with no migration.
type BillingAccount struct {
	mixin.Model[BillingAccount]

	DisplayName string `json:"displayName,omitempty"`

	// OwnerKind/OwnerId name the entity that administers this account (who may
	// bind it, add instruments, read its ledger). It is the account's steward,
	// NOT a scoping key — the money is keyed on the account Id.
	OwnerKind string `json:"ownerKind"`
	OwnerId   string `json:"ownerId"`

	Currency currency.Type `json:"currency" orm:"default:usd"`

	// ProviderType/ProviderRef are the upstream processor's handle for this
	// account (e.g. a Square/Stripe customer), when one has been provisioned.
	ProviderType string `json:"providerType,omitempty"`
	ProviderRef  string `json:"providerRef,omitempty"`
}

func (a *BillingAccount) Load(ps []datastore.Property) error  { return datastore.LoadStruct(a, ps) }
func (a *BillingAccount) Save() ([]datastore.Property, error) { return datastore.SaveStruct(a) }

// New returns an initialized BillingAccount bound to db.
func New(db *datastore.Datastore) *BillingAccount {
	a := new(BillingAccount)
	a.Init(db)
	return a
}

// Query returns a datastore query over billing accounts in db's namespace.
func Query(db *datastore.Datastore) datastore.Query { return db.Query(Kind) }

// Get loads the account with the given stable id from db's namespace, or returns
// datastore.ErrNoSuchEntity when absent. The read is key-exact (kind-qualified)
// so it round-trips on both the SQLite and Postgres backends.
func Get(db *datastore.Datastore, id string) (*BillingAccount, error) {
	a := New(db)
	if err := a.Get(db.NewKey(Kind, id, 0, nil)); err != nil {
		return nil, err
	}
	return a, nil
}

// NewAccountID mints an opaque, stable id for an account created without a
// pre-existing ledger subject to adopt. It is deliberately NOT derived from any
// tenant slug: an account is tenant-independent, so its identity must not encode
// where it happens to be bound today.
func NewAccountID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return "acct_" + hex.EncodeToString(b[:])
}
