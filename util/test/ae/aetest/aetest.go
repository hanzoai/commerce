package aetest

import (
	"github.com/zeekay/aetest"

	"crowdstart.io/util/log"
	"crowdstart.io/util/test/ae/context"
	"crowdstart.io/util/test/ae/options"
)

// Context which hase a Close() method returning an error for compatibility
// with aetest Context.
type shimContext struct {
	aetest.Context
	namespace string
}

// Unimplemented
func (c shimContext) AppID() (appid string) {
	return appid
}
func (c shimContext) CurrentUser() (user string) {
	return user
}
func (c shimContext) CurrentNamespace(namespace string) {
	c.namespace = namespace

}
func (c shimContext) GetCurrentNamespace() string {
	return c.namespace
}

// Converter so Close() method matches signature we need.
func (c shimContext) Close() {
	if err := c.Context.Close(); err != nil {
		log.Fatal("Unable to close aetest.Context: %v", err)
	}
}

// Create a new *aetest.Context
func New(opts options.Options) (context.Context, error) {
	_opts := &aetest.Options{
		StronglyConsistentDatastore: opts.StronglyConsistentDatastore,
	}
	if ctx, err := aetest.NewContext(_opts); err != nil {
		return nil, err
	} else {
		_ctx := new(shimContext)
		_ctx.Context = ctx
		return _ctx, err
	}

}
