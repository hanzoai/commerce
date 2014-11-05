package admin

import (
	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
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
		auth.VerifyUser(c, "admin") // logs in the user if credentials are valid
	})
}

func NewAdmin(c *gin.Context, f models.RegistrationForm) error {
	m := f.Admin
	db := datastore.New(c)
	q := db.Query("admin").
		Filter("Email =", m.Email).
		Limit(1)

	var admins [1]models.Admin
	_, err := q.GetAll(db.Context, &admins)

	if err != nil {
		return err
	}

	m.PasswordHash, err = f.PasswordHash()

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
