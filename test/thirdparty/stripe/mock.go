package test

import (
	"io"
	"net/url"

	"hanzo.io/util/log"

	stripeGo "github.com/stripe/stripe-go"
)

type MockBackend struct {
}

func (m *MockBackend) Call(method, path, key string, body *url.Values, params *stripeGo.Params, v interface{}) error {
	log.Panic("Method %v, Path %v, Key %v, Body %v, Params %v, v %v", method, path, key, body, params, v)
	return nil
}

func (m *MockBackend) CallMultipart(method, path, key, boundary string, body io.Reader, params *stripeGo.Params, v interface{}) error {
	log.Panic("Method %v, Path %v, Key %v, Boundary %v, Body %v, Params %v, v %v", method, path, key, boundary, body, params, v)
	return nil
}
