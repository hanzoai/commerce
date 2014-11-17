package auth

import (
	"code.google.com/p/go.crypto/bcrypt"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"github.com/gin-gonic/gin"
	"log"
)

// const sessionName = "crowdstartLogin"
const kind = "user"
const loginKey = "login-key"

func IsLoggedIn(c *gin.Context) bool {
	value, err := Get(c, loginKey)
	return err == nil && value != ""
}

func VerifyUser(c *gin.Context) error {
	f := new(models.LoginForm) // f.Email is User.Id
	err := f.Parse(c)

	if err != nil {
		return err
	}

	hash, err := f.PasswordHash()
	log.Println(string(hash))

	if err != nil {
		return err
	}

	db := datastore.New(c)
	user := new(models.User)

	if err := db.GetKey("user", f.Email, user); err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(f.Password)); err != nil {
		return err
	}

	return Set(c, loginKey, f.Email) // sets the loginKey value to the user id
}

func GetUsername(c *gin.Context) (string, error) {
	return Get(c, loginKey)
}

func Logout(c *gin.Context) {
	Delete(c, loginKey)
	c.Redirect(301, "/user/login")
}
