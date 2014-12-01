package preorder

import (
	"errors"

	"appengine/delay"

	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models"
	mail "crowdstart.io/thirdparty/mandrill"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
	"crowdstart.io/util/template"
)

// GET /:token
func GetPreorder(c *gin.Context) {
	db := datastore.New(c)

	// Fetch token
	token := new(models.InviteToken)
	db.GetKey("invite-token", c.Params.ByName("token"), token)

	// Redirect to login if token is expired or used
	if token.Expired || token.Used {
		c.Redirect(301, "/")
		return
	}

	// Should use token to lookup email
	user := new(models.User)
	if err := db.GetKey("user", token.Email, user); err != nil {
		log.Error("Failed to fetch user: %v", err, c)
		// Bad token
		c.Redirect(301, "../")
		return
	}

	// If user has password, they've previously edited the preorder
	order := new(models.Order)
	if user.HasPassword() {
		if err := db.GetKey("order", user.Email, order); err != nil {
			log.Error("Failed to fetch order for user: %v", err, c)
			c.Redirect(301, "../")
		}
	}
	orderJSON := json.Encode(order)

	// Find all of a user's contributions
	var contributions []models.Contribution
	if _, err := db.Query("contribution").Filter("Email =", user.Email).GetAll(db.Context, &contributions); err != nil {
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
		"userJSON", userJSON,
	)
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

var sendConfirmation = delay.Func("sendConfirmation", mail.SendTemplate)

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
	order.Total = order.Subtotal + order.Tax

	// Save order
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

	// ctx appengine.Context, from_name, from_email, to_name, to_email, subject string
	ctx := middleware.GetAppEngine(c)
	// sendConfirmation.Call(ctx, ctx,
	// 	"SKULLY",
	// 	"noreply@skullysystems.com",
	// 	user.Name(),
	// 	user.Email,
	// 	"Thank you for updating your preorder information",
	// 	confirmationHtml,
	// )

	req := mail.NewSendTemplateReq()
	req.AddRecipient(user.Email, user.Name())

	req.Message.Subject = "Preorder information changed"
	req.Message.FromEmail = "noreply@skullysystems.com"
	req.Message.FromName = "SKULLY"
	req.TemplateName = "preorder-confirmation-template"

	sendConfirmation.Call(ctx, ctx, &req)

	c.Redirect(301, "../thanks")
}

func Thanks(c *gin.Context) {
	template.Render(c, "thanks.html")
}

func Index(c *gin.Context) {
	template.Render(c, "login.html")
	return

	if !auth.IsLoggedIn(c) {
		template.Render(c, "login.html")
	} else {
		user := auth.GetUser(c)
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
	// Complain if user doesn't have any tokens
	if len(tokens) > 0 {
		// Redirect to order page as they have a valid token
		c.Redirect(301, "order/"+tokens[0].Id)
	} else {
		template.Render(c, "login.html", "message", "No pre-orders found for your account")
	}
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
