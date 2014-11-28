package auth

import (
	"errors"

	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/log"
)

func GetEmail(c *gin.Context) (string, error) {
	log.Debug("Retrieving email from session")
	return Get(c, loginKey)
}

// Retrieves user instance from database using email stored in session.
func GetUser(c *gin.Context) (user *models.User) {
	email, err := GetEmail(c)
	log.Debug("Email %v", email)
	if err != nil {
		log.Panic("Error retrieving email: %v", err)
	}

	db := datastore.New(c)
	if err = db.GetKey(kind, email, &user); err != nil {
		log.Panic("Unable to fetch user from database: %v", err)
	}

	log.Debug("%#v", user)
	return user
}

// Validates a form and inserts a new user into the datastore
// Checks if the Email and Id are unique, and calculates a hash for the password
func NewUser(c *gin.Context, f RegistrationForm) error {
	m := f.User
	db := datastore.New(c)

	// Both queries are run synchronously. There seems to be no support for a logical OR when querying the database.
	// If each query returns no keys, then both fields are unique.
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

	_, err = db.Put(kind, m)
	return err
}
