package auth

import (
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"crowdstart.io/models"
	"crowdstart.io/util/log"
	"crowdstart.io/util/queries"
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

func VerifyUser(c *gin.Context) (*models.User, error) {
	user := new(models.User)

	// Parse login form
	f := new(LoginForm)
	if err := f.Parse(c); err != nil {
		return user, err
	}

	q := queries.New(c)

	// Get user from database
	if err := q.GetUserByEmail(f.Email, user); err != nil {
		return user, err
	}

	log.Debug("%v = %v", user, f.Password)
	// Compare form password with saved hash
	if err := CompareHashAndPassword(user.PasswordHash, f.Password); err != nil {
		return user, err
	}

	// Set the loginKey value to the user id
	err := Login(c, user.Email)

	return user, err
}

// Login should only be used in exceptional circumstances.
// Use VerifyUser when possible.
func Login(c *gin.Context, email string) error {
	return Set(c, loginKey, email)
}

func Logout(c *gin.Context) error {
	return Delete(c, loginKey)
}
