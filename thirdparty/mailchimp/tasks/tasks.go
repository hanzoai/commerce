package tasks

import (
	"appengine"
	"appengine/delay"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mailinglist"
	"crowdstart.com/models/subscriber"
	"crowdstart.com/thirdparty/mailchimp"
)

var Subscriber = delay.Func("mailchimp-subscribe", func(ctx appengine.Context, mlJSON []byte, sJSON []byte) {
	db := datastore.New(ctx)
	ml := mailinglist.FromJSON(db, mlJSON)
	s := subscriber.FromJSON(db, sJSON)
	api := mailchimp.New(ctx, ml.Mailchimp.APIKey)
	api.Subscribe(ml, s)
})
