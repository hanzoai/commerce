package checkout

import (
	"crowdstart.io/middleware"
	"crowdstart.io/models"
	"crowdstart.io/util/form"
	"errors"
	"github.com/gin-gonic/gin"
	"strconv"
	"strings"
)

type CheckoutForm struct {
	Order models.Order
}

func (f *CheckoutForm) Parse(c *gin.Context) error {
	if err := form.Parse(c, f); err != nil {
		return err
	}

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
	Order         models.Order
	User          models.User
	RawExpiry     string
	ShipToBilling bool
}

func (f *AuthorizeForm) Parse(c *gin.Context) error {
	if err := form.Parse(c, f); err != nil {
		return err
	}

	ctx := middleware.GetAppEngine(c)
	ctx.Debugf("%v", f.RawExpiry)

	// Parse raw expiry
	parts := strings.Split(f.RawExpiry, " / ")
	if len(parts) != 2 {
		return errors.New("Invalid expiry")
	}

	month, _ := strconv.Atoi(parts[0])
	year, _ := strconv.Atoi(parts[1])

	f.Order.Account.Month = month
	f.Order.Account.Year = year
	f.Order.Account.Expiry = strings.Join(parts, "")

	if f.ShipToBilling {
		f.Order.ShippingAddress = f.Order.BillingAddress
	}

	return nil
}

func (f AuthorizeForm) Validate() (errs []string) {
	if f.Order.User.FirstName == "" {
		errs = append(errs, "Billed user's first name is required")
	}
	if f.Order.User.LastName == "" {
		errs = append(errs, "Billed user's Last name is required")
	}
	if f.Order.User.Email == "" {
		errs = append(errs, "Billed user's Email address is required")
	}
	if f.Order.User.Phone == "" {
		errs = append(errs, "Billed user's Phone number is required")
	}

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
	if len(string(f.Order.Account.CVV2)) < 3 {
		errs = append(errs, "Confirmation code is required.")
	}
	if len(f.Order.Account.Expiry) != 4 {
		errs = append(errs, "Invalid expiry")
	}

	return errs
}
