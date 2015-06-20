package amazon

import (
	"github.com/stripe/stripe-go/client"

	"appengine"
)

type Client struct {
	*client.API
	ctx appengine.Context
}

func New(ctx appengine.Context, accessToken string) *Client {
	//httpClient := urlfetch.Client(ctx)
	return &Client{}
}
