package middleware

import (
	"appengine"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"sync"

	"github.com/getsentry/raven-go"
	"github.com/gin-gonic/gin"

	"crowdstart.io/util/log"
	"crowdstart.io/util/template"
)

var once sync.Once
var sentryDsn = "https://4daf3e86c2744df4b932abbe4eb48aa8:27fa30055d9747e795ca05d5ffb96f0c@app.getsentry.com/32164"
var client *raven.Client

// Logs errors to sentry
func logToSentry(c *gin.Context, ctx appengine.Context, stack string) {

	// Only capture to sentry in production
	if appengine.IsDevAppServer() {
		return
	}

	// Get client
	once.Do(func() {
		client, err := raven.NewClient(sentryDsn, map[string]string{})
		if err != nil {
			ctx.Errorf("Unable to create Sentry client: %v, %v", client, err)
		}
	})

	// Send request
	flags := map[string]string{
		"endpoint": c.Request.RequestURI,
	}

	if client != nil {
		packet := raven.NewPacket(stack, raven.NewException(errors.New(stack), raven.NewStacktrace(2, 3, nil)))
		client.Capture(packet, flags)
	}
}

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
	c.Writer.Header().Set("Content-Type", "text/html")
	c.Writer.WriteHeader(http.StatusInternalServerError)

	if appengine.IsDevAppServer() {
		c.Writer.Write([]byte("<head><style>body{font-family:monospace; margin:20px}</style><h4>500 Internal Server Error (crowdstart/1.0.0)</h1><pre>" + stack + "</pre>"))
	} else {
		template.Render(c, "error/500.html")
	}

	ctx := GetAppEngine(c)
	log.Error(stack, ctx)
	logToSentry(c, ctx, stack)
}

// Serve custom 500 error page and log to sentry in production.
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
