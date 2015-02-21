package httpclient

import (
	"io/ioutil"
	"net/http"

	"appengine"

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
	res.Response, err = http.Get(url)
	return res, err
}
