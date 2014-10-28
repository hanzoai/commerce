package checkout

import (
	"crowdstart.io/cardconnect"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/template"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/schema"
	"log"
)

var decoder = schema.NewDecoder()

func checkout(c *gin.Context) {
	order := new(models.Order)

	c.Request.ParseForm()
	if err := decoder.Decode(order, c.Request.PostForm); err != nil {
		c.Fail(500, err)
		return
	}

	template.Render(c, "checkout/checkout.html", "order", order)
}

func authorize(c *gin.Context) {
	errs := make([]string, 5)
	order := new(models.Order)
	c.Request.ParseForm()
	err := decoder.Decode(order, c.Request.PostForm)
	log.Println(order.BillingAddress.Country)

	db := datastore.New(c)

	log.Println(err)

	if err == nil {
		if order.User.FirstName == "" {
			errs = append(errs, "First name is required")
		}
		if order.User.LastName == "" {
			errs = append(errs, "Last name is required")
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

		wantedItems := make([]models.LineItem, 5)

		for _, i := range order.Items {
			if i.Quantity > 1 {
				item := new(models.ProductVariant)
				err := db.GetKey("variant", i.SKU, &item)
				log.Println(err)
				if err != nil {
					log.Println("err is not nil")
					template.Render(c, "abskjabn.html") // 500
					return
				}
				i.Cost = int64(i.Quantity) * item.Price
				wantedItems = append(wantedItems, i)
			}
		}

		order.Items = wantedItems

		log.Println(order.Items)
		log.Println(errs)

		complete(c)

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
}

func complete(c *gin.Context) {
	template.Render(c, "checkout-complete.html")
}
