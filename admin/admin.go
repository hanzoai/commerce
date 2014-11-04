package admin

import (
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/form"
	"crowdstart.io/util/router"
	"crowdstart.io/util/template"
	"crowdstart.io/auth"
	"github.com/gin-gonic/gin"
)

func init() {
	admin := router.New("/admin/")

	// Admin index
	admin.GET("/", func(c *gin.Context) {
		template.Render(c, "index.html")
	})

	// Show stripe button
	admin.GET("/stripe/connect", func(c *gin.Context) {
		template.Render(c, "stripe/connect.html")
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
		user.StripeToken = token

		// update in datastore
		db.PutKey("user", userid, user)

		template.Render(c, "stripe/success.html")
	})

	admin.POST("/login", func(c *gin.Context) {
		f := new(models.LoginForm)
		err := form.Parse(c, f)

		if err != nil {
			c.Fail(401, err)
			return
		}

		hash, err := f.PasswordHash()
		if err != nil {
			c.Fail(401, err)
			return
		}

		var owners [1]models.Owner
		db := datastore.New(c)
		q := db.Query("owner").
			Filter("Email =", f.Email).
			Filter("PasswordHash =", hash)

		keys, err := q.GetAll(db.Context, &owners)
		if err != nil {
			c.Fail(401, err)
			return
		}

		if err == nil && len(owners) > 0 {
			auth.Login(c, keys[0].StringID())
		}
	})
}
