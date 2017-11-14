package bitcoin

import (
	"appengine"
	"appengine/urlfetch"
	"github.com/btcsuite/btcd/rpcclient"
	"net/http"
	"time"
)

type BitcoinClient struct {
	ctx        appengine.Context
	httpClient *http.Client
	client     *rpcclient.Client
}

// New creates a new RPC client based on the provided connection configuration
// details.  The notification handlers parameter may be nil if you are not
// interested in receiving notifications and will be ignored if the
// configuration is set to run in HTTP POST mode.
func New(ctx appengine.Context, host, endpoint, username, password string, certs []byte) (*BitcoinClient, error) {
	httpClient := urlfetch.Client(ctx)
	httpClient.Transport = &urlfetch.Transport{
		Context:                       ctx,
		Deadline:                      time.Duration(55) * time.Second,
		AllowInvalidServerCertificate: appengine.IsDevAppServer(),
	}
	connCfg := &rpcclient.ConnConfig{
		Host:         host,
		Endpoint:     endpoint,
		User:         username,
		Pass:         password,
		Certificates: certs,
	}

	c, err := rpcclient.New(connCfg, nil)
	if err != nil {
		return nil, err
	}
	return &BitcoinClient{ctx: ctx, client: c}, nil
}
