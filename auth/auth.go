package auth

import (
	"errors"

	"code.google.com/p/go.crypto/bcrypt"
	"github.com/gin-gonic/gin"

	"crowdstart.io/models"
	"crowdstart.io/util/log"
	"crowdstart.io/util/queries"
	"crowdstart.io/util/session"
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
	value, err := GetEmail(c)
	return err == nil && value != ""
}

func IsFacebookUser(c *gin.Context) bool {
	user, err := GetUser(c)
	if err != nil {
		log.Panic("Error while retrieving user \n%v", err)
	}
	return user.Facebook.AccessToken != "" // Checks if AccessToken is set
}

func VerifyUser(c *gin.Context) error {
	// Parse login form
	f := new(LoginForm)
	if err := f.Parse(c); err != nil {
		return err
	}

	q := queries.New(c)

	// Get user from database
	user := new(models.User)
	if err := q.GetUserByEmail(f.Email, user); err != nil {
		return err
	}

	log.Debug("%v = %v", user, f.Password)
	if !user.HasPassword() {
		return errors.New("User likely registered via Facebook")
	}

	// Compare form password with saved hash
	if err := CompareHashAndPassword(user.PasswordHash, f.Password); err != nil {
		return err
	}

	// Set the loginKey value to the user id
	return Login(c, user.Email)
}

// Login should only be used in exceptional circumstances.
// Use VerifyUser when possible.
func Login(c *gin.Context, email string) error {
	return session.Set(c, loginKey, email)
}

func Logout(c *gin.Context) error {
	return session.Delete(c, loginKey)
}
