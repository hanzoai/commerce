package middleware

import (
	"appengine"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
	"crowdstart.io/datastore"
)

// Automatically get App Engine context.
func AppEngine() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := appengine.NewContext(c.Request)
		c.Set("appengine", ctx)
	}
}

// Middleware for working with sessions
func Sessions(sessionNames ...string) gin.HandlerFunc {
	var store = sessions.NewCookieStore([]byte("ae5ZsJJ6ThySVPzkQM87KQSAtLfe67eU"))

	return func(c *gin.Context) {
		for _, sessionName := range sessionNames {
			session, _ := store.Get(c.Request, sessionName)
			c.Set(sessionName, session)
		}
	}
}

// Wrapper around appengine/datastore that uses nds for caching:
//   datastore.Get -> nds.Get
//	 datastore.Put -> nds.Put
//	 datastore.Delete -> nds.Delete
//	 datastore.RunInTransaction -> nds.RunInTransaction
func Datastore() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.MustGet("appengine").(appengine.Context)
		c.Set("datastore", datastore.New(ctx))
	}
}
