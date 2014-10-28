package checkout

import (
	"crowdstart.io/util/form"
	"crowdstart.io/models"
)

type CheckoutForm struct {
	form.Form
	Order *models.Order
}

type AuthorizeForm struct {
	form.Form
	Order models.Order
	User  models.User
}

func (f AuthorizeForm) Validate() (errs []string) {
	if f.Order.User.FirstName == "" {
		errs = append(errs, "First name is required")
	}
	if f.Order.User.LastName == "" {
		errs = append(errs, "Last name is required")
	}
	if f.Order.User.Email == "" {
		errs = append(errs, "Email address is required")
	}
	if f.Order.User.Phone == "" {
		errs = append(errs, "Phone number is required")
	}
	if f.Order.BillingAddress.Street == "" {
		errs = append(errs, "Street is required")
	}
	if f.Order.BillingAddress.Unit == "" {
		errs = append(errs, "Unit is required")
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
	if len(string(f.Order.Account.CVV2)) == 3 {
		errs = append(errs, "Confirmation code is required.")
	}
	if len(string(f.Order.Account.Expiry)) == 4 {
		errs = append(errs, "Expiry is required")
	}

	return errs
}
