package referrer

import (
	aeds "appengine/datastore"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/referral"
	"crowdstart.com/models/transaction"
	"crowdstart.com/util/val"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Referrer struct {
	mixin.Model

	Referral         referral.Referral         `json:"referral"`
	OrderId          string                    `json:"orderId"`
	UserId           string                    `json:"userId"`
	ReferredOrderIds []string                  `json:"referredOrderIds"`
	TransactionIds   []string                  `json:"transactionsIds"`
	Transactions     []transaction.Transaction `json:"transactions,omitempty"`
}

func New(db *datastore.Datastore) *Referrer {
	r := new(Referrer)
	r.Init()
	r.Model = mixin.Model{Db: db, Entity: r}
	return r
}

func (r Referrer) Init() {
	r.ReferredOrderIds = make([]string, 0)
	r.TransactionIds = make([]string, 0)
}

func (r Referrer) Kind() string {
	return "referral"
}

func (r *Referrer) Load(c <-chan aeds.Property) (err error) {
	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(r, c)); err != nil {
		return err
	}

	return err
}

func (r *Referrer) Save(c chan<- aeds.Property) (err error) {
	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(r, c))
}

func (r *Referrer) Validator() *val.Validator {
	return nil
}

func (r *Referrer) ApplyBonus() (*transaction.Transaction, error) {
	trans := transaction.New(r.Db)
	r.Referral.GetBonus(trans, len(r.ReferredOrderIds))
	trans.UserId = r.UserId
	if err := trans.Put(); err != nil {
		return nil, err
	}
	r.TransactionIds = append(r.TransactionIds, trans.Id())
	if err := r.Put(); err != nil {
		return nil, err
	}

	return trans, nil
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
