package middleware

import (
	"appengine"
	"fmt"
	"net/http"
	"runtime"

	"github.com/gin-gonic/gin"

	"crowdstart.io/util/log"
	"crowdstart.io/util/template"
)

// Not needed?
func getStack() string {
	buf := make([]byte, 32)
	for {
		n := runtime.Stack(buf, false)
		if n < len(buf) {
			break
		}
		buf = make([]byte, len(buf)*2)
	}
	return string(buf)
}

// Show our error page & log it out
func handleError(c *gin.Context, stack string) {
	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Writer.WriteHeader(http.StatusInternalServerError)

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
		template.Render(c, "error/500.html")
	}

	ctx := GetAppEngine(c)
	log.Error(stack, ctx)
}

// Serve custom 500 error page and log errors
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// On panic
		defer func() {
			if rval := recover(); rval != nil {
				errstr := fmt.Sprint(rval)
				trace := make([]byte, 1024*16)
				runtime.Stack(trace, false)
				handleError(c, errstr+"\n\n"+string(trace))
			}
		}()

		c.Next()

		// When someone calls c.Fail(500)
		if !c.Writer.Written() && c.Writer.Status() == 500 {
			stack := fmt.Sprint(c.LastError())
			// stack = stack + "\n" + getStack()
			handleError(c, stack)
		}
	}
}
