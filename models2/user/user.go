package user

import (
	"errors"
	"net/http"

	"github.com/mholt/binding"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/util/log"

	. "crowdstart.io/models2"
)

var (
	UserNotFound = errors.New("User not found.")
)

type User struct {
	mixin.Model
	mixin.Salesforce

	FirstName       string  `json:"firstName"`
	LastName        string  `json:"lastName"`
	Phone           string  `json:"phone"`
	BillingAddress  Address `json:"billingAddress,omitempty"`
	ShippingAddress Address `json:"shippingAddress,omitempty"`
	Email           string  `json:"email"`
	PasswordHash    []byte  `schema:"-" datastore:",noindex" json:"-"`

	StripeCustomerId string `json:"stripeCustomerId,omitempty"`

	Metadata []Datum `json:"metadata,omitempty"`
}

func New(db *datastore.Datastore) *User {
	u := new(User)
	u.Model = mixin.Model{Db: db, Entity: u}
	return u
}

func (u User) Kind() string {
	return "user2"
}

func (u User) Name() string {
	return u.FirstName + " " + u.LastName
}

func (u User) HasPassword() bool {
	return len(u.PasswordHash) != 0
}

func (u User) GetMetadata(key string) Datum {
	for index, datum := range u.Metadata {
		if datum.Key == key {
			return u.Metadata[index]
		}
	}
	return Datum{}
}

func (u User) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	// Name cannot be empty string.
	if u.FirstName == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"FirstName"},
			Classification: "InputError",
			Message:        "User first name cannot be empty.",
		})
	}

	if u.LastName == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"LastName"},
			Classification: "InputError",
			Message:        "User last name cannot be empty.",
		})
	}

	if u.Email == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Email"},
			Classification: "InputError",
			Message:        "User email cannot be empty.",
		})
	}

	// Validate cart implicitly.
	// errs = u.Cart.Validate(req, errs)
	errs = u.BillingAddress.Validate(req, errs)
	errs = u.ShippingAddress.Validate(req, errs)

	return errs
}

// Populates current entity from datastore by Email.
func (u *User) GetByEmail(email string) error {
	log.Debug("Searching for user '%v'", email)

	// Build query to return user
	ok, err := u.Query().Filter("Email=", email).First()

	if err != nil {
		log.Warn("Unable to fetch user from datastore: '%v'", err)
		return err
	}

	// Return error if no user found.
	if !ok {
		return UserNotFound
	}

	return nil
}

// // Insert new user
// func (u *User) Insert(db *datastore.Datastore) error {
// 	id := db.AllocateId("user")
// 	k := db.KeyFromId("user", id)

// 	log.Debug("Inserting New User with key %v", k)

// 	u.Id = k.Encode()
// 	u.CreatedAt = time.Now()
// 	u.UpdatedAt = u.CreatedAt

// 	_, err := db.PutKind("user", k, u)
// 	return err
// }

// // Actual upsert method
// func (u *User) upsert(db *datastore.Datastore) error {
// 	k, err := db.DecodeKey(u.Id)
// 	if err != nil {
// 		return err
// 	}

// 	_, err = db.PutKind("user", k, u)
// 	return err
// }

// // Idempotent user upsert method.
// func (u *User) Upsert(db *datastore.Datastore) error {
// 	// We have an ID, we can just upsert
// 	if u.Id != "" {
// 		return u.upsert(db)
// 	}

// 	// We don't have an ID, we need to figure out if this is a new user or not.
// 	user := new(User)
// 	err := user.GetByEmail(db, u.Email)

// 	// if we can't find the user, insert new user
// 	if err == UserNotFound {
// 		return u.Insert(db)
// 	}

// 	// Something bad happened, let's bail out
// 	if err != nil {
// 		return err
// 	}

// 	// Found user, set Id
// 	u.Id = user.Id
// 	u.UpdatedAt = time.Now()

// 	return u.upsert(db)
// }
