package cloudflare

import (
	"bytes"
	"context"
	"net/http"
	"time"

	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/json"

	"github.com/gin-gonic/gin"
)

type Client struct {
	Email    string
	Key      string
	Endpoint string

	client *http.Client
}

func New(c *gin.Context) *Client {
	ctx := middleware.GetAppEngine(c)

	// Set deadline
	ctx, _ = context.WithTimeout(ctx, time.Second*55)

	client := &http.Client{Timeout: 55 * time.Second}

	return &Client{
		Email:    config.Cloudflare.Email,
		Key:      config.Cloudflare.Key,
		Endpoint: "https://api.cloudflare.com/client/v4/",
		client:   client,
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
