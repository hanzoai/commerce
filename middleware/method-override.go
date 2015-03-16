package middleware

import (
	"errors"

	"github.com/gin-gonic/gin"
)

// HeaderOverride is a commonly used
// Http header to override the method.
const HeaderOverride = "X-HTTP-Method-Override"

// ParamOverride is a commonly used
// HTML form parameter to override the method.
const ParamOverride = "_method"

var httpMethods = []string{"PUT", "PATCH", "DELETE"}

// ErrInvalidOverrideMethod is returned when
// an invalid http method was given to OverrideRequestMethod.
var ErrInvalidOverrideMethod = errors.New("invalid override method")

func isValidOverrideMethod(method string) bool {
	for _, m := range httpMethods {
		if m == method {
			return true
		}
	}
	return false
}

// OverrideRequestMethod overrides the http
// request's method with the specified method.
func overrideRequestMethod(c *gin.Context, method string) error {
	c.Request.Header.Set(HeaderOverride, method)
	c.Request.Method = method
	return nil
}

func MethodOverride() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only override POST methods
		if c.Request.Method != "POST" {
			return
		}

		m := c.Request.FormValue(ParamOverride)
		if isValidOverrideMethod(m) {
			overrideRequestMethod(c, m)
		}
		m = c.Request.Header.Get(HeaderOverride)
		if isValidOverrideMethod(m) {
			overrideRequestMethod(c, m)
		}
	}
}
