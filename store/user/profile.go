package user

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
	"crowdstart.io/util/queries"
	"crowdstart.io/util/template"
	"crowdstart.io/util/val"

	mandrill "crowdstart.io/thirdparty/mandrill/tasks"
	salesforce "crowdstart.io/thirdparty/salesforce/tasks"
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

	val.SanitizeUser(&form.User)
	if errs := form.Validate(); len(errs) > 0 {
		log.Debug("Billing info is incorrect. %v", errs)
		c.JSON(400, gin.H{"message": errs})
		return false
	}

	// Update information from form.
	user.Phone = form.User.Phone
	user.FirstName = form.User.FirstName
	user.LastName = form.User.LastName

	return true
}

func updateBilling(c *gin.Context, user *models.User) bool {
	form := new(BillingForm)
	if err := form.Parse(c); err != nil {
		log.Panic("Failed to save user billing information: %v", err)
	}

	if errs := form.Validate(); len(errs) > 0 {
		log.Debug("Billing info is incorrect. %v", errs)
		c.JSON(400, gin.H{"message": errs})
		return false
	}

	user.BillingAddress = form.BillingAddress
	return true
}

func updateMetadata(c *gin.Context, user *models.User) bool {
	form := new(MetadataForm)
	if err := form.Parse(c); err != nil {
		log.Panic("Failed to save user metadata information: %v", err)
	}

	if errs := form.Validate(); len(errs) > 0 {
		log.Debug("Metadata info is incorrect. %v", errs)
		c.JSON(400, gin.H{"message": errs})
		return false
	}

	user.Metadata = form.Metadata
	return true
}

func updatePassword(c *gin.Context, user *models.User) bool {
	form := new(ChangePasswordForm)
	if err := form.Parse(c); err != nil {
		log.Panic("Failed to update user password: %v", err)
	}

	if err := auth.CompareHashAndPassword(user.PasswordHash, form.OldPassword); err != nil {
		log.Debug("Old password is incorrect.")
		c.JSON(400, gin.H{"message": "Old password is incorrect."})
		return false
	}

	if form.Password == form.ConfirmPassword {
		if errs := form.Validate(); len(errs) > 0 {
			log.Debug("Password is incorrect. %v", errs)
			c.JSON(400, gin.H{"message": errs})
			return false
		}

		user.PasswordHash = auth.HashPassword(form.Password)
	} else {
		log.Debug("Passwords do not match.")
		c.JSON(400, gin.H{"message": "Passwords do not match."})
		return false
	}

	return true
}

func SaveProfile(c *gin.Context) {
	// Get user from datastore using session
	db := datastore.New(c)
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
		} else {
			// Look up campaign to see if we need to sync with salesforce
			campaign := models.Campaign{}
			if err := db.GetKind("campaign", "dev@hanzo.ai", &campaign); err != nil {
				log.Error(err, c)
			}

			log.Debug("Synchronize with salesforce if '%v' != ''", campaign.Salesforce.AccessToken)
			if campaign.Salesforce.AccessToken != "" {
				salesforce.CallUpsertUserTask(db.Context, &campaign, user)
			}
		}
	case "change-billing":
		if valid := updateBilling(c, user); !valid {
			return
		}
	case "change-password":
		if valid := updatePassword(c, user); !valid {
			return
		}
	case "change-info":
		if valid := updateMetadata(c, user); !valid {
			return
		}
	}

	// Update user
	q := queries.New(c)
	if err = q.UpsertUser(user); err != nil {
		log.Panic("Failed to save user: %v", err)
	}

	// Send email notifying of changes
	ctx := middleware.GetAppEngine(c)
	mandrill.SendTransactional.Call(ctx, "email/account-change.html", user.Email, user.Name(), "SKULLY account changed")

	c.JSON(200, gin.H{"success": true})
}
