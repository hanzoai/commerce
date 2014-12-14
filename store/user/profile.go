package user

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models"
	"crowdstart.io/thirdparty/mandrill"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
	"crowdstart.io/util/template"
)

func Profile(c *gin.Context) {
	user, err := auth.GetUser(c)
	if err != nil {
		log.Panic("Error retrieving user \n%v", err)
	}

	userJson := json.Encode(user)

	log.Debug("Loading Profile %v", user)
	template.Render(c, "profile.html", "user", user, "userJson", userJson)
}

func updateContact(c *gin.Context, user *models.User) {
	form := new(ContactForm)
	if err := form.Parse(c); err != nil {
		log.Panic("Failed to save user profile: %v", err)
	}

	// Update information from form.
	user.Phone = form.User.Phone
	user.FirstName = form.User.FirstName
	user.LastName = form.User.LastName
}

func updateBilling(c *gin.Context, user *models.User) {
	form := new(BillingForm)
	if err := form.Parse(c); err != nil {
		log.Panic("Failed to save user billing information: %v", err)
	}

	user.BillingAddress = form.BillingAddress
}

func updatePassword(c *gin.Context, user *models.User) {
	form := new(ChangePasswordForm)
	if err := form.Parse(c); err != nil {
		log.Panic("Failed to update user password: %v", err)
	}

	if err := auth.CompareHashAndPassword(user.PasswordHash, form.OldPassword); err != nil {
		log.Panic("Password is incorrect: %v", err)
	}

	if form.Password == form.ConfirmPassword {
		user.PasswordHash = auth.HashPassword(form.Password)
	} else {
		log.Panic("Passwords do not match.")
	}
}

func SaveProfile(c *gin.Context) {
	// Get user from datastore using session
	user, err := auth.GetUser(c)
	if err != nil {
		log.Panic("Failed to retrieve user from datastore using session: %v", err)
	}

	// Parse proper form
	formName := c.Params.ByName("form")
	switch formName {
	case "change-contact":
		updateContact(c, user)
	case "change-billing":
		updateBilling(c, user)
	case "change-password":
		updatePassword(c, user)
	}

	// Update user
	db := datastore.New(c)
	if _, err = db.PutKey("user", user.Email, user); err != nil {
		log.Panic("Failed to save user: %v", err)
	}

	// Send email notifying of changes
	ctx := middleware.GetAppEngine(c)
	mandrill.SendTemplateAsync.Call(ctx, "account-change-confirmation", user.Email, user.Name(), "Your account information has been changed.")

	template.Render(c, "profile.html", "user", user, "success", true)
}
