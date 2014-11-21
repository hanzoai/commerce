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
		log.Panic("Failed to fetch user: %v", err)
	}

	// Find all of a user's contributions
	contributions := new([]models.Contribution)
	if _, err := db.Query("contribution").Filter("Email =", user.Email).GetAll(db.Context, contributions); err != nil {
		log.Panic("Failed to find contributions: %v", err)
	}

	userJSON := json.Encode(user)
	contributionsJSON := json.Encode(contributions)

	log.Debug("%#v", user)
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

	// Save user back to database
	db.PutKey("user", user.Email, user)

	c.Redirect(301, "save")
}

func Thanks(c *gin.Context) {
	template.Render(c, "thanks.html")
}

func Index(c *gin.Context) {
	template.Render(c, "login.html")
}

func Login(c *gin.Context) {
	err := auth.VerifyUser(c)
	if err != nil {
		c.Fail(500, err)
		return
	}

	user, err := auth.GetUser(c)
	if err != nil {
		c.Fail(500, err)
		return
	}

	db := datastore.New(c)
	token := new(models.InviteToken)
	err = db.GetKey("invite-token", user.Email, token)
	if err != nil {
		c.Fail(500, err)
		return
	}

	if err != nil {
		c.Redirect(301, "order/"+token.Id)
	} else {
		c.Redirect(301, "/")
	}
}
