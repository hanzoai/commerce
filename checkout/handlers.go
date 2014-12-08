package checkout

import (
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/thirdparty/stripe"
	"crowdstart.io/util/log"
	"crowdstart.io/util/template"
)

// Redirect to store on GET
func index(c *gin.Context) {
	c.Redirect(301, config.UrlFor("store", "/cart"))
}

// Display checkout form
func checkout(c *gin.Context) {
	// Parse checkout form
	form := new(CheckoutForm)
	if err := form.Parse(c); err != nil {
		log.Error("Failed to parse form: %v", err)
		c.Fail(500, err)
		return
	}

	db := datastore.New(c)

	// Populate with data from DB
	if err := form.Populate(db); err != nil {
		log.Error("Failed to populate order with information from datastore: %v", err)
		c.Fail(500, err)
		return
	}

	// Validate form
	form.Validate(c)

	// Render order for checkout page
	template.Render(c, "checkout.html",
		"order", form.Order,
		"config", config.Get(),
	)
}

// Charge a successful authorization
// LoginRequired
func charge(c *gin.Context) {
	user := auth.GetUser(c)

	form := new(ChargeForm)
	if err := form.Parse(c); err != nil {
		log.Error("Failed to parse form: %v", err)
		c.Fail(500, err)
		return
	}

	form.Order.CreatedAt = time.Now()

	ctx := middleware.GetAppEngine(c)
	db := datastore.New(ctx)

	if err := form.Order.Populate(db); err != nil {
		log.Error("Failed to repopulate order information from datastore: %v", err)
		log.Dump(form.Order)
		c.Fail(500, err)
		return
	}

	log.Debug("Charging order.")
	log.Dump(form.Order)
	if _, err := stripe.Charge(ctx, form.StripeToken, &form.Order); err != nil {
		log.Error("Stripe Charge failed: %v", err)
		c.Fail(500, err)
		return
	}

	// Save order
	log.Debug("Saving order.")
	key, err := db.Put("order", &form.Order)
	if err != nil {
		log.Error("Error saving order", err)
		c.Fail(500, err)
		return
	}

	// Add order to user;
	user.OrdersIds = append(user.OrdersIds, key)
	if _, err = db.PutKey("user", user.Email, user); err != nil {
		log.Panic("Saving user after adding order failed \n%v", err)
	}

	log.Debug("Checkout complete!")
	c.Redirect(301, config.UrlFor("checkout", "/complete"))
}

// Success
func complete(c *gin.Context) {
	template.Render(c, "complete.html")
}
