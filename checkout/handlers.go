package checkout

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/stripe"
	"crowdstart.io/util/log"
	"crowdstart.io/util/template"
)

// Renders the checkout page with an error message
func formError(c *gin.Context, err error) {
	template.Render(c, "checkout.html",
		"message", "There was an error while processing your order.",
	)
}

func checkout(c *gin.Context) {
	form := new(CheckoutForm)
	if err := form.Parse(c); err != nil {
		log.Error(err.Error())
		formError(c, err)
		return
	}

	db := datastore.New(c)

	order := form.Order

	log.Info("Processing order. %#v", order)

	for i, lineItem := range order.Items {
		// Fetch Variant for LineItem from datastore
		if err := db.GetKey("variant", lineItem.SKU(), &lineItem.Variant); err != nil {
			log.Error(err.Error())
			formError(c, err)
			return
		}

		// Fetch Product for LineItem from datastore
		if err := db.GetKey("product", lineItem.Slug(), &lineItem.Product); err != nil {
			log.Error(err.Error())
			formError(c, err)
			return
		}

		order.Items[i] = lineItem
		order.Subtotal += lineItem.Price()
	}

	order.Total = order.Subtotal + order.Tax

	template.Render(c, "checkout.html", "order", order, "config", config.Get())
}

func authorize(c *gin.Context) {
	form := new(AuthorizeForm)
	if err := form.Parse(c); err != nil {
		log.Error(err.Error())
		formError(c, err)
		log.Debug("Account %#v", form.Order.Account)
		return
	}

	db := datastore.New(c)
	order := form.Order

	log.Info("Authorizing order. Items: %#v", order.Items)

	for i, lineItem := range order.Items {
		// TODO: Figure out why this happens
		if lineItem.SKU() == "" {
			continue
		}

		log.Debug("SKU: %#v", lineItem.SKU())

		// Fetch Variant for LineItem from datastore
		if err := db.GetKey("variant", lineItem.SKU(), &lineItem.Variant); err != nil {
			log.Error(err.Error())
			formError(c, err)
			return
		}

		log.Debug("Line item: %#v", lineItem)
		order.Items[i] = lineItem
		order.Subtotal += lineItem.Price()
	}

	ctx := middleware.GetAppEngine(c)
	charge, err := stripe.Charge(ctx, &order, stripe.token)

	if err != nil {
		log.Error(err.Error())
		formError(c, err)
	}
}

func complete(c *gin.Context) {
	template.Render(c, "checkout-complete.html")
}
