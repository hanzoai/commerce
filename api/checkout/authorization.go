package checkout

import (
	"strings"

	"crowdstart.com/datastore"
	"crowdstart.com/models/order"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/user"
	stringutil "crowdstart.com/util/strings"
)

type Authorization struct {
	User    *user.User       `json:"user"`
	Payment *payment.Payment `json:"payment"`
	Order   *order.Order     `json:"order"`
}

// Correctly initialize user provided in authorization
func initUser(db *datastore.Datastore, usr *user.User, ord *order.Order) error {
	usr.Init(db)

	// If Id_ is specified this is an existing user, ensure they exist
	id := usr.Id_
	if id != "" {
		// TODO: Decide what if any values to allow to be updated via user in request
		usr = user.New(db)
		usr.SetKey(id)
		if err := usr.Get(); err != nil {
			return UserDoesNotExist
		}
	}

	// Use order billing and shipping address if missing on user
	if usr.ShippingAddress.Empty() {
		usr.ShippingAddress = ord.ShippingAddress
	}

	if usr.BillingAddress.Empty() {
		usr.BillingAddress = ord.BillingAddress
	}

	// Normalize email and username
	usr.Email = strings.ToLower(strings.TrimSpace(usr.Email))
	usr.Username = strings.ToLower(strings.TrimSpace(usr.Username))

	return nil
}

// Correctly initialize order provided in authorization
func initOrder(db *datastore.Datastore, ord *order.Order, usr *user.User) {
	ord.Init(db)

	// Use user shipping and billing information if absent from request
	if ord.ShippingAddress.Empty() {
		ord.ShippingAddress = usr.ShippingAddress
	}
	if ord.BillingAddress.Empty() {
		ord.BillingAddress = usr.BillingAddress
	}

	// Normalize country
	ord.BillingAddress.Country = strings.ToUpper(ord.BillingAddress.Country)
	ord.ShippingAddress.Country = strings.ToUpper(ord.ShippingAddress.Country)

	// Order is parented to user
	ord.Parent = usr.Key()
	ord.UserId = usr.Id()

	// Update order number
	ord.Number = ord.NumberFromId()
}

// Correctly initialize payment provided in authorization
func initPayment(db *datastore.Datastore, pay *payment.Payment, usr *user.User, ord *order.Order) {
	pay.Init(db)

	// Update payment status
	pay.Status = "unpaid"

	// Normalize card number, save last four
	number := pay.Account.Number
	number = stringutil.StripWhitespace(number)
	if len(number) >= 4 {
		pay.Account.LastFour = number[len(number)-4:]
	}
	pay.Account.Number = number

	// Default all payment types to Stripe for now, although eventually we
	// should use organization settings
	if pay.Type == "" {
		pay.Type = payment.Stripe
	}

	// Ensure order has same type as payment
	// TODO: Remove this from order
	ord.Type = pay.Type

	// User buyer information on user
	pay.Buyer = usr.Buyer()

	// Update payment with order information
	pay.Currency = ord.Currency
	pay.Description = ord.Description()

	// Payment is parented to order
	pay.Parent = ord.Key()
	pay.OrderId = ord.Id()
}

func (a *Authorization) Init(db *datastore.Datastore) error {
	if err := initUser(db, a.User, a.Order); err != nil {
		return err
	}

	// Normalize payment and order
	initOrder(db, a.Order, a.User)
	initPayment(db, a.Payment, a.User, a.Order)

	return nil
}
