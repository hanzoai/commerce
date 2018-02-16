package context

import (
	"appengine/user"
	"appengine_internal"
)

type Context interface {
	// Standard context.Context methods
	Call(service, method string, in, out appengine_internal.ProtoMessage, opts *appengine_internal.CallOptions) error
	FullyQualifiedAppID() string
	Request() interface{}

	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warningf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Criticalf(format string, args ...interface{})

	// Non-standard methods
	AppID() string
	CurrentNamespace(namespace string)
	CurrentUser() string
	GetCurrentNamespace() string
	Login(u *user.User)
	Logout()
	Close()
}
