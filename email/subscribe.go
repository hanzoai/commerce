package email

import (
	"context"

	"hanzo.io/email/tasks"
	"hanzo.io/log"
	"hanzo.io/models/form"
	"hanzo.io/models/organization"
	"hanzo.io/models/subscriber"
)

func Subscribe(c context.Context, f *form.Form, s *subscriber.Subscriber, org *organization.Organization) (err error) {
	in, err := org.Integrations.EmailMarketingProvider()
	if err != nil {
		log.Error("Could not get Email Provider from org %v: %v", org.Name, err, c)
		return err
	} else if in == nil {
		return IntegrationShouldNotBeNilError
	}

	list := List{
		Id: f.EmailList.Id,
	}

	sub := Subscriber{
		Email: Email{
			Address: s.Email,
		},
	}

	return tasks.Subscribe.Call(c, *in, list, sub)
}
