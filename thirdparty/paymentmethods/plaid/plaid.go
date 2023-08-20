package plaid

import (
	"context"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"

	"github.com/plaid/plaid-go/v15/plaid"
)

func New(ctx context.Context, client_id, secret, pub_key string, env Environment) *Client {
	// Set deadline
	ctx, _ = context.WithTimeout(ctx, time.Second*55)

	// Set HTTP Client for App engine
	httpClient := urlfetch.Client(ctx)

	httpClient.Transport = &urlfetch.Transport{
		Context:                       ctx,
		AllowInvalidServerCertificate: appengine.IsDevAppServer(),
	}
	configuration := plaid.NewConfiguration()
	configuration.HTTPClient = httpClient
	configuration.AddDefaultHeader("PLAID-CLIENT-ID", client_id)
	configuration.AddDefaultHeader("PLAID-SECRET", secret)
	configuration.UseEnvironment(plaid.Sandbox) // Available environments are Sandbox, Development, and Production
	pc := plaid.NewAPIClient(configuration)

	return &Client{pc, ctx}
}
