package stripe

import (
	"context"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"

	stripe "github.com/stripe/stripe-go/v75"
	"github.com/stripe/stripe-go/v75/client"
)

func New(ctx context.Context, accessToken string) *Client {
	// Set deadline
	ctx, _ = context.WithTimeout(ctx, time.Second*55)

	// Set HTTP Client for App engine
	httpClient := urlfetch.Client(ctx)

	httpClient.Transport = &urlfetch.Transport{
		Context:                       ctx,
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
		stripe.DefaultLeveledLogger = &stripe.LeveledLogger{
			Level: stripe.LevelInfo,
		}
	}
}
