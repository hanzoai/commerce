package http

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/util/json"
)

// Render writes src as JSON with the given status. Handlers return it:
//
//	return http.Render(c, 200, out)
func Render(c *zip.Ctx, status int, src interface{}) error {
	c.SetHeader("Content-Type", "application/json")
	return c.Bytes(status, json.EncodeBytes(src))
}

// Fail renders the canonical {"error": …} envelope and ends the request —
// callers return it, which is what stops the chain (no gin-style Abort; not
// calling Next IS the abort).
func Fail(c *zip.Ctx, status int, message interface{}, err error) error {
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

	// Force 402 on auth errors
	if res.Type == "authorization-error" {
		status = 402
	}

	// Log error
	if err != nil {
		if status < 500 {
			log.Warn(err, c)
		} else {
			log.Error(err, c)
		}
	}

	c.SetHeader("Content-Type", "application/json")
	return c.Bytes(status, json.EncodeBytes(map[string]any{"error": res}))
}
