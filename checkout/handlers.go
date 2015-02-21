package checkout

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models"
	"crowdstart.io/thirdparty/salesforce"
	"crowdstart.io/thirdparty/stripe"
	"crowdstart.io/util/cache"
	"crowdstart.io/util/log"
	"crowdstart.io/util/queries"
	"crowdstart.io/util/template"

	mandrill "crowdstart.io/thirdparty/mandrill/tasks"
)

// Helper to get campaign
func getCampaign(args ...interface{}) models.Campaign {
	c := args[0].(*gin.Context)
	db := args[1].(*datastore.Datastore)
	var campaign models.Campaign
	if err := db.GetKind("campaign", "dev@hanzo.ai", &campaign); err != nil {
		log.Error(err, c)
	}
	return campaign
}

// Cache stripe publishable key, access token for a minute each
var getStripePublishableKey = cache.Memoize(func(args ...interface{}) interface{} {
	return getCampaign(args...).Stripe.PublishableKey
}, 60)

var getStripeAccessToken = cache.Memoize(func(args ...interface{}) interface{} {
	return getCampaign(args...).Stripe.AccessToken
}, 60)

var getSalesforceTokens = cache.Memoize(func(args ...interface{}) interface{} {
	return getCampaign(args...).Salesforce
}, 60)

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
	stripePublishableKey := getStripePublishableKey(c, db).(string)

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
	form.Order.UpdatedAt = form.Order.CreatedAt

	ctx := middleware.GetAppEngine(c)
	db := datastore.New(ctx)
	q := queries.New(ctx)

	// Populate
	if err := form.Order.Populate(db); err != nil {
		log.Error("Failed to repopulate order information from datastore: %v", err)
		c.Fail(500, err)
		return
	}

	// Validation
	form.Sanitize()
	if errs := form.Validate(); len(errs) > 0 {
		c.JSON(400, gin.H{"message": errs})
		return
	}

	// Update user information
	log.Debug("Trying to get user from session...", c)
	user, err := auth.GetUser(c)
	if err != nil {
		// see if this is a returning user
		log.Debug("User is not logged in")
		returningUser := new(models.User)
		if err = q.GetUserByEmail(form.User.Email, returningUser); err != nil {
			log.Debug("Using form.User", c)
			user.Id = db.EncodeId("user", db.AllocateId("user"))
			user = &form.User
		} else {
			log.Debug("Returning User")
			user = returningUser
		}
	}
	log.Debug("User: %#v", user)

	// Set email for order
	form.Order.UserId = user.Id
	form.Order.Email = user.Email
	form.Order.CampaignId = "dev@hanzo.ai"
	form.Order.Preorder = true

	// Set test mode, minimum stripe transaction
	if strings.Contains(user.Email, "@verus.io") {
		form.Order.Test = true
		form.Order.Shipping = 0
		form.Order.Tax = 0
		form.Order.Subtotal = 50 * 100 // 50 cents is Stripe's
		form.Order.Total = 50 * 100    // minimum transaction amount.
	}

	// Get access token
	stripeAccessToken := getStripeAccessToken(c, db).(string)

	// Charging order
	log.Debug("Charging order...", c)
	log.Debug("API Key: %v, Token: %v", stripeAccessToken, form.StripeToken)
	charge, err := stripe.Charge(ctx, stripeAccessToken, form.StripeToken, &form.Order, user)
	if err != nil {
		log.Warn("stripe error %v", err)
		if charge.FailMsg != "" {
			// client error
			log.Warn("Stripe declined charge: %v", err, c)
			c.JSON(400, gin.H{"message": charge.FailMsg})
		} else {
			// internal error
			log.Error("Stripe charge failed: %v", err, c)
			c.JSON(500, gin.H{})
		}
	}

	// We'll update user even if charge failed, this ensures consistent profile
	// data and stripe customer consistency.
	log.Debug("Updating and saving user...", c)
	user.BillingAddress = form.Order.BillingAddress
	user.ShippingAddress = form.Order.ShippingAddress
	user.Phone = form.User.Phone
	user.FirstName = form.User.FirstName
	user.LastName = form.User.LastName
	if err := q.UpsertUser(user); err != nil {
		log.Error("Failed to save user: %v", err, c)
		if charge.Captured {
			c.Fail(500, err)
		}
		return
	}

	// If charge failed, bail out here
	if !charge.Captured {
		return
	}

	// Save order
	log.Debug("Saving order...", c)
	key, err := db.Put("order", &form.Order)
	encodedKey := key.Encode()
	if err != nil {
		log.Error("Failed to save order", err, c)
		c.Fail(500, err)
		return
	}
	key, _ = db.DecodeKey(encodedKey)
	orderId := key.IntID()

	// Set the id as we use it to update salesforce
	form.Order.Id = key.Encode()

	// Synchronize Salesforce
	salesforceTokens := getSalesforceTokens(c, db).(models.SalesforceTokens)

	if salesforceTokens.AccessToken != "" {
		// Launch a synchronization task
		campaign := getCampaign(c, db)
		salesforce.CallUpsertUserTask(ctx, &campaign, user)
		salesforce.CallUpsertOrderTask(ctx, &campaign, &form.Order)
	}

	// Generate invite for preorder site.
	log.Debug("Saving invite token...", c)
	invite := new(models.Token)
	invite.GenerateId()
	invite.UserId = user.Id
	invite.Email = user.Email
	if _, err := db.PutKind("invite-token", invite.Id, invite); err != nil {
		log.Error("Failed to save invite-token: %v", err, c)
		c.Fail(500, err)
		return
	}

	// Save contribution for preorder site.
	log.Debug("Saving contribution...", c)
	contribution := new(models.Contribution)
	contribution.Id = strconv.Itoa(int(orderId))
	contribution.UserId = user.Id
	contribution.Email = user.Email
	contribution.Perk = models.Perks["WINTER2014PROMO"]
	if _, err := db.PutKind("contribution", contribution.Id, contribution); err != nil {
		log.Error("Failed to save contribution: %v", err, c)
		c.Fail(500, err)
		return
	}

	// Send order confirmation email
	mandrill.SendTransactional.Call(ctx, "email/order-confirmation.html",
		user.Email,
		user.Name(),
		fmt.Sprintf("SKULLY Order confirmation #%v", orderId))

	log.Debug("Checkout complete!", c)
	c.JSON(200, gin.H{"inviteId": invite.Id, "orderId": orderId})
}

// Success
func complete(c *gin.Context) {
	template.Render(c, "complete.html")
}
