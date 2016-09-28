package user

import (
	"strings"

	aeds "appengine/datastore"
	"appengine/search"

	"crowdstart.com/auth/password"
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/order"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/referral"
	"crowdstart.com/models/referrer"
	"crowdstart.com/models/transaction"
	"crowdstart.com/models/types/country"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/json"
	"crowdstart.com/util/log"
	"crowdstart.com/util/searchpartial"
	"crowdstart.com/util/val"

	. "crowdstart.com/models"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type User struct {
	mixin.Model
	mixin.Counter
	mixin.Salesforce

	// Crowdstart Id, found in default namespace
	Cid string `json:"-"`

	Username        string   `json:"username"`
	FirstName       string   `json:"firstName"`
	LastName        string   `json:"lastName"`
	Company         string   `json:"company,omitempty"`
	Phone           string   `json:"phone,omitempty"`
	BillingAddress  Address  `json:"billingAddress,omitempty"`
	ShippingAddress Address  `json:"shippingAddress,omitempty"`
	Email           string   `json:"email"`
	PaypalEmail     string   `json:"paypalEmail,omitempty"`
	PasswordHash    []byte   `schema:"-" datastore:",noindex" json:"-"`
	Organizations   []string `json:"-"`

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
	} `json:"-"`

	Enabled bool `json:"enabled"` //whether or not the user can login yet

	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`

	Referrals []referral.Referral `json:"referrals,omitempty" datastore:"-"`
	Referrers []referrer.Referrer `json:"referrers,omitempty" datastore:"-"`
	Orders    []order.Order       `json:"orders,omitempty" datastore:"-"`

	Balances map[currency.Type]currency.Cents `json:"balances,omitempty" datastore:"-"`

	ReferrerId string `json:"referrerId,omitempty"`

	// Series of events that have occured relevant to this order
	History []Event `json:"-,omitempty"`

	IsOwner bool `json:"owner,omitempty" datastore:"-"`
}

func (u User) Document() mixin.Document {
	emailUser := strings.Split(u.Email, "@")[0]
	return &Document{
		u.Id(),
		search.Atom(u.Email),
		searchpartial.Partials(emailUser) + " " + emailUser,
		u.Username,
		searchpartial.Partials(u.Username),
		u.FirstName,
		searchpartial.Partials(u.FirstName),
		u.LastName,
		searchpartial.Partials(u.LastName),
		u.Phone,

		u.BillingAddress.Line1,
		u.BillingAddress.Line2,
		u.BillingAddress.City,
		u.BillingAddress.State,
		u.BillingAddress.Country,
		country.ByISOCodeISO3166_2[u.BillingAddress.Country].ISO3166OneEnglishShortNameReadingOrder,
		u.BillingAddress.PostalCode,

		u.ShippingAddress.Line1,
		u.ShippingAddress.Line2,
		u.ShippingAddress.City,
		u.ShippingAddress.State,
		u.ShippingAddress.Country,
		country.ByISOCodeISO3166_2[u.ShippingAddress.Country].ISO3166OneEnglishShortNameReadingOrder,
		u.ShippingAddress.PostalCode,

		u.CreatedAt,
		u.UpdatedAt,

		u.Accounts.Stripe.BalanceTransactionId,
		u.Accounts.Stripe.CardId,
		u.Accounts.Stripe.ChargeId,
		u.Accounts.Stripe.CustomerId,
		u.Accounts.Stripe.LastFour,
	}
}

func (u *User) Load(c <-chan aeds.Property) (err error) {
	// Ensure we're initialized
	u.Defaults()

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

	// sanitize email
	u.Email = strings.ToLower(strings.TrimSpace(u.Email))

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(u, c))
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
	if _, err := referrer.Query(u.Db).Filter("UserId=", u.Id()).GetAll(&u.Referrers); err != nil {
		return err
	}

	if _, err := referral.Query(u.Db).Filter("ReferrerUserId=", u.Id()).GetAll(&u.Referrals); err != nil {
		return err
	}

	log.Warn("Referrals %v", u.Referrals)

	return nil
}

func (u *User) LoadOrders() error {
	if _, err := order.Query(u.Db).Filter("UserId=", u.Id()).GetAll(&u.Orders); err != nil {
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
