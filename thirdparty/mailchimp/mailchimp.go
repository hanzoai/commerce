package mailchimp

import (
	"time"

	"golang.org/x/net/context"
	"google.golang.org/appengine/urlfetch"

	"github.com/mattbaird/gochimp"

	"hanzo.io/models/form"
	"hanzo.io/models/subscriber"
	"hanzo.io/util/log"
)

type API struct {
	ctx    context.Context
	client *gochimp.ChimpAPI
}

func New(ctx context.Context, apiKey string) *API {
	api := new(API)
	api.ctx, _ = context.WithTimeout(ctx, 60*time.Second)
	api.client = gochimp.NewChimp(apiKey, true)
	api.client.Transport = &urlfetch.Transport{
		Context: api.ctx,
	}
	return api
}

func (a API) BatchSubscribe(f *form.Form, subscribers []*subscriber.Subscriber) error {
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
		ListId:           f.Mailchimp.Id,
		Batch:            members,
		DoubleOptin:      f.Mailchimp.DoubleOptin,
		UpdateExisting:   f.Mailchimp.UpdateExisting,
		ReplaceInterests: f.Mailchimp.ReplaceInterests,
	}
	_, err := a.client.BatchSubscribe(req)
	if err != nil {
		log.Error("Batch subscribe failed: %v", err, a.ctx)
	}
	return err
}

func (a API) Subscribe(f *form.Form, s *subscriber.Subscriber) error {
	email := gochimp.Email{
		Email: s.Email,
	}
	req := gochimp.ListsSubscribe{
		Email:            email,
		MergeVars:        s.MergeVars(),
		ListId:           f.Mailchimp.Id,
		DoubleOptIn:      f.Mailchimp.DoubleOptin,
		UpdateExisting:   f.Mailchimp.UpdateExisting,
		ReplaceInterests: f.Mailchimp.ReplaceInterests,
		SendWelcome:      f.Mailchimp.SendWelcome,
	}
	_, err := a.client.ListsSubscribe(req)
	if err != nil {
		log.Error("Failed to subscribe %v: %v", s, err, a.ctx)
	}
	return err
}
