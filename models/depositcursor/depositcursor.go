// Package depositcursor persists the crypto deposit watcher's scan position:
// the last block fully committed for one (chain, token) asset. It lets a
// restart resume rather than rescan — and, more importantly, rather than SKIP:
// without it every restart would begin at the chain head and every deposit made
// while the process was down would be money received and never credited.
//
// It is a singleton per asset (deterministic storage id), lives in the global
// "system" namespace beside the HUSD indexer's cursor, and carries no money —
// it is pure scanner bookkeeping.
package depositcursor

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/orm"
)

func init() {
	orm.Register[DepositCursor]("deposit-cursor", orm.WithStringKey[DepositCursor]())
}

// DepositCursor is the watcher's last fully-committed block for one asset.
type DepositCursor struct {
	mixin.Model[DepositCursor]

	Chain     string `json:"chain"`
	Token     string `json:"token"`
	LastBlock uint64 `json:"lastBlock"`
}

func (c *DepositCursor) Load(ps []datastore.Property) error  { return datastore.LoadStruct(c, ps) }
func (c *DepositCursor) Save() ([]datastore.Property, error) { return datastore.SaveStruct(c) }

// New returns an initialized DepositCursor bound to db.
func New(db *datastore.Datastore) *DepositCursor {
	c := new(DepositCursor)
	c.Init(db)
	return c
}
