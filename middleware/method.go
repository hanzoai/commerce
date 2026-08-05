package middleware

import (
	"errors"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/log"
)

// HeaderMethodOverride is a commonly used
// Http header to override the method.
const HeaderMethodOverride = "X-HTTP-Method-Override"

// ParamMethodOverride is a commonly used
// HTML form parameter to override the method.
const ParamMethodOverride = "_method"

var HttpMethods = []string{"PUT", "PATCH", "DELETE"}

// ErrInvalidOverrideMethod is returned when
// an invalid http method was given to OverrideRequestMethod.
var ErrInvalidOverrideMethod = errors.New("invalid override method")

func IsValidMethodOverride(method string) bool {
	for _, m := range HttpMethods {
		if m == method {
			return true
		}
	}
	return false
}

// OverrideRequestMethod overrides the http
// request's method with the specified method.
func OverrideRequestMethod(c *zip.Ctx, method string) error {
	req := c.Fiber().Request()
	req.Header.Set(HeaderMethodOverride, method)
	req.Header.SetMethod(method)
	return nil
}

func MethodOverride() zip.Handler {
	return func(c *zip.Ctx) error {
		// Only override POST methods
		if c.Method() != "POST" {
			return c.Next()
		}

		// Try to override method using form / query param
		m := c.Fiber().FormValue(ParamMethodOverride)
		if m == "" {
			m = c.Query(ParamMethodOverride)
		}
		if IsValidMethodOverride(m) {
			OverrideRequestMethod(c, m)
		}

		// Try to override method using header
		m = c.Header(HeaderMethodOverride)
		if IsValidMethodOverride(m) {
			OverrideRequestMethod(c, m)
		}

		if c.Method() != "POST" {
			log.Warn("Method overriden to %v", c.Method())
		}

		return c.Next()
	}
}
