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

func New(c context.Context, settings integration.Mailchimp) *API {
	// Update timeout
	c, cancel := context.WithTimeout(c, time.Second*55)
	defer cancel()

	apiKey := settings.APIKey
	client := gochimp3.New(apiKey)
	client.Transport = &urlfetch.Transport{
		Context: c,
	}
	client.Debug = true

	return &API{
		Context: c,
		Client:  client,
		Key:     apiKey,
	}
}
