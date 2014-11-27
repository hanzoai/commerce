package checkout

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"crowdstart.io/middleware"
	"crowdstart.io/models"
	"crowdstart.io/util/form"
	"crowdstart.io/util/log"
)

type CheckoutForm struct {
	Order models.Order
}

func (f *CheckoutForm) Parse(c *gin.Context) error {
	if err := form.Parse(c, f); err != nil {
		return err
	}

	// Nasty shit. Please fix.
	if len(f.Order.Items) < 2 {
		return nil
	}

	// For some reason gorilla/schema deserializes an extra nil lineItem,
	// we need to remove this.
	if f.Order.Items[0].SKU() == "" {
		slice := make([]models.LineItem, 0)
		f.Order.Items = append(slice, f.Order.Items[1:]...)
	}

	return nil
}

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

	// Parse raw expiry
	parts := strings.Split(f.RawExpiry, " / ")
	if len(parts) != 2 {
		return errors.New("Invalid expiry")
	}

	month, err := strconv.Atoi(parts[0])
	if err != nil {
		return errors.New("Invalid month")
	}

	year, err := strconv.Atoi(parts[1])
	if err != nil {
		return errors.New("Invalid year")
	}

	f.Order.Account.Month = month
	f.Order.Account.Year = year
	f.Order.Account.Expiry = strings.Join(parts, "")

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
	/*
		if len(string(f.Order.Account.CVV2)) < 3 {
			errs = append(errs, "Confirmation code is required.")
		}
		if len(f.Order.Account.Expiry) != 4 {
			log.Debug(f.Order.Account.Expiry)
			errs = append(errs, "Invalid expiry")
		}
	*/

	return errs
}
