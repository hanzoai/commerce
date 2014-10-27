package checkout

import (
	"net/http"
	"github.com/mholt/binding"
	"crowdstart.io/models"
)

type CheckoutForm struct {
	User models.User
	Order models.Order
}

func (cf *CheckoutForm) FieldMap() binding.FieldMap {
    return binding.FieldMap{
        &cf.User.Email: "email",
    }
}

func (cf CheckoutForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	return errs
}
