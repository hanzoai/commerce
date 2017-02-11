package netlify

import (
	"appengine"

	"hanzo.io/util/log"

	"github.com/netlify/netlify-go"
)

type Client struct {
	ctx    appengine.Context
	client *netlify.Client
}

func New(ctx appengine.Context, accessToken string) *Client {
	log.Debug("Creating Netlify client using AccessToken: '%s'", accessToken, ctx)

	client := newOauthClient(ctx, accessToken)

	return &Client{
		ctx: ctx,
		client: netlify.NewClient(&netlify.Config{
			AccessToken: accessToken,
			HttpClient:  client,
			UserAgent:   "Hanzo/1.0",
		}),
	}
}
