package _default

import (
	"appengine"
	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/models/fixtures"
	"crowdstart.io/util/exec"
	"crowdstart.io/util/fs"
	"crowdstart.io/util/router"
	"github.com/gin-gonic/gin"
)

func Init() {
	router := router.New("/")

	router.GET("/hello", func(c *gin.Context) {
		c.String(200, "hi")
	})

	// Warmup: install fixtures, etc.
	router.GET("/_ah/warmup", func(c *gin.Context) {
		ctx := appengine.NewContext(c.Request)
		db := datastore.New(ctx)
		fixtures.Install(db)

		// Recompile static assets
		if config.Get().AutoCompileAssets {
			for _, bundle := range []string{"store", "checkout"} {
				exec.Run("requisite ../assets/js/" + bundle + ".coffee -g -o /tmp/" + bundle + ".js")
				a := fs.ReadFile("../static/js/" + bundle + ".js")
				b := fs.ReadFile("/tmp/" + bundle + ".js")
				if a != b {
					exec.Run("mv /tmp/" + bundle + ".js ../static/js/" + bundle + ".js")
				}
			}
		}
	})
}
