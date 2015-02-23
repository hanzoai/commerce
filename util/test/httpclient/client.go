package httpclient

import (
	"io"
	"net/http"
	"net/url"
	"strings"

	"appengine"

	"crowdstart.io/config"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
)

type Client struct {
	context    appengine.Context
	moduleName string
	baseURL    string
}

func (c *Client) determineBaseURL() {
	moduleHost, err := getModuleHost(c.context, c.moduleName)
	if err != nil {
		log.Panic("Unable to get host for module '%v': %v", c.moduleName, err)
	}

	// With our current routing setup, any non-default module is prefixed
	c.baseURL = "http://" + moduleHost + strings.TrimRight(config.Prefixes[c.moduleName], "/")
}

func (c *Client) Get(url string) (res Response, err error) {
	res.Response, err = http.Get(c.baseURL + url)
	return res, err
}

func (c *Client) Post(url, bodyType string, reader io.Reader) (res Response, err error) {
	res.Response, err = http.Post(c.baseURL+url, bodyType, reader)
	return res, err
}

func (c *Client) PostForm(url string, data url.Values) (res Response, err error) {
	res.Response, err = http.PostForm(c.baseURL+url, data)
	return res, err
}

func (c *Client) PostJSON(url string, src interface{}) (res Response, err error) {
	encoded := json.Encode(src)
	res.Response, err = http.Post(c.baseURL+url, "application/json", strings.NewReader(encoded))
	return res, err
}
