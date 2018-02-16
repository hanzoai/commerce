package httpclient

import (
	"io"
	"net/http"
	"net/url"
	"strings"

	"appengine"

	"hanzo.io/config"
	"hanzo.io/util/json"
	"hanzo.io/util/log"
)

type Client struct {
	context    context.Context
	moduleName string
	baseURL    string
}

func (c *Client) URL(path string) string {
	return c.baseURL + path
}

func (c *Client) determineBaseURL() {
	moduleHost, err := getModuleHost(c.context, c.moduleName)
	if err != nil {
		log.Panic("Unable to get host for module '%v': %v", c.moduleName, err)
	}

	url := "http://" + strings.Trim(moduleHost, "/")

	if config.IsDevelopment && c.moduleName != "default" {
		url = strings.Trim(url, "/") + "/" + c.moduleName
	}

	c.baseURL = strings.Trim(url, "/")
}

func (c *Client) getURL(path string) string {
	return c.baseURL + "/" + strings.TrimLeft(path, "/")
}

func (c *Client) Get(path string) (res Response, err error) {
	res.Response, err = http.Get(c.getURL(path))
	return res, err
}

func (c *Client) Post(path, bodyType string, reader io.Reader) (res Response, err error) {
	res.Response, err = http.Post(c.getURL(path), bodyType, reader)
	return res, err
}

func (c *Client) PostForm(path string, data url.Values) (res Response, err error) {
	res.Response, err = http.PostForm(c.getURL(path), data)
	return res, err
}

func (c *Client) PostJSON(path string, src interface{}) (res Response, err error) {
	encoded := json.Encode(src)
	res.Response, err = http.Post(c.getURL(path), "application/json", strings.NewReader(encoded))
	return res, err
}
