package shipwire

import (
	"bytes"
	"net/http"
	"time"

	"appengine/urlfetch"

	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/util/json"

	. "hanzo.io/thirdparty/shipwire/types"
)

type Client struct {
	Username string
	Password string
	Endpoint string

	client *http.Client
}

func New(c *gin.Context, username, password string) *Client {
	ctx := middleware.GetAppEngine(c)

	client := urlfetch.Client(ctx)
	client.Transport = &urlfetch.Transport{
		Context:  ctx,
		Deadline: time.Duration(20) * time.Second, // Update deadline to 20 seconds
	}

	return &Client{
		Username: username,
		Password: password,
		Endpoint: "https://api.shipwire.com/api/v3/",
		client:   client,
	}
}

func (c *Client) Request(method, url string, body interface{}, dst interface{}) (*Response, error) {
	var payload *bytes.Buffer

	if body != nil {
		payload = bytes.NewBuffer(json.EncodeBytes(body))
	}

	req, err := http.NewRequest(method, c.Endpoint+url, payload)
	if err != nil {
		return nil, err
	}

	// req.SetBasicAuth(c.Username, c.Password)
	req.Header.Add("Content-Type", "application/json")

	res := new(Response)

	// Do request
	r, err := c.client.Do(req)
	if err != nil {
		return res, err
	}

	// Automatically decode body
	if dst != nil {
		// TODO: Do we need to close this?
		err = json.Decode(r.Body, res)
		if err != nil {
			// Get first resource
			err = json.Unmarshal(res.Resource.Items[0].Resource, &dst)
		}
	}

	return res, err
}
