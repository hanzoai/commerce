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

func updateContact(c *gin.Context, user *models.User) bool {
	form := new(ContactForm)
	if err := form.Parse(c); err != nil {
		log.Panic("Failed to save user profile: %v", err)
	}

	if valid := val.AjaxUser(c, &form.User); !valid {
		return false
	}

	// Update information from form.
	user.Phone = form.User.Phone
	user.FirstName = strings.Title(form.User.FirstName)
	user.LastName = strings.Title(form.User.LastName)

	return true
}

func updateBilling(c *gin.Context, user *models.User) bool {
	form := new(BillingForm)
	if err := form.Parse(c); err != nil {
		log.Panic("Failed to save user billing information: %v", err)
	}

	if valid := val.AjaxAddress(c, &form.BillingAddress); !valid {
		return false
	}

	user.BillingAddress = form.BillingAddress
	return true
}

func updatePassword(c *gin.Context, user *models.User) bool {
	form := new(ChangePasswordForm)
	if err := form.Parse(c); err != nil {
		log.Panic("Failed to update user password: %v", err)
	}

	if err := auth.CompareHashAndPassword(user.PasswordHash, form.OldPassword); err != nil {
		log.Panic("Password is incorrect: %v", err)
	}

	if form.Password == form.ConfirmPassword {
		if valid := val.AjaxPassword(c, &form.Password); !valid {
			return false
		}

		user.PasswordHash = auth.HashPassword(form.Password)
	} else {
		log.Panic("Passwords do not match.")
	}

	return true
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
		if valid := updateContact(c, user); !valid {
			return
		}
	case "change-billing":
		if valid := updateBilling(c, user); !valid {
			return
		}
	case "change-password":
		if valid := updatePassword(c, user); !valid {
			return
		}
	}

	// Update user
	db := datastore.New(c)
	if _, err = db.PutKey("user", user.Email, user); err != nil {
		log.Panic("Failed to save user: %v", err)
	}

	// Send email notifying of changes
	ctx := middleware.GetAppEngine(c)
	mandrill.SendTemplateAsync.Call(ctx, "account-change-confirmation", user.Email, user.Name(), "Your account information has been changed.")

	c.JSON(200, gin.H{"success": true})
}
