package mailchimp

import (
	"github.com/mattbaird/gochimp"

	"crowdstart.io/models2/mailinglist"
	"crowdstart.io/models2/subscriber"
)

type API struct {
	client *gochimp.ChimpAPI
}

func New(apiKey string) *API {
	api := new(API)
	api.client = gochimp.NewChimp(apiKey, true)
	return api
}

func (a API) BatchSubscribe(ml *mailinglist.MailingList, subscribers []*subscriber.Subscriber) error {
	members := make([]gochimp.ListsMember, 0)
	for _, s := range subscribers {
		members = append(members, gochimp.ListsMember{
			Email: gochimp.Email{
				Email: s.Email,
			},
			MergeVars: s.Metadata,
		})
	}
	req := gochimp.BatchSubscribe{
		ListId:           ml.Mailchimp.Id,
		Batch:            members,
		DoubleOptin:      ml.Mailchimp.DoubleOptin,
		UpdateExisting:   ml.Mailchimp.UpdateExisting,
		ReplaceInterests: ml.Mailchimp.ReplaceInterests,
	}
	_, err := a.client.BatchSubscribe(req)
	return err
}

func (a API) Subscribe(ml *mailinglist.MailingList, s *subscriber.Subscriber) error {
	email := gochimp.Email{
		Email: s.Email,
	}
	req := gochimp.ListsSubscribe{
		Email:            email,
		MergeVars:        s.Metadata,
		ListId:           ml.Mailchimp.Id,
		DoubleOptIn:      ml.Mailchimp.DoubleOptin,
		UpdateExisting:   ml.Mailchimp.UpdateExisting,
		ReplaceInterests: ml.Mailchimp.ReplaceInterests,
		SendWelcome:      ml.Mailchimp.SendWelcome,
	}
	_, err := a.client.ListsSubscribe(req)
	return err
}
