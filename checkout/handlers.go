package checkout

import (
	"crowdstart.io/cardconnect"
	"crowdstart.io/util/template"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/schema"
	"github.com/mholt/binding"
)

func checkout(c *gin.Context) {
	template.Render(c, "checkout.html", nil)
}

func checkoutComplete(c *gin.Context) {
	template.Render(c, "checkout-complete.html", nil)
}

var decoder = schema.NewDecoder()

func submitOrder(c *gin.Context) {
	errs = make([]string, 5)

	order := new(Order)

	err := decoder.Decode(order, c.Request.PostForm)

	if err != nil {
		if order.User.Name == "" {
			errs = append(errs, "Name is required")
		}
		if order.User.Email == "" {
			errs = append(errs, "Email address is required")
		}
		if order.User.Phone == "" {
			errs = append(errs, "Phone number is required")
		}
		if order.BillingAddress.Street == "" {
			errs = append(errs, "Street is required")
		}
		if order.BillingAddress.Unit == "" {
			errs = append(errs, "Unit is required")
		}
		if order.BillingAddress.City == "" {
			errs = append(errs, "City is required")
		}
		if order.BillingAddress.State == "" {
			errs = append(errs, "State is required")
		}
		if order.BillingAddress.PostalCode == "" {
			errs = append(errs, "Postal code is required")
		}
		if order.BillingAddress.Country == "" {
			errs = append(errs, "Country is required")
		}
		if len(string(order.PaymentAccount.CVV2)) == 3 {
			errs = append(errs, "Confirmation code is required.")
		}
		if len(string(order.PaymentAccount.Expiry)) == 4 {
			errs = append(errs, "Expiry is required")
		}
	}

	// Authorize order
	ares, err := cardconnect.Authorize(checkoutForm.Order)
	switch {
	case err != nil:
		c.JSON(500, gin.H{"status": "Unable to authorize payment."})
	case ares.Status == "A":

		c.JSON(200, gin.H{"status": "ok"})
	case ares.Status == "B":
		c.JSON(200, gin.H{"status": "retry"})
	case ares.Status == "C":
		c.JSON(200, gin.H{"status": "declined"})
	}
}
