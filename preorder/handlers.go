package preorder

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models"
	"crowdstart.io/thirdparty/mandrill"
	"crowdstart.io/thirdparty/salesforce"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
	"crowdstart.io/util/queries"
	"crowdstart.io/util/template"
)

// GET /order/:token
func GetPreorder(c *gin.Context) {
	// For testing Stackdriver
	// if c.Params.ByName("token") == "test-token" {
	// 	c.Fail(500, errors.New("Test error"))
	// 	return
	// }

	db := datastore.New(c)

	// Fetch token
	token := new(models.Token)
	db.GetKey("invite-token", c.Params.ByName("token"), token)

	// Redirect to login if token is expired or used
	if token.Expired || token.Used {
		c.Redirect(301, "/")
		return
	}

	// Should use token to lookup email
	user := new(models.User)
	if err := db.Get(token.UserId, user); err != nil {
		log.Error("Failed to fetch user: %v", err, c)
		// Bad token
		c.Redirect(301, "../")
		return
	}

	// Get orders by email
	var orders []models.Order
	keys, err := db.Query("order").
		Filter("UserId =", user.Id).
		GetAll(db.Context, &orders)

	if err != nil {
		log.Panic("Error retrieving orders associated with the user's email", err)
	}

	for i := range orders {
		orders[i].LoadVariantsProducts(c)
		orders[i].Id = strconv.Itoa(int(keys[i].IntID()))
	}

	orderId := ""
	orderJSON := "{}"

	// TODO: Make this work for multiple orders? Tie token to each order?
	if len(orders) != 0 {
		order := orders[0]
		orderId = order.Id
		orderJSON = json.Encode(order)
	}

	// Find all of a user's contributions
	var contributions []models.Contribution
	if _, err := db.Query("contribution").Filter("UserId =", user.Id).GetAll(db.Context, &contributions); err != nil {
		log.Panic("Failed to find contributions: %v", err, c)
	}

	log.Debug("Contributions: %v", contributions)
	userJSON := json.Encode(user)
	contributionsJSON := json.Encode(contributions)

	// Get all products
	var products []models.Product
	db.Query("product").GetAll(db.Context, &products)

	// Create map of slug -> product
	productsMap := make(map[string]models.Product)
	for _, product := range products {
		productsMap[product.Slug] = product
	}
	productsJSON := json.Encode(productsMap)

	template.Render(c, "preorder.html",
		"tokenId", token.Id,
		"user", user,
		"productsJSON", productsJSON,
		"contributionsJSON", contributionsJSON,
		"orderJSON", orderJSON,
		"orderId", orderId,
		"userJSON", userJSON,
	)
}

// POST /order/save
func SavePreorder(c *gin.Context) {
	form := new(PreorderForm)
	if err := form.Parse(c); err != nil {
		c.Fail(500, err)
		return
	}

	ctx := middleware.GetAppEngine(c)
	db := datastore.New(ctx)
	q := queries.New(ctx)

	// Get user from datastore
	user := new(models.User)
	if err := q.GetUserByEmail(form.User.Email, user); err != nil {
		c.Fail(500, errors.New("Failed to find user."))
		return
	}

	// Ensure that token matches email
	tokens := getTokens(c, user.Id)
	if len(tokens) < 1 {
		c.Fail(500, errors.New("Failed to find pre-order token."))
		return
	} else if !hasToken(tokens, form.Token.Id) {
		c.Fail(500, errors.New("Token not valid for user email."))
		return
	}

	log.Debug("Found token")

	// Update user's password if this is the first time saving.

	if !user.HasPassword() {
		user.PasswordHash = form.User.PasswordHash
	}

	// Update user information
	user.Phone = form.User.Phone
	user.FirstName = form.User.FirstName
	user.LastName = form.User.LastName
	user.ShippingAddress = form.ShippingAddress
	log.Debug("User: %v", user)

	order := form.Order
	order.ShippingAddress = form.ShippingAddress
	log.Debug("ShippingAddress: %v", user)

	// TODO: Optimize this, multiget, use caching.
	for i, lineItem := range order.Items {
		log.Debug("Fetching variant for %v", lineItem.SKU())

		// Fetch Variant for LineItem from datastore
		if err := db.GetKey("variant", lineItem.SKU(), &lineItem.Variant); err != nil {
			log.Error("Failed to find variant for: %v", lineItem.SKU(), ctx)
			c.Fail(500, err)
			return
		}

		// Fetch Product for LineItem from datastore
		if err := db.GetKey("product", lineItem.Slug(), &lineItem.Product); err != nil {
			log.Error("Failed to find product for: %v", lineItem.Slug(), ctx)
			c.Fail(500, err)
			return
		}

		// Set SKU so we can deserialize later
		lineItem.SKU_ = lineItem.SKU()
		lineItem.Slug_ = lineItem.Slug()

		// Update item in order
		order.Items[i] = lineItem

		// Update subtotal
		order.Subtotal += lineItem.Price()
	}

	// Update Total
	order.Total = order.Subtotal + order.Shipping + order.Tax
	order.UserId = user.Id

	// Save order
	log.Debug("Saving order: %v", order)
	if order.Id != "" {
		log.Debug("Using OrderId: %v", order.Id)
		key, err := strconv.Atoi(order.Id)
		if err != nil {
			log.Error("Invalid Order.Id: %v", err, ctx)
			c.Fail(500, err)
			return
		}

		// Retrieve existing order and update things we care about
		if _, err := db.PutKey("order", key, &order); err != nil {
			log.Error("Error saving order: %v", err, ctx)
			c.Fail(500, err)
			return
		}
	} else {
		log.Debug("No order Id found")
		if _, err := db.Put("order", &order); err != nil {
			log.Error("Error saving order: %v", err, ctx)
			c.Fail(500, err)
			return
		}
	}

	// Save user back to database
	if err := q.UpsertUser(user); err != nil {
		log.Error("Error saving user information", err, ctx)
		c.Fail(500, err)
		return
	}

	// Look up campaign to see if we need to sync with salesforce
	campaign := models.Campaign{}
	if err := db.GetKey("campaign", "dev@hanzo.ai", &campaign); err != nil {
		log.Error(err, c)
	}

	log.Debug("Synchronize with salesforce if '%v' != ''", campaign.Salesforce.AccessToken)
	if campaign.Salesforce.AccessToken != "" {
		salesforce.CallUpsertUserTask(db.Context, &campaign, user)
		salesforce.CallUpsertOrderTask(db.Context, &campaign, &order)
	}

	mandrill.SendTransactional.Call(ctx, "email/preorder-updated.html",
		user.Email,
		user.Name(),
		"SKULLY preorder information updated")

	c.Redirect(301, config.UrlFor("preorder", "/thanks"))
}

// GET /thanks
func Thanks(c *gin.Context) {
	template.Render(c, "thanks.html")
}

// GET /
func Index(c *gin.Context) {
	template.Render(c, "login.html")
	return

	if !auth.IsLoggedIn(c) {
		template.Render(c, "login.html")
	} else {
		user, _ := auth.GetUser(c)
		tokens := getTokens(c, user.Id)

		// Complain if user doesn't have any tokens
		if len(tokens) > 0 {
			// Redirect to order page as they have a valid token
			c.Redirect(301, "order/"+tokens[0].Id)
		} else {
			template.Render(c, "login.html", "message", "No pre-orders found for your account")
			return
		}
	}
}

// POST /
func Login(c *gin.Context) {
	// Parse login form
	f := new(auth.LoginForm)
	if err := f.Parse(c); err != nil {
		template.Render(c, "login.html", "message", "The email or password you entered is incorrect.")
		return
	}

	// Verify password
	err := auth.VerifyUser(c)
	if err != nil {
		template.Render(c, "login.html", "message", "The email or password you entered is incorrect.")
		return
	}

	user, err := auth.GetUser(c)
	if err != nil {
		template.Render(c, "login.html", "message", "An error has occured, please try logging in again.")
	}

	tokens := getTokens(c, user.Id)
	log.Debug("Tokens: %v", tokens)
	// Complain if user doesn't have any tokens
	if len(tokens) > 0 {
		// Redirect to order page as they have a valid token
		c.Redirect(301, "order/"+tokens[0].Id)
	} else {
		template.Render(c, "login.html", "message", "No pre-orders found for your account")
	}
}

// hasToken checks whether any of the tokens have the id
func hasToken(tokens []models.Token, id string) bool {
	for _, token := range tokens {
		if token.Id == id {
			return true
		}
	}
	return false
}

func getTokens(c *gin.Context, userId string) []models.Token {
	db := datastore.New(c)

	// Look up tokens for this user
	log.Debug("Searching for valid token for: %v", userId, c)

	tokens := make([]models.Token, 0)
	if _, err := db.Query("invite-token").Filter("UserId =", userId).GetAll(db.Context, &tokens); err != nil {
		log.Panic("Failed to query for tokens: %v", err, c)
	}

	return tokens
}
