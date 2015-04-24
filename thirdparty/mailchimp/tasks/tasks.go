package tasks

import (
	"appengine"
	"appengine/delay"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mailinglist"
	"crowdstart.io/models/subscriber"
	"crowdstart.io/thirdparty/mailchimp"
)

var Subscriber = delay.Func("mailchimp-subscribe", func(ctx appengine.Context, mlJSON []byte, sJSON []byte) {
	db := datastore.New(ctx)
	ml := mailinglist.FromJSON(db, mlJSON)
	s := subscriber.FromJSON(db, sJSON)
	api := mailchimp.New(ctx, ml.Mailchimp.APIKey)
	api.Subscribe(ml, s)
})
