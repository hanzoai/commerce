package mailchimp

import (
	"context"
	"time"

	"google.golang.org/appengine/urlfetch"

	"github.com/zeekay/gochimp3"

	"hanzo.io/types/integration"
)

type API struct {
	Context context.Context
	Client  *gochimp3.API
	Key     string
}

func New(ctx context.Context, settings integration.Mailchimp) *API {
	// Update timeout
	ctx, _ = context.WithTimeout(ctx, time.Second*55)

	apiKey := settings.APIKey
	client := gochimp3.New(apiKey)
	client.Transport = &urlfetch.Transport{
		Context: ctx,
	}
	client.Debug = true

	return &API{
		Context: ctx,
		Client:  client,
		Key:     apiKey,
	}
}
