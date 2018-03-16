package http

import (
	"strings"

	"github.com/gin-gonic/gin"

	"hanzo.io/log"
	"hanzo.io/thirdparty/stripe/errors"
	"hanzo.io/util/json"
)

func Render(c *gin.Context, status int, src interface{}) {
	// Write headers
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(status)

	// Render response
	c.Writer.Write(json.EncodeBytes(src))
}

func Fail(c *gin.Context, status int, message interface{}, err error) {
	// Default response
	res := Error{"api-error", "", "", ""}

	// Parse message type
	switch v := message.(type) {
	case Error:
		res = v
	case string:
		res.Message = v
	case map[string]string:
		if typ, ok := v["type"]; ok {
			res.Type = typ
		}
		if code, ok := v["code"]; ok {
			res.Code = code
		}
		if msg, ok := v["message"]; ok {
			res.Message = msg
		}
		if param, ok := v["param"]; ok {
			res.Param = param
		}
	}

	// Support various custom errors
	switch v := err.(type) {
	case *errors.StripeError:
		if v.Type == "card_error" {
			res.Type = "authorization-error"
		} else {
			res.Type = "stripe-error"
		}

		// Use stripe message, param in call cases
		res.Message = v.Message
		res.Param = v.Param

		// Replace underscores in code to make consistent with our API.
		res.Code = strings.Replace(v.Code, "_", "-", -1)
	}

	// Force 402 on auth errors
	if res.Type == "authorization-error" {
		status = 402
	}

	// Write headers
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(status)

	// Render JSON error message
	c.Writer.Write(json.EncodeBytes(gin.H{"error": res}))

	// Log error
	if err != nil {
		if status < 500 {
			log.Warn(err, c)
		} else {
			log.Error(err, c)
		}
	}

	// Stop processing middleware
	c.AbortWithStatus(status)
}
