package autorecharge

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/val"
	"github.com/hanzoai/orm"
)

func init() { orm.Register[AutoRecharge]("auto-recharge") }

// AutoRecharge is a per-org automatic top-up rule. When the org's available
// balance (balance minus holds) drops below ThresholdCents, the recharge cron
// charges the org's default payment method AmountCents and credits the balance.
// Mirrors the Claude/OpenAI "auto-reload" setting. One record per org (keyed by
// UserId = the org slug, which is the billing key used by topup/balance).
type AutoRecharge struct {
	mixin.Model[AutoRecharge]

	UserId         string `json:"userId"` // org slug (billing key)
	Enabled        bool   `json:"enabled"`
	ThresholdCents int64  `json:"thresholdCents"`
	AmountCents    int64  `json:"amountCents"`
	Currency       string `json:"currency"`

	// LastRechargedAt stores the ISO timestamp of the last successful auto
	// top-up — used for observability and as a light guard against a runaway
	// recharge loop within a single cron window.
	LastRechargedAt string `json:"lastRechargedAt,omitempty"`

	// Day is the UTC date ("2006-01-02") Reloads counts, and Reloads how many
	// reloads charged the card on that day. The sweep stops at its daily limit.
	Day     string `json:"day,omitempty"`
	Reloads int    `json:"reloads,omitempty"`
	// Txn is the transaction of the last reload counted, so a replayed charge
	// (the same reload answered twice) is counted once.
	Txn string `json:"txn,omitempty"`
}

func (a *AutoRecharge) Validator() *val.Validator {
	return val.New()
}

func New(db *datastore.Datastore) *AutoRecharge {
	a := new(AutoRecharge)
	a.Init(db)
	a.Parent = db.NewKey("synckey", "", 1, nil)
	return a
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("auto-recharge")
}
