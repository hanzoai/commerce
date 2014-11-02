package checkout

import (
	"crowdstart.io/cardconnect"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/util/template"
	"github.com/gin-gonic/gin"
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

	// Pass in user model so we have token to use.
	user := new(models.User)
	db.GetKey("user", "skully", &user)

	template.Render(c, "checkout/checkout.html", "order", order, "user", &user)
}

func authorize(c *gin.Context) {
	form := new(AuthorizeForm)
	if err := form.Parse(c); err != nil {
		c.Fail(500, err)
		return
	}

	db := datastore.New(c)
	order := form.Order
	user := form.User
	order.User = user

	ctx := middleware.GetAppEngine(c)
	ctx.Infof("%#v", order.Items)

	for i, lineItem := range order.Items {
		// TODO: Figure out why this happens
		if lineItem.SKU() == "" {
			continue
		}

		ctx.Infof("SKU: %#v", lineItem.SKU())

		// Fetch Variant for LineItem from datastore
		if err := db.GetKey("variant", lineItem.SKU(), &lineItem.Variant); err != nil {
			c.Fail(500, err)
			return
		}

		ctx.Debugf("%#v", lineItem)
		order.Items[i] = lineItem
		order.Subtotal += lineItem.Price()
	}

	// Authorize order
	ares, err := cardconnect.Authorize(ctx, form.Order)
	switch {
	case err != nil:
		ctx.Errorf("%v", err)
		c.JSON(500, gin.H{"status": "Unable to authorize payment."})
	case ares.Status == "A":
		ctx.Debugf("%#v", ares)
		c.JSON(200, gin.H{"status": "ok"})
	case ares.Status == "B":
		ctx.Debugf("%#v", ares)
		c.JSON(200, gin.H{"status": "retry", "message": ares.Text})
	case ares.Status == "C":
		ctx.Debugf("%#v", ares)
		c.JSON(200, gin.H{"status": "declined", "message": ares.Text})
	}
}

func complete(c *gin.Context) {
	template.Render(c, "checkout-complete.html")
}
