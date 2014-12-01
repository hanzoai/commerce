package checkout

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/form"
	"crowdstart.io/util/log"
)

// Load order from checkout form
type CheckoutForm struct {
	Order models.Order
}

// Parse form
func (f *CheckoutForm) Parse(c *gin.Context) error {
	if err := form.Parse(c, f); err != nil {
		return err
	}

	form.SchemaFix(&f.Order) // Fuck you schema

	return nil
}

// Populate form with data from database
func (f *CheckoutForm) Populate(db *datastore.Datastore) error {
	return f.Order.Populate(db)
}

func (f CheckoutForm) Validate(c *gin.Context) {}

// Charge after successful authorization
type ChargeForm struct {
	User          models.User
	Order         models.Order
	ShipToBilling bool
	StripeToken   string
}

func (f *ChargeForm) Parse(c *gin.Context) error {
	if err := form.Parse(c, f); err != nil {
		log.Panic("Unable to parse form: %v", err)
		return err
	}

	form.SchemaFix(&f.Order) // Fuck you schema

	if f.ShipToBilling {
		f.Order.ShippingAddress = f.Order.BillingAddress
	}

	return nil
}

func (f ChargeForm) Validate() (errs []string) {
	if f.Order.BillingAddress.Line1 == "" {
		errs = append(errs, "Address line 1 is required")
	}
	if f.Order.BillingAddress.City == "" {
		errs = append(errs, "City is required")
	}
	if f.Order.BillingAddress.State == "" {
		errs = append(errs, "State is required")
	}
	if f.Order.BillingAddress.PostalCode == "" {
		errs = append(errs, "Postal code is required")
	}
	if f.Order.BillingAddress.Country == "" {
		errs = append(errs, "Country is required")
	}
	if f.Order.ShippingAddress.Line1 == "" {
		errs = append(errs, "Address line 1 is required")
	}
	if f.Order.ShippingAddress.City == "" {
		errs = append(errs, "City is required")
	}
	if f.Order.ShippingAddress.State == "" {
		errs = append(errs, "State is required")
	}
	if f.Order.ShippingAddress.PostalCode == "" {
		errs = append(errs, "Postal code is required")
	}
	if f.Order.ShippingAddress.Country == "" {
		errs = append(errs, "Country is required")
	}

	if f.StripeToken == "" {
		errs = append(errs, "Invalid stripe token")
	}

	// if len(string(f.Order.Account.CVV2)) < 3 {
	// 	errs = append(errs, "Confirmation code is required.")
	// }
	// if len(f.Order.Account.Expiry) != 4 {
	// 	log.Debug(f.Order.Account.Expiry)
	// 	errs = append(errs, "Invalid expiry")
	// }

	// log.Info("Processing order. %#v", form.Order)
	// err := form.Order.Process(c)
	// if err != nil {
	// 	log.Error(err.Error())
	// 	return
	// }

	return errs
}
