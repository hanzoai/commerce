package referrer

import (
	"time"

	aeds "appengine/datastore"

	"crowdstart.com/datastore"
	"crowdstart.com/models/affiliate"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/referral"
	"crowdstart.com/models/referralprogram"
	"crowdstart.com/models/transaction"
	"crowdstart.com/models/types/client"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/json"
	"crowdstart.com/util/log"
	"crowdstart.com/util/timeutil"

	. "crowdstart.com/models"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

// Is a link that can refer customers to buy products
type Referrer struct {
	mixin.Model

	Code      string  `json:"code"`
	Program   Program `json:"program"`
	ProgramId string  `json:"programId"`
	OrderId   string  `json:"orderId"`
	UserId    string  `json:"userId"`

	AffiliateId     string              `json:"affiliateId,omitempty"`
	Affiliate       affiliate.Affiliate `json:"affiliate,omitempty" datastore:"-"`
	FirstReferredAt time.Time           `json:"firstReferredAt"`

	Client      client.Client `json:"-"`
	Blacklisted bool          `json:"blacklisted,omitempty"`
	Duplicate   bool          `json:"duplicate,omitempty"`

	State  Map    `json:"state,omitempty" datastore:"-"`
	State_ string `json:"-" datastore:",noindex"`
}

type Referrent interface {
	Id() string
	Kind() string
}

func (r *Referrer) Save(c chan<- aeds.Property) (err error) {
	// Serialize unsupported properties
	r.State_ = string(json.EncodeBytes(&r.State))

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(r, c))
}

func (r *Referrer) Load(c <-chan aeds.Property) (err error) {
	// Ensure we're initialized
	r.Defaults()

	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(r, c)); err != nil {
		return err
	}

	if len(r.State_) > 0 {
		err = json.DecodeBytes([]byte(r.State_), &r.State)
	}

	return err
}

func (r *Referrer) SaveReferral(typ referral.Type, rfn Referrent) (*referral.Referral, error) {
	log.Debug("Creating referral")
	// Create new referral
	rfl := referral.New(r.Db)
	rfl.Type = typ
	rfl.Referrer.Id = r.Id()
	rfl.Referrer.AffiliateId = r.AffiliateId
	rfl.Referrer.UserId = r.UserId

	// Save referrent's id
	switch rfn.Kind() {
	case "order":
		log.Debug("Saving referral for new order")
		rfl.OrderId = rfn.Id()
	case "user":
		log.Debug("Saving referral for new user")
		rfl.UserId = rfn.Id()
	}

	log.JSON("Saving referral", rfl)

	// Try to save referral
	if err := rfl.Create(); err != nil {
		return rfl, err
	}

	// If this is the first referral, update referrer
	if timeutil.IsZero(r.FirstReferredAt) {
		r.FirstReferredAt = time.Now()
		r.Update()
	}

	if r.ProgramId != "" {
		prog := referralprogram.New(r.Db)
		if err := prog.GetById(r.ProgramId); err != nil {

		}
	}

	// Apply any program actions if they are configured
	if len(r.Program.Actions) > 0 {
		if err := r.Program.ApplyActions(r); err != nil {
			return rfl, err
		}
	}

	return rfl, nil
}

func (r *Referrer) LoadAffiliate() error {
	if r.AffiliateId == "" {
		return nil
	}

	aff := affiliate.New(r.Db)

	if err := aff.GetById(r.AffiliateId); err != nil {
		return err
	}

	r.Affiliate = *aff

	return nil
}

func (r *Referrer) Referrals() ([]*referral.Referral, error) {
	referrals := make([]*referral.Referral, 0)
	_, err := referral.Query(r.Db).Filter("ReferrerId=", r.Id()).GetAll(referrals)
	return referrals, err
}

func (r *Referrer) Transactions() ([]*transaction.Transaction, error) {
	transactions := make([]*transaction.Transaction, 0)
	_, err := transaction.Query(r.Db).Filter("ReferrerId=", r.Id()).GetAll(transactions)
	return transactions, err
}

// Referral Program stuff

func (r *Referrer) TestTrigger(p *Program) error {
	switch p.Trigger.Type {
	case CreditGreaterThan:
		return nil
	case ReferralsGreaterThan:
		return nil
	}

	return nil
}

func (r *Referrer) ApplyActions(p *Program) error {
	for i, _ := range p.ReferralTriggers {
		action := p.Actions[i]
		switch action.Type {
		case StoreCredit:
			return saveStoreCredit(r, action.Amount, action.Currency)
		case Refund:
		}
	}

	// No actions triggered for this referral
	return nil
}

// Credit user with store credit by saving transaction
func saveStoreCredit(r *Referrer, amount currency.Cents, cur currency.Type) error {
	trans := transaction.New(r.Db)
	trans.Type = transaction.Deposit
	trans.Amount = amount
	trans.Currency = cur
	trans.SourceId = r.Id()
	trans.SourceKind = r.Kind()
	trans.UserId = r.UserId
	trans.Notes = "Deposit due to referral"
	trans.Tags = "referral"
	return trans.Create()
}
