package bitcoin

import (
	"io/ioutil"
	"log"
	"path/filepath"
	"time"

	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

type BitcoinClient struct {
	client *rpcclient.Client
}

// New creates a new RPC client based on the provided connection configuration
// details.  The notification handlers parameter may be nil if you are not
// interested in receiving notifications and will be ignored if the
// configuration is set to run in HTTP POST mode.
func New(host, endpoint, username, password string, certs []byte) (*BitcoinClient, error) {
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
	return &BitcoinClient{client: c}, nil
}
