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

// Retrieves the user id from the session and queries the db for a User object
func GetUser(c *gin.Context) (user models.User, err error) {
	email, err := GetEmail(c)
	log.Debug("Email ", email)
	if err != nil {
		log.Error("Error retrieving email", err)
		return user, err
	}

	db := datastore.New(c)
	log.Debug(kind)
	err = db.GetKey(kind, email, &user)
	log.Debug("%#v", user)
	return user, err
}

// Validates a form and inserts a new user into the datastore
// Checks if the Email and Id are unique, and calculates a hash for the password
func NewUser(c *gin.Context, f models.RegistrationForm) error {
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
