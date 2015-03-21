package middleware

import (
	"github.com/gin-gonic/gin"

	"appengine"

	"crowdstart.io/datastore"
	"crowdstart.io/models2/organization"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
	"crowdstart.io/util/session"
)

// Require login to view route
func TokenRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the access token from the Request
		accessToken := c.Request.Header.Get("Authorization")

		// If not set using Authorization header, check for token query param.
		if accessToken == "" {
			query := c.Request.URL.Query()
			accessToken = query.Get("token")
		}

		// During development cookie may be set from development pages.
		if appengine.IsDevAppServer() && accessToken == "" {
			accessToken = session.MustGet(c, "access-token").(string)
		}

		log.Debug("access token: %v", accessToken)

		// Bail if we still don't have an access token
		if accessToken == "" {
			json.Fail(c, 401, "No access token provided.", nil)
			return
		}

		ctx := GetAppEngine(c)
		db := datastore.New(ctx)
		o := organization.New(db)

		// Try to validate the org's access token
		if err := o.GetWithAccessToken(accessToken); err != nil {
			json.Fail(c, 401, "Unable to retrieve organization associated with access token: "+err.Error(), err)
			return
		}

		// Try to get the namespace to the org's key
		if ctx2, err := appengine.Namespace(ctx, o.Id()); err != nil {
			json.Fail(c, 500, "Failed to get namespace for organization: %v"+o.Id(), err)
		} else {
			// Save organization in context
			c.Set("org", o)
			// Save namespace in context
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
