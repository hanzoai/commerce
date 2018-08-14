package email

import (
	"context"

	"hanzo.io/models/form"
	"hanzo.io/models/organization"
	"hanzo.io/models/subscriber"
)

func Subscribe(c context.Context, f *form.Form, s *subscriber.Subscriber, org *organization.Organization) (err error) {
	return nil
	// // Default to built-in email provider
	// in := &config.Email.Provider

	// // If org is provider use their email provider
	// if org != nil {
	// 	if in, err = org.Integrations.EmailMarketingProvider(); err != nil {
	// 		log.Error("Could not get Email Marketing Provider from org %v: %v", org.Name, err, c)
	// 		return err
	// 	} else if in == nil {
	// 		return IntegrationShouldNotBeNilError
	// 	}
	// }

	// // Fire off task to send email
	// return tasks.Subscribe.Call(c, *in, message)
}
