package netlify

import (
	"net/http"
	"time"

	"appengine"
	"appengine/urlfetch"
)

type OauthTransport struct {
	*urlfetch.Transport
	AccessToken string
}

func (t *OauthTransport) RoundTrip(req *http.Request) (res *http.Response, err error) {
	req.Header.Set("Authorization", "Bearer "+t.AccessToken)
	return t.Transport.RoundTrip(req)
}

func newOauthClient(ctx context.Context, accessToken string) *http.Client {
	client := urlfetch.Client(ctx)
	client.Transport = &OauthTransport{
		AccessToken: accessToken,
		Transport: &urlfetch.Transport{
			Context:  ctx,
			Deadline: time.Duration(20) * time.Second, // Update deadline to 20 seconds
		},
	}
	return client
}
