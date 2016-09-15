package account

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.com/auth/password"
	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/referrer"
	"crowdstart.com/models/user"
	"crowdstart.com/thirdparty/mailchimp"
	"crowdstart.com/util/counter"
	"crowdstart.com/util/emails"
	"crowdstart.com/util/json"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/log"
)

var emailRegex = regexp.MustCompile("(\\w[-._\\w]*\\w@\\w[-._\\w]*\\w\\.\\w{2,4})")

type createReq struct {
	*user.User
	Password        string `json:"password"`
	PasswordConfirm string `json:"passwordConfirm"`
}

func create(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	req := &createReq{}
	req.User = user.New(db)

	// Default these fields to exotic unicode character to test if they are set to empty
	req.Email = "\u263A"
	req.FirstName = "\u263A"
	req.LastName = "\u263A"

	// Decode response body to create new user
	if err := json.Decode(c.Request.Body, req); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	// Pull out user
	usr := req.User

	// Email is required
	if usr.Email == "" || usr.Email == "\u263A" {
		http.Fail(c, 400, "Email is required", errors.New("Email is required"))
		return
	}

	if usr.FirstName == "" || usr.FirstName == "\u263A" {
		http.Fail(c, 400, "First name cannot be blank", errors.New("First name cannot be blank"))
		return
	}

	if usr.LastName == "" || usr.LastName == "\u263A" {
		http.Fail(c, 400, "Last name cannot be blank", errors.New("Last name cannot be blank"))
		return
	}

	usr.Email = strings.ToLower(strings.TrimSpace(usr.Email))

	// Email can't already exist
	if err := usr.GetByEmail(usr.Email); err == nil {
		http.Fail(c, 400, "Email is in use", errors.New("Email is in use"))
		return
	}

	// Email must be valid
	if ok := emailRegex.MatchString(usr.Email); !ok {
		http.Fail(c, 400, "Email is not valid", errors.New("Email is not valid"))
		return
	}

	// Password should be at least 6 characters long
	if len(req.Password) < 6 {
		http.Fail(c, 400, "Password needs to be atleast 6 characters", errors.New("Password needs to be atleast 6 characters"))
		return
	}

	// Password confirm must match
	if req.Password != req.PasswordConfirm {
		http.Fail(c, 400, "Passwords need to match", errors.New("Passwords need to match"))
		return
	}

	// Hash password
	if hash, err := password.Hash(req.Password); err != nil {
		http.Fail(c, 400, "Failed to hash user password", err)
	} else {
		usr.PasswordHash = hash
	}

	ctx := org.Db.Context
	if err := counter.IncrUsers(ctx, org, time.Now()); err != nil {
		log.Warn("Redis Error %s", err, ctx)
	}

	// Test key users are automatically confirmed
	if !org.Live {
		usr.Enabled = true
	}

	usr.Enabled = org.SignUpOptions.AccountsEnabledByDefault

	// Save new user
	if err := usr.Put(); err != nil {
		http.Fail(c, 400, "Failed to create user", err)
	}

	ref := referrer.New(usr.Db)

	// if ReferrerId refers to non-existing token, then remove from order
	if err := ref.GetById(usr.ReferrerId); err != nil {
		usr.ReferrerId = ""
	} else {
		// Try to save referral, save updated referrer
		if _, err := ref.SaveSignUpReferral(usr.Id(), usr.FirstName, usr.ReferrerId, usr.Db); err != nil {
			log.Warn("Unable to save referral: %v", err, c)
		}
	}

	// Render user
	http.Render(c, 201, usr)

	// Don't send email confirmation if test key is used
	if org.Live {
		// Send welcome, email confirmation emails
		ctx := middleware.GetAppEngine(c)
		emails.SendAccountCreationConfirmationEmail(ctx, org, usr)
		emails.SendWelcomeEmail(ctx, org, usr)
	}

	// Save user as customer in Mailchimp if configured
	if org.Mailchimp.APIKey != "" {
		// Create new mailchimp client
		client := mailchimp.New(ctx, org.Mailchimp.APIKey)

		// Create customer in mailchimp for this user
		if err := client.CreateCustomer(org.DefaultStore, usr); err != nil {
			log.Warn("Failed to create Mailchimp customer: %v", err, ctx)
		}
	}
}
