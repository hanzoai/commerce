package checkout

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"appengine"
	. "appengine/datastore"
	"appengine/delay"

	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models"
	"crowdstart.io/thirdparty/mandrill"
	"crowdstart.io/thirdparty/salesforce"
	"crowdstart.io/thirdparty/stripe"
	"crowdstart.io/util/cache"
	"crowdstart.io/util/log"
	"crowdstart.io/util/queries"
	"crowdstart.io/util/template"
)

// Helper to get campaign
func getCampaign(args ...interface{}) models.Campaign {
	c := args[0].(*gin.Context)
	db := args[1].(*datastore.Datastore)
	var campaign models.Campaign
	if err := db.GetKey("campaign", "dev@hanzo.ai", &campaign); err != nil {
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

// Deferred Tasks
// This function upserts a contact into salesforce
var salesforceUpsertTask = delay.Func("SalesforceUpsert", func(c *gin.Context, api *salesforce.Api, contact *salesforce.Contact) {
	// The email is required as it is the external ID used in salesforce
	if contact.Email == "" {
		log.Panic("Email is required for upsert")
	}

	db := datastore.New(c)

	// Query out all orders (since preorder is stored as a single string)
	var orders []models.Order
	_, err := db.Query("order").
		Filter("Email =", contact.Email).
		GetAll(db.Context, &orders)

	// Ignore any field mismatch errors.
	if err != nil {
		if _, ok := err.(*ErrFieldMismatch); ok {
			log.Warn("Field mismatch when getting order", db.Context)
			err = nil
		} else {
			log.Panic("Error retrieving orders associated with the user's email", err)
		}
	}

	// Query out any preorder order items and sum different skus up for totals
	items := make(map[string]int)

	for _, order := range orders {
		if order.Preorder {
			for _, item := range order.Items {
				items[item.SKU_] = items[item.SKU_] + item.Quantity
			}
		}
	}

	// Stringify
	preorders := ""

	for key, item := range items {
		preorders += fmt.Sprintf("%s: %d", key, item)
	}

	// Assign to contact and synchronize
	contact.PreorderC = preorders

	if err := salesforce.UpsertContact(api, contact); err != nil {
		log.Panic("UpsertContactTask failed: %v", err)
	}
})

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
	encodedKey, err := db.Put("order", &form.Order)
	if err != nil {
		log.Error("Failed to save order", err, c)
		c.Fail(500, err)
		return
	}
	key, _ := db.DecodeKey(encodedKey)
	orderId := key.IntID()

	// Synchronize Salesforce
	salesforceTokens := getSalesforceTokens(c, db).(struct {
		AccessToken  string
		RefreshToken string
		InstanceUrl  string
		Id           string
		IssuedAt     string
		Signature    string
	})

	if salesforceTokens.AccessToken != "" {
		api, err := salesforce.Init(
			c,
			salesforceTokens.AccessToken,
			salesforceTokens.RefreshToken,
			salesforceTokens.InstanceUrl,
			salesforceTokens.Id,
			salesforceTokens.IssuedAt,
			salesforceTokens.Signature)

		if err != nil {
			contact := salesforce.Contact{
				LastName:           user.LastName,
				FirstName:          user.FirstName,
				Phone:              user.Phone,
				Email:              user.Email,
				ShippingAddressC:   user.ShippingAddress.Line1 + user.ShippingAddress.Line2,
				ShippingCityC:      user.ShippingAddress.City,
				ShippingStateC:     user.ShippingAddress.State,
				ShippingPostalZipC: user.ShippingAddress.PostalCode,
				ShippingCountryC:   user.ShippingAddress.Country,
			}

			// Launch a synchronization task
			salesforceUpsertTask.Call(appengine.NewContext(c.Request), c, api, &contact)
		} else {
			log.Debug("Could not synchronize with salesforce.")
		}
	}

	// Generate invite for preorder site.
	log.Debug("Saving invite token...", c)
	invite := new(models.Token)
	invite.GenerateId()
	invite.UserId = user.Id
	if _, err := db.PutKey("invite-token", invite.Id, invite); err != nil {
		log.Error("Failed to save invite-token: %v", err, c)
		c.Fail(500, err)
		return
	}

	// Save contribution for preorder site.
	log.Debug("Saving contribution...", c)
	contribution := new(models.Contribution)
	contribution.Id = strconv.Itoa(int(orderId))
	contribution.UserId = user.Id
	contribution.Perk = models.Perks["WINTER2014PROMO"]
	if _, err := db.PutKey("contribution", contribution.Id, contribution); err != nil {
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
