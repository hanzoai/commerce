package twilio

import (
	"context"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"

	"github.com/sfreiberg/gotwilio"
)

type Client struct {
	Client *gotwilio.Twilio
	Ctx    context.Context
}

func New(ctx context.Context, accountSid, authToken string) *Client {
	ctx, _ = context.WithTimeout(ctx, time.Second*55)

	httpClient := urlfetch.Client(ctx)

	httpClient.Transport = &urlfetch.Transport{
		Context:                       ctx,
		AllowInvalidServerCertificate: appengine.IsDevAppServer(),
	}

	c := gotwilio.NewTwilioClient(accountSid, authToken)
	
	return &Client{c, ctx}
}

func (c Client) SendSms(fromNumber, toNumber, message string) (string, error) {
	res, _, err := c.Client.SendSMS(fromNumber, toNumber, message, "", "")
	return res.Status, err
}
