package sendgrid

import (
	"context"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"

	"github.com/sendgrid/rest"
	"github.com/sendgrid/sendgrid-go"

	"hanzo.io/types/integration"
)

func New(ctx context.Context, settings integration.SendGrid) *Client {
	// Set deadline
	ctx, _ = context.WithTimeout(ctx, time.Second*55)

	// Set HTTP Client for App engine
	httpClient := urlfetch.Client(ctx)

	httpClient.Transport = &urlfetch.Transport{
		Context: ctx,
		AllowInvalidServerCertificate: appengine.IsDevAppServer(),
	}
	rest.DefaultClient = &rest.Client{HTTPClient: httpClient}
	client := sendgrid.NewSendClient(settings.APIKey)

	return &Client{ctx, client}
}
