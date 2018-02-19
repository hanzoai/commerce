package netlify

import (
	"context"
	"net/http"
	"time"

	"google.golang.org/appengine/urlfetch"
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
	// Set deadline
	d := time.Now().Add(time.Second * 30)
	ctx, _ = context.WithDeadline(ctx, d)

	client := urlfetch.Client(ctx)
	client.Transport = &OauthTransport{
		AccessToken: accessToken,
		Transport:   &urlfetch.Transport{Context: ctx},
	}
	return client
}
