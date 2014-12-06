package user

import (
	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"errors"
	"github.com/gin-gonic/gin"
)

const kind = "user"

func Login(c *gin.Context) {
	if err := auth.VerifyUser(c); err == nil {
		// Success
	} else {
		c.Fail(401, err)
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
