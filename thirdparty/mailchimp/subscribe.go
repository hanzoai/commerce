package mailchimp

import (
	"github.com/zeekay/gochimp3"

	"hanzo.io/log"
	// "hanzo.io/models/form"
	// "hanzo.io/models/subscriber"
	// "hanzo.io/models/types/client"
	"hanzo.io/types/email"
	"hanzo.io/util/json"

	. "hanzo.io/types"
)

func (api API) Subscribe(list *email.List, sub *email.Subscriber) error {
	c := api.Context

	return wrapError(func() error {
		if list.Id == "" {
			log.Warn("Failed to subscribe, list ID missing: %v", list, c)
			return nil
		}

		mcl, err := api.Client.GetList(list.Id, nil)
		if err != nil {
			log.Error("Failed to subscribe %v: %v", sub, err, c)
			return err
		}

		status := "subscribed"
		if list.DoubleOptin {
			status = "pending"
		}

		req := &gochimp3.MemberRequest{
			EmailAddress: sub.Email.Address,
			Status:       status,
			StatusIfNew:  status,
			MergeFields:  sub.Metadata,
			Interests:    make(map[string]bool),
			// Language:     sub.Client.Language,
			VIP: false,
			Location: &gochimp3.MemberLocation{
				Latitude:  0.0,
				Longitude: 0.0,
				GMTOffset: 0,
				DSTOffset: 0,
				// CountryCode: s.Client.Country,
				Timezone: "",
			},
		}

		log.Info("Update or create list member: %v", json.Encode(req), c)

		// Try to update subscriber, create new member if that fails.
		log.Info("Update list member '%v'", sub.Email, c)
		if _, err := mcl.UpdateMember(sub.Md5(), req); err != nil {
			log.Info("Create list member '%v'", sub.Email, c)
			if _, err := mcl.CreateMember(req); err != nil {
				log.Info("Failed create Mailchimp list member '%v': %v", sub.Email, json.Encode(err), c)
				return err
			}
		}

		return nil
	})
}

func (api API) SubscribeCustomer(listId string, buy Buyer, referralUrl string) *Error {
	return wrapError(func() error {
		// f := new(form.Form)
		// f.EmailList.Id = listId
		// s := &subscriber.Subscriber{
		// 	Email:  buy.Email,
		// 	UserId: idOrEmail(buy.UserId, buy.Email),
		// 	Client: client.Client{
		// 		Country: buy.BillingAddress.Country,
		// 	},
		// 	Metadata: Map{
		// 		"FNAME":    buy.FirstName,
		// 		"LNAME":    buy.LastName,
		// 		"ADDRESS1": buy.BillingAddress.Line1,
		// 		"ADDRESS2": buy.BillingAddress.Line2,
		// 		"CITY":     buy.BillingAddress.City,
		// 		"STATE":    buy.BillingAddress.State,
		// 		"POSTAL":   buy.BillingAddress.PostalCode,
		// 		"COUNTRY":  buy.BillingAddress.Country,
		// 		"PHONE":    buy.Phone,
		// 		"REFERRAL": referralUrl,
		// 	},
		// }
		// return nil
		list := &email.List{
			Id: listId,
		}
		sub := &email.Subscriber{
			Email: email.Email{
				Address: buy.Email,
			},
			Metadata: map[string]interface{}{
				"FNAME":    buy.FirstName,
				"LNAME":    buy.LastName,
				"ADDRESS1": buy.BillingAddress.Line1,
				"ADDRESS2": buy.BillingAddress.Line2,
				"CITY":     buy.BillingAddress.City,
				"STATE":    buy.BillingAddress.State,
				"POSTAL":   buy.BillingAddress.PostalCode,
				"COUNTRY":  buy.BillingAddress.Country,
				"PHONE":    buy.Phone,
				"REFERRAL": referralUrl,
			},
		}
		return api.Subscribe(list, sub)
	})
}

func (api API) SubscribeForm(listId, emailStr, firstName, lastName string) *Error {
	return wrapError(func() error {
		// f := new(form.Form)
		// f.EmailList.Id = listId
		// s := &subscriber.Subscriber{
		// 	Email:  buy.Email,
		// 	UserId: idOrEmail(buy.UserId, buy.Email),
		// 	Client: client.Client{
		// 		Country: buy.BillingAddress.Country,
		// 	},
		// 	Metadata: Map{
		// 		"FNAME":    buy.FirstName,
		// 		"LNAME":    buy.LastName,
		// 		"ADDRESS1": buy.BillingAddress.Line1,
		// 		"ADDRESS2": buy.BillingAddress.Line2,
		// 		"CITY":     buy.BillingAddress.City,
		// 		"STATE":    buy.BillingAddress.State,
		// 		"POSTAL":   buy.BillingAddress.PostalCode,
		// 		"COUNTRY":  buy.BillingAddress.Country,
		// 		"PHONE":    buy.Phone,
		// 		"REFERRAL": referralUrl,
		// 	},
		// }
		// return nil
		list := &email.List{
			Id: listId,
		}
		sub := &email.Subscriber{
			Email: email.Email{
				Address: emailStr,
			},
			Metadata: map[string]interface{}{
				"FNAME": firstName,
				"LNAME": lastName,
			},
		}
		return api.Subscribe(list, sub)
	})
}
