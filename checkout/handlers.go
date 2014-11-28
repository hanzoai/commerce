package checkout

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/config"
	"crowdstart.io/middleware"
	"crowdstart.io/stripe"
	"crowdstart.io/util/log"
	"crowdstart.io/util/template"
)

// Renders the checkout page with an error message
func formError(c *gin.Context) {
	template.Render(c, "checkout.html",
		"message", "There was an error while processing your order.",
	)
}

func checkout(c *gin.Context) {
	form := new(CheckoutForm)
	if err := form.Parse(c); err != nil {
		log.Error(err.Error())
		formError(c)
		return
	}

	// TODO: VALIDATE PLZ GOD
	errs := form.Validate()
	template.Render(c, "checkout.html",
		"order", form.Order,
		"config", config.Get(),
		"errors", errs)
}

func authorize(c *gin.Context) {
	log.Debug("Request")
	form := new(AuthorizeForm)
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

	log.Info("Authorizing order. Items: %#v", order.Items)
	ctx := middleware.GetAppEngine(c)
	_, err = stripe.Charge(ctx, form.StripeToken, &order)

	if err != nil {
		log.Error(err.Error())
		formError(c)
	}
}

func complete(c *gin.Context) {
	template.Render(c, "checkout-complete.html")
}
