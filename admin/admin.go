package admin

import (
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/router"
	"crowdstart.io/util/template"
	"github.com/gin-gonic/gin"
	"net/http"
)

func init() {
	router := router.New()

	admin := router.Group("/admin")

	// Show stripe button
	admin.GET("/stripe/connect", func(c *gin.Context) {
		template.Render("admin/stripe/connect.html")
	})

	// Redirected on success from connect button.
	admin.POST("/stripe/success/:userid/:token", func(c *gin.Context) {
		db := datastore.New(c)
		token := c.Params.ByName("token")
		userid := c.Params.ByName("userid")

		// get user instance
		user := new(models.User)
		db.GetKey("user", userid, user)

		// update  stripe token
		user.stripeToken = token

		// update in datastore
		db.PutKey("user", userid, user)

		template.Render("admin/stripe/success.html")
	})

	admin.GET("/", func(c *gin.Context) {
		c.String(200, "api")
	})

	http.Handle("/admin/", router)
}
