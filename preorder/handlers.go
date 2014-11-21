package preorder

import (
	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
	"crowdstart.io/util/template"
	"github.com/gin-gonic/gin"
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
		c.Redirect(301, "../../")
	}

	// Find all of a user's contributions
	contributions := new([]models.Contribution)
	if _, err := db.Query("contribution").Filter("Email =", user.Email).GetAll(db.Context, contributions); err != nil {
		log.Panic("Failed to find contributions: %v", err, c)
	}

	userJSON := json.Encode(user)
	contributionsJSON := json.Encode(contributions)

	template.Render(c, "preorder.html", "user", user, "userJSON", userJSON, "contributionsJSON", contributionsJSON)
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

	// Update user from form
	user.PasswordHash = form.User.PasswordHash
	user.Phone = form.User.Phone
	user.FirstName = form.User.FirstName
	user.LastName = form.User.LastName
	user.ShippingAddress = form.ShippingAddress

	// Save user back to database
	db.PutKey("user", user.Email, user)

	c.Redirect(301, "../thanks")
}

func Thanks(c *gin.Context) {
	template.Render(c, "thanks.html")
}

func Index(c *gin.Context) {
	template.Render(c, "login.html")
}

func Login(c *gin.Context) {
	// Parse login form
	f := new(models.LoginForm)
	if err := f.Parse(c); err != nil {
		template.Render(c, "login.html", "message", "Please ensure you have correctly entered your username and password")
		return
	}

	// Verify password
	err := auth.VerifyUser(c)
	if err != nil {
		template.Render(c, "login.html", "message", "Please ensure you have correctly entered your username and password")
		return
	}

	db := datastore.New(c)

	// Look up tokens for this user
	log.Debug("Searching for valid token for: %v", f.Email, c)

	tokens := make([]models.InviteToken, 0)
	if _, err = db.Query("invite-token").Filter("Email =", f.Email).GetAll(db.Context, &tokens); err != nil {
		log.Panic("Failed to query for tokens: %v", err, c)
		return
	}

	// Complain if user doesn't have any tokens
	if len(tokens) > 0 {
		// Redirect to order page as they have a valid token
		c.Redirect(301, "order/"+tokens[0].Id)
	} else {
		template.Render(c, "login.html", "message", "No pre-orders found for your account")
	}
}
