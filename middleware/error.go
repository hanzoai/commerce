package middleware

import (
	"bytes"
	"fmt"
	"runtime"
	"strings"

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

// Recovers panics. A panic is unhandled by definition, so 500 is the honest
// status and the stack is all we know.
//
// A returned error is NOT a panic: the handler chose it and encoded what it
// means, e.g. zip.ErrForbidden -> 403. Rendering that here would discard the
// status (there is no status to read off a bare `error`) and flatten every
// refusal into a 500. So returned errors pass through to zip's handler, which
// reads *zip.HTTPError and honors it. One error path, one envelope.
func errorHandler(displayError ErrorDisplayer) zip.Handler {
	return func(c *zip.Ctx) (result error) {
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

		return c.Next()
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
