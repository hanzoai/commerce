package checkout

import (
	"crowdstart.io/models"
	"crowdstart.io/util/form"
	"github.com/gin-gonic/gin"
	"strings"
	"strconv"
)

type CheckoutForm struct {
	Order models.Order
}

func (f *CheckoutForm) Parse(c *gin.Context) error {
	return form.Parse(c, f)
}

type AuthorizeForm struct {
	Order models.Order
	User models.User
	RawExpiry string
}

func (f *AuthorizeForm) Parse(c *gin.Context) error {
	if err := form.Parse(c, f); err != nil {
		return err
	}

	// Parse raw expiry
	parts := strings.Split(f.RawExpiry, "/")
	strMonth, strYear := parts[0], parts[1]
	month, _ := strconv.Atoi(strMonth)
	year, _  := strconv.Atoi(strYear)

	f.Order.Account.Month = month
	f.Order.Account.Year = year
	f.Order.Account.Expiry = strMonth + strYear

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
