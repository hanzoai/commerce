package test

import (
	"io"
	"net/url"

	"crowdstart.io/thirdparty/stripe2"
	"crowdstart.io/util/log"

	. "crowdstart.io/util/test/ginkgo"
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

var _ = Describe("Authorize", func() {
	Context("Authorize payment", func() {
		It("Authorize a new payment", func() {
			stripe.New(ctx, "")
			stripeGo.SetBackend(stripeGo.APIBackend, &MockBackend{})
		})
	})
})
