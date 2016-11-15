package tasks

import (
	"appengine"
	"appengine/delay"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mailinglist"
	"crowdstart.com/models/subscriber"
	"crowdstart.com/thirdparty/mailchimp"
	"crowdstart.com/util/log"
)

var Subscriber = delay.Func("mailchimp-subscribe", func(ctx appengine.Context, mlJSON []byte, sJSON []byte) {
	db := datastore.New(ctx)
	ml := mailinglist.FromJSON(db, mlJSON)
	s := subscriber.FromJSON(db, sJSON)
	api := mailchimp.New(ctx, ml.Mailchimp.APIKey)
	if err := api.Subscribe(ml, s); err != nil {
		if err.Status == 401 {
			log.Warn("Invalid API Key: %v", err, ctx)
			return
		}

		if err.Status > 499 {
			panic(err)
		}
	}
})
