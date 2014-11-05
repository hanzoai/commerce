package store

import (
	"appengine"
	"crowdstart.io/datastore"
	"crowdstart.io/models/fixtures"
	"crowdstart.io/store/cart"
	"crowdstart.io/store/products"
	"crowdstart.io/util/exec"
	"crowdstart.io/util/fs"
	"crowdstart.io/util/router"
	"github.com/gin-gonic/gin"
)

func init() {
	router := router.New("/")

	// Products
	router.GET("/", products.List)
	router.GET("/products", products.List)
	router.GET("/products/:slug", products.Get)

	// Cart
	router.GET("/cart", cart.Get)

	// Warmup, install fixtures, etc.
	router.GET("_ah/warmup", func(c *gin.Context) {
		ctx := appengine.NewContext(c.Request)
		db := datastore.New(ctx)
		fixtures.Install(db)

		// Recompile static assets
		if appengine.IsDevAppServer() {
			exec.Run("/usr/local/bin/requisite ../assets/js/crowdstart.coffee -g -o /tmp/crowdstart.js")
			a := fs.ReadFile("../static/js/crowdstart.js")
			b := fs.ReadFile("/tmp/crowdstart.js")
			if a != b {
				exec.Run("/bin/mv /tmp/crowdstart.js ../static/js/crowdstart.js")
			}
		}
	})
}
