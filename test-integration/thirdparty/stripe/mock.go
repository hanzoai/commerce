package test

import (
	"bytes"

	"github.com/hanzoai/commerce/log"

	stripe "github.com/stripe/stripe-go/v84"
)

// MockBackend is a no-op Stripe backend used in integration tests.
type MockBackend struct{}

func (m *MockBackend) Call(method, path, key string, params stripe.ParamsContainer, v stripe.LastResponseSetter) error {
	log.Panic("Mock Call: %v %v", method, path)
	return nil
}

func (m *MockBackend) CallStreaming(method, path, key string, params stripe.ParamsContainer, v stripe.StreamingLastResponseSetter) error {
	log.Panic("Mock CallStreaming: %v %v", method, path)
	return nil
}

func (m *MockBackend) CallRaw(method, path, key string, body []byte, params *stripe.Params, v stripe.LastResponseSetter) error {
	log.Panic("Mock CallRaw: %v %v", method, path)
	return nil
}

func (m *MockBackend) CallMultipart(method, path, key, boundary string, body *bytes.Buffer, params *stripe.Params, v stripe.LastResponseSetter) error {
	log.Panic("Mock CallMultipart: %v %v", method, path)
	return nil
}

func (m *MockBackend) SetMaxNetworkRetries(maxNetworkRetries int64) {}

// Compile-time check.
var _ stripe.Backend = (*MockBackend)(nil)
