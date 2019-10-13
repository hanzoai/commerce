package plaid

import (
	"context"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"

	"github.com/plaid/plaid-go/plaid"
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
	pc, _ := plaid.NewClient(
		plaid.ClientOptions{
			client_id,
			secret,
			pub_key,
			plaid.Sandbox, // Available environments are Sandbox, Development, and Production
			httpClient,    // This parameter is optional
		},
	)

	return &Client{pc, ctx}
}
