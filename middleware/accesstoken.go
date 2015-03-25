package middleware

import (
	"encoding/base64"
	"strings"

	"github.com/gin-gonic/gin"

	"appengine"

	"crowdstart.io/datastore"
	"crowdstart.io/models2/organization"
	"crowdstart.io/util/bit"
	"crowdstart.io/util/json"
	"crowdstart.io/util/permission"
	"crowdstart.io/util/session"
)

func splitAuthorization(fieldValue string) (string, string) {
	parts := strings.Split(fieldValue, " ")
	if len(parts) == 1 {
		return "", parts[0]
	}
	return parts[0], parts[1]
}

func accessTokenFromHeader(fieldValue string) string {
	method, accessToken := splitAuthorization(fieldValue)
	if method == "Basic" {
		bytes, _ := base64.StdEncoding.DecodeString(accessToken)
		return string(bytes)
	}
	return accessToken
}

func GetPermissions(c *gin.Context) bit.Field {
	return c.MustGet("permissions").(bit.Field)
}

// Require login to view route
func TokenRequired(masks ...bit.Mask) gin.HandlerFunc {
	// Any permissions acceptable by default (i.e., only valid token required)
	permissions := permission.Any

	// Any arguments passed will be used as new permissions
	if len(masks) > 0 {
		permissions = permission.None
		for _, mask := range masks {
			permissions |= mask
		}
	}

	return func(c *gin.Context) {
		// Get the access token from the Request
		accessToken := accessTokenFromHeader(c.Request.Header.Get("Authorization"))

		// If not set using Authorization header, check for token query param.
		if accessToken == "" {
			query := c.Request.URL.Query()
			accessToken = query.Get("token")

			// During development cookie may be set from development pages.
			if appengine.IsDevAppServer() && accessToken == "" {
				value, _ := session.Get(c, "access-token")
				if tokstr, ok := value.(string); ok {
					accessToken = tokstr
				}
			}
		}

		// Bail if we still don't have an access token
		if accessToken == "" {
			json.Fail(c, 401, "No access token provided.", nil)
			return
		}

		ctx := GetAppEngine(c)
		db := datastore.New(ctx)
		org := organization.New(db)

		// Try to validate the org's access token
		tok, err := org.GetWithAccessToken(accessToken)
		if err != nil {
			json.Fail(c, 401, "Unable to retrieve organization associated with access token: "+err.Error(), err)
			return
		}

		// Verify token signature
		if !tok.Verify(org.SecretKey) {
			json.Fail(c, 403, "Unable to verify token: "+err.Error(), err)
		}

		// Verify permissions
		if !tok.HasPermission(permissions) {
			json.Fail(c, 403, "Token doesn't support this scope", err)
		}

		// Save organization in context
		c.Set("permissions", tok.Permissions)
		c.Set("organization", org)
		c.Set("organizationId", org.Id())
	}
}
