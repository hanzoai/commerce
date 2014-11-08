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

func IsLoggedIn(c *gin.Context) bool {
	value, err := GetSession(c, loginKey)
	return err == nil && value != ""
}

func VerifyUser(c *gin.Context) error {
	f := new(models.LoginForm)
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

	SetSession(c, loginKey, "PLEASE WORK") // sets cookie
	return nil
}
