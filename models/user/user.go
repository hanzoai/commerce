package user

import (
	"strings"

	"hanzo.io/auth/password"
	"hanzo.io/models/mixin"
	"hanzo.io/models/order"
	"hanzo.io/models/referral"
	"hanzo.io/models/referrer"
	"hanzo.io/models/subscriber"
	"hanzo.io/models/transaction"
	"hanzo.io/models/types/currency"
	"hanzo.io/util/log"
	"hanzo.io/util/val"

	token "hanzo.io/models/token2"

	. "hanzo.io/models"
)

type UAuth struct {
	Username     string `json:"username"`
	Email        string `json:"email"`
	PasswordHash []byte `schema:"-" datastore:",noindex" json:"-"`

	ReferenceTokens []*token.Token `json:"referenceTokens,omitempty" datastore:"-"`
}

type UAccount struct {
	// Hanzo Id, found in default namespace
	Cid string `json:"-"`

	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Company   string `json:"company"`
	Phone     string `json:"phone"`

	// Deprecate this
	Organizations []string `json:"-"`

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

	Enabled bool `json:"enabled"` //whether or not the user can login yet

	Metadata  Map    `json:"metadata" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`

	// Series of events that have occured relevant to this order
	History []Event `json:"-"`
}

type UCustomer struct {
	PaypalEmail     string  `json:"paypalEmail"`
	BillingAddress  Address `json:"billingAddress,omitempty"`
	ShippingAddress Address `json:"shippingAddress,omitempty"`

	// Account to use for new orders when customer creates new orders
	Accounts struct {
		Stripe Account `json:"stripe,omitempty"`
		PayPal Account `json:"paypal,omitempty"`
		Affirm Account `json:"affirm,omitempty"`
	} `json:"-"`

	Referrals []referral.Referral `json:"referrals,omitempty" datastore:"-"`
	Referrers []referrer.Referrer `json:"referrers,omitempty" datastore:"-"`
	Orders    []order.Order       `json:"orders,omitempty" datastore:"-"`

	Balances map[currency.Type]currency.Cents `json:"balances" datastore:"-"`
}

type User struct {
	mixin.Model
	mixin.Salesforce

	UAccount
	UAuth
	UCustomer
}

func (u *User) Defaults() {
	u.Metadata = make(Map)
	u.History = make([]Event, 0)
}

func (u User) Name() string {
	return u.FirstName + " " + u.LastName
}

func (u User) HasPassword() bool {
	return len(u.PasswordHash) != 0
}

func (u User) ComparePassword(pass string) bool {
	return password.HashAndCompare(u.PasswordHash, pass)
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
	return val.New().Check("FirstName").Exists().
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
	email = strings.ToLower(strings.TrimSpace(email))
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

func (u *User) LoadReferrals() error {
	if _, err := referrer.Query(u.Db).Filter("UserId=", u.Id()).LoadAll(&u.Referrers); err != nil {
		return err
	}

	if _, err := referral.Query(u.Db).Filter("ReferrerUserId=", u.Id()).LoadAll(&u.Referrals); err != nil {
		return err
	}

	return nil
}

func (u *User) LoadOrders() error {
	if _, err := order.Query(u.Db).Filter("UserId=", u.Id()).LoadAll(&u.Orders); err != nil {
		return err
	}

	return nil
}

func (u *User) CalculateBalances() error {
	trans, err := transaction.Query(u.Db).Filter("UserId=", u.Id()).Filter("Test=", false).GetEntities()
	if err != nil {
		return err
	}

	u.Balances = make(map[currency.Type]currency.Cents)
	for i := range trans {
		t := trans[i].(*transaction.Transaction)
		cents := u.Balances[t.Currency]

		if t.Type == transaction.Withdraw {
			u.Balances[t.Currency] = cents - t.Amount
		} else {
			u.Balances[t.Currency] = cents + t.Amount
		}
	}

	return nil
}

func (u *User) SetPassword(newPassword string) error {
	hash, err := password.Hash(newPassword)
	if err != nil {
		return err
	}

	u.PasswordHash = hash
	return nil
}

func (u *User) GetSub(segmentId string) (*subscriber.Subscriber, error) {
	sub := subscriber.New(u.Db)
	if ok, _ := sub.Query().Filter("UserId=", u.Id()).Filter("SegmentId=", segmentId).First(); !ok {
		if ok, err := sub.Query().Filter("Email=", u.Email).Filter("SegmentId=", segmentId).First(); !ok {
			return nil, err
		}
	}

	sub.UserId = u.Id()

	return sub, sub.Update()
}

func (u *User) GetOrCreateSub(segmentId string) (*subscriber.Subscriber, error) {
	// create a corresponding sub
	sub, err := u.GetSub(segmentId)

	if err != nil {
		sub = subscriber.New(u.Db)
		sub.Email = u.Email
		sub.UserId = u.Id()
		sub.SegmentId = segmentId
		return sub, sub.Create()
	}

	return sub, nil
}

func (u *User) DeleteSub(segmentId string) error {
	sub, err := u.GetSub(segmentId)
	if err == nil {
		return sub.Delete()
	}

	return nil
}

// Check if user is part of an organization
func (u *User) InOrganization(orgId string) bool {
	for i := range u.Organizations {
		if u.Organizations[i] == orgId {
			return true
		}
	}
	return false
}

// Save organization to organization slice.
func (u *User) AddOrganization(orgId string) {
	if !u.InOrganization(orgId) {
		u.Organizations = append(u.Organizations, orgId)
	}
}

func (u *User) LoadReferenceTokens() error {
	slice, err := token.Query(u.Db).
		Filter("Claims.UserId=", u.Id()).
		Filter("Claims.Type=", token.Reference).
		Filter("Revoked=", false).
		GetAll()

	u.ReferenceTokens = slice.([]*token.Token)

	return err
}
