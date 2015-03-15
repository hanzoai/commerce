package middleware

import (
	"fmt"
	"runtime"

	"appengine"

	"github.com/gin-gonic/gin"

	"crowdstart.io/util/template"
)

// Show our error page & log it out
func displayError(c *gin.Context, stack string) {
	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Abort(500)

	// Trim beginning of stacktrace
	// lines := strings.Split(stack, "\n")
	// msg := lines[0]
	// lines = append([]string{msg + "\n"}, lines[5:]...)
	// stack = strings.Join(lines, "\n")

	if appengine.IsDevAppServer() {
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
		<h4>500 Internal Server Error (crowdstart/1.0.0)</h4>

		<pre>` + stack + "</pre></body></html>"))
	} else {
		ctx := c.MustGet("appengine").(appengine.Context)
		ctx.Criticalf("500: %v", stack)
		template.Render(c, "error/500.html")
	}
}

// Serve custom 500 error page and log errors
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// On panic
		defer func() {
			if r := recover(); r != nil {
				errstr := fmt.Sprint(r)
				trace := make([]byte, 1024*8)
				runtime.Stack(trace, false)
				displayError(c, errstr+"\n\n"+string(trace))
			}
		}()

		c.Next()

		// When someone calls c.Fail(500)
		if !c.Writer.Written() && c.Writer.Status() == 500 {
			err := c.LastError()
			stack := fmt.Sprint(err)
			// stack = stack + "\n" + getStack()
			displayError(c, stack)
		}
	}
}
