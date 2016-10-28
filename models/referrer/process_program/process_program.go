package process_program

import (
	"crowdstart.com/models/organization"
	"crowdstart.com/models/referrer"
	"crowdstart.com/models/transaction"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/models/user"
	"crowdstart.com/util/emails"
)

func ApplyActions(org *organization.Organization, p *referrer.Program, r *referrer.Referrer, alreadyRewarded int, newReferrals int) []ActionError {
	var ret []ActionError
	
	if (newReferrals <= alreadyRewarded) {
		return ret
	}

	for _, a := range p.Actions {
		tr := a.Trigger
		if tr.MinReferrals == 0 || (newReferrals >= tr.MinReferrals && tr.MinReferrals > alreadyRewarded) {
			var err error
			switch a.Type {
			case referrer.StoreCredit:
				err = saveStoreCredit(r, a.Amount, a.Currency)
			case referrer.EmailUser:
				usr := user.New(r.Db)
				err = usr.GetById(r.UserId)
				if err == nil {
					emails.SendReferralUpgradeEmail(r.Context(), usr, org)
				}
				
			case referrer.Refund:
			}
			if err != nil {
				ret = append(ret, ActionError{a, err})
			}
		}
	}

	return ret
}

type ActionError struct {
	Action referrer.Action
	Error error
}

// Credit user with store credit by saving transaction
func saveStoreCredit(r *referrer.Referrer, amount currency.Cents, cur currency.Type) error {
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
