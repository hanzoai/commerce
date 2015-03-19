package auth

import (
	"errors"

	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/log"
	"crowdstart.io/util/queries"
	"crowdstart.io/util/session"
)

func GetEmail(c *gin.Context) (string, error) {
	log.Debug("Retrieving email from session")
	return session.Get(c, loginKey)
}

// Retrieves user instance from database using email stored in session.
func GetUser(c *gin.Context) (*models.User, error) {
	q := queries.New(c)
	user := new(models.User)

	email, err := GetEmail(c)
	if err != nil {
		log.Warn("Error retrieving email: %v", err)
		return user, err
	}

	err = q.GetUserByEmail(email, user)

	return user, err
}

// Validates a form and inserts a new user into the datastore
// Checks if the Email and Id are unique, and calculates a hash for the password
func NewUser(c *gin.Context, f *RegistrationForm) (*models.User, error) {
	m := f.User
	db := datastore.New(c)
	q := queries.New(c)

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
		return nil, errors.New("Email is already registered")
	}

	if m.PasswordHash, err = f.PasswordHash(); err != nil {
		return nil, err
	}

	if err = q.UpsertUser(&m); err != nil {
		return nil, err
	}

	return &m, nil
}
