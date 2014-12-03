package auth

import (
	"code.google.com/p/go.crypto/bcrypt"
	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/log"
)

// const sessionName = "crowdstartLogin"
const kind = "user"
const loginKey = "login-key"

func CompareHashAndPassword(hash []byte, password string) error {
	return bcrypt.CompareHashAndPassword(hash, []byte(password))
}

func HashPassword(password string) []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		log.Panic("Failed to hash password: %v", err)
	}
	return hash
}

func IsLoggedIn(c *gin.Context) bool {
	value, err := Get(c, loginKey)
	return err == nil && value != ""
}

func IsFacebookUser(c *gin.Context) bool {
	user := GetUser(c)
	if user  == nil {
		return false
	}
	return user.Facebook.AccessToken != "" // Checks if AccessToken is set
}

func VerifyUser(c *gin.Context) error {
	// Parse login form
	f := new(LoginForm)
	if err := f.Parse(c); err != nil {
		return err
	}

	db := datastore.New(c)

	// Get user from database
	user := new(models.User)
	if err := db.GetKey("user", f.Email, user); err != nil {
		return err
	}

	// Compare form password with saved hash
	if err := CompareHashAndPassword(user.PasswordHash, f.Password); err != nil {
		return err
	}

	// Set the loginKey value to the user id
	return Login(c, loginKey, f.Email)
}

func Login(c *gin.Context, email string) error {
	return Set(c, loginKey email)
}

func Logout(c *gin.Context) {
	Delete(c, loginKey)
	c.Redirect(301, "/user/login")
}
