package admin

import (
	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/form"
	"crowdstart.io/util/router"
	"crowdstart.io/util/template"
	"errors"
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

		var admins [1]models.Admin
		db := datastore.New(c)
		q := db.Query("admin").
			Filter("Email =", f.Email).
			Filter("PasswordHash =", hash).
			Limit(1)

		_, err = q.GetAll(db.Context, &admins)
		if err != nil {
			c.Fail(401, err)
			return
		}

		if err == nil && len(admins) == 1 {
			auth.Login(c, admins[0].Email)
		}
	})
}

func NewAdmin(c *gin.Context, m models.Admin) error {
	db := datastore.New(c)
	q := db.Query("admin").
		Filter("Email =", m.Email).
		Limit(1)

	var admins [1]models.Admin
	_, err := q.GetAll(db.Context, &admins)

	if err != nil {
		return err
	}

	if len(admins) == 1 {
		return errors.New("Email is already registered")
	} else {
		_, err := db.Put("admin", m)
		return err
	}
}
