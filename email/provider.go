package email

import (
	"appengine"
)

type Provider interface {
	Send(appengine.Context, Email)
	SendTemplate(appengine.Context, Email)
	SendProviderTemplate(appengine.Context, Email)
}
