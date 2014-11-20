package preorder

import (
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
	"crowdstart.io/util/template"
	"github.com/gin-gonic/gin"
)

func Get(c *gin.Context) {
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

func Login(c *gin.Context) {
	template.Render(c, "login.html")
}
