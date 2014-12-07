package user

import (
	"errors"

	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/template"
)

const kind = "user"

func Login(c *gin.Context) {
	if err := auth.VerifyUser(c); err == nil {
		c.Redirect(300, "/user/")
	} else {
		template.Render(c, "platform/user/login.html",
			"error", "Invalid email or password",
		)
	}
}

func NewUser(c *gin.Context, f auth.RegistrationForm) error {
	m := f.User
	db := datastore.New(c)

	user := new(models.User)
	db.GetKey("user", m.Email, user)
	if user == nil {
		m.PasswordHash, _ = f.PasswordHash()
		_, err := db.Put("user", m)
		return err
	}

	return errors.New("Email is already registered")
}
