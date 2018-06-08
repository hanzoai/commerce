package smtprelay

import (
	"bytes"
	"context"
	"net/http"
	"time"

	"google.golang.org/appengine/urlfetch"

	"hanzo.io/config"
	"hanzo.io/util/json"
)

type Request struct {
	Username string   `json:"username"`
	Password string   `json:"password"`
	Host     string   `json:"host"`
	Port     string   `json:"port"`
	MailFrom string   `json:"mailFrom"`
	MailTo   []string `json:"mailTo"`
	Msg      string   `json:"msg"`
}

type Client struct {
	Endpoint string
	Username string
	Password string

	client *http.Client
}

func New(ctx context.Context) *Client {
	ctx, _ = context.WithTimeout(ctx, time.Second*55)

	client := urlfetch.Client(ctx)

	return &Client{
		client:   client,
		Endpoint: config.SmtpRelay.Endpoint,
		Username: config.SmtpRelay.Username,
		Password: config.SmtpRelay.Password,
	}
}

func (c *Client) Send(r *Request) (*http.Response, error) {
	var payload *bytes.Reader

	if r != nil {
		payload = bytes.NewReader(json.EncodeBytes(r))
	}

	req, err := http.NewRequest("POST", c.Endpoint, payload)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(c.Username, c.Password)
	req.Header.Add("Content-Type", "application/json")

	return c.client.Do(req)
}
