package checkout

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/middleware"
	"crowdstart.io/models"
	"crowdstart.io/util/form"
	"crowdstart.io/util/log"
)

// Load order from checkout form
type CheckoutForm struct {
	Order models.Order
}

func (f *CheckoutForm) Parse(c *gin.Context) error {
	if err := form.Parse(c, f); err != nil {
		return err
	}

	// Schema creates the Order.Items slice sized to whatever is the largest
	// index form item. This creates a slice with a huge number of nil structs,
	// so we create a new slice of items and use that instead.
	items := make([]models.LineItem, 0)
	for _, lineItem := range f.Order.Items {
		if lineItem.SKU() != "" {
			items = append(items, lineItem)
		}
	}
	f.Order.Items = items

	return nil
}

func (f CheckoutForm) Validate() (errs []string) {
	return errs
}

// Charge after successful authorization
type AuthorizeForm struct {
	User          models.User
	Order         models.Order
	RawExpiry     string
	ShipToBilling bool

	StripeToken string
}

func (f *AuthorizeForm) Parse(c *gin.Context) error {
	if err := form.Parse(c, f); err != nil {
		log.Panic("Parsing AuthorizeForm %s", err)
		return err
	}

	ctx := middleware.GetAppEngine(c)
	ctx.Debugf("%v", f.RawExpiry)

	if f.ShipToBilling {
		f.Order.ShippingAddress = f.Order.BillingAddress
	}

	return nil
}

func (f AuthorizeForm) Validate() (errs []string) {
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
