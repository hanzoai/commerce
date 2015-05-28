package user

import (
	"time"

	aeds "appengine/datastore"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/json"
	"crowdstart.com/util/log"
	"crowdstart.com/util/val"

	. "crowdstart.com/models"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type User struct {
	mixin.Model
	mixin.Salesforce

	// Crowdstart Id, found in default namespace
	Cid string `json:"-"`

	Username        string   `json:"username"`
	FirstName       string   `json:"firstName"`
	LastName        string   `json:"lastName"`
	Company         string   `json:"company"`
	Phone           string   `json:"phone"`
	BillingAddress  Address  `json:"billingAddress,omitempty"`
	ShippingAddress Address  `json:"shippingAddress,omitempty"`
	Email           string   `json:"email"`
	PasswordHash    []byte   `schema:"-" datastore:",noindex" json:"-"`
	Organizations   []string `json:"organizations"`

	Facebook struct {
		AccessToken string `facebook:"-"`
		UserId      string `facebook:"id"`
		FirstName   string `facebook:"first_name"`
		LastName    string `facebook:"last_name"`
		MiddleName  string `facebook:"middle_name"`
		Name        string `facebook:"name" datastore:"-"`
		NameFormat  string `facebook:"name_format"` // For Chinese, Japanese, and Korean names. Possibly used in the future.
		Email       string `facebook:"email" datastore:"-"`
		Verified    bool   `facebook:"verified" datastore:"-"`
	} `json:"-"`

	// Account to use for new orders when customer creates new orders
	Accounts struct {
		Stripe payment.Account `json:"stripe,omitempty"`
		PayPal payment.Account `json:"paypal,omitempty"`
		Affirm payment.Account `json:"affirm,omitempty"`
	} `json:"accounts"`

	Credit struct {
		Currency currency.Type  `json:"currency"`
		Amount   currency.Cents `json:"amount"`

		LastUpdated time.Time `json:"lastUpdated"`
	} `json:"credit"`

	Metadata  Metadata `json:"metadata" datastore:"-"`
	Metadata_ string   `json:"-" datastore:",noindex"`
}

func (u *User) Init() {
	u.Metadata = make(Metadata)
}

func New(db *datastore.Datastore) *User {
	u := new(User)
	u.Init()
	u.Model = mixin.Model{Db: db, Entity: u}
	return u
}

func (u User) Kind() string {
	return "user"
}

func (u User) Document() mixin.Document {
	return &Document{
		u.Id(),
		u.Email,
		u.Username,
		u.FirstName,
		u.LastName,
		u.Phone,

		u.BillingAddress.Line1,
		u.BillingAddress.Line2,
		u.BillingAddress.City,
		u.BillingAddress.State,
		u.BillingAddress.Country,
		u.BillingAddress.PostalCode,

		u.ShippingAddress.Line1,
		u.ShippingAddress.Line2,
		u.ShippingAddress.City,
		u.ShippingAddress.State,
		u.ShippingAddress.Country,
		u.ShippingAddress.PostalCode,

		u.Accounts.Stripe.BalanceTransactionId,
		u.Accounts.Stripe.CardId,
		u.Accounts.Stripe.ChargeId,
		u.Accounts.Stripe.CustomerId,
		u.Accounts.Stripe.LastFour,
	}
}

func (u *User) Load(c <-chan aeds.Property) (err error) {
	// Ensure we're initialized
	u.Init()

	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(u, c)); err != nil {
		return err
	}

	// Update balance when queried out
	// now := time.Now()
	// var transactions []transaction.Transaction
	// if _, err = transaction.Query(u.Db).Filter("CreatedAt >=", u.Credit.LastUpdated).GetAll(&transactions); err != nil {
	// 	return
	// }

	// for _, trans := range transactions {
	// 	switch trans.Type {
	// 	case transaction.Deposit:
	// 		u.Credit.Amount += trans.Amount
	// 	case transaction.Withdraw:
	// 		u.Credit.Amount -= trans.Amount
	// 	}
	// }

	// u.Credit.LastUpdated = now

	// Deserialize from datastore
	if len(u.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(u.Metadata_), &u.Metadata)
	}

	return
}

func (u *User) Save(c chan<- aeds.Property) (err error) {
	// Serialize unsupported properties
	u.Metadata_ = string(json.EncodeBytes(&u.Metadata))

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(u, c))
}

func (u User) Name() string {
	return u.FirstName + " " + u.LastName
}

func (u User) HasPassword() bool {
	return len(u.PasswordHash) != 0
}

func (u User) Buyer() Buyer {
	return Buyer{
		Email:     u.Email,
		UserId:    u.Id(),
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Company:   u.Company,
		Phone:     u.Phone,
		Address:   u.BillingAddress,
	}
}

func (u *User) Validator() *val.Validator {
	return val.New(u).Check("FirstName").Exists().
		Check("LastName").Exists().
		Check("Email").Exists()
	// // Name cannot be empty string.
	// if u.FirstName == "" {
	// 	errs = append(errs, binding.Error{
	// 		FieldNames:     []string{"FirstName"},
	// 		Classification: "InputError",
	// 		Message:        "User first name cannot be empty.",
	// 	})
	// }

	// if u.LastName == "" {
	// 	errs = append(errs, binding.Error{
	// 		FieldNames:     []string{"LastName"},
	// 		Classification: "InputError",
	// 		Message:        "User last name cannot be empty.",
	// 	})
	// }

	// if u.Email == "" {
	// 	errs = append(errs, binding.Error{
	// 		FieldNames:     []string{"Email"},
	// 		Classification: "InputError",
	// 		Message:        "User email cannot be empty.",
	// 	})
	// }

	// // Validate cart implicitly.
	// // errs = u.Cart.Validate(req, errs)
	// errs = u.BillingAddress.Validate(req, errs)
	// errs = u.ShippingAddress.Validate(req, errs)

	// return errs
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

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
