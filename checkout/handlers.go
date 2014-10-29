package checkout

import (
	"crowdstart.io/cardconnect"
	"crowdstart.io/middleware"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/template"
	"github.com/gin-gonic/gin"
	"log"
)

func checkout(c *gin.Context) {
	form := new(CheckoutForm)

	if err := form.Parse(c); err != nil {
		c.Fail(500, err)
		return
	}

	order := form.Order
	db := datastore.New(c)
	ctx := middleware.GetAppEngine(c)
	ctx.Infof("%#v", order)

	for i, lineItem := range order.Items {
		// Fetch Variant for LineItem from datastore
		if err := db.GetKey("variant", lineItem.SKU(), &lineItem.Variant); err != nil {
			c.Fail(500, err)
			return
		}

		// Fetch Product for LineItem from datastore
		if err := db.GetKey("product", lineItem.Slug(), &lineItem.Product); err != nil {
			c.Fail(500, err)
			return
		}

		order.Items[i] = lineItem
		order.Subtotal += lineItem.Price()
	}

	order.Total = order.Subtotal + order.Tax
	template.Render(c, "checkout/checkout.html", "order", order)
}

func authorize(c *gin.Context) {
	form := new(AuthorizeForm)
	if err := form.Parse(c); err != nil {
		c.Fail(500, err)
		return
	}

	db := datastore.New(c)

	wantedItems := make([]models.LineItem, 5)

	for _, i := range form.Order.Items {
		if i.Quantity > 1 {
			item := new(models.ProductVariant)
			err := db.GetKey("variant", i.SKU(), &item)
			log.Println(err)
			if err != nil {
				c.Fail(500, err)
				return
			}
			wantedItems = append(wantedItems, i)
		}
	}

	form.Order.Items = wantedItems

	complete(c)

	// Authorize order
	ares, err := cardconnect.Authorize(form.Order)
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

func complete(c *gin.Context) {
	template.Render(c, "checkout-complete.html")
}
