package tasks

import (
	"golang.org/x/net/context"
	"google.golang.org/appengine/delay"

	"hanzo.io/datastore"
	"hanzo.io/models/form"
	"hanzo.io/models/subscriber"
	"hanzo.io/thirdparty/mailchimp"
)

var Subscriber = delay.Func("mailchimp-subscribe", func(ctx context.Context, fJSON []byte, sJSON []byte) {
	db := datastore.New(ctx)
	f := form.FromJSON(db, fJSON)
	s := subscriber.FromJSON(db, sJSON)
	api := mailchimp.New(ctx, f.Mailchimp.APIKey)
	api.Subscribe(f, s)
})
