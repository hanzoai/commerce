package context

import (
	"crowdstart.io/util/log"

	"appengine"
)

var registry = make(map[string]appengine.Context)

func Register(requestId string, ctx appengine.Context) {
	log.Debug("registering %v", requestId)
	registry[requestId] = ctx
}

func Get(requestId string) appengine.Context {
	log.Debug("getting %v", requestId)
	return registry[requestId]
}
