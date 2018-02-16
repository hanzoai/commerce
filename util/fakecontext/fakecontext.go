package fakecontext

import (
	"encoding/gob"
	"net/http"
	"net/url"
	"strings"

	"google.golang.org/appengine"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/organization"
	"hanzo.io/util/log"
)

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

// context.Context replacement that can be almost completely be serialized to/from a context.Context
type Context struct {
	Keys    map[string]interface{}
	Params  gin.Params
	Request *Request
}

func (c *Context) cloneKeys(keys map[string]interface{}) {
	for k, v := range keys {
		// Skip app engine
		if k == "google.golang.org/appengine" {
			continue
		}

		// save organization id so we can fetch it on the other side
		if k == "organization" {
			c.Keys["organization-id"] = (v.(*organization.Organization)).Id()
			continue
		}

		c.Keys[k] = v
	}
}

func (c Context) Context(aectx *context.Context) (ctx *context.Context, err error) {
	ctx = new(context.Context)
	ctx.Errors = ctx.Errors[0:0]
	ctx.Keys = c.Keys
	ctx.Params = c.Params

	ctx.Request, err = c.Request.Request()
	if err != nil {
		log.Warn("Failed to create Request from Request: %v", err)
	}

	// If we don't have an appengine context, this is all we can do for now
	if aectx == nil {
		return ctx, err
	}

	// ...otherwise use appengine context to update gin context
	ctx.Set("google.golang.org/appengine", *aectx)

	// Fetch organization if organization-id is set
	if value, ok := ctx.Get("organization-id"); !ok {
		if id, ok := value.(string); ok {
			db := datastore.New(*aectx)
			org := organization.New(db)
			org.GetById(id)
			ctx.Set("organization", org)
		}
	}
	return ctx, err
}

func NewContext(c *context.Context) *Context {
	ctx := new(Context)

	ctx.Keys = make(map[string]interface{}, 0)

	ctx.Params = c.Params

	// Need to create request context, because c.Request cannot be gob-encoded
	if c.Request != nil {
		ctx.Request = NewRequest(c.Request)
	} else {
		ctx.Request = &Request{}
	}

	// Clone keys, skipping app engine context (can't gob encode, also no point)
	ctx.cloneKeys(c.Keys)

	return ctx
}

func init() {
	gob.Register(&Context{})
}
