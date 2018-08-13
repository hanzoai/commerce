package sendgrid

import (
	"context"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"

	"github.com/hanzoai/sendgrid-go"
	"github.com/sendgrid/rest"

	"hanzo.io/types/integration"
)

const HOST = "https://api.sendgrid.com"

type Client struct {
	apiKey string
	client *rest.Client
	ctx    context.Context
}

// Make a single API call to Sendgrid
func (c *Client) Request(method string, url string, params map[string]string, body []byte) (*rest.Response, error) {
	req := sendgrid.GetRequest(c.apiKey, url, HOST)
	req.Method = rest.Method(method)
	if params != nil {
		req.QueryParams = params
	}

	if body != nil {
		req.Body = body
	}

	return c.client.API(req)
}

func New(ctx context.Context, settings integration.SendGrid) *Client {
	// Set deadline
	ctx, _ = context.WithTimeout(ctx, time.Second*55)

	// Set HTTP Client for App engine
	httpClient := urlfetch.Client(ctx)

	httpClient.Transport = &urlfetch.Transport{
		Context: ctx,
		AllowInvalidServerCertificate: appengine.IsDevAppServer(),
	}

	client := &rest.Client{HTTPClient: httpClient}

	return &Client{settings.APIKey, client, ctx}
}
