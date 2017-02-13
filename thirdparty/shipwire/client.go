package shipwire

import (
	"bytes"
	"net/http"
	"net/http/httputil"
	"time"

	"appengine"
	"appengine/urlfetch"

	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/util/json"
	"hanzo.io/util/log"

	. "hanzo.io/thirdparty/shipwire/types"
)

type Client struct {
	Username string
	Password string
	Endpoint string

	client *http.Client
	ctx    appengine.Context
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
		ctx:      ctx,
	}
}

func (c *Client) Request(method, url string, body interface{}, dst interface{}) (*Response, error) {
	var payload *bytes.Buffer
	var res Response

	if body != nil {
		payload = bytes.NewBuffer(json.EncodeBytes(body))
	}

	req, err := http.NewRequest(method, c.Endpoint+url, payload)
	if err != nil {
		log.Error("Failed to create Shipwire request: %v", err, c.ctx)
		return nil, err
	}

	// req.SetBasicAuth(c.Username, c.Password)
	req.Header.Add("Content-Type", "application/json")

	log.Warn("Shipwire request:\n%v\n%v", req, c.ctx)

	// Do request
	r, err := c.client.Do(req)
	if err != nil {
		log.Error("Shipwire request failed: %v", err, c.ctx)
		return &res, err
	}
	defer r.Body.Close()

	dump, _ := httputil.DumpResponse(r, true)
	log.Warn("Shipwire response:\n%s", dump, c.ctx)

	if dst == nil {
		return &res, nil
	}

	// Automatically decode response
	err = json.Decode(r.Body, &res)
	if err == nil {
		// Try to decode inner resource as dst assuming list of one item
		err = json.Unmarshal(res.Resource.Items[0].Resource, &dst)
	}

	if err != nil {
		log.Warn("Failed to decode Shipwire response:\n%v", err, c.ctx)
	}

	return &res, err
}
