package middleware

import (
	"encoding/base64"
	"strconv"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/user"
)

func parseAuthHeader(fieldValue string) (string, string) {
	method, encoded := splitAuthorization(fieldValue)
	if method != "Basic" {
		return method, encoded
	}

	bytes, _ := base64.StdEncoding.DecodeString(encoded)
	credentials := strings.Split(string(bytes), ":")
	return credentials[0], credentials[1]
}

func BasicAuth() zip.Handler {
	realm := "Basic realm=" + strconv.Quote("Authorization Required")

	return func(c *zip.Ctx) error {
		email, password := parseAuthHeader(c.Header("Authorization"))

		db := datastore.New(c.Context())
		usr := user.New(db)
		if err := usr.GetByEmail(email); err != nil {
			c.SetHeader("WWW-Authenticate", realm)
			log.Warn("Unable to get user with email '%v': %v", email, err, c)
			return c.NoContent(401)
		}

		// Validate password
		if !usr.ComparePassword(password) {
			c.SetHeader("WWW-Authenticate", realm)
			log.Warn("Invalid password for user: %v", usr, c)
			return c.NoContent(401)
		}

		// Login user on session
		auth.Login(c, usr)
		return c.Next()
	}
}
