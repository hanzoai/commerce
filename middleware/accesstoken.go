package middleware

import (
	"github.com/gin-gonic/gin"

	"appengine"

	"crowdstart.io/datastore"
	"crowdstart.io/models2/organization"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
)

// Require login to view route
func TokenRequired() gin.HandlerFunc {
	fail := func(c *gin.Context, code int, message string, err error) {
		c.Writer.Header().Set("Content-Type", "application/json")
		c.Writer.Write(json.EncodeBytes(gin.H{"code": code, "message": message}))

		if err != nil {
			log.Error(message+": %v", err, c)
		}

		c.Abort(code)
	}

	return func(c *gin.Context) {
		// Get the access token from the Request
		accessToken := c.Request.Header.Get("Authorization")

		// If not set using Authorization header, check for token query param.
		if accessToken == "" {
			query := c.Request.URL.Query()
			accessToken = query.Get("token")
		}

		// Bail if we still don't have an access token
		if accessToken == "" {
			fail(c, 401, "Access token is invalid.", nil)
			return
		}

		ctx := GetAppEngine(c)
		db := datastore.New(ctx)
		o := organization.New(db)

		// Try to validate the org's access token
		if err := o.GetWithAccessToken(accessToken); err != nil {
			fail(c, 401, "Access token is invalid.", nil)
			return
		}

		// Try to get the namespace to the org's key
		if ctx2, err := appengine.Namespace(ctx, o.Id()); err != nil {
			fail(c, 500, "Failed to get namespace for organization.", err)
		} else {
			// Save organization in session
			c.Set("org", o)
			// Save namespace in session
			// Overwrite hte old appengine Context on gin Context
			c.Set("namespace", ctx2)
		}
	}
}

func GetNamespace(c *gin.Context) appengine.Context {
	return c.MustGet("namespace").(appengine.Context)
}

func GetOrg(c *gin.Context) *organization.Organization {
	return c.MustGet("org").(*organization.Organization)
}
