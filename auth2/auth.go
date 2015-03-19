package auth

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/auth2/password"
	"crowdstart.io/datastore"
	"crowdstart.io/models2/user"
	"crowdstart.io/util/log"
	"crowdstart.io/util/session"
)

const loginKey = "login-key"

func GetCurrentUserId(c *gin.Context) (string, error) {
	log.Debug("Retrieving email from session")
	return session.Get(c, loginKey)
}

func GetCurrentUser(c *gin.Context) (*user.User, error) {
	id, err := GetCurrentUserId(c)
	if err != nil {
		return nil, err
	}

	db := datastore.New(c)
	u := user.New(db)

	if err := u.Get(id); err != nil {
		return nil, err
	}

	return u, nil
}

// Validates a form and inserts a new user into the datastore
// Checks if the Email and Id are unique, and calculates a hash for the password
func RegisterNewUser(c *gin.Context) (*user.User, error) {
	// Parse register form
	f := new(RegistrationForm)
	if err := f.Parse(c); err != nil {
		return nil, err
	}

	m := f.User
	db := datastore.New(c)

	// If each query returns no keys, then both fields are unique.
	qEmail := db.Query("user").
		Filter("Email =", m.Email).
		KeysOnly().
		Limit(1)

	keys, err := qEmail.GetAll(db.Context, nil)
	if err != nil {
		return nil, err
	}

	log.Debug("Checking if user exists")
	if len(keys) > 0 {
		return nil, ErrorUserExists
	}

	if m.PasswordHash, err = f.PasswordHash(); err != nil {
		return nil, err
	}

	if err = m.Put(); err != nil {
		return nil, err
	}

	return &m, nil
}

func VerifyUser(c *gin.Context) (*user.User, error) {
	// Parse login form
	f := new(LoginForm)
	if err := f.Parse(c); err != nil {
		return nil, err
	}

	db := datastore.New(c)

	// Get user from database
	u := user.New(db)
	if err := u.GetByEmail(f.Email); err != nil {
		return nil, err
	}

	// Compare form password with saved hash
	if !password.HashAndCompare(u.PasswordHash, f.Password) {
		return nil, ErrorPasswordMismatch
	}

	// Set the loginKey value to the user id
	return u, Login(c, u)
}

func Login(c *gin.Context, u *user.User) error {
	return session.Set(c, loginKey, u.Id())
}

func Logout(c *gin.Context) error {
	return session.Delete(c, loginKey)
}
