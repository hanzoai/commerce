package mailchimp

import (
	"context"
	"net/http"
	"time"

	"github.com/hanzoai/gochimp3"

	"github.com/hanzoai/commerce/types/integration"
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

	// Use standard HTTP transport with timeout
	client.Transport = &http.Transport{
		ResponseHeaderTimeout: time.Second * 55,
	}
	client.Debug = true

	return &API{
		Context: ctx,
		Client:  client,
		Key:     apiKey,
	}
}
