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
		Endpoint: "https://api.shipwire.com/api/v3",
		client:   client,
		ctx:      ctx,
	}
}

func (c *Client) Request(method, url string, body interface{}, dst interface{}) (*Response, error) {
	var data *bytes.Buffer
	var res Response

	// Encode body
	if body != nil {
		data = bytes.NewBuffer(json.EncodeBytes(body))
	}

	// Create request
	req, err := http.NewRequest(method, c.Endpoint+url, data)
	if err != nil {
		log.Error("Failed to create Shipwire request: %v", err, c.ctx)
		return nil, err
	}

	// Set headers
	req.Header.Add("Content-Type", "application/json")
	req.SetBasicAuth(c.Username, c.Password)

	// Do request
	r, err := c.client.Do(req)

	dump, _ := httputil.DumpRequest(req, true)
	log.Error("Shipwire request:\n%s", dump, c.ctx)

	// Shipwire does not always provide a status
	res.Status = r.StatusCode

	// Request failed
	if err != nil {
		log.Error("Shipwire request failed: %v", err, c.ctx)
		return &res, err
	}

	defer r.Body.Close()

	dump, _ = httputil.DumpResponse(r, true)
	log.Error("Shipwire response:\n%s", dump, c.ctx)

	// Automatically decode response
	err = json.Decode(r.Body, &res)
	if err == nil && dst != nil {
		if len(res.Resource.Items) > 0 {
			err = json.Unmarshal(res.Resource.Items[0].Resource, dst)
		}
	}

	if err != nil {
		log.Warn("Failed to decode Shipwire response:\n%v", err, c.ctx)
	}

	return &res, err
}
