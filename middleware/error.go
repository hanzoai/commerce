package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"sync"
	"appengine"
	"github.com/getsentry/raven-go"
	"github.com/gin-gonic/gin"
)

func Recovery() gin.HandlerFunc {
	var once sync.Once
	var sentryDsn = "https://4daf3e86c2744df4b932abbe4eb48aa8:27fa30055d9747e795ca05d5ffb96f0c@app.getsentry.com/32164"
	var client *raven.Client

	return func(c *gin.Context) {
		defer func() {
			ctx := GetAppEngine(c)

			flags := map[string]string{
				"endpoint": c.Request.RequestURI,
			}

			if rval := recover(); rval != nil {
				c.Writer.WriteHeader(http.StatusInternalServerError)
				debug.PrintStack()
				rvalStr := fmt.Sprint(rval)
				ctx.Errorf(rvalStr)

				if !appengine.IsDevAppServer() {
					once.Do(func() {
						client, err := raven.NewClient(sentryDsn, map[string]string{})
						if err != nil {
							ctx.Errorf("Unable to create Sentry client: %v, %v", client, err)
						}
					})

					if client != nil {
						packet := raven.NewPacket(rvalStr, raven.NewException(errors.New(rvalStr), raven.NewStacktrace(2, 3, nil)))
						client.Capture(packet, flags)
					}
				}

				c.Writer.WriteHeader(http.StatusInternalServerError)
				http.ServeFile(c.Writer, c.Request, "../static/500.html")
			}
		}()
		c.Next()
	}
}
