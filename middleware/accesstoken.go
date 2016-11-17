package middleware

import (
	"encoding/base64"
	"errors"
	"strings"

	"github.com/gin-gonic/gin"

	"appengine"

	"crowdstart.com/datastore"
	"crowdstart.com/models/organization"
	"crowdstart.com/util/bit"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/log"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/session"
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
		return strings.Split(string(bytes), ":")[0]
	}
	return accessToken
}

func ParseToken(c *gin.Context) {
	query := c.Request.URL.Query()

	// Check for `key` query param by default
	accessToken := query.Get("key")

	// Fallback to `token` query param
	if accessToken == "" {
		accessToken = query.Get("token")
	}

	// Try to grab key from Authorization header (basic auth)
	if accessToken == "" {
		accessToken = accessTokenFromHeader(c.Request.Header.Get("Authorization"))
	}

	// If it's still not set and dev server is running, grab from session
	if accessToken == "" && appengine.IsDevAppServer() {
		value, _ := session.Get(c, "access-token")
		if tokstr, ok := value.(string); ok {
			accessToken = tokstr
		}
	}

	c.Set("access-token", accessToken)
}

// Permissions required to access route
func TokenPermits(masks ...bit.Mask) gin.HandlerFunc {
	// Any permissions acceptable by default (i.e., only valid token required)
	permissions := permission.All

	// Any arguments passed will be used as new permissions
	if len(masks) > 0 {
		permissions = permission.None
		for _, mask := range masks {
			permissions |= mask
		}
	}

	return func(c *gin.Context) {
		// Verify permissions
		if !GetPermissions(c).Has(permissions) {
			http.Fail(c, 403, "Token doesn't support this scope", errors.New("Token doesn't support this scope"))
		}
	}
}

// Parses token, default permissions check
func TokenRequired(masks ...bit.Mask) gin.HandlerFunc {
	// Any permissions acceptable by default (i.e., only valid token required)
	permissions := permission.All

	// Any arguments passed will be used as new permissions
	if len(masks) > 0 {
		permissions = permission.None
		for _, mask := range masks {
			permissions |= mask
		}
	}

	return func(c *gin.Context) {
		// Parse token
		ParseToken(c)

		// Try to fetch access token
		accessToken := GetAccessToken(c)

		// Bail if we still don't have an access token
		if accessToken == "" {
			http.Fail(c, 401, "No access token provided.", errors.New("No access token provided"))
			return
		}

		ctx := GetAppEngine(c)
		db := datastore.New(ctx)
		org := organization.New(db)

		// Try to find organization using access token
		tok, err := org.GetWithAccessToken(accessToken)
		if err != nil {
			http.Fail(c, 401, "Unable to retrieve organization associated with access token: "+err.Error(), err)
			return
		}

		// Verify token signature
		if ok, err := tok.Verify(ctx, org.SecretKey); !ok {
			log.Error("Token '%s'\nVerify error '%s' with secret '%s'", tok, err, org.SecretKey, ctx)
			http.Fail(c, 403, "Unable to verify token.", err)
			return
		}

		// Verify permissions
		if !tok.HasPermission(permissions) {
			http.Fail(c, 403, "Token doesn't support this scope", errors.New("Token doesn't support this scope"))
		}

		// Whether or not we can make live calls
		org.Live = tok.HasPermission(permission.Live)

		// Save organization in context
		c.Set("permissions", tok.Permissions)
		c.Set("organization", org)
		c.Set("token", tok)
	}
}

func GetAccessToken(c *gin.Context) string {
	tok, ok := c.Get("access-token")
	if !ok {
		return ""
	}

	return tok.(string)
}

func GetPermissions(c *gin.Context) bit.Field {
	return c.MustGet("permissions").(bit.Field)
}
