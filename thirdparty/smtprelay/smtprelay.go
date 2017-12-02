package smtprelay

import (
	"bytes"
	"net/http"
	"time"

	"appengine"
	"appengine/urlfetch"

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

func New(ctx appengine.Context) *Client {
	client := urlfetch.Client(ctx)
	client.Transport = &urlfetch.Transport{
		Context:  ctx,
		Deadline: time.Duration(45) * time.Second,
	}

	return &Client{
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
