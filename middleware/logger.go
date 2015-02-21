package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.io/util/log"
)

var (
	green   = string([]byte{27, 91, 57, 55, 59, 52, 50, 109})
	white   = string([]byte{27, 91, 57, 48, 59, 52, 55, 109})
	yellow  = string([]byte{27, 91, 57, 55, 59, 52, 51, 109})
	red     = string([]byte{27, 91, 57, 55, 59, 52, 49, 109})
	blue    = string([]byte{27, 91, 57, 55, 59, 52, 52, 109})
	magenta = string([]byte{27, 91, 57, 55, 59, 52, 53, 109})
	cyan    = string([]byte{27, 91, 57, 55, 59, 52, 54, 109})
	reset   = string([]byte{27, 91, 48, 109})
)

func ErrorLogger() gin.HandlerFunc {
	return ErrorLoggerT(gin.ErrorTypeAll)
}

func ErrorLoggerT(typ uint32) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		errs := c.Errors.ByType(typ)
		if len(errs) > 0 {
			// -1 status code = do not change current one
			c.JSON(-1, c.Errors)
		}
	}
}

// Try and detect verbose flag set on request, we only log DEBUG level in
// production if verbose=1 is added as a query param.
func DetectVerbose(c *gin.Context) bool {
	query := c.Request.URL.Query()

	// We check for both v=true or verbose=true
	param := query.Get("v")
	if param == "" {
		param = query.Get("verbose")
	}

	if param != "" && (param == "1" || param == "true") {
		return true
	}

	return false
}

func Log(c *gin.Context) {
	// Start timer
	start := time.Now()

	// Set verbose mode on context for logger.
	c.Set("verbose", DetectVerbose(c))

	// Process request
	c.Next()

	// Stop timer
	end := time.Now()
	latency := end.Sub(start)

	method := c.Request.Method
	statusCode := c.Writer.Status()
	statusColor := colorForStatus(statusCode)
	methodColor := colorForMethod(method)

	log.Info("%s%3d%s %s%s%s %s %v",
		statusColor, statusCode, reset,
		methodColor, method, reset,
		c.Request.URL.Path,
		latency,
	)
}

func Logger() gin.HandlerFunc {
	return Log
}

func colorForStatus(code int) string {
	switch {
	case code >= 200 && code <= 299:
		return green
	case code >= 300 && code <= 399:
		return white
	case code >= 400 && code <= 499:
		return yellow
	default:
		return red
	}
}

func colorForMethod(method string) string {
	switch {
	case method == "GET":
		return blue
	case method == "POST":
		return cyan
	case method == "PUT":
		return yellow
	case method == "DELETE":
		return red
	case method == "PATCH":
		return green
	case method == "HEAD":
		return magenta
	case method == "OPTIONS":
		return white
	default:
		return reset
	}
}
