package preorder

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
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

	// Redirect to login if token is expired or userd
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

	// Find all of a user's contributions
	contributions := new([]models.Contribution)
	if _, err := db.Query("contribution").Filter("Email =", user.Email).GetAll(db.Context, contributions); err != nil {
		log.Panic("Failed to find contributions: %v", err, c)
	}

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
	allProductsJSON := json.Encode(productsMap)

	template.Render(c, "preorder.html",
		"user", user,
		"tokenId", token.Id,
		"userJSON", userJSON,
		"contributionsJSON", contributionsJSON,
		"allProductsJSON", allProductsJSON,
	)
}

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
		return
	} else if tokens[0].Id != form.Token.Id {
		return
	}

	// Update user's password if this is the first time saving.
	if !user.HasPassword() {
		user.PasswordHash = form.User.PasswordHash
	}
	user.Phone = form.User.Phone
	user.FirstName = form.User.FirstName
	user.LastName = form.User.LastName
	user.ShippingAddress = form.ShippingAddress

	order := form.Order

	for i, lineItem := range order.Items {
		if i == 0 {
			continue
		}

		// Fetch Variant for LineItem from datastore
		if err := db.GetKey("variant", lineItem.SKU(), &lineItem.Variant); err != nil {
			c.Fail(500, err)
			log.Error("Getting variant failed", err)
			log.Info("SKU", lineItem.SKU())
			return
		}

		// Fetch Product for LineItem from datastore
		if err := db.GetKey("product", lineItem.Slug(), &lineItem.Product); err != nil {
			c.Fail(500, err)
			log.Error("Getting product failed", err)
			log.Info("Slug", lineItem.Slug())
			return
		}

		// Update item in order
		order.Items[i] = lineItem

		// Update subtotal
		order.Subtotal += lineItem.Price()
	}

	// Update Total
	order.Total = order.Subtotal + order.Tax

	// Save order
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

	c.Redirect(301, "../thanks")
}

func Thanks(c *gin.Context) {
	template.Render(c, "thanks.html")
}

func Index(c *gin.Context) {
	if !auth.IsLoggedIn(c) {
		template.Render(c, "login.html")
	} else {
		user, err := auth.GetUser(c)
		if err != nil {
			log.Error("Error getting user", err)
			template.Render(c, "login.html", "message", "A server side error occurred")
			return
		}
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
	f := new(models.LoginForm)
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
