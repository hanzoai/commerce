package aetest

import (
	"errors"

	"appengine/aetest"

	"crowdstart.com/util/log"
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
	var err error
	defer func() {
		if r := recover(); r != nil {
			switch v := r.(type) {
			case string:
				err = errors.New(v)
			case error:
				err = v
			}
		}
		if err != nil {
			log.Error("Unable to close aetest.Context: %v", err)
		}
	}()
	err = c.Context.Close()
}
