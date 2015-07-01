package middleware

import (
	"encoding/base64"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"crowdstart.com/auth"
	"crowdstart.com/datastore"
	"crowdstart.com/models/user"
	"crowdstart.com/util/log"
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

func BasicAuth() gin.HandlerFunc {
	realm := "Basic realm=" + strconv.Quote("Authorization Required")

	return func(c *gin.Context) {
		id, password := parseAuthHeader(c.Request.Header.Get("Authorization"))

		db := datastore.New(c)
		usr := user.New(db)
		if err := usr.GetById(id); err != nil {
			c.Request.Header.Set("WWW-Authenticate", realm)
			c.Abort(401)
			log.Warn("Unable to get user by id '%v': %v", id, err, c)
		}

		// Validate password
		if !usr.ComparePassword(password) {
			c.Request.Header.Set("WWW-Authenticate", realm)
			c.Abort(401)
			log.Warn("Invalid password for user: %v", usr, c)
		}

		// Login user on session
		auth.Login(c, usr)
	}
}
