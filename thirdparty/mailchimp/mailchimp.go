package mailchimp

import (
	"time"

	"appengine"
	"appengine/urlfetch"

	"github.com/mattbaird/gochimp"

	"crowdstart.com/models/mailinglist"
	"crowdstart.com/models/subscriber"
	"crowdstart.com/util/log"
)

type API struct {
	ctx    appengine.Context
	client *gochimp.ChimpAPI
}

func New(ctx appengine.Context, apiKey string) *API {
	api := new(API)
	api.ctx = ctx
	api.client = gochimp.NewChimp(apiKey, true)
	api.client.Transport = &urlfetch.Transport{
		Context:  ctx,
		Deadline: time.Duration(60) * time.Second, // Update deadline to 60 seconds
	}
	return api
}

func (a API) BatchSubscribe(ml *mailinglist.MailingList, subscribers []*subscriber.Subscriber) error {
	members := make([]gochimp.ListsMember, 0)
	for _, s := range subscribers {
		members = append(members, gochimp.ListsMember{
			Email: gochimp.Email{
				Email: s.Email,
			},
			MergeVars: s.MergeVars(),
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
	if err != nil {
		log.Error("Batch subscribe failed: %v", err, a.ctx)
	}
	return err
}

func (a API) Subscribe(ml *mailinglist.MailingList, s *subscriber.Subscriber) error {
	email := gochimp.Email{
		Email: s.Email,
	}
	req := gochimp.ListsSubscribe{
		Email:            email,
		MergeVars:        s.MergeVars(),
		ListId:           ml.Mailchimp.Id,
		DoubleOptIn:      ml.Mailchimp.DoubleOptin,
		UpdateExisting:   ml.Mailchimp.UpdateExisting,
		ReplaceInterests: ml.Mailchimp.ReplaceInterests,
		SendWelcome:      ml.Mailchimp.SendWelcome,
	}
	_, err := a.client.ListsSubscribe(req)
	if err != nil {
		log.Error("Failed to subscribe %v: %v", s, err, a.ctx)
	}
	return err
}
