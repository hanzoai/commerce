package middleware

import (
	"errors"

	"github.com/gin-gonic/gin"

	"crowdstart.com/auth"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/log"
	"crowdstart.com/util/session"
)

func AccountRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		if u, err := auth.GetCurrentUser(c); err != nil {
			log.Warn("Unable to acquire account.")
			session.Clear(c)
			http.Fail(c, 403, "Access Denied", errors.New("Access Denied"))
		} else {
			c.Set("user", u)
		}
	}
}
