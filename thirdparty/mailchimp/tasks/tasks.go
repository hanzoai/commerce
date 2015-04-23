package tasks

import (
	"appengine"
	"appengine/delay"

	"crowdstart.io/datastore"
	"crowdstart.io/models2/mailinglist"
	"crowdstart.io/models2/subscriber"
	"crowdstart.io/thirdparty/mailchimp"
)

var Subscriber = delay.Func("mailchimp-subscribe", func(c appengine.Context, mlJSON []byte, sJSON []byte) {
	db := datastore.New(c)
	ml := mailinglist.FromJSON(db, mlJSON)
	s := subscriber.FromJSON(db, sJSON)
	api := mailchimp.New(ml.Mailchimp.APIKey)
	api.Subscribe(ml, s)
})
