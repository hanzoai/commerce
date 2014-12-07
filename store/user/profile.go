package user

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/form"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
	"crowdstart.io/util/template"
)

func Profile(c *gin.Context) {
	user := auth.GetUser(c)
	userJson := json.Encode(user)
	template.Render(c, "profile.html", "user", user, "userJson", userJson)
}

func SaveProfile(c *gin.Context) {
	modifiedUser := new(models.User)
	err := form.Parse(c, modifiedUser)
	if err != nil {
		log.Panic("Error parsing user \n%v", err)
	}

	user := auth.GetUser(c)
	log.Debug("Email: %#v", user)
	if err != nil {
		log.Panic("Error getting logged in user from the datastore \n%v", err)
	}

	user.Phone = modifiedUser.Phone
	user.BillingAddress.Line1 = modifiedUser.BillingAddress.Line1
	user.BillingAddress.Line2 = modifiedUser.BillingAddress.Line2
	user.FirstName = modifiedUser.FirstName
	user.LastName = modifiedUser.LastName

	db := datastore.New(c)
	_, err = db.PutKey("user", user.Email, user)
	if err != nil {
		log.Panic("Error saving the user \n%v", err)
	}

	userJson := json.Encode(user)
	template.Render(c, "profile.html", "user", user, "userJson", userJson)
}
