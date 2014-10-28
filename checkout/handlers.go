package checkout

import (
	"crowdstart.io/cardconnect"
	"crowdstart.io/models"
	"crowdstart.io/util/template"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/schema"
)

func checkout(c *gin.Context) {
	template.Render(c, "checkout.html")
}

func checkoutComplete(c *gin.Context) {
	template.Render(c, "checkout-complete.html")
}

var decoder = schema.NewDecoder()

func submitOrder(c *gin.Context) {
	errs := make([]string, 5)

	order := new(models.Order)

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
		if len(string(order.Account.CVV2)) == 3 {
			errs = append(errs, "Confirmation code is required.")
		}
		if len(string(order.Account.Expiry)) == 4 {
			errs = append(errs, "Expiry is required")
		}
	}
	
	// Authorize order
	if len(errs) == 0 {
		ares, err := cardconnect.Authorize(*order)
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
}
