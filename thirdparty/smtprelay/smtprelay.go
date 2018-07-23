package smtprelay

import (
	"bytes"
	"context"
	"net/http"
	"time"

	"google.golang.org/appengine/urlfetch"

	"hanzo.io/config"
	"hanzo.io/log"
	"hanzo.io/types/email"
	"hanzo.io/types/integration"
	"hanzo.io/util/json"
)

type Request struct {
	Username string        `json:"username"`
	Password string        `json:"password"`
	Host     string        `json:"host"`
	Port     string        `json:"port"`
	From     email.Email   `json:"mailFrom"`
	To       []email.Email `json:"mailTo"`
	Msg      string        `json:"msg"`
}

type Client struct {
	endpoint string
	username string
	password string
	settings integration.SMTPRelay
	client   *http.Client
}

func New(ctx context.Context, settings integration.SMTPRelay) *Client {
	ctx, _ = context.WithTimeout(ctx, time.Second*55)

	client := urlfetch.Client(ctx)

	return &Client{
		client:   client,
		endpoint: config.SmtpRelay.Endpoint,
		username: config.SmtpRelay.Username,
		password: config.SmtpRelay.Password,
		settings: settings,
	}
}

func (c *Client) Request(r *Request) error {
	var payload *bytes.Reader

	if r != nil {
		payload = bytes.NewReader(json.EncodeBytes(r))
	}

	req, err := http.NewRequest("POST", c.endpoint, payload)
	if err != nil {
		return err
	}

	req.SetBasicAuth(c.username, c.password)
	req.Header.Add("Content-Type", "application/json")

	res, err := c.client.Do(req)

	log.Debug(res)

	return err
}

func (c *Client) Send(message *email.Message) error {
	req := new(Request)
	return c.Request(req)
}
