package middleware

import (
	"appengine"
	"appengine/datastore"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
	"github.com/qedus/nds"
	"net/http"
	"time"
)

// Automatically get App Engine context.
func AppEngine() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := appengine.NewContext(c.Request)
		c.Set("appengine", ctx)
	}
}

// Middleware for working with sessions
func Sessions(sessions ...string) gin.HandlerFunc {
	var store = sessions.NewCookieStore([]byte("ae5ZsJJ6ThySVPzkQM87KQSAtLfe67eU"))

	return func(ctx *gin.Context) {
		for _, sessionName := range sessions {
			session, _ := store.Get(c.Request, sessionName)
			c.Set(session, sessionName)
		}
	}
}

// Wrapper around appengine/datastore that uses nds for caching:
//   datastore.Get -> nds.Get
//	 datastore.Put -> nds.Put
//	 datastore.Delete -> nds.Delete
//	 datastore.RunInTransaction -> nds.RunInTransaction
func Datastore() gin.HandlerFunc {
	return func(ctx *gin.Context) {
	}
}
