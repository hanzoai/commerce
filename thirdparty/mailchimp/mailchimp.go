package mailchimp

import (
	"time"

	"appengine"
	"appengine/urlfetch"

	"github.com/zeekay/gochimp/chimp_v3"

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

func (api API) Subscribe(ml *mailinglist.MailingList, s *subscriber.Subscriber) error {
	list, err := api.client.GetList(ml.Mailchimp.Id, nil)
	if err != nil {
		log.Error("Failed to subscribe %v: %v", s, err, api.ctx)
		return err
	}

	status := "subscribed"
	if ml.Mailchimp.DoubleOptin {
		status = "pending"
	}

	req := &gochimp.MemberRequest{
		EmailAddress: s.Email,
		Status:       status,
		MergeFields:  s.MergeFields(),
		Interests:    make(map[string]interface{}),
		Language:     s.Client.Language,
		VIP:          false,
		Location: gochimp.MemberLocation{
			Latitude:    0.0,
			Longitude:   0.0,
			GMTOffset:   0,
			DSTOffset:   0,
			CountryCode: s.Client.Country,
			Timezone:    "",
		},
	}

	// Try to update subscriber, create new member if that fails.
	if _, err := list.UpdateMember(s.Md5(), req); err != nil {
		_, err := list.CreateMember(req)
		return err
	}
	return nil
}
