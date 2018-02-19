package middleware

import (
	"bytes"
	"fmt"
	"runtime"
	"strings"

	"google.golang.org/appengine"

	"github.com/gin-gonic/gin"

	"hanzo.io/util/json"
	"hanzo.io/log"
	"hanzo.io/util/template"
)

type ErrorDisplayer func(c *context.Context, message string, err error)

// Display errors in JSON
func ErrorJSON(c *context.Context, stack string, err error) {
	c.Writer.Header().Set("Content-Type", "application/json")
	c.AbortWithStatus(500)
	jsonErr := gin.H{
		"error": gin.H{
			"type":    "api-error",
			"message": "Unable to process request. Please try again later. If this continues, please message support@hanzo.io",
		},
	}
	c.Writer.Write(json.EncodeBytes(jsonErr))
	log.Error(stack, c)
}

func ErrorJSONDev(c *context.Context, stack string, err error) {
	c.Writer.Header().Set("Content-Type", "application/json")
	c.AbortWithStatus(500)
	jsonErr := gin.H{
		"error": gin.H{
			"type":    "api-error",
			"message": strings.Split(stack, "\n")[0],
		},
	}
	c.Writer.Write(json.EncodeBytes(jsonErr))
	log.Error(stack)
}

// Display errors in HTML
func ErrorHTML(c *context.Context, stack string, err error) {
	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.AbortWithStatus(500)
	template.Render(c, "error/500.html")
	log.Error(stack, c)
}

func ErrorHTMLDev(c *context.Context, stack string, err error) {
	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.AbortWithStatus(500)
	c.Writer.Write([]byte(`<html>
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
		<h4>500 Internal Server Error (hanzo/1.0)</h4>

		<pre>` + stack + "</pre></body></html>"))
	log.Error(stack)
}

// Handle errors with appropriate ErrorDisplayer
func errorHandler(displayError ErrorDisplayer) gin.HandlerFunc {
	return func(c *context.Context) {
		// On panic
		defer func() {
			if r := recover(); r != nil {
				errstr := fmt.Sprint(r)
				trace := make([]byte, 1024*8)
				runtime.Stack(trace, false)
				stack := string(bytes.Trim(trace, "\x00"))
				err, _ := r.(error)
				displayError(c, errstr+"\n\n"+stack, err)
			}
		}()

		c.Next()

		// When someone calls c.Fail(500)
		if !c.Writer.Written() && c.Writer.Status() == 500 {
			err := c.Errors.Last()
			errstr := err.Error()
			displayError(c, errstr, err)
		}
	}
}

// Error middleware
func ErrorHandler() gin.HandlerFunc {
	if appengine.IsDevAppServer() {
		return errorHandler(ErrorHTMLDev)
	} else {
		return errorHandler(ErrorHTML)
	}
}

func ErrorHandlerJSON() gin.HandlerFunc {
	if appengine.IsDevAppServer() {
		return errorHandler(ErrorJSONDev)
	} else {
		return errorHandler(ErrorJSON)
	}
}
