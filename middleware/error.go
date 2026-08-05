package middleware

import (
	"bytes"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
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

		// A downstream handler returned an error — the c.Fail analog. The error's
		// KIND decides the status: a typed *zip.HTTPError carries its status as a
		// VALUE (ErrForbidden→403, ErrUnauthorized→401, ErrNotFound→404,
		// Errorf(status,…)), so it renders with THAT status through the canonical
		// envelope. The status is derived here, in this ONE place, never re-guessed
		// per handler. Anything else — a plain error or a typed 500 — is a genuine
		// internal fault and falls through to the generic 500 renderer, which hides
		// internals rather than leaking a real cause to the client.
		if err := c.Next(); err != nil {
			var he *zip.HTTPError
			if errors.As(err, &he) && he.Status != 0 && he.Status != 500 {
				return http.Fail(c, he.Status, he.Msg, he)
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
