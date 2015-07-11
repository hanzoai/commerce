package paypal

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"crowdstart.com/thirdparty/paypal/responses"
	"crowdstart.com/util/log"

	"appengine"
	"appengine/urlfetch"
)

type Client struct {
	ctx                   appengine.Context
	ClientId              string
	SecretKey             string
	AccessToken           string
	AccessTokenExpiration time.Time
}

func New(ctx appengine.Context, clientId string, secretKey string) (*Client, error) {

	// Need to get the access token.  Sample request:

	// curl -v https://api.sandbox.paypal.com/v1/oauth2/token \
	//      -H "Accept: application/json" \
	//      -H "Accept-Language: en_US" \
	//      -u "<ClientId>:<SecretKey> \
	//      -d "grant_type=client_credentials"

	// The REST verb is not noted on the example, so it is presumably a GET.

	// That will return a Json object consistent with responses/accesstoken.go's AccessTokenResponse.

	res := responses.AccessTokenResponse{}
	err := json.Unmarshal(nil, &res)
	if err != nil {
		return nil, err
	}

	expireTime := time.Now().Add(time.Second * time.Duration(res.ExpiresIn))

	return &Client{ctx: ctx, ClientId: clientId, SecretKey: secretKey, AccessToken: res.AccessToken, AccessTokenExpiration: expireTime}, nil
}

func (c Client) Request(method string, url string, req interface{}, res interface{}) (interface{}, error) {
	client := urlfetch.Client(c.ctx)

	log.Debug("Req: %v", req, c.ctx)

	// Marshal request
	if req != nil {
		b, err := json.Marshal(req)
		if err != nil {
			return res, err
		}
		_ = bytes.NewBuffer(b)
	} else {
		_ = bytes.NewBuffer(nil)
	}

	// Create new http request using request buffer
	hreq, err := http.NewRequest(method, url, nil)
	if err != nil {
		return res, err
	}

	hres, err := client.Do(hreq)
	defer hres.Body.Close()

	if err != nil {
		return res, err
	}

	if hres.StatusCode == 200 {
		body, err := ioutil.ReadAll(hres.Body)
		if err != nil {
			return res, err
		}
		json.Unmarshal(body, &res)
	}

	return res, nil
}

func (c Client) Authorize() error {
	return nil
}
