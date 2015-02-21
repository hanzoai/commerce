package httpclient

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"appengine"

	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
)

func getModuleHost(ctx appengine.Context, moduleName string) (host string, err error) {
	return appengine.ModuleHostname(ctx, moduleName, "", "")
}

type Response struct {
	*http.Response
}

func (r *Response) Body() (body string, err error) {
	defer r.Response.Body.Close()

	if bytes, err := ioutil.ReadAll(r.Response.Body); err != nil {
		return "", err
	} else {
		body = string(bytes)
	}

	return body, err
}

type Client struct {
	context    appengine.Context
	moduleName string
	baseURL    string
}

func New(ctx appengine.Context, moduleName string) *Client {
	client := new(Client)
	client.context = ctx
	client.moduleName = moduleName
	client.determineBaseURL()
	return client
}

func (c *Client) determineBaseURL() {
	// Build URL
	moduleHost, err := getModuleHost(c.context, c.moduleName)
	if err != nil {
		log.Panic("Unable to get host for module: %v", c.moduleName)
	}

	c.baseURL = "http://" + moduleHost
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
