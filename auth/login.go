package auth

import (
	"code.google.com/p/go.crypto/bcrypt"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"errors"
	"github.com/gin-gonic/gin"
	"log"
)

// const sessionName = "crowdstartLogin"
const kind = "user"
const loginKey = "login-key"

func IsLoggedIn(c *gin.Context) bool {
	value, err := GetSession(c, loginKey)
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

	return SetSession(c, loginKey, f.Email) // sets cookie value to the user id
}

// Validates a form and inserts a new user into the datastore
func NewUser(c *gin.Context, f models.RegistrationForm) error {
	// Checks if the Email and Id are unique, and calculates a hash for the password
	m := f.User
	db := datastore.New(c)

	// Both queries are run synchronously
	qEmail := db.Query("user").
		Filter("Email =", m.Email).
		KeysOnly().
		Limit(1)
	qId := db.Query("user").
		Filter("Id =", m.Id).
		KeysOnly().
		Limit(1)

	keys, err := qEmail.GetAll(db.Context, nil)
	if err != nil {
		return err
	}
	if len(keys) > 0 {
		return errors.New("Email is already registered")
	}

	keys, err = qId.GetAll(db.Context, nil)
	if err != nil {
		return err
	}
	if len(keys) > 0 {
		return errors.New("Id is already taken")
	}

	m.PasswordHash, err = f.PasswordHash()

	_, err = db.Put("user", m)
	return err
}
