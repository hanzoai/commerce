package zipclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/zap-proto/fiber/v3/middleware/adaptor"
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/test/ae"
	"github.com/hanzoai/commerce/util/zipctx"

	. "github.com/hanzoai/commerce/util/test/ginkgo"
)

type ApiError struct {
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

type defaultsFunc func(c *http.Request)

// Client drives a zip.App under test the way the old ginclient drove a
// gin.Engine: register routes/middleware on Router, then issue requests that
// return an *httptest.ResponseRecorder (Code/Body) exactly as before. Requests
// run through the zap-proto/fiber net/http adaptor, so the full middleware chain
// and routing are exercised — no gin, no shims.
//
// There is deliberately no post-request Context handle (the old ginclient
// exposed one): fiber pools and resets the *Ctx when the request completes, so
// reading it afterward panics. A test that needs the resolved context state
// reads it INSIDE the handler it registers (where the Ctx is live).
type Client struct {
	Router zip.Router

	app          *zip.App
	defaultsFn   defaultsFunc
	ignoreErrors bool
	headers      http.Header
}

// WithHeaders returns a copy of the client that sends hdr on every request.
//
// Some endpoints are defined by a header rather than by a body — POST
// /billing/deposit refuses any credit that does not name the settlement that
// caused it, and the name travels in X-Idempotency-Key. Without a way to send
// one, a suite can only assert the refusal, never the success, and the money
// path itself goes untested.
//
// The copy is deliberate: a header set for one request must not leak into the
// rest of a suite that shares the client.
func (cl *Client) WithHeaders(hdr http.Header) *Client {
	next := *cl
	next.headers = hdr.Clone()
	return &next
}

// newApp builds the zip.App backing a Client, seeding every request with the
// default test locals via zipctx (the successor to gincontext) before any suite
// middleware runs — the same first-in-chain SetDefaults the gin router installed.
func newApp(ctx context.Context) *zip.App {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Use(zip.H(func(c *zip.Ctx) error {
		zipctx.SetDefaults(c, ctx)
		return c.Next()
	}))
	return app
}

func New(ctx ae.Context) *Client {
	cl := new(Client)
	cl.app = newApp(ctx)
	// Mount routes on a root group so cl.Router satisfies zip.Router (which
	// *zip.App does not — its Fiber() returns *fiber.App, not fiber.Router).
	cl.Router = cl.app.Group("")
	cl.Defaults(func(r *http.Request) {})
	return cl
}

func (cl *Client) IgnoreErrors(ignore bool) {
	cl.ignoreErrors = ignore
}

// Add a new handler to router
func (cl *Client) Handle(method, path string, handler zip.Handler) {
	switch strings.ToUpper(method) {
	case http.MethodGet:
		cl.Router.Get(path, handler)
	case http.MethodPost:
		cl.Router.Post(path, handler)
	case http.MethodPut:
		cl.Router.Put(path, handler)
	case http.MethodPatch:
		cl.Router.Patch(path, handler)
	case http.MethodDelete:
		cl.Router.Delete(path, handler)
	case http.MethodHead:
		cl.Router.Head(path, handler)
	case http.MethodOptions:
		cl.Router.Options(path, handler)
	default:
		cl.Router.All(path, handler)
	}
}

// Add middleware to router
func (cl *Client) Use(mw ...zip.Handler) {
	for _, m := range mw {
		cl.Router.Use(m)
	}
}

// Set defaults for each request
func (cl *Client) Defaults(fn defaultsFunc) {
	cl.defaultsFn = fn
}

func (cl *Client) NewRequest(method, uri string, reader io.Reader) *http.Request {
	// SERVER-style request (httptest): RequestURI + Host populated, which is
	// what a handler-side dispatch needs. A client-style http.NewRequest
	// leaves RequestURI empty and the adaptor builds a pathless fasthttp URI
	// → every route 404s.
	r := httptest.NewRequest(method, uri, reader)

	// Run any sort of setup code necessary
	cl.defaultsFn(r)

	// Caller-supplied headers win over the defaults: they are the reason this
	// request differs from every other one the suite sends.
	for k, vs := range cl.headers {
		r.Header.Del(k)
		for _, v := range vs {
			r.Header.Add(k, v)
		}
	}

	return r
}

// serve runs a request through the zip app via the net/http adaptor, recording
// the response the way ServeHTTP did for the gin engine.
func (cl *Client) serve(r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	adaptor.FiberApp(cl.app.Fiber()).ServeHTTP(w, r)
	return w
}

// Make request without a body
func (cl *Client) doRequest(method, uri string) *httptest.ResponseRecorder {
	return cl.serve(cl.NewRequest(method, uri, nil))
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

	return cl.serve(r)
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
		msg := fmt.Sprintf("Unable to decode body, %v:\n'%v'", err, w.Body)
		if !cl.ignoreErrors {
			Expect2(err == nil).To(BeTrue(), msg)
		}
	}

	if code == 0 {
		msg := fmt.Sprintf("Request failed with invalid status:\n%s", w.Body)
		if !cl.ignoreErrors {
			Expect2(w.Code).To(BeNumerically("<", 400), msg)
		}
	} else {
		msg := fmt.Sprintf("Request failed with invalid status:\n%s", w.Body)
		if !cl.ignoreErrors {
			Expect2(w.Code).To(Equal(code), msg)
		}
	}

	return w
}

func (c *Client) Do(req *http.Request) *httptest.ResponseRecorder {
	// Run any sort of setup code necessary
	c.defaultsFn(req)
	return c.serve(req)
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
	// Encode into the BODY: the old gin engine read req.PostForm (a parsed
	// server-side field), but the wire truth is urlencoded bytes — fasthttp
	// parses PostArgs from the body, so that is what we send.
	req := c.NewRequest("POST", path, strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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
// Accepts optional args: int (expected status code, default 204), url.Values (query params),
// or any other type (decoded as JSON response destination).
func (cl *Client) Delete(uri string, args ...interface{}) *httptest.ResponseRecorder {
	var res interface{}
	var statusCode int = 204
	var filteredArgs []interface{}
	for _, arg := range args {
		switch v := arg.(type) {
		case int:
			statusCode = v
		case url.Values:
			filteredArgs = append(filteredArgs, v)
		default:
			res = arg
		}
	}
	return cl.request("DELETE", uri, nil, res, append(filteredArgs, statusCode)...)
}

// App exposes the backing zip app (route dumps, direct Fiber().Test drives).
func (cl *Client) App() *zip.App { return cl.app }
