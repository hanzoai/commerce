package fakecontext

import (
	"context"
	"encoding/gob"
	"net/http"
	"net/url"
	"strings"

	"github.com/zap-proto/fiber/v3/middleware/adaptor"
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
)

// app backs the synthetic *zip.Ctx a delayed task runs against.
var app = zip.New(zip.Config{DisableStartupMessage: true})

// serializedKeys is the set of request locals carried across the delay queue.
// zip.Ctx (fiber) locals are not enumerable the way gin's Keys map was, so we
// snapshot the known request-scoped values a background task needs. The
// "organization" local is captured separately as its id (below).
var serializedKeys = []string{"test", "verbose", "namespace"}

// Param is one captured route parameter (gin.Param without the gin dependency).
type Param struct {
	Key   string
	Value string
}

// Request that can be almost completely be serialized to/from a real Request
type Request struct {
	Close            bool
	ContentLength    int64
	Form             url.Values
	Header           http.Header
	Host             string
	Method           string
	PostForm         url.Values
	Proto            string
	ProtoMajor       int
	ProtoMinor       int
	RemoteAddr       string
	RequestURI       string
	Trailer          http.Header
	TransferEncoding []string
}

func (r Request) Request() (req *http.Request, err error) {
	req = &http.Request{
		Close:            r.Close,
		ContentLength:    r.ContentLength,
		Form:             r.Form,
		Header:           r.Header,
		Host:             r.Host,
		Method:           r.Method,
		PostForm:         r.PostForm,
		Proto:            r.Proto,
		ProtoMajor:       r.ProtoMajor,
		ProtoMinor:       r.ProtoMinor,
		RemoteAddr:       r.RemoteAddr,
		RequestURI:       r.RequestURI,
		Trailer:          r.Trailer,
		TransferEncoding: r.TransferEncoding,
	}

	// Rebuild URL object
	rawurl := r.RequestURI
	justAuthority := r.Method == "CONNECT" && !strings.HasPrefix(rawurl, "/")
	if justAuthority {
		rawurl = "http://" + rawurl
	}

	if req.URL, err = url.ParseRequestURI(rawurl); err != nil {
		return nil, err
	}

	return req, nil
}

func NewRequest(r *http.Request) *Request {
	return &Request{
		Close:            r.Close,
		ContentLength:    r.ContentLength,
		Form:             r.Form,
		Header:           r.Header,
		Host:             r.Host,
		Method:           r.Method,
		PostForm:         r.PostForm,
		Proto:            r.Proto,
		ProtoMajor:       r.ProtoMajor,
		ProtoMinor:       r.ProtoMinor,
		RemoteAddr:       r.RemoteAddr,
		RequestURI:       r.RequestURI,
		Trailer:          r.Trailer,
		TransferEncoding: r.TransferEncoding,
	}
}

// zip.Ctx replacement that can be almost completely be serialized to/from
// a request context.
type Context struct {
	Keys    map[string]interface{}
	Params  []Param
	Request *Request
}

func (c Context) Context(ctx context.Context) (zc *zip.Ctx, err error) {
	method := "GET"
	path := "/"
	if c.Request != nil {
		if c.Request.Method != "" {
			method = c.Request.Method
		}
		if c.Request.RequestURI != "" {
			path = c.Request.RequestURI
		}
	}

	zc = app.TestCtx(method, path)
	for k, v := range c.Keys {
		zc.Locals(k, v)
	}

	// If we don't have a context, this is all we can do for now
	if ctx == nil {
		return zc, nil
	}

	// ...otherwise use context to update the request context
	zc.SetContext(ctx)

	// Fetch organization if organization-id is set
	if value := zc.Locals("organization-id"); value == nil {
		if id, ok := value.(string); ok {
			db := datastore.New(ctx)
			org := organization.New(db)
			org.GetById(id)
			zc.Locals("organization", org)
		}
	}
	return zc, nil
}

func NewContext(c *zip.Ctx) *Context {
	ctx := new(Context)

	ctx.Keys = make(map[string]interface{}, 0)

	// Snapshot route params off the matched route.
	if route := c.Fiber().Route(); route != nil {
		for _, name := range route.Params {
			ctx.Params = append(ctx.Params, Param{Key: name, Value: c.Param(name)})
		}
	}

	// Need to create request context, because the live request cannot be
	// gob-encoded — bridge to a net/http request and snapshot its fields.
	if req, err := adaptor.ConvertRequest(c.Fiber(), false); err == nil {
		ctx.Request = NewRequest(req)
	} else {
		ctx.Request = &Request{}
	}

	// Save organization id so we can fetch it on the other side.
	if org, ok := c.Locals("organization").(*organization.Organization); ok && org != nil {
		ctx.Keys["organization-id"] = org.Id()
	}

	// Clone the known serializable request locals (context is skipped — it
	// can't gob encode, and there's no point).
	for _, k := range serializedKeys {
		if v := c.Locals(k); v != nil {
			ctx.Keys[k] = v
		}
	}

	return ctx
}

func init() {
	gob.Register(&Context{})
}
