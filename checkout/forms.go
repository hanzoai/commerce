package checkout

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
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

	// Fix order
	form.SchemaFix(&f.Order)

	return nil
}

// Populate form with data from database
func (f *CheckoutForm) Populate(c *gin.Context) {
	db := datastore.New(c)

	// TODO: Optimize this, multiget, use caching.
	for i, item := range f.Order.Items {
		log.Debug("Fetching variant for %v", item.SKU())

		// Fetch Variant for LineItem from datastore
		if err := db.GetKey("variant", item.SKU(), &item.Variant); err != nil {
			log.Error("Failed to find variant for: %v", item.SKU(), c)
			c.Fail(500, err)
		}

		// Fetch Product for LineItem from datastore
		if err := db.GetKey("product", item.Slug(), &item.Product); err != nil {
			log.Error("Failed to find product for: %v", item.Slug(), c)
			c.Fail(500, err)
		}

		// Set SKU so we can deserialize later
		item.SKU_ = item.SKU()
		item.Slug_ = item.Slug()

		// Update item in order
		f.Order.Items[i] = item

		// Update subtotal
		f.Order.Subtotal += item.Price()
	}

	// Update grand total
	f.Order.Total = f.Order.Subtotal + f.Order.Tax
}

func (f CheckoutForm) Validate(c *gin.Context) {}

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
