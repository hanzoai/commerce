package ginclient

import (
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"

	"crowdstart.io/util/gincontext"
	"crowdstart.io/util/test/ae"
)

type Client struct {
	router  *gin.Engine
	Context *gin.Context
}

func newRouter(ctx ae.Context) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		gincontext.SetDefaults(c, ctx)
	})
	return router
}

func Handler(ctx ae.Context, method, path string, handler gin.HandlerFunc) *Client {
	client := new(Client)

	// Wrapper handler to save state of context
	wrapper := func(c *gin.Context) {
		handler(c)
		client.Context = c
	}

	router := newRouter(ctx)
	router.Handle(method, path, []gin.HandlerFunc{wrapper})

	client.router = router

	return client
}

func Middleware(ctx ae.Context, mw gin.HandlerFunc) *Client {
	client := new(Client)

	router := newRouter(ctx)

	// Helper middleware to save state of context
	router.Use(func(c *gin.Context) {
		c.Next()
		client.Context = c
	})
	router.Use(mw)

	client.router = router

	return client
}

func (c *Client) NewRequest(method, path string, reader io.Reader) *http.Request {
	req, err := http.NewRequest(method, path, reader)
	if err != nil {
		panic(err)
	}
	return req
}

func (c *Client) Do(req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c.router.ServeHTTP(w, req)
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
