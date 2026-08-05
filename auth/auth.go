package auth

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/user"
	"github.com/hanzoai/commerce/util/session"
)

const loginKey = "loggedIn"

func GetCurrentUserId(c *zip.Ctx) (string, error) {
	log.Debug("Retrieving current id from session")
	value, err := session.Get(c, loginKey)
	if err != nil {
		return "", err
	}

	if value == nil {
		return "", err
	}

	return value.(string), nil
}

func GetCurrentUser(c *zip.Ctx) (*user.User, error) {
	log.Debug("Retrieving current user from session")
	id, err := GetCurrentUserId(c)
	if err != nil {
		log.Warn("Failed to retrieve current user from session")
		return nil, err
	}

	db := datastore.New(c.Context())
	u := user.New(db)

	if err := u.GetById(id); err != nil {
		log.Warn("Failed to retrieve current user from session")
		return nil, err
	}

	log.Debug("Retrieved current user from session")
	return u, nil
}

// // Validates a form and inserts a new user into the datastore
// // Checks if the Email and Id are unique, and calculates a hash for the password
// func RegisterNewUser(c *zip.Ctx) (*user.User, error) {
// 	// Parse register form
// 	f := new(RegistrationForm)
// 	if err := f.Parse(c); err != nil {
// 		return nil, err
// 	}

// 	m := f.User
// 	db := datastore.New(c.Context())

// 	// If each query returns no keys, then both fields are unique.
// 	qEmail := db.Query("user").
// 		Filter("Email =", m.Email).
// 		KeysOnly().
// 		Limit(1)

// 	keys, err := qEmail.GetAll(db.Context, nil)
// 	if err != nil {
// 		return nil, err
// 	}

// 	log.Debug("Checking if user exists")
// 	if len(keys) > 0 {
// 		return nil, ErrorUserExists
// 	}

// 	if m.PasswordHash, err = f.PasswordHash(); err != nil {
// 		return nil, err
// 	}

// 	if err = m.Put(); err != nil {
// 		return nil, err
// 	}

// 	return &m, nil
// }

func IsLoggedIn(c *zip.Ctx) bool {
	value, err := session.Get(c, loginKey)
	if err != nil {
		return false
	}

	userId, _ := value.(string)
	if userId == "" {
		return false
	}

	return true
}

func Login(c *zip.Ctx, u *user.User) error {
	return session.Set(c, loginKey, u.Id())
}

func Logout(c *zip.Ctx) error {
	return session.Clear(c)
}
