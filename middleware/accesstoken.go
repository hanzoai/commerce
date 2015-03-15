package middleware

import (
	"github.com/gin-gonic/gin"

	"appengine"

	"crowdstart.io/datastore"
	"crowdstart.io/models2/organization"
)

// Require login to view route
func RequiresOrgToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		accessToken := c.Request.Header.Get("Authorization")

		db := datastore.New(c)
		o := organization.New(db)

		if err := o.GetWithAccessToken(accessToken); err != nil {
			c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			c.Abort(500)
		}

		if c2, err := appengine.Namespace(GetAppEngine(c), o.Id()); err != nil {
			c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			c.Abort(500)
		} else {
			c.Set("appengine", c2)
		}
	}
}
