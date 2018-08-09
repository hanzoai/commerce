package tasks

import (
	"context"

	"hanzo.io/delay"

	"hanzo.io/datastore"
	"hanzo.io/log"
	"hanzo.io/models/form"
	"hanzo.io/models/subscriber"
	"hanzo.io/thirdparty/mailchimp"
)

var Subscribe = delay.Func("mailchimp-subscribe", func(ctx context.Context, fJSON []byte, sJSON []byte) error {
	db := datastore.New(ctx)
	f := form.FromJSON(db, fJSON)
	s := subscriber.FromJSON(db, sJSON)
	api := mailchimp.New(ctx, f.Mailchimp.APIKey)
	if err := api.Subscribe(ml, s); err != nil {
		log.Error("Subscribe Error %v", err, ctx)
		log.Error("Mailinglist %v", ml, ctx)
		log.Error("Subscriber %v", s, ctx)

		if err.Mailchimp == nil {
			return err
		}

		if err.Status == 401 {
			log.Warn("Invalid API Key: %v", err, ctx)
			return nil
		}

		if err.Status > 499 {
			log.Error("Failed to subscribe user: %v", err, ctx)
			return err
		}
	}
	return nil
})
