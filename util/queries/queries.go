// Put all commonly used complex (ie, not key lookups, everything that uses Query)
// datastore queries in here so we don't duplicate them everywhere
package queries

import (
	"errors"

	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/log"
)

type Client struct {
	Datastore *datastore.Datastore
}

func New(ctx interface{}) *Client {
	return &Client{datastore.New(ctx)}
}

// Get By Email

// Find a User By Email is one of the most common operations, it used to be that
// Users were keyed by email but this made changing email a huge hassle
func (c *Client) GetUserByEmail(email string, user *models.User) error {
	users := make([]*models.User, 0)

	_, err := c.Datastore.
		Query("user").
		Filter("Email=", email).
		Limit(1).
		GetAll(c.Datastore.Context, &users)

	if err != nil {
		log.Warn("Unable to fetch user from database: %v", err)
		return err
	}

	if len(users) == 0 {
		return errors.New("No users using " + email)
	}

	*user = *users[0]
	return nil
}

// Upserts

// Upserting a User is non trivial since we have to assign its Id to the encoded
// key strings.  It also sets the Id if it is not set for insert.
func (c *Client) UpsertUser(user *models.User) error {
	if user.Id == "" {
		_user := new(models.User)
		// We check is the User already exists if it comes in with no id
		c.GetUserByEmail(user.Email, _user)
		if _user.Id == "" {
			id := c.Datastore.AllocateId("user")
			user.Id = c.Datastore.EncodeId("user", id)
			log.Debug("UpsertUser doing Insert")
		} else {
			user.Id = _user.Id
			log.Debug("UpsertUser doing Update")
		}
	}

	k, err := c.Datastore.DecodeKey(user.Id)
	if err != nil {
		return err
	}

	_, err = c.Datastore.PutKey("user", k, user)
	return err
}
