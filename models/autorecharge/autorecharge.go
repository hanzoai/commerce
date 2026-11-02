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
	// Txn is the deposit transaction of the last reload the rule recorded.
	Txn string `json:"txn,omitempty"`
	// Seq is how many reloads this rule has completed, and After the payer's
	// available balance once the last of them was credited: the balance the
	// trigger was read at plus the amount reloaded. A balance below the threshold
	// is a new trigger only when it is also below After — money was spent since
	// that reload — and the trigger's reload carries Seq in its idempotency key, so
	// one trigger is charged once however many sweeps see it.
	Seq   int64 `json:"seq,omitempty"`
	After int64 `json:"after,omitempty"`
	// Tries is how many times the processor refused the reload for the trigger Seq
	// names; each try is a key of its own, so a refused card is asked again rather
	// than answered from the refusal. Declines counts the refusals on Day.
	Tries    int `json:"tries,omitempty"`
	Declines int `json:"declines,omitempty"`
	// Amount and Card are the open trigger's charge, fixed when it is reserved:
	// every ask of the trigger sends exactly that amount on exactly that card under
	// the same key until the processor answers, whatever the rule or the default
	// card has become since. Base is the available balance the trigger was read at.
	// Amount is 0 when no trigger is open.
	Amount int64  `json:"amount,omitempty"`
	Card   string `json:"card,omitempty"`
	Base   int64  `json:"base,omitempty"`
	// Ref is the processor's reference of the open trigger's charge once it has
	// settled, until its credit lands; the sweep credits it before anything else.
	Ref string `json:"ref,omitempty"`
	// Plan is the plan whose reload switched this rule on, or awaits a card to
	// (Waiting): a rule the payer set themselves names none. Cancelling the plan, or
	// moving to one with no reload, switches a rule a plan switched on off.
	Plan    string `json:"plan,omitempty"`
	Waiting bool   `json:"waiting,omitempty"`
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
