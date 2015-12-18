package netlify

import (
	"net/http"
	"time"

	"appengine/urlfetch"

	"appengine"
)

type Transport struct {
	*urlfetch.Transport
	AccessToken string
}

func (t *Transport) RoundTrip(req *http.Request) (res *http.Response, err error) {
	req.URL.Query().Add("access_token", t.AccessToken)
	return t.Transport.RoundTrip(req)
}

func newHttpClient(ctx appengine.Context, token string) *http.Client {
	client := urlfetch.Client(ctx)
	client.Transport = &Transport{
		AccessToken: token,
		Transport: &urlfetch.Transport{
			Context:  ctx,
			Deadline: time.Duration(20) * time.Second, // Update deadline to 20 seconds
		},
	}
	return client
}
