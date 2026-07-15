package middleware

import (
	"bytes"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"github.com/zap-proto/fiber/v3"
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/util/json"
)

type ErrorDisplayer func(c *zip.Ctx, message string, err error)

// Display errors in JSON
func ErrorJSON(c *zip.Ctx, stack string, err error) {
	c.SetHeader("Content-Type", "application/json")
	jsonErr := map[string]any{
		"error": map[string]any{
			"type":    "api-error",
			"message": "Unable to process request. Please try again later.",
		},
	}
	c.Bytes(500, json.EncodeBytes(jsonErr))
	log.Error(stack, c)
}

func ErrorJSONDev(c *zip.Ctx, stack string, err error) {
	c.SetHeader("Content-Type", "application/json")
	jsonErr := map[string]any{
		"error": map[string]any{
			"type":    "api-error",
			"message": strings.Split(stack, "\n")[0],
		},
	}
	c.Bytes(500, json.EncodeBytes(jsonErr))
	log.Error(stack)
}

// Display errors in HTML
func ErrorHTML(c *zip.Ctx, stack string, err error) {
	c.SetHeader("Content-Type", "text/html; charset=utf-8")
	c.Bytes(500, []byte(`<html>
		<h1>500 - Internal Server Error</h1>
		<p>We weren't able to complete your request. Please try again later.</p>
	</html>`))
	log.Error(stack, c)
}

func ErrorHTMLDev(c *zip.Ctx, stack string, err error) {
	c.SetHeader("Content-Type", "text/html; charset=utf-8")
	c.Bytes(500, []byte(`<html>
	<head>
		<title>Error: 500</title>
		<meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
		<style>
			body {
				font-family:monospace;
				margin:20px
			}
		</style>
	</head>
	<body>
		<h4>500 Internal Server Error </h4>

		<pre>`+stack+"</pre></body></html>"))
	log.Error(stack)
}

// Handle errors with appropriate ErrorDisplayer. Recovers panics and renders
// any error a downstream handler returns — the commerce envelope, not zip's
// default {error,code,status}. Returning nil after rendering ends the request.
func errorHandler(displayError ErrorDisplayer) zip.Handler {
	return func(c *zip.Ctx) (result error) {
		// On panic
		defer func() {
			if r := recover(); r != nil {
				errstr := fmt.Sprint(r)
				trace := make([]byte, 1024*8)
				runtime.Stack(trace, false)
				stack := string(bytes.Trim(trace, "\x00"))
				err, _ := r.(error)
				displayError(c, errstr+"\n\n"+stack, err)
				result = nil
			}
		}()

		// When a downstream handler returns an error (the c.Fail(500) analog).
		if err := c.Next(); err != nil {
			// A downstream handler that RETURNS a typed HTTP error (zip.HTTPError
			// or fiber.Error) already carries its own status — re-return it so
			// zip's faithful default handler renders THAT status. Commerce's
			// generic 500 envelope is reserved for UNTYPED failures (and panics),
			// so this middleware never clobbers another subsystem's 401/403/404
			// into a 500 when it shares the /v1 prefix (co-residence).
			var he *zip.HTTPError
			var fe *fiber.Error
			if errors.As(err, &he) || errors.As(err, &fe) {
				return err
			}
			displayError(c, err.Error(), err)
			return nil
		}
		return nil
	}
}

// Error middleware
func ErrorHandler() zip.Handler {
	if config.IsDevelopment {
		return errorHandler(ErrorHTMLDev)
	} else {
		return errorHandler(ErrorHTML)
	}
}

func ErrorHandlerJSON() zip.Handler {
	if config.IsDevelopment {
		return errorHandler(ErrorJSONDev)
	} else {
		return errorHandler(ErrorJSON)
	}
}
