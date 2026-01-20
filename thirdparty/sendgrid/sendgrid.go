package sendgrid

import (
	"context"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"

	"github.com/hanzoai/sendgrid-go"
	"github.com/sendgrid/rest"

	"github.com/hanzoai/commerce/types/integration"
)

type API struct {
	Context context.Context
	Client  *rest.Client
	Key     string
}

func New(c context.Context, settings integration.SendGrid) *API {
	// Set deadline
	c, _ = context.WithTimeout(c, time.Second*55)

	// Set HTTP Client for App engine
	httpClient := urlfetch.Client(c)

	httpClient.Transport = &urlfetch.Transport{
		Context: c,
		AllowInvalidServerCertificate: appengine.IsDevAppServer(),
	}

	client := &rest.Client{HTTPClient: httpClient}

	return &API{
		Client:  client,
		Context: c,
		Key:     settings.APIKey,
	}
}

// Make a single API call to Sendgrid
func (api API) Request(method string, url string, params map[string]string, body []byte) (*rest.Response, error) {
	req := sendgrid.GetRequest(api.Key, url, "")

	req.Method = rest.Method(method)
	if params != nil {
		req.QueryParams = params
	}

	if body != nil {
		req.Body = body
	}

	return api.Client.API(req)
}
