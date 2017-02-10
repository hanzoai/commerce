package shipwire

import (
	"bytes"
	"net/http"
	"time"

	"hanzo.io/middleware"
	"hanzo.io/models/order"
	"hanzo.io/util/json"

	"github.com/gin-gonic/gin"

	"appengine/urlfetch"
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
		Endpoint: "https://api.shipwire.com/api/3/",
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

	req.SetBasicAuth(c.Username, c.Password)
	req.Header.Add("Content-Type", "application/json")

	return c.client.Do(req)
}

func (c *Client) CreateOrder(ord *order.Order) {
	// req := OrderRequest{}
}

func (c *Client) CreateReturn(ord *order.Order) {
	// req := ReturnRequest{}
}
