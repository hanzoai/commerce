package checkout

import (
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models"
	"crowdstart.io/thirdparty/mandrill"
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

	// Merge duplicate line items.
	form.Merge(c)

	// Validate form
	form.Validate(c)

	// Get API Key.
	var campaign models.Campaign
	db.GetKey("campaign", "dev@hanzo.ai", &campaign)

	// Render order for checkout page
	template.Render(c, "checkout.html",
		"order", form.Order,
		"StripeAPIKey", campaign.StripeAPIKey,
	)
}

// Charge a successful authorization
// LoginRequired
func charge(c *gin.Context) {
	form := new(ChargeForm)
	if err := form.Parse(c); err != nil {
		log.Error("Failed to parse form: %v", err)
		c.Fail(500, err)
		return
	}

	form.Order.CreatedAt = time.Now()

	ctx := middleware.GetAppEngine(c)
	db := datastore.New(ctx)

	// Populate
	if err := form.Order.Populate(db); err != nil {
		log.Error("Failed to repopulate order information from datastore: %v", err)
		log.Dump(form.Order)
		c.Fail(500, err)
		return
	}

	// Charging order
	log.Debug("Charging order.")
	log.Dump(form.Order)
	if _, err := stripe.Charge(ctx, form.StripeToken, &form.Order); err != nil {
		log.Error("Stripe Charge failed: %v", err)
		c.Fail(500, err)
		return
	}

	// Save order
	log.Debug("Saving order.")
	_, err := db.Put("order", &form.Order)
	if err != nil {
		log.Error("Error saving order", err)
		c.Fail(500, err)
		return
	}

	// Update user information
	user, err := auth.GetUser(c)
	if err != nil {
		user = &form.User
	}

	user.BillingAddress = form.Order.BillingAddress
	user.ShippingAddress = form.Order.ShippingAddress
	db.PutKey("user", user.Email, user)

	// Send confirmation email
	mandrill.SendTemplateAsync.Call(ctx, "confirmation-order", user.Email, user.Name())

	log.Debug("Checkout complete!")
	c.Redirect(301, config.UrlFor("checkout", "/complete"))
}

// Success
func complete(c *gin.Context) {
	template.Render(c, "complete.html")
}
