package checkout

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/config"
	"crowdstart.io/middleware"
	"crowdstart.io/thirdparty/stripe"
	"crowdstart.io/util/log"
	"crowdstart.io/util/template"
)

// Renders the checkout page with an error message
func formError(c *gin.Context) {
	template.Render(c, "checkout.html",
		"message", "There was an error while processing your order.",
	)
}

// Redirect to store on GET
func index(c *gin.Context) {
	c.Redirect(301, config.UrlFor("store", "/cart"))
}

// Display checkout form
func checkout(c *gin.Context) {
	// Parse checkout form
	form := new(CheckoutForm)
	if err := form.Parse(c); err != nil {
		log.Error(err.Error())
		formError(c)
		return
	}

	// Populate with data from DB
	form.Populate(c)

	// Validate form
	form.Validate(c)

	// Render order for checkout page
	template.Render(c, "checkout.html",
		"order", form.Order,
		"config", config.Get(),
	)
}

// Charge a successful authorization
func charge(c *gin.Context) {
	form := new(ChargeForm)
	if err := form.Parse(c); err != nil {
		log.Error(err.Error())
		formError(c)
		log.Debug("Account %#v", form.Order.Account)
		return
	}

	order := form.Order
	err := order.Process(c)
	if err != nil {
		log.Error(err.Error())
		return
	}

	log.Info("Charging order. Items: %#v", order.Items)
	ctx := middleware.GetAppEngine(c)
	_, err = stripe.Charge(ctx, form.StripeToken, &order)

	if err != nil {
		log.Error(err.Error())
		formError(c)
	}
}

// Success
func complete(c *gin.Context) {
	template.Render(c, "checkout-complete.html")
}
