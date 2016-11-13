package referrer

import (
	"errors"
	"fmt"
	"time"

	"appengine"
	aeds "appengine/datastore"

	"crowdstart.com/datastore"
	"crowdstart.com/models/affiliate"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/referral"
	"crowdstart.com/models/referralprogram"
	"crowdstart.com/models/transaction"
	"crowdstart.com/models/types/client"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/delay"
	"crowdstart.com/util/json"
	"crowdstart.com/util/log"
	"crowdstart.com/util/timeutil"

	. "crowdstart.com/models"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

// Is a link that can refer customers to buy products
type Referrer struct {
	mixin.Model

	Code      string                          `json:"code"`
	Program   referralprogram.ReferralProgram `json:"program"`
	ProgramId string                          `json:"programId"`
	OrderId   string                          `json:"orderId"`
	UserId    string                          `json:"userId"`

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

func (r *Referrer) SaveReferral(ctx appengine.Context, orgId string, event referral.Event, rfn Referrent) (*referral.Referral, error) {
	log.Debug("Creating referral")
	// Create new referral
	rfl := referral.New(r.Db)
	rfl.Type = event
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
			return rfl, err
		}
		r.Program = *prog
	}

	// Apply any program actions if they are configured
	if r.Program.Trigger.Type == "" {
		log.Debug("Old Triggers")
		// Deprecate this soon
		if len(r.Program.Actions) > 0 {
			if err := r.ApplyActions(ctx, orgId, &r.Program); err != nil {
				return rfl, err
			}
		}
	} else {
		log.Debug("New Triggers")
		if ok, err := r.TestTrigger(&r.Program, event); ok {
			if err != nil {
				return rfl, err
			}
			if err := r.ApplyActions(ctx, orgId, &r.Program); err != nil {
				return rfl, err
			}
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

func (r *Referrer) TestTrigger(p *referralprogram.ReferralProgram, event referral.Event) (bool, error) {
	if p.Trigger.Event != "" && event != p.Trigger.Event {
		log.Debug("Event mismatch '%s' != '%s'", event, p.Trigger.Event)
		return false, nil
	}

	switch p.Trigger.Type {
	case referralprogram.CreditGreaterThan:
		log.Debug("CreditGreaterThan Trigger")
		// Get all transactions
		trans := make([]*transaction.Transaction, 0)
		if _, err := transaction.Query(r.Db).Filter("UserId=", r.UserId).Filter("Currency=", p.Trigger.Currency).Filter("Test=", false).GetAll(&trans); err != nil {
			return false, err
		}

		// Total balance
		balance := 0
		for _, t := range trans {
			if t.Type == transaction.Withdraw {
				balance -= int(t.Amount)
			} else {
				balance += int(t.Amount)
			}
		}

		// 'Forward' any balance increments from this trigger executing
		for _, action := range p.Actions {
			log.Debug("Looking at actions with credit to forward '%s': '%s' ? '%s'", action.Type, action.Currency, p.Trigger.Currency)
			if action.Type == referralprogram.StoreCredit && action.Currency == p.Trigger.Currency {
				done, ok := r.State[action.Name+"_done"].(bool)
				if action.Once && ok && done {
					log.Debug("Don't forward since this was executed once")
					continue
				}
				balance += int(action.Amount)
				log.Debug("Balance Amount %s", balance)
			}
		}

		// Check trigger
		if balance > p.Trigger.CreditGreaterThan {
			return true, nil
		}
	case referralprogram.ReferralsGreaterThan:
		log.Debug("ReferralsGreaterThan Trigger")

		// Count number of referrals
		if count, err := referral.Query(r.Db).Filter("Referrer.Id=", r.Id()).Count(); err != nil {
			return false, err
			// Check trigger
		} else if count > p.Trigger.ReferralsGreaterThan {
			return true, nil
		}
		return false, nil
	default:
		log.Debug("Unknown Trigger")
		return false, errors.New(fmt.Sprintf("Unknown Trigger '%s'", p.Trigger.Type))
	}

	return false, nil
}

func (r *Referrer) ApplyActions(ctx appengine.Context, orgId string, p *referralprogram.ReferralProgram) error {
	for _, action := range p.Actions {
		// Only execute if state isn't done
		done, ok := r.State[action.Name+"_done"].(bool)
		if action.Once && ok && done {
			log.Debug("This was executed once")
			continue
		}

		switch action.Type {
		case referralprogram.StoreCredit:
			if !done && action.Once {
				r.State[action.Name+"_done"] = true
				r.MustUpdate()
			}

			return saveStoreCredit(r, action.Amount, action.Currency)
		// case referralprogram.Refund:
		// 	return nil
		case referralprogram.SendUserEmail:
			if !done && action.Once {
				r.State[action.Name+"_done"] = true
				r.MustUpdate()
			}

			fn := delay.FuncByKey("referrer-send-user-email")
			fn.Call(ctx, orgId, action.EmailTemplate, r.UserId)
			return nil
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
