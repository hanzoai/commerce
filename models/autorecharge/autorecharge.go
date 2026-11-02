package autorecharge

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/val"
	"github.com/hanzoai/orm"
)

func init() { orm.Register[AutoRecharge]("auto-recharge") }

// AutoRecharge is one payer's automatic top-up rule. When the payer's available
// balance drops below ThresholdCents, the recharge sweep charges the payer's
// default payment method AmountCents and credits the payer's balance. One record
// per payer, keyed by UserId: the payer's billing subject — the org slug for the
// org's own account, "<org>/<name>" for a person who pays from a wallet of their
// own — the key topup and balance use.
type AutoRecharge struct {
	mixin.Model[AutoRecharge]

	UserId         string `json:"userId"` // payer subject (billing key)
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
	// Seq is how many reloads this rule has completed, and After the payer's
	// available balance read once the last of them was credited. A balance below
	// the threshold is a new trigger only when it is also below After — money was
	// spent since that reload — and the trigger's reload carries Seq in its
	// idempotency key, so one trigger is charged once however many sweeps see it.
	Seq   int64 `json:"seq,omitempty"`
	After int64 `json:"after,omitempty"`
	// Tries is how many times the processor declined the reload for the trigger
	// Seq names; each try is a key of its own, so a declined card is asked again
	// rather than answered from the decline. Declines counts the declines on Day.
	Tries    int `json:"tries,omitempty"`
	Declines int `json:"declines,omitempty"`
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
