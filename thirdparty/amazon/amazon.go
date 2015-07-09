package amazon

import (
	"time"

	"github.com/stripe/stripe-go/client"

	"crowdstart.com/models/order"
	"crowdstart.com/models/payment"

	"appengine"
	"appengine/urlfetch"
)

type Client struct {
	*client.API
	ctx appengine.Context
}

func New(ctx appengine.Context, accessToken string) *Client {
	httpClient := urlfetch.Client(ctx)
	httpClient.Transport = &urlfetch.Transport{
		Context:  ctx,
		Deadline: time.Duration(20) * time.Second, // Deadline to 10 seconds
	}

	return &Client{}
}

func (c Client) Authorize(pay *payment.Payment, ord *order.Order) error {

	return nil
}
