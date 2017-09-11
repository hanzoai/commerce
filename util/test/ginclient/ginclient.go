package ginclient

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"hanzo.io/util/gincontext"
	"hanzo.io/util/json"
	"hanzo.io/util/test/ae"

	. "hanzo.io/util/test/ginkgo"
)

type defaultsFunc func(c *http.Request)

func defaultStatus(code int) func([]interface{}) []interface{} {
	return func(args []interface{}) []interface{} {
		newargs := make([]interface{}, len(args))
		for _, arg := range args {
			switch v := arg.(type) {
			case int:
				code = v
			default:
				newargs = append(newargs, arg)
			}
		}
		return append(newargs, code)
	}
}

type Client struct {
	Router     *gin.Engine
	Context    *gin.Context
	defaultsFn defaultsFunc
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
	cl := new(Client)
	router := newRouter(ctx)
	cl.Router = router
	cl.Defaults(func(r *http.Request) {})
	return cl
}

// Add a new handler to router
func (cl *Client) Handle(method, path string, handler gin.HandlerFunc) {
	wrapper := func(c *gin.Context) {
		handler(c)
		cl.Context = c
	}

	cl.Router.Handle(method, path, wrapper)
}

// Add middleware to router
func (cl *Client) Use(mw ...gin.HandlerFunc) {
	for _, m := range mw {
		cl.Router.Use(func(c *gin.Context) {
			c.Next()
			cl.Context = c
		})

		cl.Router.Use(m)
	}
}

// Set defaults for each request
func (cl *Client) Defaults(fn defaultsFunc) {
	cl.defaultsFn = fn
}

func (cl *Client) NewRequest(method, uri string, reader io.Reader) *http.Request {
	// Create new request
	r, err := http.NewRequest(method, uri, reader)
	if err != nil {
		panic(err)
	}

	// Run any sort of setup code necessary
	cl.defaultsFn(r)

	return r
}

// Make request without a body
func (cl *Client) doRequest(method, uri string) *httptest.ResponseRecorder {
	// Create request
	r := cl.NewRequest(method, uri, nil)

	// Do request
	w := httptest.NewRecorder()
	cl.Router.ServeHTTP(w, r)
	return w
}

// Make request with body
func (cl *Client) doRequestBody(method, uri string, body interface{}) *httptest.ResponseRecorder {
	var r *http.Request

	// Create request
	switch v := body.(type) {
	case url.Values:
		// Posting a form
		reader := strings.NewReader(v.Encode())
		r = cl.NewRequest(method, uri, reader)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	case string:
		// Assume strings are already JSON-encoded
		reader := strings.NewReader(v)
		r = cl.NewRequest(method, uri, reader)
		r.Header.Set("Content-Type", "application/json")
	case nil:
		reader := strings.NewReader("{}")
		r = cl.NewRequest(method, uri, reader)
		r.Header.Set("Content-Type", "application/json")
	default:
		// Blindly JSON encode!
		buf := json.EncodeBuffer(body)
		r = cl.NewRequest(method, uri, buf)
		r.Header.Set("Content-Type", "application/json")
	}

	// Do request
	w := httptest.NewRecorder()
	cl.Router.ServeHTTP(w, r)
	return w
}

// Generic request handler
func (cl *Client) request(method, uri string, body interface{}, res interface{}, args ...interface{}) (w *httptest.ResponseRecorder) {
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
	case "OPTIONS", "HEAD", "GET", "DELETE":
		w = cl.doRequest(method, uri)
	case "POST", "PUT", "PATCH":
		w = cl.doRequestBody(method, uri, body)
	}

	// Automatically decode body
	if res != nil {
		// TODO: Do we need to close this?
		err := json.DecodeBuffer(w.Body, res)
		msg := fmt.Sprintf("Unable to decode body, %v:\n%v", err, w.Body)
		Expect2(err == nil).To(BeTrue(), msg)
	}

	if code == 0 {
		msg := fmt.Sprintf("Request failed with invalid status:\n%s", w.Body)
		Expect2(w.Code).To(BeNumerically("<", 400), msg)
	} else {
		msg := fmt.Sprintf("Request failed with invalid status:\n%s", w.Body)
		Expect2(w.Code).To(Equal(code), msg)
	}

	return w
}

func (c *Client) Do(req *http.Request) *httptest.ResponseRecorder {
	// Run any sort of setup code necessary
	c.defaultsFn(req)

	w := httptest.NewRecorder()
	c.Router.ServeHTTP(w, req)
	return w
}

// Make OPTIONS request
func (cl *Client) Options(uri string, args ...interface{}) *httptest.ResponseRecorder {
	return cl.request("OPTIONS", uri, nil, nil, args...)
}

// Make HEAD request
func (cl *Client) Head(uri string, args ...interface{}) *httptest.ResponseRecorder {
	return cl.request("HEAD", uri, nil, nil, args...)
}

// Make GET request
func (cl *Client) Get(uri string, res interface{}, args ...interface{}) *httptest.ResponseRecorder {
	return cl.request("GET", uri, nil, res, args...)
}

// Make PATCH request
func (cl *Client) Patch(uri string, body interface{}, res interface{}, args ...interface{}) *httptest.ResponseRecorder {
	return cl.request("PATCH", uri, body, res, args...)
}

// Make POST request
func (cl *Client) Post(uri string, body interface{}, res interface{}, args ...interface{}) *httptest.ResponseRecorder {
	return cl.request("POST", uri, body, res, args...)
}

// Make POST with Form Data
func (c *Client) PostForm(path string, data url.Values) *httptest.ResponseRecorder {
	req := c.NewRequest("POST", path, nil)
	req.PostForm = data
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	return c.Do(req)
}

// Make POST with JSON Data
func (c *Client) PostJSON(path string, src interface{}) *httptest.ResponseRecorder {
	encoded := json.Encode(src)
	reader := strings.NewReader(encoded)
	req := c.NewRequest("POST", path, reader)
	req.Header.Set("Content-Type", "application/json")
	return c.Do(req)
}

// Make POST with Raw JSON Data
func (c *Client) PostRawJSON(path string, src string) *httptest.ResponseRecorder {
	reader := strings.NewReader(src)
	req := c.NewRequest("POST", path, reader)
	req.Header.Set("Content-Type", "application/json")
	return c.Do(req)
}

// Make PUT request
func (cl *Client) Put(uri string, body interface{}, res interface{}, args ...interface{}) *httptest.ResponseRecorder {
	return cl.request("PUT", uri, body, res, args...)
}

// Make DELETE request
func (cl *Client) Delete(uri string, args ...interface{}) *httptest.ResponseRecorder {
	args = defaultStatus(204)(args)
	return cl.request("DELETE", uri, nil, nil, args...)
}
