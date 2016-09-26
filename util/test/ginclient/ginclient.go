package ginclient

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/gin-gonic/gin"

	"crowdstart.com/util/gincontext"
	"crowdstart.com/util/json"
	"crowdstart.com/util/test/ae"
)

type setupFn func(c *http.Request)

type Client struct {
	Router  *gin.Engine
	Context *gin.Context
	setupFn setupFn
}

func newRouter(ctx ae.Context) *gin.Engine {
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

func (c *Client) NewRequest(method, path string, reader io.Reader) *http.Request {
	// Create new request
	req, err := http.NewRequest(method, path, reader)
	if err != nil {
		panic(err)
	}

	// Run any sort of setup code necessary
	c.setupFn(req)

	return req
}

func (c *Client) Do(req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c.Router.ServeHTTP(w, req)
	return w
}

func (c *Client) Get(path string) *httptest.ResponseRecorder {
	req := c.NewRequest("GET", path, nil)
	return c.Do(req)
}

func (c *Client) Post(path, bodyType string, reader io.Reader) *httptest.ResponseRecorder {
	req := c.NewRequest("POST", path, reader)
	req.Header.Set("Content-Type", bodyType)
	return c.Do(req)
}

func (c *Client) PostJSON(path string, src interface{}) *httptest.ResponseRecorder {
	var req *http.Request

	switch v := src.(type) {
	case string:
		// Assume strings are already JSON-encoded
		reader := strings.NewReader(v)
		req = c.NewRequest("POST", path, reader)
	default:
		// Blindly JSON encode!
		buf := json.EncodeBuffer(src)
		req = c.NewRequest("POST", path, buf)
	}

	req.Header.Set("Content-Type", "application/json")
	return c.Do(req)
}
