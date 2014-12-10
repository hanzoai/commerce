package preorder

import (
	"errors"

	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models"
	"crowdstart.io/thirdparty/mandrill"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
	"crowdstart.io/util/template"
)

// GET /order/:token
// GetMultiPreorder should:
//	1) extract an invite-token from the url.
//	2) retrieve the invite-token from the datastore.
//  3) use the email from the invite-token to search for orders associated with it.
//  4) if there is
var productsJSON string
var productsMap map[string]models.Product
var variantsMap map[string]models.ProductVariant

func loadProducts(c *gin.Context) {
	if productsJSON == "" || productsMap == nil {
		db := datastore.New(c)
		// Get all products
		var products []models.Product
		db.Query("product").GetAll(db.Context, &products)

		// Create map of slug -> product
		for _, product := range products {
			productsMap[product.Slug] = product
		}
		productsJSON = json.Encode(productsMap)
	}
}

func loadVariants(c *gin.Context) {
	if variantsMap == nil {
		db := datastore.New(c)
		var variants []models.ProductVariant
		db.Query("variant").GetAll(db.Context, &variants)
		for _, variant := range variants {
			variantsMap[variant.SKU] = variant
		}
	}
}

func GetMultiPreorder(c *gin.Context) {
	db := datastore.New(c)

	loadProducts(c)

	token := new(models.InviteToken)
	if err := db.GetKey("invite-token", c.Params.ByName("token"), token); err != nil {
		log.Panic("Unable to retrieve invite-token \nToken: %s \nError: %v", c.Params.ByName("token"), err)
	}
	if token.Expired || token.Used {
		c.Redirect(301, "/")
		return
	}

	user := new(models.User)
	if err := db.GetKey("user", token.Email, user); err != nil {
		log.Panic("Unable to retrieve user. \nEmail: %s \nError: %v", token.Email, err)
	}

	var orders []models.Order
	if user.HasPassword() {
		_, err := db.Query("order").
			Filter("Email =", user.Email).
			GetAll(db.Context, &orders)
		if err != nil {
			log.Panic("No orders found for Email: %s", user.Email)
		}
	}

	var contributions []models.Contribution
	if _, err := db.Query("contribution").
		Filter("Email =", user.Email).
		GetAll(db.Context, &contributions); err != nil {
		log.Panic("Failed to find contributions: %v", err, c)
	}
	log.Debug("Contributions: %v", contributions)

	userJSON := json.Encode(user)
	contributionsJSON := json.Encode(contributions)

	ordersJSON := "[]"
	indiegogoPreorder := new(models.Order)
	if err := db.GetKey("order", user.Email, indiegogoPreorder); err == nil {
		indiegogoPreorder.Preorder = true
		orders = append(orders, *indiegogoPreorder)
		ordersJSON = json.Encode(orders)
	}

	template.Render(c, "preorder.html",
		"tokenId", token.Id,
		"user", user,
		"productsJSON", productsJSON,
		"contributionsJSON", contributionsJSON,
		"ordersJSON", ordersJSON,
		"userJSON", userJSON,
	)
}

// POST /order/save
func SaveMultiPreorder(c *gin.Context) {
	form := new(PreorderForm)
	if err := form.Parse(c); err != nil {
		log.Panic("Error parsing preorder form \n%v", err)
	}

	db := datastore.New(c)

	user := new(models.User)
	if err := db.GetKey("user", form.User.Email, user); err != nil {
		log.Panic("Error retrieving user \n%v", err)
	}

	tokens := getTokens(c, user.Email)
	if len(tokens) < 1 {
		log.Panic("Failed to find preorder token")
	} else if !hasToken(tokens, form.Token.Id) {
		log.Panic("Token not valid for user email")
	}

	if !user.HasPassword() {
		user.PasswordHash = form.User.PasswordHash
	}

	// Update user information
	user.Phone = form.User.Phone
	user.FirstName = form.User.FirstName
	user.LastName = form.User.LastName
	user.ShippingAddress = form.ShippingAddress
	log.Debug("User: %v", user)

	loadVariants()
	loadProducts()

	orders := form.Orders
	for i, order := range orders {
		order.ShippingAddress = form.ShippingAddress
		for i, item := range order.Items {
			if variant, ok := variantsMap[item.SKU_]; ok {
				item.Variant = variant
			} else {
				log.Panic("Invalid variant \nSKU: %s", item.SKU_)
			}

			if product, ok := productsMap[item.Slug_]; ok {
				item.Product = product
			} else {
				log.Panic("Invalid product \nSlug_: %s", item.Slug_)
			}
			order.Items[i] = item
		}
		order.Total = order.Subtotal + order.Shipping + order.Tax
		order.Email user.Email

		orders[i] = order
	}
}

// POST /order/save
func SavePreorder(c *gin.Context) {
	form := new(PreorderForm)
	if err := form.Parse(c); err != nil {
		c.Fail(500, err)
		return
	}

	db := datastore.New(c)

	// Get user from datastore
	user := new(models.User)
	db.GetKey("user", form.User.Email, user)

	// Ensure that token matches email
	tokens := getTokens(c, user.Email)
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
			log.Error("Failed to find variant for: %v", lineItem.SKU(), c)
			c.Fail(500, err)
			return
		}

		// Fetch Product for LineItem from datastore
		if err := db.GetKey("product", lineItem.Slug(), &lineItem.Product); err != nil {
			log.Error("Failed to find product for: %v", lineItem.Slug(), c)
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
	order.Email = user.Email

	// Save order
	// TODO: Need to not putkey on email, but reuse order id
	log.Debug("Saving order: %v", order)
	_, err := db.PutKey("order", user.Email, &order)
	if err != nil {
		log.Error("Error saving order", err)
		c.Fail(500, err)
		return
	}

	// Save user back to database
	_, err = db.PutKey("user", user.Email, user)
	if err != nil {
		log.Error("Error saving user information", err)
		c.Fail(500, err)
		return
	}

	ctx := middleware.GetAppEngine(c)
	mandrill.SendTemplateAsync.Call(ctx, "preorder-confirmation-template", user.Email, user.Name())

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
		tokens := getTokens(c, user.Email)

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

	tokens := getTokens(c, f.Email)
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
func hasToken(tokens []models.InviteToken, id string) bool {
	for _, token := range tokens {
		if token.Id == id {
			return true
		}
	}
	return false
}

func getTokens(c *gin.Context, email string) []models.InviteToken {
	db := datastore.New(c)

	// Look up tokens for this user
	log.Debug("Searching for valid token for: %v", email, c)

	tokens := make([]models.InviteToken, 0)
	if _, err := db.Query("invite-token").Filter("Email =", email).GetAll(db.Context, &tokens); err != nil {
		log.Panic("Failed to query for tokens: %v", err, c)
	}

	return tokens
}
