package cloudflare

import (
	"bytes"
	"net/http"
	"time"

	"hanzo.io/config"
	"hanzo.io/middleware"
	"hanzo.io/util/json"

	"github.com/gin-gonic/gin"

	"google.golang.org/appengine/urlfetch"
)

type Client struct {
	Email    string
	Key      string
	Endpoint string

	client *http.Client
}

func New(c *context.Context) *Client {
	ctx := middleware.GetAppEngine(c)

	client := urlfetch.Client(ctx)
	client.Transport = &urlfetch.Transport{
		Context:  ctx,
		Deadline: time.Duration(20) * time.Second, // Update deadline to 20 seconds
	}

	return &Client{
		Email:    config.Cloudflare.Email,
		Key:      config.Cloudflare.Key,
		Endpoint: "https://api.cloudflare.com/client/v4/",
	}
}

func (c *Client) Request(method, url string, data interface{}) (*http.Response, error) {
	var payload *bytes.Reader

	if data != nil {
		payload = bytes.NewReader(json.EncodeBytes(data))
	}

	req, err := http.NewRequest(method, c.Endpoint+url, payload)
	if err != nil {
		return nil, err
	}

	req.Header.Add("X-AUTH-EMAIL", c.Email)
	req.Header.Add("X-AUTH-KEY", c.Key)
	req.Header.Add("Content-Type", "application/json")

	return c.client.Do(req)
}

func (c *Client) Purge(zone string, files []string) {
	type Request struct {
		Files []string `json:"files"`
	}

	c.Request("DELETE", "/zones/"+zone+"/purge_cache", &Request{Files: files})
}
