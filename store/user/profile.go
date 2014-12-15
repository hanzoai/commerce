package user

import (
	"strings"

	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models"
	"crowdstart.io/thirdparty/mandrill"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
	"crowdstart.io/util/template"
	"crowdstart.io/util/val"
)

func Profile(c *gin.Context) {
	user, err := auth.GetUser(c)
	if err != nil {
		log.Panic("GetUser Error: %v", err)
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

	fUser := form.User
	if !val.Check(fUser.FirstName).Exists().IsValid {
		log.Debug("Form posted without first name")
		template.Render(c, "profile.html", "changeContactError", "Please enter a first name.")
		return
	}

	if !val.Check(fUser.LastName).Exists().IsValid {
		log.Debug("Form posted without last name")
		template.Render(c, "profile.html", "changeContactError", "Please enter a last name.")
		return
	}

	if !val.Check(fUser.Phone).Exists().IsValid {
		log.Debug("Form posted without phone number")
		template.Render(c, "profile.html", "changeContactError", "Please enter a phone number.")
		return
	}

	// Update information from form.
	user.Phone = form.User.Phone
	user.FirstName = strings.Title(form.User.FirstName)
	user.LastName = strings.Title(form.User.LastName)
}

func updateBilling(c *gin.Context, user *models.User) {
	form := new(BillingForm)
	if err := form.Parse(c); err != nil {
		log.Panic("Failed to save user billing information: %v", err)
	}

	billingAddress := form.BillingAddress
	if !val.Check(billingAddress.Line1).Exists().IsValid {
		log.Debug("Form posted without address")
		template.Render(c, "profile.html", "changeContactError", "Please enter an address.")
		return
	}

	if !val.Check(billingAddress.City).Exists().IsValid {
		log.Debug("Form posted without city")
		template.Render(c, "profile.html", "changeContactError", "Please enter a city.")
		return
	}

	if !val.Check(billingAddress.State).Exists().IsValid {
		log.Debug("Form posted without state")
		template.Render(c, "profile.html", "changeContactError", "Please enter a state.")
		return
	}

	if !val.Check(billingAddress.PostalCode).Exists().IsValid {
		log.Debug("Form posted without postal code")
		template.Render(c, "profile.html", "changeContactError", "Please enter a zip/postal code.")
		return
	}

	if !val.Check(billingAddress.Country).Exists().IsValid {
		log.Debug("Form posted without country")
		template.Render(c, "profile.html", "changeContactError", "Please enter a country.")
		return
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
		if !val.Check(form.Password).IsPassword().IsValid {
			log.Debug("Form posted invalid password")
			template.Render(c, "profile.html", "registerError", "Password Must be atleast 6 characters long.")
			return
		}

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
