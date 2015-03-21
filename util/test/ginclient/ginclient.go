package ginclient

import (
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"

	"crowdstart.io/util/gincontext"
	"crowdstart.io/util/test/ae"
)

type setupFn func(c *http.Request)

type Client struct {
	Router  *gin.Engine
	Context *gin.Context
	setup   setupFn
}

func newRouter(ctx ae.Context) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		gincontext.SetDefaults(c, ctx)
	})
	return router
}

func New(ctx ae.Context) *Client {
	client := new(Client)
	router := newRouter(ctx)
	client.Router = router
	return client
}

func Handler(ctx ae.Context, method, path string, handler gin.HandlerFunc) *Client {
	client := New(ctx)

	// Wrapper handler to save state of context
	wrapper := func(c *gin.Context) {
		handler(c)
		client.Context = c
	}

	client.Router.Handle(method, path, []gin.HandlerFunc{wrapper})

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

func (c *Client) Setup(setup setupFn) {
	c.setup = setup
}

func (c *Client) NewRequest(method, path string, reader io.Reader) *http.Request {
	req, err := http.NewRequest(method, path, reader)
	if err != nil {
		panic(err)
	}
	if c.setup != nil {
		c.setup(req)
	}
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
