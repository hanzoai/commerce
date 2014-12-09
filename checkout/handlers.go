package checkout

import (
	"strings"
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

// Cache stripe keys
var stripePublishableKey string
var stripeAccessToken string

// GET /
func index(c *gin.Context) {
	// Redirect to store on GET
	c.Redirect(301, config.UrlFor("store", "/cart"))
}

// POST /
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

	// Get PublishableKey Key.
	if stripePublishableKey == "" {
		var campaign models.Campaign
		if err := db.GetKey("campaign", "dev@hanzo.ai", &campaign); err != nil {
			log.Error(err, c)
		} else {
			stripePublishableKey = campaign.Stripe.PublishableKey
		}
	}

	// Try to get user from datastore based on email in session.
	user, _ := auth.GetUser(c)

	// Render order for checkout page
	template.Render(c, "checkout.html",
		"order", form.Order,
		"stripePublishableKey", stripePublishableKey,
		"user", user,
	)
}

// Charge a successful authorization
// POST /charge
func charge(c *gin.Context) {
	form := new(ChargeForm)
	if err := form.Parse(c); err != nil {
		log.Error("Failed to parse form: %v", err, c)
		c.Fail(500, err)
		return
	}

	form.Order.CreatedAt = time.Now()

	ctx := middleware.GetAppEngine(c)
	db := datastore.New(ctx)

	// Populate
	if err := form.Order.Populate(db); err != nil {
		log.Error("Failed to repopulate order information from datastore: %v", err)
		c.Fail(500, err)
		return
	}

	// Update user information
	log.Debug("Trying to get user from session...", c)
	user, err := auth.GetUser(c)
	if err != nil {
		log.Debug("Using form.User", c)
		user = &form.User
	}
	log.Debug("User: %#v", user)

	// Set email for order
	form.Order.Email = user.Email

	// Set test mode, minimum stripe transaction
	if strings.Contains(user.Email, "@verus.io") {
		form.Order.Test = true
		form.Order.Shipping = 0
		form.Order.Tax = 0
		form.Order.Subtotal = 50 * 100 // 50 cents is Stripe's
		form.Order.Total = 50 * 100    // minimum transaction amount.
	}

	// Save order
	log.Debug("Saving order...", c)
	if _, err := db.Put("order", &form.Order); err != nil {
		log.Error("Error saving order", err, c)
		c.Fail(500, err)
		return
	}

	log.Debug("Updating and saving user...", c)
	user.BillingAddress = form.Order.BillingAddress
	user.ShippingAddress = form.Order.ShippingAddress
	user.Phone = form.User.Phone
	user.FirstName = form.User.FirstName
	user.LastName = form.User.LastName
	if _, err := db.PutKey("user", user.Email, user); err != nil {
		log.Error("Error saving order", err, c)
		c.Fail(500, err)
		return
	}

	// Get access token
	if stripeAccessToken == "" {
		var campaign models.Campaign
		if err := db.GetKey("campaign", "dev@hanzo.ai", &campaign); err != nil {
			log.Error("Unable to get stripe access token: %v", err)
			c.Fail(500, err)
			return
		} else {
			stripeAccessToken = campaign.Stripe.AccessToken
		}
	}

	// Charging order
	log.Debug("Charging order...", c)
	log.Debug("API Key: %v, Token: %v", stripeAccessToken, form.StripeToken)
	if _, err := stripe.Charge(ctx, stripeAccessToken, form.StripeToken, &form.Order); err != nil {
		log.Error("Stripe Charge failed: %v", err, c)
		c.Fail(500, err)
		return
	}

	// Send confirmation email
	mandrill.SendTemplateAsync.Call(ctx, "confirmation-order", user.Email, user.Name())

	log.Debug("Checkout complete!", c)
	c.Redirect(301, config.UrlFor("checkout", "/complete"))
}

// Success
func complete(c *gin.Context) {
	template.Render(c, "complete.html")
}
