package ginclient

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/context"

	"hanzo.io/util/gincontext"
	"hanzo.io/util/json"
)

type setupFn func(c *http.Request)

type Client struct {
	Router  *gin.Engine
	Context *gin.Context
	setupFn setupFn
}

func newRouter(ctx context.Context) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	router.Use(func(c *gin.Context) {
		gincontext.SetDefaults(c, ctx)
	})

	return router
}

func New(ctx context.Context) *Client {
	ctx, _ = context.WithTimeout(ctx, time.Second*60)
	c := new(Client)
	router := newRouter(ctx)
	c.Router = router
	c.Setup(func(r *http.Request) {})
	return c
}

func Handler(ctx context.Context, method, path string, handler gin.HandlerFunc) *Client {
	client := New(ctx)

	// Wrapper handler to save state of context
	wrapper := func(c *gin.Context) {
		handler(c)
		client.Context = c
	}

	client.Router.Handle(method, path, wrapper)

	return client
}

func Middleware(ctx context.Context, mw gin.HandlerFunc) *Client {
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

	return req
}

func (c *Client) Do(req *http.Request) *httptest.ResponseRecorder {
	// Run any sort of setup code necessary
	c.setupFn(req)

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

func (c *Client) PostForm(path string, data url.Values) *httptest.ResponseRecorder {
	req := c.NewRequest("POST", path, nil)
	req.PostForm = data
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	return c.Do(req)
}

func (c *Client) PostJSON(path string, src interface{}) *httptest.ResponseRecorder {
	encoded := json.Encode(src)
	reader := strings.NewReader(encoded)
	req := c.NewRequest("POST", path, reader)
	req.Header.Set("Content-Type", "application/json")
	return c.Do(req)
}

func (c *Client) PostRawJSON(path string, src string) *httptest.ResponseRecorder {
	reader := strings.NewReader(src)
	req := c.NewRequest("POST", path, reader)
	req.Header.Set("Content-Type", "application/json")
	return c.Do(req)
}
