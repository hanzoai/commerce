package http

import (
	"strings"

	"github.com/gin-gonic/gin"

	"crowdstart.io/thirdparty/stripe"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
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
	res := Error{"api-error", "", ""}

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
	}

	// Support various custom errors
	switch v := err.(type) {
	case stripe.Error:
		if v.Type == "card_error" {
			res.Type = "authorization-error"
		} else {
			res.Type = "stripe-error"
		}

		// Use stripe message in call cases
		res.Message = v.Message

		// Replace underscores in code to make consistent with our API.
		res.Code = strings.Replace(v.Code, "_", "-", -1)
	}

	// Write headers
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(status)

	// Render JSON error message
	c.Writer.Write(json.EncodeBytes(gin.H{"error": res}))

	// Log error
	if err != nil {
		log.Error(err, c)
	}

	// Stop processing middleware
	c.Abort(status)
}
