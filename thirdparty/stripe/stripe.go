package stripe

import (
	"time"

	"appengine"
	"appengine/urlfetch"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"
)

func New(ctx context.Context, accessToken string) *Client {
	// Set HTTP Client for App engine
	httpClient := urlfetch.Client(ctx)
	httpClient.Transport = &urlfetch.Transport{
		Context:                       ctx,
		Deadline:                      time.Duration(55) * time.Second,
		AllowInvalidServerCertificate: appengine.IsDevAppServer(),
	}
	stripe.SetBackend(stripe.APIBackend, nil)
	stripe.SetHTTPClient(httpClient)

	sc := &client.API{}
	sc.Init(accessToken, nil)
	return &Client{sc, ctx}
}

// Enable debug logging in development
func init() {
	if appengine.IsDevAppServer() {
		stripe.LogLevel = 2
	}
}
