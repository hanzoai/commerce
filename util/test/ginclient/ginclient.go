package ginclient

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"crowdstart.com/util/gincontext"
	"crowdstart.com/util/json"
	"crowdstart.com/util/test/ae"

	. "crowdstart.com/util/test/ginkgo"
)

type setupFn func(c *http.Request)

type Client struct {
	Router  *gin.Engine
	Context *gin.Context
	setupFn setupFn
}

func newRouter(ctx ae.Context) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		gincontext.SetDefaults(c, ctx)
	})
	return router
}

func New(ctx ae.Context) *Client {
	c := new(Client)
	router := newRouter(ctx)
	c.Router = router
	c.Setup(func(r *http.Request) {})
	return c
}

func Handler(ctx ae.Context, method, path string, handler gin.HandlerFunc) *Client {
	client := New(ctx)

	// Wrapper handler to save state of context
	wrapper := func(c *gin.Context) {
		handler(c)
		client.Context = c
	}

	client.Router.Handle(method, path, wrapper)

	return client
}

func Middleware(ctx ae.Context, mw gin.HandlerFunc) *Client {
	client := New(ctx)

	// Helper middleware to save state of context
	client.Router.Use(func(c *gin.Context) {
		c.Next()
		client.Context = c
	})
	client.Router.Use(mw)

	return client
}

func (c *Client) Setup(fn setupFn) {
	c.setupFn = fn
}

func (c *Client) newRequest(method, path string, reader io.Reader) *http.Request {
	// Create new request
	req, err := http.NewRequest(method, path, reader)
	if err != nil {
		panic(err)
	}

	// Run any sort of setup code necessary
	c.setupFn(req)

	return req
}

// Make request without a body
func (c *Client) doRequest(method, uri string) *httptest.ResponseRecorder {
	// Create request
	r := c.newRequest(method, uri, nil)

	// Do request
	w := httptest.NewRecorder()
	c.Router.ServeHTTP(w, r)
	return w
}

// Make request with body
func (c *Client) doRequestBody(method, uri string, body interface{}) *httptest.ResponseRecorder {
	var r *http.Request

	// Create request
	switch v := body.(type) {
	case url.Values:
		// Posting a form
		reader := strings.NewReader(v.Encode())
		r = c.newRequest(method, uri, reader)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	case string:
		// Assume strings are already JSON-encoded
		reader := strings.NewReader(v)
		r = c.newRequest(method, uri, reader)
		r.Header.Set("Content-Type", "application/json")
	default:
		// Blindly JSON encode!
		buf := json.EncodeBuffer(body)
		r = c.newRequest(method, uri, buf)
		r.Header.Set("Content-Type", "application/json")
	}

	// Do request
	w := httptest.NewRecorder()
	c.Router.ServeHTTP(w, r)
	return w
}

// Generic request handler
func (c *Client) request(method, uri string, body interface{}, res interface{}, args ...interface{}) (w *httptest.ResponseRecorder) {
	var code int

	// Parse optional args. Two types of optional arguments may be passed:
	//	 int:		  for required exit code
	//   url.Values:  to be used as query param
	for _, arg := range args {
		switch v := arg.(type) {
		case int:
			code = v
		case url.Values:
			uri = uri + v.Encode()
		default:
			panic("Unknown optional argument")
		}
	}

	// Handle various request methods
	switch method {
	case "GET", "HEAD", "OPTIONS":
		w = c.doRequest(method, uri)
	case "POST", "PUT", "PATCH", "DELETE":
		w = c.doRequestBody(method, uri, body)
	}

	// Automatically decode body
	if res != nil {
		// TODO: Do we need to close this?
		err := json.DecodeBuffer(w.Body, res)
		Expect2(err).ToNot(HaveOccurred())
	}

	if code == 0 {
		Expect2(w.Code < 400).To(BeTrue())
	} else {
		Expect2(w.Code == code).To(BeTrue())
	}

	return w
}

// Make OPTIONS request
func (c *Client) Options(uri string, args ...interface{}) *httptest.ResponseRecorder {
	return c.request("OPTIONS", uri, nil, nil, args...)
}

// Make HEAD request
func (c *Client) Head(uri string, args ...interface{}) *httptest.ResponseRecorder {
	return c.request("HEAD", uri, nil, nil, args...)
}

// Make GET request
func (c *Client) Get(uri string, res interface{}, args ...interface{}) *httptest.ResponseRecorder {
	return c.request("GET", uri, nil, res, args...)
}

// Make PATCH request
func (c *Client) Patch(uri string, body interface{}, res interface{}, args ...interface{}) *httptest.ResponseRecorder {
	return c.request("PATCH", uri, body, res, args...)
}

// Make POST request
func (c *Client) Post(uri string, body interface{}, res interface{}, args ...interface{}) *httptest.ResponseRecorder {
	return c.request("POST", uri, body, res, args...)
}

// Make PUT request
func (c *Client) Put(uri string, body interface{}, res interface{}, args ...interface{}) *httptest.ResponseRecorder {
	return c.request("PUT", uri, body, res, args...)
}

// Make DELETE request
func (c *Client) Delete(uri string, args ...interface{}) *httptest.ResponseRecorder {
	return c.request("DELETE", uri, nil, nil, args...)
}
