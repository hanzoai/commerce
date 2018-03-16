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
	// Update timeout
	ctx, _ = context.WithTimeout(ctx, time.Second*30)

	client := urlfetch.Client(ctx)
	client.Transport = &OauthTransport{
		AccessToken: accessToken,
		Transport:   &urlfetch.Transport{Context: ctx},
	}
	return client
}
