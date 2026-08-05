package fee

import (
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/blockchains"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/commission"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/models/types/schedule"
	"github.com/hanzoai/orm"
)

func init() { orm.Register[Fee]("fee") }

type Type string

const (
	Platform    Type = "platform"
	Affiliate   Type = "affiliate"
	Partner     Type = "partner"
	Contributor Type = "contributor"
)

type Status string

const (
	Pending  Status = "pending"  // accrued, still inside the clawback buffer
	Disputed Status = "disputed" // held; not payable
	Payable  Status = "payable"  // matured; owed and payable now
	Refunded Status = "refunded" // the charge behind it came back
)

type Bitcoin struct {
	FinalTransactionTxId string         `json:"finalTransactionTxId,omitempty"`
	FinalAddress         string         `json:"finalAddress,omitempty"`
	FinalAmount          currency.Cents `json:"finalAmount,omitempty"`
	FinalVOut            int64          `json:"vout,omitempty"`
}

type Ethereum struct {
	FinalTransactionHash string                `json:"finalTransactionHash,omitempty"`
	FinalTransactionCost blockchains.BigNumber `json:"finalTransactionCost,omitempty"`
	FinalAddress         string                `json:"finalAddress,omitempty"`
	FinalAmount          blockchains.BigNumber `json:"finalAmount,omitempty"`
}

type Fee struct {
	mixin.Model[Fee]

	Name string `json:"name"`

	// Type names WHY we owe (which program earned it); PayeeId names WHO. One
	// payee reference, not one field per program — the program is a field on the
	// payable, not a shape of it.
	Type    Type   `json:"type"`
	PayeeId string `json:"payeeId,omitempty"`

	// PaymentId is the event that earned it — the charge or usage transaction.
	PaymentId string `json:"paymentId"`

	Commission commission.Commission `json:"commission,omitempty"`

	Currency       currency.Type  `json:"currency"`
	Amount         currency.Cents `json:"amount"`
	AmountRefunded currency.Cents `json:"amountRefunded,omitempty"`

	Status Status `json:"status" orm:"default:pending"`

	Ethereum Ethereum `json:"ethereum"`
	Bitcoin  Bitcoin  `json:"bitcoin"`

	// Stripe livemode
	Live bool `json:"live"`

	// Internal testing flag
	Test bool `json:"-"`
}

// Owed is what this payable accrued, as money.
func (f *Fee) Owed() currency.Money { return currency.Exact(f.Currency.Amount(f.Amount)) }

// DefaultSchedule is the clawback buffer a payable matures through: 30 days
// daily-rolling, the same default affiliate.New seeds.
var DefaultSchedule = schedule.Schedule{Type: schedule.DailyRolling, Period: 30}

// Qualify is the Pending -> Payable transition, and the ONE definition of when
// a commission has matured.
//
// A commission is EARNED the moment the referee spends, but it is not yet ours
// to pay: the charge behind it can still be refunded or charged back. It becomes
// PAYABLE once it is older than the clawback buffer. That buffer is exactly what
// schedule.Schedule already computes and what the payout crons already filtered
// on — this supplies the transition they were waiting for.
//
// Nothing in the repo ever wrote Payable while four consumers filtered on it, so
// every commission accrued Pending and was structurally unpayable. Returns the
// number of fees matured.
func Qualify(db *datastore.Datastore, s schedule.Schedule, now time.Time) (int, error) {
	if s.Type == "" {
		s = DefaultSchedule
	}
	cutoff := s.Cutoff(now)

	fees := make([]*Fee, 0)
	keys, err := Query(db).Filter("Status=", Pending).Filter("CreatedAt<", cutoff).GetAll(&fees)
	if err != nil {
		return 0, err
	}

	var n int
	for i, f := range fees {
		// A query-loaded entity carries neither a datastore nor its key, so
		// Update() would allocate a NEW key and write a duplicate row. Rebind
		// (not Init — Init re-applies Defaults() over the loaded values) plus the
		// key the query already returned makes it an update in place.
		f.Rebind(db)
		if err := f.SetKey(keys[i]); err != nil {
			return n, err
		}
		f.Status = Payable
		if err := f.Update(); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// New creates a new Fee wired to the given datastore.
func New(db *datastore.Datastore) *Fee {
	f := new(Fee)
	f.Init(db)
	return f
}

// Query returns a datastore query for fees.
func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("fee")
}
