package _default

import (
	"appengine"
	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/models/fixtures"
	"crowdstart.io/util/exec"
	"crowdstart.io/util/router"
	"github.com/gin-gonic/gin"
)

func Init() {
	router := router.New("/")

	// Warmup: install fixtures, etc.
	router.GET("/_ah/warmup", func(c *gin.Context) {
		ctx := appengine.NewContext(c.Request)
		db := datastore.New(ctx)
		fixtures.Install(db)

		conf := config.Get()

		// Recompile static assets
		if conf.AutoCompileAssets {
			exec.Run("make autocompile-assets")
		}
	})
}
