package aetest

import (
	"log"

	"appengine/aetest"
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
		log.Error("Unable to close aetest.Context: %v", err)
	}
}
