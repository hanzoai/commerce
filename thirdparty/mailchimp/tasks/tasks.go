package tasks

import (
	"google.golang.org/appengine"
	"google.golang.org/appengine/delay"

	"hanzo.io/datastore"
	"hanzo.io/models/mailinglist"
	"hanzo.io/models/subscriber"
	"hanzo.io/thirdparty/mailchimp"
	"hanzo.io/util/log"
)

var Subscribe = delay.Func("mailchimp-subscribe", func(ctx context.Context, mlJSON []byte, sJSON []byte) error {
	db := datastore.New(ctx)
	ml := mailinglist.FromJSON(db, mlJSON)
	s := subscriber.FromJSON(db, sJSON)
	api := mailchimp.New(ctx, ml.Mailchimp.APIKey)
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
